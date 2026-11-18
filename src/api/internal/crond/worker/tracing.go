package worker

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// traceTask 为每次任务执行创建消费者跨度，覆盖校验、业务和任务日志。
func traceTask(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) (err error) {
		// 只提取标准追踪上下文，载荷校验仍由业务处理函数负责。
		var payload struct {
			TraceContext propagation.MapCarrier `json:"trace_context"`
		}
		if json.Unmarshal(task.Payload(), &payload) == nil {
			ctx = propagation.TraceContext{}.Extract(ctx, payload.TraceContext)
		}
		ctx, span := tracing.Tracer.Start(
			ctx,
			task.Type(),
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		// 仅记录队列元数据，不将任务正文或用户身份写入追踪。
		if taskID, ok := asynq.GetTaskID(ctx); ok {
			span.SetAttributes(attribute.String("messaging.message.id", taskID))
		}
		if queue, ok := asynq.GetQueueName(ctx); ok {
			span.SetAttributes(attribute.String("messaging.destination.name", queue))
		}
		if retryCount, ok := asynq.GetRetryCount(ctx); ok {
			span.SetAttributes(attribute.Int("asynq.retry_count", retryCount))
		}

		// 异常继续交给队列恢复并重试，追踪不保存可能包含敏感值的异常正文。
		defer func() {
			if value := recover(); value != nil {
				span.RecordError(errors.New("task execution panicked"))
				span.SetStatus(codes.Error, "Task execution panicked")
				panic(value)
			}
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "Task execution failed")
			}
		}()

		// 保留上下文取消、超时和原始错误，由队列决定后续重试。
		return next.ProcessTask(ctx, task)
	})
}
