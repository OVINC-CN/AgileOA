package httpd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestMCPCatalogOnlyExposesWorkspaceJSONQueries(t *testing.T) {
	// 显式登记项必须匹配实际路由和内嵌 Swagger，不能扩大允许的查询范围。
	engine := New()
	tools, version, err := mcpQueryTools(
		swaggerDocumentJSON, engine.Routes(), mcpQueryRegistrations(),
	)
	if err != nil || version == "" {
		t.Fatalf("查询目录生成失败：%v", err)
	}
	paths := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if !mcpQueryPath(tool.Path) || !tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("暴露了非工作空间查询：%s", tool.Path)
		}
		paths[tool.Path] = true
		input := tool.Tool.InputSchema.(map[string]any)
		if input["additionalProperties"] != false || tool.Tool.OutputSchema == nil {
			t.Fatalf("工具缺少固定输入或响应模型：%s", tool.Tool.Name)
		}
	}
	for _, path := range []string{
		"/v1/workspaces", "/v1/workspaces/{workspace_id}",
		"/v1/workspaces/{workspace_id}/students",
		"/v1/workspaces/{workspace_id}/students/{id}/sensitive",
		"/v1/workspaces/{workspace_id}/cpc-people/{id}/timeline",
		"/v1/workspaces/{workspace_id}/monthly-work/owners",
	} {
		if !paths[path] {
			t.Fatalf("遗漏了工作空间查询：%s", path)
		}
	}
	for _, path := range []string{
		"/v1/user-info", "/v1/system/users", "/v1/system/users/{id}/api-token",
		"/v1/workspaces/{workspace_id}/students/export",
		"/v1/workspaces/{workspace_id}/students/template",
		"/v1/workspaces/{workspace_id}/attachments/{id}/download",
	} {
		if paths[path] {
			t.Fatalf("错误暴露了查询范围外接口：%s", path)
		}
	}
}

func TestMCPQueriesRequireExplicitRegistration(t *testing.T) {
	// 路由与 Swagger 中存在的其他查询不能因满足路径规则而自动暴露。
	engine := New()
	registrations := []mcpQueryRegistration{
		{Name: "explicit_students", Path: mcpWorkspaceQueryPrefix + "/students"},
	}
	tools, _, err := mcpQueryTools(swaggerDocumentJSON, engine.Routes(), registrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Path != registrations[0].Path ||
		tools[0].Tool.Name != "explicit_students" {
		t.Fatal("工具开放范围或名称没有严格使用显式登记清单")
	}
}

func TestMCPRejectsInvalidRegistrations(t *testing.T) {
	engine := New()
	students := mcpWorkspaceQueryPrefix + "/students"
	fields := students + "/fields"
	for _, test := range []struct {
		name          string
		registrations []mcpQueryRegistration
	}{
		{
			name: "unknown-query",
			registrations: []mcpQueryRegistration{
				{Name: "unknown", Path: mcpWorkspaceQueryPrefix + "/unregistered-query"},
			},
		},
		{
			name: "system-query",
			registrations: []mcpQueryRegistration{
				{Name: "system_users", Path: "/v1/system/users"},
			},
		},
		{
			name: "file-export",
			registrations: []mcpQueryRegistration{
				{Name: "students_export", Path: students + "/export"},
			},
		},
		{
			name: "invalid-name",
			registrations: []mcpQueryRegistration{
				{Name: "invalid name", Path: students},
			},
		},
		{
			name: "duplicate-name",
			registrations: []mcpQueryRegistration{
				{Name: "students", Path: students},
				{Name: "students", Path: fields},
			},
		},
		{
			name: "duplicate-path",
			registrations: []mcpQueryRegistration{
				{Name: "students", Path: students},
				{Name: "other_students", Path: students},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// 显式登记写错时直接拒绝初始化，避免静默遗漏或暴露范围变化。
			if _, _, err := mcpQueryTools(
				swaggerDocumentJSON, engine.Routes(), test.registrations,
			); err == nil {
				t.Fatal("无效 MCP 登记项没有被拒绝")
			}
		})
	}
}

func TestMCPArgumentsPreserveFiltersAndLargeIdentifiers(t *testing.T) {
	tool := mcpQueryTool{
		Path: "/v1/workspaces/{workspace_id}/students/{id}",
		Parameters: []mcpParameter{
			{Name: "workspace_id", In: "path", Type: "string", Required: true},
			{Name: "id", In: "path", Type: "integer", Required: true},
			{Name: "major", In: "query", Type: "array", CollectionFormat: "multi",
				Items: map[string]any{"type": "string"}},
			{Name: "include_sensitive", In: "query", Type: "boolean"},
		},
	}
	target, err := mcpQueryURL(tool, json.RawMessage(`{
		"workspace_id":"a1b2c3d4e5f6g7h8","id":9007199254740993,
		"major":["","示例专业"],"include_sensitive":false
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(target.Path, "/9007199254740993") ||
		len(target.Query()["major"]) != 2 || target.Query()["major"][0] != "" ||
		target.Query().Get("include_sensitive") != "false" {
		t.Fatal("参数转换丢失大整数、空筛选值或布尔值")
	}
	for _, raw := range []string{
		`{"workspace_id":"../system","id":1}`,
		`{"workspace_id":"a%2fb","id":1}`,
		`{"workspace_id":"a1b2c3d4e5f6g7h8","id":"1"}`,
		`{"workspace_id":"a1b2c3d4e5f6g7h8","id":1,"user_id":1}`,
		`{"workspace_id":"a1b2c3d4e5f6g7h8","id":1,"major":null}`,
		`{"workspace_id":"a1b2c3d4e5f6g7h8"}`, `null`, `[]`, `{} {}`,
	} {
		if _, err := mcpQueryURL(tool, json.RawMessage(raw)); err == nil {
			t.Fatalf("未拒绝无效查询参数：%s", raw)
		}
	}
	if _, err := mcpQueryURL(mcpQueryTool{Path: "/v1/workspaces"}, nil); err != nil {
		t.Fatal("无参数查询不能省略 arguments")
	}
}

func TestMCPOutputSchemaIncludesOnlyReachableModels(t *testing.T) {
	root := map[string]any{"$ref": "#/definitions/Envelope"}
	definitions := map[string]map[string]any{
		"Envelope": {"type": "object", "properties": map[string]any{
			"data": map[string]any{"$ref": "#/definitions/Row", "x-nullable": true},
		}},
		"Row": {"type": "object", "properties": map[string]any{
			"parent": map[string]any{"$ref": "#/definitions/Row"},
		}},
		"Unrelated": {"type": "object"},
	}
	output, err := mcpOutputSchema(root, definitions)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "#/definitions/") || strings.Contains(text, "Unrelated") ||
		!strings.Contains(text, `"type":"null"`) || !strings.Contains(text, "#/$defs/Row") {
		t.Fatal("输出模型没有正确转换引用、可空字段或递归模型")
	}
}

func TestMCPStatelessTransportUsesStandardProtocol(t *testing.T) {
	// 直接运行中间件调用链，不启动监听端口或外部服务。
	engine := echo.New()
	serveHealthz(engine)
	engine.Use(mcpConversion(engine))
	for _, test := range []struct {
		method, body string
		status       int
	}{
		{http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
			`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
			`"clientInfo":{"name":"fixture-client","version":"1.0"}}}`, http.StatusOK},
		{http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, http.StatusOK},
		{http.MethodPost, `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			http.StatusAccepted},
		{http.MethodGet, "", http.StatusMethodNotAllowed},
		{http.MethodDelete, "", http.StatusMethodNotAllowed},
	} {
		request := httptest.NewRequestWithContext(t.Context(), test.method, mcpEndpointPath,
			strings.NewReader(test.body))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		request.Header.Set(echo.HeaderAccept, "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2025-11-25")
		writer := httptest.NewRecorder()
		engine.ServeHTTP(writer, request)
		if writer.Code != test.status {
			t.Fatalf("MCP 状态码错误：%d，%s", writer.Code, writer.Body.String())
		}
		if test.status == http.StatusAccepted {
			if writer.Body.Len() != 0 {
				t.Fatal("通知响应不应包含响应体")
			}
			continue
		}
		var wire mcpWireResponse
		if json.Unmarshal(writer.Body.Bytes(), &wire) != nil || wire.JSONRPC != "2.0" {
			t.Fatal("传输层没有返回标准 JSON-RPC 响应")
		}
		if test.status == http.StatusOK && wire.Error != nil {
			t.Fatalf("MCP 握手或目录查询失败：%s", wire.Error.Message)
		}
	}
}

func TestMCPDispatchReusesIdentityWithoutForwardingCredentials(t *testing.T) {
	// 使用公开 action 的内存路由隔离适配器，不建立数据库或第二层认证。
	engine := echo.New()
	engine.HTTPErrorHandler = errorHandler
	called := false
	engine.GET("/v1/home/calendar", func(c echo.Context) error {
		called = true
		identity, err := userUtils.Identity(c.Request().Context())
		if err != nil || identity.User.ID != 7 ||
			c.Request().Header.Get(echo.HeaderAuthorization) != "" ||
			c.Request().Header.Get(echo.HeaderCookie) != "" ||
			c.QueryParam("date") != "2026-11-11" || c.Request().Body != http.NoBody {
			t.Error("目标路由未复用可信身份，或收到原始凭据及错误参数")
		}
		return response.Success(c, map[string]string{"date": c.QueryParam("date")})
	})
	ctx := auditUtils.WithRequest(
		t.Context(),
		auditModels.Request{Method: "POST", Path: mcpEndpointPath},
	)
	ctx = userUtils.WithIdentity(ctx, userModels.Identity{User: userModels.User{ID: 7}})
	parent := httptest.NewRequestWithContext(ctx, http.MethodPost, mcpEndpointPath,
		strings.NewReader(`{"jsonrpc":"2.0"}`))
	parent.Header.Set(echo.HeaderAuthorization, "Bearer fictional-credential")
	parent.Header.Set(echo.HeaderCookie, "fixture=fictional-session")
	ctx = mcpRequestContext(parent)
	tool := mcpQueryTool{
		Tool: &mcp.Tool{Name: "fixture"}, Path: "/v1/home/calendar", Route: "/v1/home/calendar",
		Parameters: []mcpParameter{{Name: "date", In: "query", Type: "string"}},
	}
	result, err := mcpQueryHandler(engine, tool)(ctx, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{"date":"2026-11-11"}`)},
	})
	if err != nil || !called || result.IsError {
		t.Fatalf("原始路由调用失败：%v", err)
	}
}

func TestMCPResponseWriterRejectsPartialOversizedData(t *testing.T) {
	writer := newMCPResponseWriter()
	writer.limit = 4
	if _, err := io.WriteString(writer, "1234"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(writer, "5"); err != nil {
		t.Fatal(err)
	}
	if !writer.overflow || writer.body.Len() != 0 {
		t.Fatal("超限响应仍保留部分数据")
	}
}

func TestMCPRejectsUntrustedOriginBeforeAuthentication(t *testing.T) {
	engine := New()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, mcpEndpointPath, nil)
	request.Header.Set(echo.HeaderOrigin, "https://untrusted.example")
	writer := httptest.NewRecorder()
	engine.ServeHTTP(writer, request)
	if config.Config.MCP.Enabled && writer.Code != http.StatusForbidden {
		t.Fatal("MCP 未在认证前拒绝不可信来源")
	}
}
