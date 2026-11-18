package httpd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestEmailCallbackRequiresConfiguredSign(t *testing.T) {
	original := config.Config.Email.CallbackSign
	t.Cleanup(func() { config.Config.Email.CallbackSign = original })
	server := New()

	// 发送无效事件，只有签名匹配时才会进入事件校验并返回 400。
	const configured = "Test+Sign"
	for _, test := range []struct {
		name   string
		sign   string
		query  string
		status int
	}{
		{"未配置", "", "?sign=", http.StatusUnauthorized},
		{"配置为空白", " ", "?sign=%20", http.StatusUnauthorized},
		{"缺少签名", configured, "", http.StatusUnauthorized},
		{"签名为空", configured, "?sign=", http.StatusUnauthorized},
		{"大小写不同", configured, "?sign=test%2Bsign", http.StatusUnauthorized},
		{"签名重复", configured, "?sign=Test%2BSign&sign=other", http.StatusUnauthorized},
		{"无效编码", configured, "?sign=Test%2BSign&sign=%zz", http.StatusUnauthorized},
		{"签名匹配", configured, "?sign=Test%2BSign", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			config.Config.Email.CallbackSign = test.sign
			request := httptest.NewRequestWithContext(
				t.Context(),
				http.MethodPost,
				"/v1/callback/email"+test.query,
				strings.NewReader(`{"sign":"Test+Sign"}`),
			)
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("回调状态码为 %d，期望 %d", recorder.Code, test.status)
			}
		})
	}
}

func TestEmailCallbackDoesNotRequireBrowserCredentials(t *testing.T) {
	t.Parallel()

	// 云端 POST 无需 Cookie、Origin 或 CSRF 标识，伪造 Cookie 也不触发账号认证。
	for _, test := range []struct {
		method string
		cookie bool
	}{
		{method: http.MethodPost},
		{method: http.MethodPost, cookie: true},
		{method: http.MethodGet},
	} {
		request := httptest.NewRequestWithContext(
			t.Context(),
			test.method,
			"/v1/callback/email",
			nil,
		)
		if test.cookie {
			request.AddCookie(&http.Cookie{
				Name: config.Config.User.CookieName, Value: "ignored-cookie",
				Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
			})
		}
		c := echo.New().NewContext(request, httptest.NewRecorder())
		c.SetPath("/v1/callback/email")
		called := false
		handler := identifyUser(Authorize(func(echo.Context) error {
			called = true
			return nil
		}))
		err := handler(c)
		if test.method == http.MethodPost {
			action, found := accessConstants.ForEndpoint(test.method, c.Path())
			if err != nil || !called || !found || action.Scope != accessConstants.ScopePublic {
				t.Fatalf("云端邮件回调被浏览器认证或跨站写保护拦截：%v", err)
			}
		} else if err == nil || called {
			t.Fatal("邮件回调豁免放行了未登记的方法")
		}
	}
}
