package accessSvc_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
)

func checkPermissionUserList(
	t *testing.T,
	server *echo.Echo,
	admin *http.Cookie,
	workspaceID, otherWorkspaceID string,
	userID, otherUserID uint64,
) {
	t.Helper()
	path := "/v1/system/access/workspaces/" + workspaceID + "/users"
	list := func(query string) accessModels.UserListData {
		t.Helper()
		return decode[accessModels.UserListData](t,
			request(t, server, http.MethodGet, path+query, nil, admin, 200))
	}

	// 未授权账号、系统管理员和其他空间的权限均不能进入当前空间列表。
	replaceActions(t, server, admin, otherWorkspaceID, otherUserID,
		[]string{accessConstants.StudentList}, 200)
	if result := list(""); result.Total != 0 || len(result.Items) != 0 {
		t.Fatal("未授权账号或其他空间授权出现在权限列表")
	}
	for _, query := range []string{
		"?permission_status=",
		"?permission_status=unknown",
		"?permission_status=granted&permission_status=ungranted",
	} {
		request(t, server, http.MethodGet, path+query, nil, admin, 400)
	}

	// 业务权限独立生效，不要求先授予进入空间权限；同一用户多项授权只占一行。
	replaceActions(t, server, admin, workspaceID, userID,
		[]string{accessConstants.StudentList, accessConstants.StudentCreate}, 200)
	result := list("?page_size=1")
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != userID ||
		result.Items[0].PermissionCount != 2 {
		t.Fatal("权限列表未在分页前按用户去重并统计有效操作")
	}
	if result := list("?page=2&page_size=1"); result.Total != 1 || len(result.Items) != 0 {
		t.Fatal("授权用户总数受分页影响")
	}
	if result := list("?keyword=" + url.QueryEscape("fixture.user")); result.Total != 1 {
		t.Fatal("授权用户关键词筛选失效")
	}
	if result := list("?permission_status=ungranted&keyword=fixture.user"); result.Total != 0 {
		t.Fatal("已授权用户仍出现在新增授权候选中")
	}
	result = list("?permission_status=ungranted&keyword=fixture.other")
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != otherUserID ||
		result.Items[0].PermissionCount != 0 {
		t.Fatal("其他空间授权影响了当前空间的未授权候选")
	}

	// 撤销全部权限后移出列表，重新成为新增授权候选。
	replaceActions(t, server, admin, workspaceID, userID, []string{}, 200)
	if result := list(""); result.Total != 0 || len(result.Items) != 0 {
		t.Fatal("撤销全部权限后账号仍在权限列表")
	}
	if result := list("?permission_status=ungranted&keyword=fixture.user"); result.Total != 1 {
		t.Fatal("撤销全部权限后账号没有恢复为新增授权候选")
	}
	replaceActions(t, server, admin, otherWorkspaceID, otherUserID, []string{}, 200)
}
