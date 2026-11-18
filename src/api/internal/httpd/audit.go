package httpd

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
)

func auditRequest(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 来源只由服务端入口确定，不接受客户端请求头或参数覆盖。
		source := auditConstants.SourceWeb
		if c.Path() == mcpEndpointPath {
			source = auditConstants.SourceMCP
		}

		// 每个请求生成独立请求 ID，批量业务事件共享该上下文；不记录原始 URL 参数。
		ctx := auditUtils.WithRequest(c.Request().Context(), auditModels.Request{
			Method:    c.Request().Method,
			Path:      c.Path(),
			Source:    source,
			ClientIP:  c.RealIP(),
			UserAgent: c.Request().UserAgent(),
		})
		c.SetRequest(c.Request().WithContext(ctx))
		c.Response().Header().Set(auditConstants.RequestIDHeader, auditUtils.Request(ctx).ID)
		return next(c)
	}
}

func auditUnhandledFailure(c echo.Context, status int, message string) error {
	// 请求体限制等中间件错误也必须审计，业务已经处理过的错误不重复记录。
	if auditUtils.Attempted(c.Request().Context()) {
		return nil
	}
	action, ok := accessConstants.ForEndpoint(c.Request().Method, c.Path())
	if !ok || action.Value == accessConstants.EmailCallbackReceive {
		return nil
	}
	event := auditModels.Event{
		Action:       action.Value,
		ObjectType:   action.Group.Value,
		ErrorCode:    "http_" + strconv.Itoa(status),
		ErrorMessage: message,
	}
	if id, err := accessUtils.WorkspaceID(c); err == nil {
		event.TenantID = &id
	}
	return auditSvc.Failure(c.Request().Context(), event)
}
