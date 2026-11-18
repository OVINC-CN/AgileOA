package httpd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestRequestCredentialUsesEndpointAuthentication(t *testing.T) {
	// 同时携带两种凭据时也只读取入口允许的类型，禁止跨入口回退。
	const cookieValue = "fictional-cookie"
	const tokenValue = "fictional-token"
	for _, test := range []struct {
		name       string
		path       string
		headers    []string
		cookie     bool
		credential string
		apiToken   bool
	}{
		{
			name: "mcp-token", path: mcpEndpointPath,
			headers: []string{"Bearer " + tokenValue}, credential: tokenValue, apiToken: true,
		},
		{
			name: "mcp-both", path: mcpEndpointPath, cookie: true,
			headers: []string{"Bearer " + tokenValue}, credential: tokenValue, apiToken: true,
		},
		{
			name: "mcp-case-insensitive-scheme", path: mcpEndpointPath,
			headers: []string{"bearer " + tokenValue}, credential: tokenValue, apiToken: true,
		},
		{name: "mcp-cookie-only", path: mcpEndpointPath, cookie: true, apiToken: true},
		{
			name: "mcp-invalid-scheme", path: mcpEndpointPath, cookie: true,
			headers: []string{"Basic " + tokenValue}, apiToken: true,
		},
		{
			name: "mcp-empty-token", path: mcpEndpointPath, cookie: true,
			headers: []string{"Bearer"}, apiToken: true,
		},
		{
			name: "mcp-duplicate-header", path: mcpEndpointPath, cookie: true,
			headers: []string{"Bearer " + tokenValue, "Bearer " + tokenValue}, apiToken: true,
		},
		{name: "mcp-query-and-body-only", path: mcpEndpointPath, apiToken: true},
		{
			name: "api-cookie", path: "/v1/user-info", cookie: true, credential: cookieValue,
		},
		{
			name: "api-both", path: "/v1/user-info", cookie: true,
			headers: []string{"Bearer " + tokenValue}, credential: cookieValue,
		},
		{
			name: "api-invalid-header", path: "/v1/user-info", cookie: true,
			headers: []string{"Basic " + tokenValue}, credential: cookieValue,
		},
		{
			name: "api-duplicate-header", path: "/v1/user-info", cookie: true,
			headers:    []string{"Bearer " + tokenValue, "Bearer " + tokenValue},
			credential: cookieValue,
		},
		{
			name: "api-token-only", path: "/v1/user-info", headers: []string{"Bearer " + tokenValue},
		},
		{name: "api-query-and-body-only", path: "/v1/user-info"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// 查询参数和请求体始终放入伪造凭据，确认两种入口均不会读取。
			request := httptest.NewRequestWithContext(
				t.Context(), http.MethodPost, test.path+"?token=fictional-query",
				strings.NewReader(`{"token":"fictional-body"}`),
			)
			for _, value := range test.headers {
				request.Header.Add(echo.HeaderAuthorization, value)
			}
			if test.cookie {
				request.AddCookie(&http.Cookie{
					Name: config.Config.User.CookieName, Value: cookieValue,
					Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
				})
			}
			c := echo.New().NewContext(request, httptest.NewRecorder())
			c.SetPath(test.path)

			credential, apiToken := requestCredential(c)
			if credential != test.credential || apiToken != test.apiToken {
				t.Fatal("未按接口选择认证凭据")
			}
		})
	}
}
