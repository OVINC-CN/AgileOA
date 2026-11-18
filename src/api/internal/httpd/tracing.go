package httpd

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

func traceRequests(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 解析标准 traceparent，缺失或无效时由 SDK 创建新追踪。
		request := c.Request()
		parent := otel.GetTextMapPropagator().Extract(
			request.Context(),
			propagation.HeaderCarrier(request.Header),
		)
		ctx, span := tracing.Tracer.Start(
			parent,
			request.Method+"#"+c.Path(),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		c.SetRequest(request.WithContext(ctx))
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(c.Response().Header()))

		// 在追踪仍有效时写出失败响应，确保错误响应使用相同 Trace ID。
		if err := next(c); err != nil && !c.Response().Committed {
			c.Error(err)
		}

		// 记录最终状态，不提供跳过或关闭请求追踪的选项。
		status := c.Response().Status
		span.SetAttributes(
			attribute.String("http.request.method", request.Method),
			attribute.String("http.route", c.Path()),
			attribute.Int("http.response.status_code", status),
		)
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}
		return nil
	}
}
