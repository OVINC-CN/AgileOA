package tracing_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

func exceptionLogs(t *testing.T) *test.Hook {
	t.Helper()

	// 保留 Trace ID 钩子，测试结束后恢复全局日志配置。
	original := make(logrus.LevelHooks, len(logger.Logger.Hooks))
	for level, hooks := range logger.Logger.Hooks {
		original[level] = append([]logrus.Hook(nil), hooks...)
	}
	level := logger.Logger.GetLevel()
	logger.Logger.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() {
		logger.Logger.ReplaceHooks(original)
		logger.Logger.SetLevel(level)
	})
	return test.NewLocal(logger.Logger)
}

func TestSpanExceptionsAreLoggedWithoutExport(t *testing.T) {
	hook := exceptionLogs(t)
	_, span := tracing.Tracer.Start(
		t.Context(),
		"test.component.failure",
		trace.WithAttributes(attribute.String("db.query.text", "private query parameters")),
	)
	span.RecordError(errors.New("database connection failed"))
	span.AddEvent(
		"component metadata",
		trace.WithAttributes(attribute.String("private", "private event parameters")),
	)
	span.RecordError(fmt.Errorf("transaction failed: %w", errors.New("commit failed")))
	digest := strings.Repeat("a", 64)
	span.RecordError(fmt.Errorf("Duplicate entry '%s' for key 'token_hash'", digest))
	span.End()

	// 官方组件共用 Provider，每条异常都应有详情和真实 Trace ID。
	entries := hook.AllEntries()
	if len(entries) != 3 {
		t.Fatalf("异常日志数量不正确：%d", len(entries))
	}
	for index, message := range []string{
		"database connection failed",
		"commit failed",
		"Duplicate entry '[redacted]' for key 'token_hash'",
	} {
		entry := entries[index]
		if entry.Level != logrus.ErrorLevel ||
			entry.Data["trace"] != span.SpanContext().TraceID().String() ||
			!strings.Contains(entry.Message, "operation: test.component.failure") ||
			!strings.Contains(entry.Message, message) ||
			strings.Contains(entry.Message, "private") || strings.Contains(entry.Message, digest) {
			t.Fatalf("异常详情、Trace ID 或日志范围不正确：%v", entry)
		}
	}
}

func TestSpanErrorStatusIsLoggedWithoutException(t *testing.T) {
	hook := exceptionLogs(t)
	_, span := tracing.Tracer.Start(t.Context(), "test.component.status")
	span.SetStatus(codes.Error, "component request failed")
	span.End()
	entries := hook.AllEntries()
	if len(entries) != 1 ||
		entries[0].Data["trace"] != span.SpanContext().TraceID().String() ||
		!strings.Contains(entries[0].Message, "component request failed") {
		t.Fatalf("没有异常事件的失败未记录关联日志：%v", entries)
	}
}

func TestSuccessfulSpansDoNotLogErrors(t *testing.T) {
	hook := exceptionLogs(t)
	_, span := tracing.Tracer.Start(t.Context(), "test.component.success")
	span.AddEvent("component completed")
	span.End()
	if len(hook.AllEntries()) != 0 {
		t.Fatal("成功请求产生了异常日志")
	}
}
