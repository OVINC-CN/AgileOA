package auditRouters

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
)

func TestAuditScalarFilters(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		query   string
		invalid bool
	}{
		{name: "default"},
		{name: "all-actions", query: "?action_id="},
		{name: "system", query: "?workspace_id=system&user_identify_tenant_id=system"},
		{name: "workspace", query: "?workspace_id=w7m2k9p4s6n8c3x5"},
		{name: "maximum-user", query: "?user_id=18446744073709551615"},
		{name: "zero-user-and-result", query: "?user_id=0&result_code=0"},
		{name: "numeric-codes", query: "?result_code=0&result_code=403&access_type=1&access_type=2"},
		{name: "empty-instance", query: "?instance_id="},
		{name: "leading-zero-instance", query: "?instance_id=0000123"},
		{name: "trace", query: "?request_id=4bf92f3577b34da6a3ce929d0e0e4736"},
		{name: "utc-window", query: "?start_time=1794787200000&end_time=1794873600000"},
		{name: "unknown", query: "?unknown=x", invalid: true},
		{name: "old-keyword", query: "?keyword=someone", invalid: true},
		{name: "json-path", query: "?snapshot_user_info.id=1", invalid: true},
		{name: "old-source", query: "?source=web", invalid: true},
		{name: "old-action", query: "?action=audit.read", invalid: true},
		{name: "unknown-action", query: "?action_id=not_registered", invalid: true},
		{name: "global-is-retired", query: "?workspace_id=global", invalid: true},
		{name: "invalid-user", query: "?user_id=1.2", invalid: true},
		{name: "overflow-user", query: "?user_id=18446744073709551616", invalid: true},
		{name: "multiple-users", query: "?user_id=1&user_id=2", invalid: true},
		{name: "multiple-instances", query: "?instance_id=1&instance_id=2", invalid: true},
		{name: "zero-trace", query: "?request_id=00000000000000000000000000000000", invalid: true},
		{name: "partial-trace", query: "?request_id=4bf92f35", invalid: true},
		{name: "invalid-access", query: "?access_type=3", invalid: true},
		{name: "text-result", query: "?result_code=success", invalid: true},
		{name: "overflow-result", query: "?result_code=2147483648", invalid: true},
		{name: "reversed-window", query: "?start_time=1794873600000&end_time=1794787200000", invalid: true},
		{name: "same-window", query: "?start_time=1794787200000&end_time=1794787200000", invalid: true},
		{name: "window-too-long", query: "?start_time=1767225600000&end_time=1794873600000", invalid: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
				"/v1/system/audit/logs"+scenario.query, nil)
			query, err := parse(echo.New().NewContext(request, httptest.NewRecorder()))
			if (err != nil) != scenario.invalid {
				t.Fatalf("标量条件校验错误：%v", err)
			}
			if !scenario.invalid && query.Until.Location().String() != "UTC" {
				t.Fatal("时间边界没有使用 UTC")
			}
			if scenario.name == "maximum-user" && (query.UserID == nil ||
				strconv.FormatUint(*query.UserID, 10) != "18446744073709551615") {
				t.Fatal("大整数用户编号丢失精度")
			}
			if scenario.name == "empty-instance" &&
				(query.InstanceID == nil || *query.InstanceID != "") {
				t.Fatal("对象编号的显式空值被忽略")
			}
		})
	}
}

func TestAuditActionUnionAndDefault(t *testing.T) {
	for _, scenario := range []struct {
		query   string
		actions []string
	}{
		{actions: []string{auditConstants.FilterExcludeAudit}},
		{query: "?action_id="},
		{query: "?action_id=audit.read&action_id=audit.detail&action_id=audit.read",
			actions: []string{accessConstants.AuditRead, accessConstants.AuditDetail}},
		{query: "?action_id=exclude_audit&action_id=audit.detail",
			actions: []string{auditConstants.FilterExcludeAudit, accessConstants.AuditDetail}},
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/v1/system/audit/logs"+scenario.query, nil)
		query, err := parse(echo.New().NewContext(request, httptest.NewRecorder()))
		if err != nil || !slices.Equal(query.Actions, scenario.actions) {
			t.Fatalf("操作并集或默认值错误：%q，%v", query.Actions, err)
		}
	}
}
