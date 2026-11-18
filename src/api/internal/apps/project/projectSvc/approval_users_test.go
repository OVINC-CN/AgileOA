package projectSvc_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"
)

func checkApprovalUserWorkspaceAccess(
	t *testing.T,
	server *echo.Echo,
	workspaceID, otherWorkspaceID string,
	admin, cookie *http.Cookie,
) {
	t.Helper()

	// 同空间进入权限不能由其他业务授权、其他空间授权或管理员身份替代。
	allowed, _ := fixture(t, "fixture.approval.allowed", strings.Repeat("81", 32), false)
	businessOnly, _ := fixture(t, "fixture.approval.business", strings.Repeat("82", 32), false)
	foreign, _ := fixture(t, "fixture.approval.foreign", strings.Repeat("83", 32), false)
	disabled, _ := fixture(t, "fixture.approval.disabled", strings.Repeat("84", 32), false)
	ungrantedAdmin, _ := fixture(t, "fixture.approval.admin", strings.Repeat("85", 32), true)
	grant(t, server, admin, workspaceID, allowed.ID,
		[]string{accessConstants.AccessWorkspace, accessConstants.ProjectList})
	grant(t, server, admin, workspaceID, businessOnly.ID, []string{accessConstants.ProjectList})
	grant(t, server, admin, otherWorkspaceID, foreign.ID, []string{accessConstants.AccessWorkspace})
	grant(t, server, admin, workspaceID, disabled.ID, []string{accessConstants.AccessWorkspace})
	if err := db.DB().WithContext(t.Context()).Model(&userModels.User{}).
		Where("id = ?", disabled.ID).Update("is_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	base := "/v1/workspaces/" + workspaceID + "/projects"
	optionsPath := base + "/approval-options?kind=user&keyword=fixture.approval."
	options := decode[projectModels.ApprovalOptions](t,
		request(t, server, "GET", optionsPath, nil, cookie, 200))
	if len(options.Items) != 1 || options.Items[0].Value != strconvID(allowed.ID) {
		t.Fatal("审批人选项未限定为当前空间具备进入权限的启用账号，或返回了重复账号")
	}
	for _, user := range []userModels.User{businessOnly, foreign, disabled, ungrantedAdmin} {
		searchPath := base + "/approval-options?kind=user&keyword=" + user.Username
		searched := decode[projectModels.ApprovalOptions](t,
			request(t, server, "GET", searchPath, nil, cookie, 200))
		if len(searched.Items) != 0 {
			t.Fatal("按用户名搜索绕过了当前空间进入权限或账号状态限制")
		}
	}

	// 直接提交编号也必须复用候选项条件，不能绕过下拉菜单的过滤。
	input := projectModels.ProjectInput{
		Name: "虚构空间审批权限项目", Year: 2026,
		ApprovalSteps: []projectModels.ApprovalStepInput{{
			Kind: projectConstants.ApproverUser, SignMode: projectConstants.SignAny,
			UserIDs: []uint64{allowed.ID},
		}},
	}
	project := decode[projectModels.ProjectInfo](t, request(t, server, "POST", base,
		projectModels.ProjectRequest{ProjectInput: input}, cookie, 200))
	for _, user := range []userModels.User{businessOnly, foreign, disabled, ungrantedAdmin} {
		input.ApprovalSteps[0].UserIDs = []uint64{user.ID}
		request(t, server, "POST", base,
			projectModels.ProjectRequest{ProjectInput: input}, cookie, 404)
		request(
			t,
			server,
			"PUT",
			base+"/"+strconvID(project.ID),
			projectModels.ProjectRequest{ProjectInput: input, Revision: project.Revision},
			cookie,
			404,
		)
	}

	// 撤销进入权限后立即移除候选项，导入固定基础列，不接收附加审批配置。
	grant(t, server, admin, workspaceID, allowed.ID, []string{accessConstants.ProjectList})
	options = decode[projectModels.ApprovalOptions](t,
		request(t, server, "GET", optionsPath, nil, cookie, 200))
	if len(options.Items) != 0 {
		t.Fatal("撤销进入权限后审批人选项仍包含该账号")
	}
	input.ApprovalSteps[0].UserIDs = []uint64{allowed.ID}
	steps, err := json.Marshal(input.ApprovalSteps)
	if err != nil {
		t.Fatal(err)
	}
	content := "项目名称,归属年份,说明,子项名称(JSON),允许自行申报,审批流(JSON)\n" +
		fmt.Sprintf("虚构撤权导入项目,2026,,[],false,\"%s\"\n",
			strings.ReplaceAll(string(steps), "\"", "\"\""))
	request(t, server, "POST", base+"/import",
		projectModels.ImportRequest{Content: spreadsheettest.Fixture(t, content)}, cookie, 400)
	var count int64
	if err := db.DB().WithContext(t.Context()).Model(&projectModels.Project{}).
		Where("workspace_id = ? AND name = ?", workspaceID, "虚构撤权导入项目").
		Count(&count).Error; err != nil || count != 0 {
		t.Fatal("奖项导入错误接纳附加审批配置")
	}
}
