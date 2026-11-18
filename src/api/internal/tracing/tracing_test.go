package tracing_test

import (
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

func TestTracingIsAlwaysRecording(t *testing.T) {
	t.Parallel()

	// 即使默认不对外上报，SDK 仍创建并记录真实追踪。
	ctx, span := tracing.Tracer.Start(t.Context(), "test.tracing")
	defer span.End()
	if !span.IsRecording() || !span.SpanContext().IsValid() ||
		tracing.TraceID(ctx) != span.SpanContext().TraceID().String() {
		t.Fatal("追踪被关闭或使用了无效标识")
	}
}
