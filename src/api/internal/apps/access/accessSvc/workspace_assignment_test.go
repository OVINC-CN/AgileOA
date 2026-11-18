package accessSvc_test

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

func checkAssignmentWorkspaceAccess(
	t *testing.T,
	server *echo.Echo,
	admin, reviewer *http.Cookie,
	workspaceID string,
	userID uint64,
) {
	t.Helper()
	// 在独立验收库构造配置节点及历史流程节点，不通过指派产生任何权限记录。
	connection := db.DB().WithContext(t.Context())
	project := projectModels.Project{
		WorkspaceID: workspaceID, Name: "虚构审批授权项目", Year: 2026, Revision: 1,
	}
	if err := connection.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, project.ID).
			Delete(&projectModels.Project{}).Error; err != nil {
			t.Error(err)
		}
	}()
	step := projectModels.ApprovalStep{
		WorkspaceID: workspaceID, ProjectID: project.ID, Position: 1,
		Kind: projectConstants.ApproverUser, UserID: &userID,
		SignMode: projectConstants.SignAny, ReviewerLabel: "虚构组织审批人",
	}
	if err := connection.Create(&step).Error; err != nil {
		t.Fatal(err)
	}
	path := "/v1/workspaces/" + workspaceID
	assertDenied := func() {
		t.Helper()
		list := decode[workspaceModels.ListData](t,
			request(t, server, http.MethodGet, "/v1/workspaces", nil, reviewer, 200))
		if list.Total != 0 || len(list.Items) != 0 {
			t.Fatal("节点指派使未获进入权限的账号发现了工作空间")
		}
		request(t, server, http.MethodGet, path, nil, reviewer, 403)
	}

	// 项目是否开放均不能替代进入权限。
	for _, open := range []bool{false, true} {
		if err := connection.Model(&projectModels.Project{}).
			Where("workspace_id = ? AND id = ?", workspaceID, project.ID).
			Update("allow_self_apply", open).Error; err != nil {
			t.Fatal(err)
		}
		assertDenied()
	}
	if err := connection.Where("workspace_id = ? AND project_id = ?", workspaceID, project.ID).
		Delete(&projectModels.ApprovalStep{}).Error; err != nil {
		t.Fatal(err)
	}
	student := studentModels.Student{
		WorkspaceID: workspaceID,
		BasicRecord: studentModels.BasicRecord{
			StudentNo: "fixture.assignment", Name: "虚构申报学生",
			Status: studentConstants.StatusEnrolled, Revision: 1,
		},
	}
	if err := connection.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, student.ID).
			Delete(&studentModels.Student{}).Error; err != nil {
			t.Error(err)
		}
	}()
	application := projectModels.Application{
		WorkspaceID: workspaceID, ProjectID: project.ID, StudentID: student.ID,
		Status: projectConstants.ApplicationPending, CurrentStep: 1, Revision: 1,
	}
	if err := connection.Create(&application).Error; err != nil {
		t.Fatal(err)
	}
	node := projectModels.ApplicationNode{
		WorkspaceID: workspaceID, ApplicationID: application.ID, Position: 1,
		Kind: projectConstants.ApproverUser, UserID: &userID,
		SignMode: projectConstants.SignAny, ReviewerLabel: "虚构组织审批人",
	}
	if err := connection.Create(&node).Error; err != nil {
		t.Fatal(err)
	}

	// 仅有申报快照节点时，审批中及已通过单据均不能隐式授予空间权限。
	for _, status := range []string{
		projectConstants.ApplicationPending,
		projectConstants.ApplicationApproved,
	} {
		if err := connection.Model(&projectModels.Application{}).
			Where("workspace_id = ? AND id = ?", workspaceID, application.ID).
			Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		assertDenied()
	}

	// 显式授权保留正常发现和进入能力，审批人提示仍须同时具备填报查看权限。
	for _, actions := range [][]string{
		{accessConstants.AccessWorkspace},
		{accessConstants.AccessWorkspace, accessConstants.ProjectApplicationList},
	} {
		replaceActions(t, server, admin, workspaceID, userID, actions, 200)
		list := decode[workspaceModels.ListData](t,
			request(t, server, http.MethodGet, "/v1/workspaces", nil, reviewer, 200))
		if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != workspaceID {
			t.Fatal("显式进入授权未恢复工作空间列表")
		}
		context := decode[accessModels.ContextData](t,
			request(t, server, http.MethodGet, path, nil, reviewer, 200))
		if context.CanReviewProjectApplications != (len(actions) == 2) {
			t.Fatal("审批人提示绕过了填报查看权限或没有保留已有行为")
		}
	}

	// 撤销进入权限立即隐藏并拒绝进入，保留填报权限或历史节点都不能恢复访问。
	for _, actions := range [][]string{{accessConstants.ProjectApplicationList}, {}} {
		replaceActions(t, server, admin, workspaceID, userID, actions, 200)
		assertDenied()
	}
}
