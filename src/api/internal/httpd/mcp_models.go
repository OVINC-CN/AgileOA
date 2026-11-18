package httpd

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPRequest 描述标准 JSON-RPC 请求，不使用业务接口的请求封装。
type MCPRequest struct {
	// JSONRPC 是协议标识，固定为 2.0。
	JSONRPC string `json:"jsonrpc" example:"2.0"`
	// ID 是客户端请求编号，通知不提供该字段。
	ID any `json:"id,omitempty"`
	// Method 是协议方法，例如 initialize、tools/list 或 tools/call。
	Method string `json:"method" example:"tools/list"`
	// Params 是对应协议方法的参数，工具参数只接受现有接口声明的字段。
	Params map[string]any `json:"params,omitempty"`
}

// MCPResponse 描述标准 JSON-RPC 响应；通知接受后返回 202 且无响应体。
type MCPResponse struct {
	// JSONRPC 是协议标识，固定为 2.0。
	JSONRPC string `json:"jsonrpc" example:"2.0"`
	// ID 对应客户端请求编号，无法识别请求时为空。
	ID any `json:"id"`
	// Result 是协议结果；工具的 structuredContent 保留原 API Envelope。
	Result any `json:"result,omitempty"`
	// Error 是协议错误，业务查询失败由工具结果的 isError 表达。
	Error *MCPError `json:"error,omitempty"`
}

// MCPError 保留标准协议错误码，说明使用简体中文。
type MCPError struct {
	// Code 是标准 JSON-RPC 错误码。
	Code int64 `json:"code" example:"-32600"`
	// Message 是简体中文错误说明。
	Message string `json:"message" example:"MCP 请求无效"`
}

type mcpDocument struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Paths map[string]struct {
		Get *mcpOperation `json:"get"`
	} `json:"paths"`
	Definitions map[string]map[string]any `json:"definitions"`
}

type mcpOperation struct {
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	Parameters  []mcpParameter `json:"parameters"`
	Responses   map[string]struct {
		Schema map[string]any `json:"schema"`
	} `json:"responses"`
}

type mcpParameter struct {
	Name             string         `json:"name"`
	In               string         `json:"in"`
	Type             string         `json:"type"`
	Description      string         `json:"description"`
	Required         bool           `json:"required"`
	Items            map[string]any `json:"items"`
	Enum             []any          `json:"enum"`
	Default          any            `json:"default"`
	CollectionFormat string         `json:"collectionFormat"`
}

type mcpQueryTool struct {
	Tool       *mcp.Tool
	Path       string
	Route      string
	Action     string
	ObjectType string
	Parameters []mcpParameter
}

// mcpQueryRegistration 显式指定允许暴露的工具名称和 GET 接口，不推导业务权限。
type mcpQueryRegistration struct {
	Name string
	Path string
}

type mcpRequestKey struct{}

type mcpResponseWriter struct {
	header    http.Header
	body      bytes.Buffer
	status    int
	limit     int64
	overflow  bool
	committed bool
}

type mcpWireResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *MCPError       `json:"error"`
}
