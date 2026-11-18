package httpd

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Authorize 统一按操作目录鉴权，所有业务路由自动生效，未登记接口默认拒绝。
func Authorize(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if config.Config.App.Debug && c.Path() == "/v1/swagger/*" {
			return next(c)
		}
		// 探针、文档与不存在的路由沿用现有处理；所有已注册 API 必须登记操作。
		if !strings.HasPrefix(c.Path(), "/api/") && !strings.HasPrefix(c.Path(), "/v1/") {
			return next(c)
		}
		action, ok := accessConstants.ForEndpoint(c.Request().Method, c.Path())
		if !ok {
			return echo.ErrForbidden
		}
		workspaceID := ""
		if c.Param("workspace_id") != "" {
			var err error
			workspaceID, err = accessUtils.WorkspaceID(c)
			if err != nil {
				return echo.ErrBadRequest
			}
		}
		if err := accessUtils.Check(
			c.Request().Context(),
			action.Value,
			workspaceID,
		); err != nil {
			return err
		}

		// 仅云端邮件回调豁免浏览器 CSRF 标识，其余写接口继续统一检查。
		if action.Value == accessConstants.EmailCallbackReceive {
			return next(c)
		}

		// 新接口也统一使用跨站写保护，不能因后续 app 漏挂路由中间件而放行。
		if c.Request().Method != http.MethodGet && c.Request().Method != http.MethodHead &&
			c.Request().Method != http.MethodOptions {
			if !validWrite(c) {
				event := auditModels.Event{ObjectType: action.Group.Value}
				if workspaceID != "" {
					event.TenantID = &workspaceID
				}
				return accessUtils.Reject(
					c.Request().Context(),
					action.Value,
					event,
					&response.BusinessError{
						Status: http.StatusForbidden, Message: response.MessageForbidden,
					},
				)
			}
		}
		return next(c)
	}
}
