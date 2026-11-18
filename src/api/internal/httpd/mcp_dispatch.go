package httpd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func mcpQueryHandler(engine *echo.Echo, tool mcpQueryTool) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 沿用外层认证身份与追踪，只派生原接口的审计范围，不发起 HTTP 回环请求。
		ctx, span := tracing.Tracer.Start(ctx, "mcp."+tool.Tool.Name)
		defer span.End()
		ctx = auditUtils.WithOperation(ctx, http.MethodGet, tool.Route)
		target, err := mcpQueryURL(tool, request.Params.Arguments)
		if err != nil {
			return mcpQueryFailure(ctx, tool, http.StatusBadRequest, err.Error()), nil
		}
		parent, ok := ctx.Value(mcpRequestKey{}).(*http.Request)
		if !ok {
			return mcpQueryFailure(ctx, tool, http.StatusServiceUnavailable,
				"MCP 请求上下文不可用"), nil
		}
		// 旧协议允许传输层分离取消信号，业务查询仍必须跟随原 HTTP 请求取消。
		ctx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(parent.Context(), cancel)
		defer cancel()
		defer stop()
		if parent.Context().Err() != nil {
			cancel()
		}
		innerRequest := parent.Clone(ctx)
		innerRequest.Method = http.MethodGet
		innerRequest.URL = target
		innerRequest.RequestURI = target.RequestURI()
		innerRequest.Body = http.NoBody
		innerRequest.GetBody = nil
		innerRequest.ContentLength = 0
		writer := newMCPResponseWriter()
		inner := engine.NewContext(innerRequest, writer)
		engine.Router().Find(http.MethodGet, target.Path, inner)
		if inner.Path() != tool.Route {
			return mcpQueryFailure(ctx, tool, http.StatusNotFound, "查询接口不存在"), nil
		}

		// 再次执行目标 action 的权限检查，原路由继续负责参数、事务及读取审计。
		handler := requestLogger()(recoverRequests()(Authorize(inner.Handler())))
		if err := handler(inner); err != nil &&
			!inner.Response().Committed {
			inner.Error(err)
		}
		if writer.overflow {
			return mcpQueryFailure(ctx, tool, http.StatusRequestEntityTooLarge,
				"查询结果过大，请缩小查询范围"), nil
		}
		body := writer.body.Bytes()
		var envelope map[string]json.RawMessage
		if json.Unmarshal(body, &envelope) != nil || len(envelope) != 3 ||
			envelope["trace"] == nil || envelope["error_msg"] == nil || envelope["data"] == nil {
			response.RecordError(ctx, errors.New("MCP query returned an invalid API envelope"))
			return mcpQueryFailure(ctx, tool, http.StatusServiceUnavailable, "查询响应格式无效"), nil
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(body)}},
			StructuredContent: json.RawMessage(body),
			IsError:           writer.status >= http.StatusBadRequest,
			Meta:              mcp.Meta{"http_status": writer.status},
		}, nil
	}
}

func mcpQueryFailure(
	ctx context.Context,
	tool mcpQueryTool,
	status int,
	message string,
) *mcp.CallToolResult {
	// 转换阶段的无效参数也按原接口 action 审计，审计失败时阻断数据。
	err := accessUtils.Reject(ctx, tool.Action,
		auditModels.Event{ObjectType: tool.ObjectType},
		&response.BusinessError{Status: status, Message: message},
	)
	var business *response.BusinessError
	if errors.As(err, &business) {
		status, message = business.Status, business.Message
	}
	return mcpResultFailure(ctx, status, message)
}

func mcpResultFailure(ctx context.Context, status int, message string) *mcp.CallToolResult {
	result := response.Envelope[any]{Trace: tracing.TraceID(ctx), ErrorMsg: message, Data: nil}
	text, _ := json.Marshal(result)
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(text)}},
		StructuredContent: result, IsError: true,
		Meta: mcp.Meta{"http_status": status},
	}
}

func newMCPResponseWriter() *mcpResponseWriter {
	return &mcpResponseWriter{
		header: make(http.Header), status: http.StatusOK,
		limit: config.Config.MCP.MaxResponseBytes,
	}
}

func (writer *mcpResponseWriter) Header() http.Header { return writer.header }

func (writer *mcpResponseWriter) WriteHeader(status int) {
	if !writer.committed {
		writer.status = status
		writer.committed = true
	}
}

func (writer *mcpResponseWriter) Write(value []byte) (int, error) {
	if !writer.committed {
		writer.WriteHeader(http.StatusOK)
	}
	// 超限时继续消费但不保留内容，避免返回部分业务数据或无限增长内存。
	if writer.overflow || int64(writer.body.Len())+int64(len(value)) > writer.limit {
		writer.overflow = true
		writer.body.Reset()
		return len(value), nil
	}
	return writer.body.Write(value)
}

func (writer *mcpResponseWriter) Flush() {}

var _ http.ResponseWriter = (*mcpResponseWriter)(nil)
