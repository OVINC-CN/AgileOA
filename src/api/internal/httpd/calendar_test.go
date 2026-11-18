package httpd_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/httpd"
)

func TestCalendarValidatesDatesWithoutLogin(t *testing.T) {
	t.Parallel()

	// 公开接口仍纳入操作目录，不依赖账号或工作空间权限。
	action, ok := accessConstants.ForEndpoint(http.MethodGet, "/v1/home/calendar")
	if !ok || action.Scope != accessConstants.ScopePublic {
		t.Fatal("首页节假日接口未登记为公开操作")
	}

	// 匿名请求应先得到日期校验结果，不能被会话检查或失败审计阻断。
	server := httpd.New()
	for _, query := range []string{
		"", "?date=2026-02-29", "?date=0999-01-01", "?date=2027-13-01",
		"?date=2027-1-01", "?date=2027-01-01&date=2027-01-02",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/v1/home/calendar"+query, nil,
		)
		server.ServeHTTP(recorder, request)
		result := checkResponse(t, recorder)
		if recorder.Code != http.StatusBadRequest || result.ErrorMsg == "" {
			t.Fatalf("匿名日期校验失败：%s", recorder.Body.String())
		}
	}
}
