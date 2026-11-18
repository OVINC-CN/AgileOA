package response

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// Send 只返回追踪标识、错误说明和业务数据三个字段。
func Send[T any](c echo.Context, status int, errorMessage string, data T) error {
	// 从请求上下文读取追踪，不生成其他请求编号或业务状态码。
	return c.JSON(status, Envelope[T]{
		Trace:    tracing.TraceID(c.Request().Context()),
		ErrorMsg: errorMessage,
		Data:     data,
	})
}

// Success 使用空错误说明返回业务数据。
func Success[T any](c echo.Context, data T) error {
	// HTTP 状态表达请求结果，响应中不再包含 code 字段。
	return Send(c, http.StatusOK, "", data)
}
