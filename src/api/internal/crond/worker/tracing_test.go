package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

func recordTasks(t *testing.T) (*tracetest.SpanRecorder, *test.Hook) {
	t.Helper()

	// 复用生产 Provider 和日志钩子，测试完成后恢复全局资源。
	recorder := tracetest.NewSpanRecorder()
	tracing.Provider.RegisterSpanProcessor(recorder)
	original := make(logrus.LevelHooks, len(logger.Logger.Hooks))
	for level, hooks := range logger.Logger.Hooks {
		original[level] = append([]logrus.Hook(nil), hooks...)
	}
	originalLevel := logger.Logger.GetLevel()
	logger.Logger.SetLevel(logrus.InfoLevel)
	t.Cleanup(func() {
		tracing.Provider.UnregisterSpanProcessor(recorder)
		logger.Logger.ReplaceHooks(original)
		logger.Logger.SetLevel(originalLevel)
	})
	return recorder, test.NewLocal(logger.Logger)
}

func TestTaskTraceRestoresParentAndCorrelatesLogs(t *testing.T) {
	recorder, hook := recordTasks(t)
	parentCtx, parent := tracing.Tracer.Start(t.Context(), "test.enqueue")
	defer parent.End()
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(parentCtx, carrier)
	content, err := json.Marshal(map[string]any{
		"trace_context": carrier,
		"private":       "private-task-content",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 通过真实路由中间件执行，业务和开始、结束日志应使用同一个消费者跨度。
	mux := asynq.NewServeMux()
	mux.Use(traceTask, logTask)
	var taskContext trace.SpanContext
	mux.HandleFunc("test:task", func(ctx context.Context, _ *asynq.Task) error {
		taskContext = trace.SpanContextFromContext(ctx)
		return nil
	})
	if err := mux.ProcessTask(t.Context(), asynq.NewTask("test:task", content)); err != nil {
		t.Fatal(err)
	}
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("任务未生成唯一执行跨度：%d", len(spans))
	}
	span := spans[0]
	if span.Name() != "test:task" || span.SpanKind() != trace.SpanKindConsumer ||
		!taskContext.IsValid() || !span.SpanContext().Equal(taskContext) ||
		taskContext.TraceID() != parent.SpanContext().TraceID() ||
		span.Parent().SpanID() != parent.SpanContext().SpanID() || !span.Parent().IsRemote() {
		t.Fatal("任务未恢复父追踪或业务未使用消费者跨度")
	}
	entries := hook.AllEntries()
	if len(entries) != 2 {
		t.Fatalf("任务开始和结束日志数量不正确：%d", len(entries))
	}
	for _, entry := range entries {
		if entry.Data["trace"] != taskContext.TraceID().String() ||
			entry.Data["span_id"] != taskContext.SpanID().String() {
			t.Fatal("任务日志未关联执行跨度")
		}
	}
	if strings.Contains(
		fmt.Sprint(span.Attributes(), span.Events(), entries),
		"private-task-content",
	) {
		t.Fatal("任务正文进入追踪或日志")
	}
}

func TestTaskTraceCoversFailuresAndLegacyPayloads(t *testing.T) {
	failure := errors.New("task processing failed")
	for _, scenario := range []struct {
		name, payload string
		result        error
		cancel        bool
	}{
		{name: "legacy-payload", payload: `{}`},
		{name: "invalid-json", payload: `{`, result: asynq.SkipRetry},
		{name: "invalid-carrier", payload: `{"trace_context":1}`, result: asynq.SkipRetry},
		{name: "business-failure", payload: `{}`, result: failure},
		{name: "cancelled", payload: `{}`, result: context.Canceled, cancel: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			recorder, _ := recordTasks(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if scenario.cancel {
				cancel()
			}
			handler := traceTask(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
				// 追踪不能阻止业务接收原载荷，也不能丢失队列的取消状态。
				if !trace.SpanContextFromContext(ctx).IsValid() ||
					(scenario.cancel && !errors.Is(ctx.Err(), context.Canceled)) {
					t.Fatal("任务追踪或取消状态无效")
				}
				return scenario.result
			}))
			err := handler.ProcessTask(ctx, asynq.NewTask("test:task", []byte(scenario.payload)))
			if !errors.Is(err, scenario.result) {
				t.Fatalf("任务原始错误被修改：%v", err)
			}
			spans := recorder.Ended()
			if len(spans) != 1 || !spans[0].SpanContext().IsValid() ||
				spans[0].Parent().IsValid() {
				t.Fatal("无有效父上下文的任务未生成独立追踪")
			}
			if scenario.result != nil && (spans[0].Status().Code != codes.Error ||
				len(spans[0].Events()) != 1) {
				t.Fatal("失败任务未记录异常和错误状态")
			}
		})
	}
}

func TestTaskTracePreservesPanicForQueueRecovery(t *testing.T) {
	recorder, _ := recordTasks(t)
	handler := traceTask(asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
		panic("private-panic-content")
	}))

	// 模拟队列在处理函数外恢复异常，原异常值和后续重试行为必须保留。
	func() {
		defer func() {
			if value := recover(); value != "private-panic-content" {
				t.Fatalf("任务异常未继续交给队列：%v", value)
			}
		}()
		_ = handler.ProcessTask(t.Context(), asynq.NewTask("test:panic", nil))
	}()
	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Status().Code != codes.Error || len(spans[0].Events()) != 1 {
		t.Fatal("异常退出的任务未完成错误跨度")
	}
	if strings.Contains(fmt.Sprint(spans[0].Events()), "private-panic-content") {
		t.Fatal("异常正文进入追踪")
	}
}
