package userRouters

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestSessionCookieUsesConfiguredDomainAndRootPath(t *testing.T) {
	original := config.Config.User
	t.Cleanup(func() { config.Config.User = original })
	for _, domain := range []string{"", "example.com", ".example.com"} {
		config.Config.User.CookieDomain = domain
		for _, maxAge := range []int{60, -1} {
			recorder := httptest.NewRecorder()
			ctx := echo.New().NewContext(
				httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/login", nil),
				recorder,
			)
			setCookie(ctx, "fictional-session", maxAge)
			result := recorder.Result()
			cookies := result.Cookies()
			if err := result.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if len(cookies) != 1 {
				t.Fatal("会话 Cookie 数量不正确")
			}
			cookie := cookies[0]
			if cookie.Domain != strings.TrimPrefix(domain, ".") || cookie.Path != "/" ||
				!cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode ||
				cookie.Secure != config.Config.User.CookieSecure {
				t.Fatal("设置与清除会话未使用根路径或未遵循同一 YAML Cookie 域和安全配置")
			}
		}
	}
}
