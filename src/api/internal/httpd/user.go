package httpd

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func identifyUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 首页公开资料与云端回调不读取会话，避免依赖账号状态或冒用用户身份。
		action, registered := accessConstants.ForEndpoint(c.Request().Method, c.Path())
		if registered && (action.Value == accessConstants.HomeCalendar ||
			action.Value == accessConstants.EmailCallbackReceive) {
			return next(c)
		}

		// 外部凭据仅在此处按入口读取，MCP 内部调用直接复用已确认的身份。
		token, isAPIToken := requestCredential(c)
		if token != "" &&
			(strings.HasPrefix(c.Path(), "/api/") || strings.HasPrefix(c.Path(), "/v1/")) &&
			db.DB() != nil {
			identity, err := userUtils.Authenticate(
				c.Request().Context(), token, isAPIToken,
			)
			if err == nil {
				ctx := userUtils.WithIdentity(c.Request().Context(), identity)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			// 无效会话交给统一鉴权，预读取中的组件或事务异常不能静默丢弃。
			var business *response.BusinessError
			if err != nil && !errors.As(err, &business) {
				response.RecordError(
					c.Request().Context(),
					fmt.Errorf("request identity lookup failed: %w", err),
				)
				if registered && action.Scope != accessConstants.ScopePublic {
					return &response.BusinessError{
						Status: http.StatusServiceUnavailable, Message: "账号服务暂时不可用",
					}
				}
			}
		}
		return next(c)
	}
}

func requestCredential(c echo.Context) (string, bool) {
	// MCP 只接受 Bearer Token，缺失或无效时不回退到 Cookie。
	if c.Path() == mcpEndpointPath {
		values := c.Request().Header.Values(echo.HeaderAuthorization)
		if len(values) != 1 {
			return "", true
		}
		fields := strings.Fields(values[0])
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			return "", true
		}
		return fields[1], true
	}

	// 其他接口只接受 Cookie，忽略认证头及查询参数、请求体中的凭据。
	cookie, err := c.Cookie(config.Config.User.CookieName)
	if err != nil {
		return "", false
	}
	return cookie.Value, false
}

func validWrite(c echo.Context) bool {
	return c.Request().Header.Get(userConstants.CSRFHeader) == userConstants.CSRFValue &&
		validOrigin(c)
}

func validOrigin(c echo.Context) bool {
	origin := c.Request().Header.Get("Origin")
	sameOrigin := c.Scheme() + "://" + c.Request().Host
	return origin == "" || origin == sameOrigin ||
		slices.Contains(config.Config.HTTP.CORSAllowOrigins, origin)
}

func passwordLimiter(action string) echo.MiddlewareFunc {
	// 使用直连 IP，避免未经配置的 X-Forwarded-For 绕过限流。
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(config.Config.User.LoginRate),
				Burst:     config.Config.User.LoginBurst,
				ExpiresIn: config.Config.User.LoginLimiterTTL,
			},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return echo.ExtractIPDirect()(c.Request()), nil
		},
		DenyHandler: rejectRateLimit(action),
	})
}

func rejectRateLimit(action string) func(echo.Context, string, error) error {
	return func(c echo.Context, _ string, _ error) error {
		return userSvc.Reject(c.Request().Context(), action, http.StatusTooManyRequests)
	}
}
