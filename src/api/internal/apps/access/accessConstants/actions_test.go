package accessConstants_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
)

func TestActionDirectoryIsExplicitAndUnambiguous(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`^[a-z][a-z0-9._]*$`)
	actions := map[string]bool{}
	endpoints := map[string]bool{}
	for _, action := range accessConstants.Catalog() {
		if !pattern.MatchString(action.Value) || len(action.Value) > 64 || actions[action.Value] ||
			action.Label == "" || action.Description == "" || action.Group.Value == "" || action.Group.Label == "" {
			t.Fatalf("操作目录不完整或存在重复标识：%s", action.Value)
		}
		actions[action.Value] = true
		if accessConstants.Grantable(
			action.Value,
		) != (action.Scope == accessConstants.ScopeWorkspace) {
			t.Fatalf("操作范围与可授权状态不一致：%s", action.Value)
		}
		for _, endpoint := range action.Endpoints {
			key := endpoint.Method + " " + endpoint.Path
			if endpoints[key] {
				t.Fatalf("接口重复对应多个操作：%s", key)
			}
			endpoints[key] = true
			if action.Scope == accessConstants.ScopeWorkspace &&
				endpoint.Path != "/v1/workspaces/:workspace_id" &&
				!strings.HasPrefix(endpoint.Path, "/v1/workspaces/:workspace_id/") {
				t.Fatalf("工作空间操作缺少显式租户路径：%s", key)
			}
			if strings.HasPrefix(endpoint.Path, "/v1/system/") &&
				action.Scope != accessConstants.ScopeSystem {
				t.Fatalf("系统管理接口可被普通权限放行：%s", key)
			}
		}
	}
	for _, action := range []string{"", "unregistered.read", accessConstants.UserCreate, accessConstants.AccessActionsUpdate} {
		if accessConstants.Grantable(action) {
			t.Fatalf("系统或未知操作可提交授权：%s", action)
		}
	}
}

func TestCatalogCannotBeMutatedByCallers(t *testing.T) {
	t.Parallel()
	items := accessConstants.Catalog()
	for index := range items {
		if len(items[index].Endpoints) == 0 {
			continue
		}
		original := items[index].Endpoints[0].Path
		items[index].Endpoints[0].Path = "/untrusted"
		lookup, ok := accessConstants.Lookup(items[index].Value)
		if !ok || lookup.Endpoints[0].Path != original {
			t.Fatal("调用方修改了全局接口目录")
		}
		return
	}
	t.Fatal("操作目录中没有接口")
}

func TestActionLookupsReturnIndependentEndpoints(t *testing.T) {
	t.Parallel()
	const path = "/v1/workspaces/:workspace_id/students/statistics"
	for _, lookup := range []func() (accessModels.Action, bool){
		func() (accessModels.Action, bool) {
			return accessConstants.Lookup(accessConstants.StudentStatistics)
		},
		func() (accessModels.Action, bool) {
			return accessConstants.ForEndpoint("GET", path)
		},
	} {
		item, ok := lookup()
		if !ok || len(item.Endpoints) == 0 {
			t.Fatal("查询操作缺少接口")
		}
		item.Endpoints[0].Path = "/untrusted"
		original, ok := accessConstants.ForEndpoint("GET", path)
		if !ok || original.Endpoints[0].Path != path {
			t.Fatal("查询结果修改了全局接口索引")
		}
	}
}

func TestReadOnlyActionsKeepMixedAndUnknownOperationsExclusive(t *testing.T) {
	t.Parallel()
	for _, action := range []string{
		accessConstants.StudentStatistics,
		accessConstants.StudentList,
		accessConstants.StudentExport,
		accessConstants.AccessWorkspace,
		accessConstants.AccessMyWorkspaces,
		accessConstants.UserList,
		accessConstants.UserAPITokenRead,
		accessConstants.AuditRead,
		accessConstants.StudentNotificationList,
	} {
		if !accessConstants.ReadOnly(action) {
			t.Fatalf("查询操作未允许并行读取：%s", action)
		}
	}
	for _, action := range []string{
		"",
		"unregistered.read",
		accessConstants.StudentCreate,
		accessConstants.ProjectApplicationDelete,
		accessConstants.StudentSensitiveHandle,
		accessConstants.CPCPersonTimelineTransferReview,
		accessConstants.CPCPersonRecalculate,
		accessConstants.AccessActionsUpdate,
		accessConstants.WorkspaceUpdate,
		accessConstants.UserAPITokenWrite,
	} {
		if accessConstants.ReadOnly(action) {
			t.Fatalf("写入、混合或未知操作被降为共享锁：%s", action)
		}
	}
}

func TestCPCDashboardHasIndependentWorkspaceActions(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"/v1/workspaces/:workspace_id/cpc-dashboard":        accessConstants.CPCDashboardList,
		"/v1/workspaces/:workspace_id/cpc-dashboard/fields": accessConstants.CPCDashboardList,
		"/v1/workspaces/:workspace_id/cpc-dashboard/export": accessConstants.CPCDashboardExport,
	} {
		action, ok := accessConstants.ForEndpoint("GET", path)
		if !ok || action.Value != want || action.Scope != accessConstants.ScopeWorkspace ||
			!accessConstants.Grantable(action.Value) {
			t.Fatalf("仪表接口未使用独立工作空间权限：%s，%+v", path, action)
		}
	}
}

func TestStudentDashboardHasIndependentWorkspaceActions(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"":                              accessConstants.StudentDashboardList,
		"/fields":                       accessConstants.StudentDashboardList,
		"/export":                       accessConstants.StudentDashboardExport,
		"/accommodation":                accessConstants.StudentDashboardList,
		"/accommodation/filter-options": accessConstants.StudentDashboardList,
		"/accommodation/export":         accessConstants.StudentDashboardExport,
	} {
		action, ok := accessConstants.ForEndpoint(
			"GET", "/v1/workspaces/:workspace_id/student-dashboard"+path,
		)
		if !ok || action.Value != want || action.Scope != accessConstants.ScopeWorkspace ||
			!accessConstants.Grantable(action.Value) || action.Group.Value != "student_dashboard" {
			t.Fatalf("学生仪表接口未使用独立工作空间权限：%s，%+v", path, action)
		}
	}
}

func TestTimelineCloneUsesCreatePermission(t *testing.T) {
	t.Parallel()
	action, ok := accessConstants.ForEndpoint(
		"POST", "/v1/workspaces/:workspace_id/cpc-timeline-templates/:id/clone",
	)
	if !ok || action.Value != accessConstants.CPCTimelineCreate ||
		action.Scope != accessConstants.ScopeWorkspace || !accessConstants.Grantable(action.Value) {
		t.Fatal("时间线模板克隆没有复用新建权限")
	}
}

func TestBusinessExportsHaveIndependentWorkspaceActions(t *testing.T) {
	t.Parallel()

	// 导出路径必须先于动态记录路径匹配，且各自绑定独立的可授予权限。
	for path, want := range map[string]string{
		"/v1/workspaces/:workspace_id/projects/export":                accessConstants.ProjectExport,
		"/v1/workspaces/:workspace_id/project-participations/export":  accessConstants.ParticipationExport,
		"/v1/workspaces/:workspace_id/graduation-destinations/export": accessConstants.GraduationExport,
		"/v1/workspaces/:workspace_id/cpc-party-branches/export":      accessConstants.CPCPartyBranchExport,
	} {
		action, ok := accessConstants.ForEndpoint("GET", path)
		if !ok || action.Value != want || action.Scope != accessConstants.ScopeWorkspace ||
			!accessConstants.Grantable(action.Value) {
			t.Fatalf("导出接口未使用独立工作空间权限：%s，%+v", path, action)
		}
	}
}

func TestNotificationsRequireIndependentPersonalActions(t *testing.T) {
	for _, endpoint := range []struct{ method, path, action string }{
		{"GET", "/v1/notifications", accessConstants.NotificationList},
		{"GET", "/v1/notifications/unread", accessConstants.NotificationUnread},
		{"POST", "/v1/notifications/read", accessConstants.NotificationRead},
	} {
		action, ok := accessConstants.ForEndpoint(endpoint.method, endpoint.path)
		if !ok || action.Value != endpoint.action || action.Scope != accessConstants.ScopeSession ||
			accessConstants.Grantable(action.Value) {
			t.Fatalf("通知接口未限定当前会话账号：%s %s", endpoint.method, endpoint.path)
		}
	}
}

func TestStudentNotificationsRequireBoundStudentActions(t *testing.T) {
	const base = "/v1/workspaces/:workspace_id/student-service/notifications"
	for _, endpoint := range []struct{ method, path, action string }{
		{"GET", base, accessConstants.StudentNotificationList},
		{"GET", base + "/unread", accessConstants.StudentNotificationUnread},
		{"POST", base + "/read", accessConstants.StudentNotificationRead},
	} {
		action, ok := accessConstants.ForEndpoint(endpoint.method, endpoint.path)
		if !ok || action.Value != endpoint.action || action.Scope != accessConstants.ScopeSession ||
			!accessConstants.StudentServiceAction(
				action.Value,
			) || accessConstants.Grantable(action.Value) {
			t.Fatalf("学生通知未限定绑定空间的可信学生会话：%+v", endpoint)
		}
	}
}

func TestTimelineTransferReviewRequiresIndependentAction(t *testing.T) {
	t.Parallel()
	const base = "/v1/workspaces/:workspace_id/cpc-people/:id/timeline/"
	for _, endpoint := range []struct{ method, path, action string }{
		{"POST", "transfer", accessConstants.CPCPersonUpdate},
		{"POST", "transfer-cancel", accessConstants.CPCPersonUpdate},
		{"GET", "review", accessConstants.CPCPersonTimelineTransferReview},
		{"GET", "transfer-review", accessConstants.CPCPersonTimelineTransferReview},
		{"POST", "transfer-review", accessConstants.CPCPersonTimelineTransferReview},
		{"POST", "transfer-reject", accessConstants.CPCPersonTimelineTransferReview},
	} {
		action, ok := accessConstants.ForEndpoint(endpoint.method, base+endpoint.path)
		if !ok || action.Value != endpoint.action ||
			action.Scope != accessConstants.ScopeWorkspace {
			t.Fatalf("转移审核未与人员编辑权限隔离：%s %s", endpoint.method, endpoint.path)
		}
	}
}
