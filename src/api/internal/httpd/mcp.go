package httpd

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/bytes"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

const mcpEndpointPath = "/v1/mcp"

// MCP 注册标准传输入口，实际转换由公共中间件完成。
// @Summary 工作空间只读 MCP
// @Description 使用 Streamable HTTP 无状态 JSON 模式。POST 接收 initialize、tools/list 和 tools/call；通知返回 202。GET 与 DELETE 返回 405。查询工具按名称与 GET 路径显式登记，参数和响应模型复用现有接口 Swagger，未登记接口默认不开放。调用复用认证身份、权限、校验及审计，不提供写入、导出、模板或下载工具。
// @Tags MCP
// @Accept json
// @Produce json
// @Param Authorization header string true "仅支持 API Token，格式为 Bearer <token>，不接受 Cookie"
// @Param X-CSRF-Protection header string false "POST 请求必须为 1" default(1)
// @Param MCP-Protocol-Version header string false "由客户端与官方 SDK 协商的协议版本"
// @Param body body MCPRequest false "POST 使用标准 JSON-RPC 请求"
// @Success 200 {object} MCPResponse "标准 JSON-RPC 结果；工具结果保留原 API Envelope"
// @Success 202 "通知已接受，无响应体"
// @Failure 400 {object} MCPResponse "MCP 请求或参数无效"
// @Failure 401 {object} MCPResponse "API Token 缺失或无效"
// @Failure 403 {object} MCPResponse "权限、来源或跨站保护校验失败"
// @Failure 404 {object} MCPResponse "MCP 未启用或接口不存在"
// @Failure 405 {object} MCPResponse "当前无状态模式仅支持 POST"
// @Failure 413 {object} MCPResponse "请求或响应超出配置限制"
// @Failure 415 {object} MCPResponse "请求必须使用 JSON"
// @Failure 500 {object} MCPResponse "MCP 服务内部错误"
// @Failure 503 {object} MCPResponse "账号、业务或暂时无法完成操作，请稍后重试"
// @Failure 504 {object} MCPResponse "请求超时"
// @Router /v1/mcp [post]
// @Router /v1/mcp [get]
// @Router /v1/mcp [delete]
func MCP(_ echo.Context) error {
	return echo.ErrMethodNotAllowed
}

func mcpOriginProtection(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Path() != mcpEndpointPath {
			return next(c)
		}
		if !config.Config.MCP.Enabled {
			return echo.ErrNotFound
		}
		// 协议要求所有连接检查来源，POST 同时沿用公共跨站写保护。
		if !validOrigin(c) {
			return echo.ErrForbidden
		}
		return next(c)
	}
}

func mcpConversion(engine *echo.Echo) echo.MiddlewareFunc {
	if !config.Config.MCP.Enabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	tools, version, err := mcpQueryTools(
		swaggerDocumentJSON, engine.Routes(), mcpQueryRegistrations(),
	)
	if err != nil {
		logger.Logger.Fatalf("MCP query catalog initialization failed: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: config.Config.App.Name, Version: version},
		&mcp.ServerOptions{
			PageSize: config.Config.HTTP.MaxPageSize,
			Instructions: "仅提供工作空间查询。先使用 get_workspaces 获取工作空间编号，" +
				"再按权限查询对应工作空间。多选筛选参数使用数组，工具结果保留原接口响应。",
		})
	for _, tool := range tools {
		server.AddTool(tool.Tool, mcpQueryHandler(engine, tool))
	}
	limit, _ := bytes.Parse(config.Config.HTTP.BodyLimit)
	transport := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless: true, JSONResponse: true,
			MaxRequestBodyBytes: limit, PropagateRequestCancellation: true,
			DisableLocalhostProtection: true,
		})
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Path() != mcpEndpointPath {
				return next(c)
			}

			// 业务适配器只持有去除认证凭据和请求体后的 HTTP 元信息。
			ctx := mcpRequestContext(c.Request())
			writer := newMCPResponseWriter()
			transport.ServeHTTP(writer, c.Request().WithContext(ctx))
			return mcpTransportResponse(c, writer)
		}
	}
}

func mcpRequestContext(request *http.Request) context.Context {
	parent := request.Clone(request.Context())
	parent.Header.Del(echo.HeaderAuthorization)
	parent.Header.Del(echo.HeaderCookie)
	parent.Body, parent.GetBody, parent.ContentLength = http.NoBody, nil, 0
	return context.WithValue(request.Context(), mcpRequestKey{}, parent)
}

func mcpTransportResponse(c echo.Context, writer *mcpResponseWriter) error {
	// 保留 SDK 协议头，认证、追踪和禁止缓存等公共响应头仍由外层中间件维护。
	for name, values := range writer.header {
		c.Response().Header()[name] = values
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	if writer.overflow {
		return mcpTransportFailure(c, http.StatusRequestEntityTooLarge,
			"查询结果过大，请缩小查询范围")
	}
	if writer.status == http.StatusAccepted {
		return c.NoContent(writer.status)
	}
	var wire mcpWireResponse
	parsed := json.Unmarshal(writer.body.Bytes(), &wire) == nil && wire.JSONRPC == "2.0"
	if writer.status >= http.StatusBadRequest || (parsed && wire.Error != nil) {
		status := writer.status
		code := int64(jsonrpc.CodeInvalidRequest)
		message := mcpHTTPErrorMessage(status)
		if parsed && wire.Error != nil {
			code = wire.Error.Code
			message = mcpProtocolErrorMessage(code)
		}
		if err := auditUnhandledFailure(
			c,
			max(status, http.StatusBadRequest),
			message,
		); err != nil {
			response.RecordError(c.Request().Context(), err)
			return sendMCPError(c, http.StatusServiceUnavailable, jsonrpc.CodeInternalError,
				"暂时无法完成操作，请稍后重试", wire.ID)
		}
		return sendMCPError(c, status, code, message, wire.ID)
	}
	if !parsed {
		return mcpTransportFailure(c, http.StatusServiceUnavailable, "MCP 响应格式无效")
	}
	return c.Blob(writer.status, echo.MIMEApplicationJSON, writer.body.Bytes())
}

func mcpTransportFailure(c echo.Context, status int, message string) error {
	if err := auditUnhandledFailure(c, status, message); err != nil {
		response.RecordError(c.Request().Context(), err)
		status, message = http.StatusServiceUnavailable, "暂时无法完成操作，请稍后重试"
	}
	return sendMCPError(c, status, jsonrpc.CodeInternalError, message, nil)
}

func sendMCPError(c echo.Context, status int, code int64, message string, id any) error {
	c.Response().Header().Del(echo.HeaderContentLength)
	return c.JSON(status, MCPResponse{
		JSONRPC: "2.0", ID: id, Error: &MCPError{Code: code, Message: message},
	})
}

func mcpProtocolErrorMessage(code int64) string {
	switch code {
	case jsonrpc.CodeParseError:
		return "MCP 请求格式无效"
	case jsonrpc.CodeMethodNotFound:
		return "不支持的 MCP 方法或工具"
	case jsonrpc.CodeInvalidParams:
		return "MCP 请求参数无效"
	case jsonrpc.CodeInternalError:
		return "MCP 服务暂时不可用"
	default:
		return "MCP 请求无效"
	}
}

func mcpHTTPErrorMessage(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "MCP 请求格式、协议版本或请求头无效"
	case http.StatusUnsupportedMediaType:
		return "MCP 请求必须使用 JSON"
	default:
		return errorMessage(status)
	}
}
