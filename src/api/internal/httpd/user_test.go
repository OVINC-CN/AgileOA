package httpd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func TestSessionRoutesHandleRepeatedRequestsFromSameIP(t *testing.T) {
	t.Parallel()

	// 使用生产路由注册。校验失败由各业务入口返回，无需建立会话或发送邮件。
	server := echo.New()
	server.HTTPErrorHandler = errorHandler
	serveHealthz(server)
	for _, scenario := range []struct {
		path   string
		status int
	}{
		{"/v1/login-captcha", http.StatusOK},
		{"/v1/login-email", http.StatusBadRequest},
		{"/v1/student-login-email", http.StatusBadRequest},
		{"/v1/login-email-verify", http.StatusBadRequest},
		{"/v1/change-password", http.StatusBadRequest},
	} {
		t.Run(scenario.path, func(t *testing.T) {
			for attempt := range 8 {
				request := httptest.NewRequestWithContext(
					t.Context(), http.MethodPost, scenario.path, strings.NewReader("{"),
				)
				request.RemoteAddr = "192.0.2.10:12345"
				request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				result := httptest.NewRecorder()
				server.ServeHTTP(result, request)
				if result.Code != scenario.status {
					t.Fatalf("第 %d 次请求返回 %d，期望 %d", attempt+1, result.Code, scenario.status)
				}
			}
		})
	}
}

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
