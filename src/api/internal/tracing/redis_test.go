package tracing_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func TestRedisScriptFallbackPreservesTraceAndFinalErrors(t *testing.T) {
	scriptErr := errors.New("NOPERM script execution denied")
	for _, scenario := range []struct {
		name         string
		shaErr       error
		evalErr      error
		wantErr      error
		wantCommands []string
		wantLevels   []logrus.Level
	}{
		{
			name:         "cache-miss-recovered",
			shaErr:       redis.ErrNoScript,
			wantCommands: []string{"evalsha", "eval"},
			wantLevels:   []logrus.Level{logrus.DebugLevel},
		},
		{
			name:         "fallback-failed",
			shaErr:       redis.ErrNoScript,
			evalErr:      scriptErr,
			wantErr:      scriptErr,
			wantCommands: []string{"evalsha", "eval"},
			wantLevels:   []logrus.Level{logrus.DebugLevel, logrus.ErrorLevel},
		},
		{
			name:         "script-failed",
			shaErr:       scriptErr,
			wantErr:      scriptErr,
			wantCommands: []string{"evalsha"},
			wantLevels:   []logrus.Level{logrus.ErrorLevel},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// 使用真实客户端、脚本运行器和官方适配器，不连接外部 Redis。
			hook := exceptionLogs(t)
			logger.Logger.SetLevel(logrus.DebugLevel)
			recorder := tracetest.NewSpanRecorder()
			tracing.Provider.RegisterSpanProcessor(recorder)
			t.Cleanup(func() { tracing.Provider.UnregisterSpanProcessor(recorder) })
			client := redis.NewClient(&redis.Options{
				MaintNotificationsConfig: &maintnotifications.Config{
					Mode: maintnotifications.ModeDisabled,
				},
			})
			t.Cleanup(func() { _ = client.Close() })
			if err := redisotel.InstrumentTracing(
				client,
				redisotel.WithTracerProvider(tracing.Provider),
				redisotel.WithDBStatement(false),
			); err != nil {
				t.Fatal(err)
			}
			var commands []string
			client.AddHook(scriptResultHook{
				process: func(_ context.Context, cmd redis.Cmder) error {
					commands = append(commands, cmd.Name())
					err := scenario.shaErr
					if cmd.Name() == "eval" {
						err = scenario.evalErr
					}
					cmd.SetErr(err)
					return err
				},
			})
			err := redis.NewScript("return 1").Run(t.Context(), client, nil).Err()
			if !errors.Is(err, scenario.wantErr) || !slices.Equal(commands, scenario.wantCommands) {
				t.Fatalf("脚本回退结果不正确：%v，%v", commands, err)
			}

			// 缓存未命中仍保留异常跨度，后续失败必须独立产生错误日志。
			spans := recorder.Ended()
			entries := hook.AllEntries()
			if len(spans) != len(commands) || len(entries) != len(scenario.wantLevels) {
				t.Fatalf("脚本跨度或日志数量不正确：%d，%d", len(spans), len(entries))
			}
			for index, level := range scenario.wantLevels {
				if entries[index].Level != level || spans[index].Status().Code != codes.Error ||
					len(spans[index].Events()) != 1 ||
					entries[index].Data["trace"] != spans[index].SpanContext().TraceID().String() {
					t.Fatalf("脚本异常的日志级别或追踪信息不正确：%v", entries[index])
				}
			}
		})
	}
}

func TestOtherScriptErrorsKeepErrorLevel(t *testing.T) {
	for _, scenario := range []struct{ scope, operation string }{
		{"github.com/redis/go-redis/extra/redisotel", "eval"},
		{"github.com/redis/go-redis/extra/redisotel", "redis.pipeline evalsha"},
		{"test.component", "evalsha"},
	} {
		t.Run(scenario.scope+"/"+scenario.operation, func(t *testing.T) {
			// 同名异常出现在其他组件或无法回退的命令中时，不降低错误级别。
			hook := exceptionLogs(t)
			_, span := tracing.Provider.Tracer(scenario.scope).Start(
				t.Context(),
				scenario.operation,
				trace.WithSpanKind(trace.SpanKindClient),
			)
			span.RecordError(redis.ErrNoScript)
			span.SetStatus(codes.Error, redis.ErrNoScript.Error())
			span.End()
			entries := hook.AllEntries()
			if len(entries) != 1 || entries[0].Level != logrus.ErrorLevel {
				t.Fatalf("其他脚本错误被降级或重复记录：%v", entries)
			}
		})
	}
}

type scriptResultHook struct {
	process redis.ProcessHook
}

func (hook scriptResultHook) DialHook(next redis.DialHook) redis.DialHook {
	// 命令结果由测试钩子提供，不修改连接行为。
	return next
}

func (hook scriptResultHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	// 直接提供命令结果，避免脚本回退测试访问外部服务。
	return hook.process
}

func (hook scriptResultHook) ProcessPipelineHook(
	next redis.ProcessPipelineHook,
) redis.ProcessPipelineHook {
	// 测试仅覆盖独立脚本调用，流水线保持原有行为。
	return next
}
