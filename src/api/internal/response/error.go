package response

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// BusinessError 仅承载业务主动返回的公开错误，不包含数据库或组件内部异常。
type BusinessError struct {
	Status  int
	Message string
}

// Error 返回固定的简体中文业务说明。
func (f *BusinessError) Error() string { return f.Message }

// RecordError 在转换公开响应前记录异常，由全局追踪处理器同步输出关联日志。
func RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	// 业务提示只用于公开响应，诊断信息使用英文且不包含用户提交的内容。
	var business *BusinessError
	var httpError *echo.HTTPError
	var numberError *strconv.NumError
	switch {
	case errors.As(err, &business):
		err = fmt.Errorf("business operation rejected with HTTP status %d", business.Status)
	case errors.As(err, &httpError):
		if httpError.Internal != nil {
			RecordError(ctx, httpError.Internal)
			return
		}
		err = fmt.Errorf("HTTP request rejected with status %d", httpError.Code)
	case errors.As(err, &numberError):
		// 数字转换异常可能包含原始请求值，只保留转换函数和失败原因。
		err = fmt.Errorf("%s failed: %w", numberError.Func, numberError.Err)
	}

	// HTTP、服务与组件共用真实 Span，关闭上报后日志仍保留异常详情。
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, "operation failed")
}
