package httpd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
)

func TestAuditSourceComesFromEntryRoute(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		method string
		path   string
		source string
	}{
		{"web-query", http.MethodGet, "/v1/system/users", auditConstants.SourceWeb},
		{"web-write", http.MethodPost, "/v1/logout", auditConstants.SourceWeb},
		{"mcp", http.MethodPost, mcpEndpointPath, auditConstants.SourceMCP},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// 客户端伪造另一个合法来源，也不能通过请求头、查询参数或 UA 覆盖入口。
			override := auditConstants.SourceMCP
			if scenario.source == auditConstants.SourceMCP {
				override = auditConstants.SourceWeb
			}
			request := httptest.NewRequestWithContext(t.Context(), scenario.method,
				scenario.path+"?source="+override, nil)
			request.Header.Set("X-Audit-Source", override)
			request.Header.Set("User-Agent", override)
			context := echo.New().NewContext(request, httptest.NewRecorder())
			context.SetPath(scenario.path)
			handler := auditRequest(func(c echo.Context) error {
				if auditUtils.Request(c.Request().Context()).Source != scenario.source {
					t.Fatal("审计来源没有使用服务端入口")
				}
				return nil
			})
			if err := handler(context); err != nil {
				t.Fatal(err)
			}
		})
	}
}
