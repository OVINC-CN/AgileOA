package httpd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func errorHandler(err error, c echo.Context) {
	// 已提交响应后的异常也必须记录，不能因无法改写响应而丢失诊断信息。
	response.RecordError(c.Request().Context(), err)

	// 已经写出的响应不能重复输出。
	if c.Response().Committed {
		return
	}

	// HTTP 状态表达请求结果，错误说明保持简体中文。
	status := http.StatusInternalServerError
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) {
		status = httpError.Code
	}
	message := errorMessage(status)
	var failure *response.BusinessError
	if errors.As(err, &failure) {
		status = failure.Status
		message = failure.Message
	}
	if auditErr := auditUnhandledFailure(c, status, message); auditErr != nil {
		response.RecordError(c.Request().Context(), auditErr)
		status = http.StatusServiceUnavailable
		message = "暂时无法完成操作，请稍后重试"
	}

	// 不返回内部异常、业务状态码或其他请求编号。
	if c.Path() == mcpEndpointPath {
		if err := sendMCPError(c, status, jsonrpc.CodeInvalidRequest, message, nil); err != nil {
			response.RecordError(
				c.Request().Context(),
				fmt.Errorf("MCP error response write failed: %w", err),
			)
		}
		return
	}
	if err := response.Send[*response.ErrorData](c, status, message, nil); err != nil {
		response.RecordError(c.Request().Context(), fmt.Errorf("response write failed: %w", err))
	}
}

func errorMessage(status int) string {
	// 根据状态选择固定的简体中文文案。
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return response.MessageBadRequest
	case http.StatusUnauthorized:
		return response.MessageUnauthorized
	case http.StatusForbidden:
		return response.MessageForbidden
	case http.StatusNotFound:
		return response.MessageNotFound
	case http.StatusMethodNotAllowed:
		return response.MessageMethodNotAllowed
	case http.StatusRequestEntityTooLarge:
		return response.MessageRequestTooLarge
	case http.StatusTooManyRequests:
		return response.MessageRateLimited
	case http.StatusServiceUnavailable:
		return response.MessageNotReady
	case http.StatusGatewayTimeout:
		return response.MessageTimeout
	default:
		return response.MessageInternalError
	}
}
