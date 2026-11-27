package httpd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
)

var mcpToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func mcpQueryTools(
	documentJSON string,
	routes []*echo.Route,
	registrations []mcpQueryRegistration,
) ([]mcpQueryTool, string, error) {
	// 仅加载显式登记项，Swagger 和路由中的新增接口不会自动开放。
	var document mcpDocument
	if err := json.Unmarshal([]byte(documentJSON), &document); err != nil {
		return nil, "", err
	}
	registered := make(map[string]bool, len(routes))
	for _, route := range routes {
		if route.Method == http.MethodGet {
			registered[route.Path] = true
		}
	}
	tools := make([]mcpQueryTool, 0, len(registrations))
	names := make(map[string]bool)
	paths := make(map[string]bool)
	for _, registration := range registrations {
		path := registration.Path
		scope := mcpQueryScope(path)
		if scope == "" {
			return nil, "", fmt.Errorf("MCP query registration is outside query scope: %s", path)
		}
		operation := document.Paths[path].Get
		if operation == nil {
			return nil, "", fmt.Errorf("MCP query registration has no GET documentation: %s", path)
		}
		schema := operation.Responses["200"].Schema
		if len(schema) == 0 {
			return nil, "", fmt.Errorf("MCP query registration has no response schema: %s", path)
		}
		if ref, _ := schema["$ref"].(string); strings.HasSuffix(ref, ".FileResponse") {
			return nil, "", fmt.Errorf("MCP query registration returns a file: %s", path)
		}
		route := strings.NewReplacer("{", ":", "}", "").Replace(path)
		action, ok := accessConstants.ForEndpoint(http.MethodGet, route)
		if !registered[route] || !ok || action.Scope != scope {
			return nil, "", fmt.Errorf("MCP query endpoint is not registered: %s", path)
		}
		tool, err := mcpToolDefinition(registration, operation, schema, document.Definitions)
		if err != nil {
			return nil, "", err
		}
		if names[tool.Name] {
			return nil, "", fmt.Errorf("duplicate MCP tool name: %s", tool.Name)
		}
		if paths[path] {
			return nil, "", fmt.Errorf("duplicate MCP query registration: %s", path)
		}
		names[tool.Name] = true
		paths[path] = true
		tools = append(tools, mcpQueryTool{
			Tool: tool, Path: path, Route: route, Action: action.Value,
			ObjectType: action.Group.Value, Parameters: operation.Parameters,
		})
	}
	return tools, document.Info.Version, nil
}

func mcpQueryScope(path string) string {
	for _, suffix := range []string{"/export", "/template", "/download"} {
		if strings.HasSuffix(path, suffix) {
			return ""
		}
	}
	switch {
	case path == "/v1/user-info" || path == "/v1/workspaces":
		return accessConstants.ScopeSession
	case strings.HasPrefix(path, "/v1/workspaces/"):
		return accessConstants.ScopeWorkspace
	case strings.HasPrefix(path, mcpSystemQueryPrefix+"/"):
		return accessConstants.ScopeSystem
	default:
		return ""
	}
}

func mcpToolDefinition(
	registration mcpQueryRegistration,
	operation *mcpOperation,
	responseSchema map[string]any,
	definitions map[string]map[string]any,
) (*mcp.Tool, error) {
	// 名称由登记清单固定，保持客户端工具名称稳定。
	path, name := registration.Path, registration.Name
	if !mcpToolNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid MCP tool registration name: %s", name)
	}

	// 参数字段完全沿用接口目录，多选参数保留数组及重复查询参数的含义。
	properties := make(map[string]any, len(operation.Parameters))
	required := make([]string, 0)
	for _, parameter := range operation.Parameters {
		if parameter.In != "path" && parameter.In != "query" {
			return nil, fmt.Errorf("unsupported MCP parameter location: %s", path)
		}
		if _, exists := properties[parameter.Name]; exists {
			return nil, fmt.Errorf("duplicate MCP query parameter: %s", path)
		}
		property := map[string]any{"type": parameter.Type, "description": parameter.Description}
		if parameter.Items != nil {
			property["items"] = parameter.Items
		}
		if len(parameter.Enum) > 0 {
			property["enum"] = parameter.Enum
		}
		if parameter.Default != nil {
			property["default"] = parameter.Default
		}
		properties[parameter.Name] = property
		if parameter.Required || parameter.In == "path" {
			required = append(required, parameter.Name)
		}
	}
	output, err := mcpOutputSchema(responseSchema, definitions)
	if err != nil {
		return nil, err
	}
	return &mcp.Tool{
		Name: name, Title: operation.Summary,
		Description: strings.TrimSpace(operation.Summary + "。" + operation.Description),
		InputSchema: map[string]any{
			"type": "object", "properties": properties,
			"required": required, "additionalProperties": false,
		},
		OutputSchema: output,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true, IdempotentHint: true,
			DestructiveHint: new(bool), OpenWorldHint: new(bool),
		},
	}, nil
}

func mcpOutputSchema(
	root map[string]any,
	definitions map[string]map[string]any,
) (map[string]any, error) {
	// 只携带当前响应实际引用的模型，避免每个工具重复附带完整 Swagger。
	resolved := make(map[string]any)
	var rewrite func(any) (any, error)
	rewrite = func(value any) (any, error) {
		switch current := value.(type) {
		case map[string]any:
			result := make(map[string]any, len(current))
			for key, item := range current {
				if key == "x-nullable" {
					continue
				}
				if key == "$ref" {
					ref, ok := item.(string)
					if !ok || !strings.HasPrefix(ref, "#/definitions/") {
						return nil, fmt.Errorf("invalid MCP response schema reference")
					}
					name := strings.TrimPrefix(ref, "#/definitions/")
					definition, exists := definitions[name]
					if !exists {
						return nil, fmt.Errorf("missing MCP response definition: %s", name)
					}
					result[key] = "#/$defs/" + name
					if _, exists := resolved[name]; !exists {
						resolved[name] = nil
						converted, err := rewrite(definition)
						if err != nil {
							return nil, err
						}
						resolved[name] = converted
					}
					continue
				}
				converted, err := rewrite(item)
				if err != nil {
					return nil, err
				}
				result[key] = converted
			}
			if current["x-nullable"] == true {
				return map[string]any{"anyOf": []any{result, map[string]any{"type": "null"}}}, nil
			}
			return result, nil
		case []any:
			result := make([]any, len(current))
			for index, item := range current {
				converted, err := rewrite(item)
				if err != nil {
					return nil, err
				}
				result[index] = converted
			}
			return result, nil
		default:
			return value, nil
		}
	}
	value, err := rewrite(root)
	if err != nil {
		return nil, err
	}
	result := value.(map[string]any)
	result["$defs"] = resolved
	return result, nil
}
