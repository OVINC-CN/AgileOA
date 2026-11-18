package httpd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

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
