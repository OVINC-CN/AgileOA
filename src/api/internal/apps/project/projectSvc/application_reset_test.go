package projectSvc

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func expectApplicationReset(
	mock sqlmock.Sqlmock,
	application projectModels.Application,
	historyError error,
) {
	// 严格限制写入范围：保留申报内容、评分、附件及原历史，只重建流程和有效签署。
	mock.ExpectExec("DELETE FROM `project_application_approvals`").
		WithArgs(application.WorkspaceID, application.ID, 0).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM `project_application_nodes`").
		WithArgs(application.WorkspaceID, application.ID).
		WillReturnResult(sqlmock.NewResult(0, 3))
	current, status := 1, projectConstants.ApplicationPending
	positions := []int{0, 1}
	if application.Status == projectConstants.ApplicationDraft {
		current, status, positions = 0, projectConstants.ApplicationDraft, []int{0}
	}
	for _, position := range positions {
		mock.ExpectExec("INSERT INTO `project_application_nodes`").
			WithArgs(application.WorkspaceID, application.ID, position, 0,
				projectConstants.SignAny, true, projectConstants.ApproverApplicant,
				"", nil, nil, "申报人").
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("UPDATE `project_applications` SET .* WHERE .*revision = \\?").
		WithArgs(current, application.Revision+1, status,
			application.WorkspaceID, application.ProjectID, application.ID, application.Revision).
		WillReturnResult(sqlmock.NewResult(0, 1))
	history := mock.ExpectExec("INSERT INTO `project_application_history`").
		WithArgs(application.WorkspaceID, application.ID, application.CurrentStep,
			projectConstants.ApplicationReset, "user", 9, "虚构编辑人",
			"奖项审批流已更新，未完成申报的审批状态已重置")
	if historyError != nil {
		history.WillReturnError(historyError)
	} else {
		history.WillReturnResult(sqlmock.NewResult(1, 1))
	}
}

func TestApplicationWorkflowResetRestartsSubmittedAndKeepsDraft(t *testing.T) {
	for _, item := range []struct {
		status string
		step   int
	}{
		{projectConstants.ApplicationPending, 3},
		{projectConstants.ApplicationRejected, 0},
		{projectConstants.ApplicationDraft, 0},
	} {
		t.Run(item.status, func(t *testing.T) {
			database, mock := autoSubmitDatabase(t)
			application := projectModels.Application{
				ID: 5, WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3,
				Status: item.status, CurrentStep: item.step, Revision: 4,
			}
			mock.ExpectBegin()
			expectApplicationReset(mock, application, nil)
			mock.ExpectCommit()
			err := database.Transaction(func(tx *gorm.DB) error {
				return resetApplicationWorkflow(
					tx, application,
					[]projectModels.ApprovalStepInput{
						{
							Kind:     projectConstants.ApproverApplicant,
							SignMode: projectConstants.SignAny,
						},
					},
					applicationActor{UserID: 9, Name: "虚构编辑人"},
				)
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApplicationResetHistoryFailureRollsBack(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	application := projectModels.Application{
		ID: 5, WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3,
		Status: projectConstants.ApplicationPending, CurrentStep: 2, Revision: 4,
	}
	failure := errors.New("history write failed")
	mock.ExpectBegin()
	expectApplicationReset(mock, application, failure)
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		return resetApplicationWorkflow(
			tx, application,
			[]projectModels.ApprovalStepInput{{
				Kind: projectConstants.ApproverApplicant, SignMode: projectConstants.SignAny,
			}},
			applicationActor{UserID: 9, Name: "虚构编辑人"},
		)
	})
	if !errors.Is(err, failure) {
		t.Fatalf("重置历史失败时必须回滚签署、节点、状态和版本：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectWorkflowResetCannotRemoveLastSubmittedStep(t *testing.T) {
	for _, status := range []string{
		projectConstants.ApplicationPending, projectConstants.ApplicationRejected,
	} {
		t.Run(status, func(t *testing.T) {
			database, mock := autoSubmitDatabase(t)
			mock.ExpectQuery("SELECT .* FROM `project_applications` WHERE .*status IN").
				WithArgs("w7m2k9p4s6n8c3x5", 3, "draft", "pending", "rejected").
				WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(5, status))
			_, _, err := resetProjectApplications(
				database, "w7m2k9p4s6n8c3x5", projectModels.ProjectInfo{ID: 3}, applicationActor{},
			)
			if err == nil {
				t.Fatal("不能清空仍有未完成申报的审批流程")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectWorkflowResetAuditFailureRollsBack(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	workspaceID := "w7m2k9p4s6n8c3x5"
	ctx := auditUtils.WithRequest(t.Context(), auditModels.Request{
		Method: "PUT", Path: "/v1/workspaces/" + workspaceID + "/projects/3",
	})
	auditUtils.SetActor(ctx, auditModels.Actor{
		ID: 9, Username: "fixture.editor", DisplayName: "虚构编辑人",
	})
	ctx, span := tracing.Tracer.Start(ctx, "project.update")
	defer span.End()
	application := projectModels.Application{
		ID: 5, WorkspaceID: workspaceID, ProjectID: 3,
		Status: projectConstants.ApplicationDraft, Revision: 4,
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_applications` WHERE .*status IN").
		WithArgs(workspaceID, 3, "draft", "pending", "rejected").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "workspace_id", "project_id", "status", "revision",
		}).AddRow(5, workspaceID, 3, "draft", 4))
	expectSnapshots := func() {
		mock.ExpectQuery("SELECT .* FROM `project_application_nodes`").
			WithArgs(workspaceID, 5).
			WillReturnRows(sqlmock.NewRows([]string{"position", "kind"}).AddRow(0, "applicant"))
		mock.ExpectQuery("SELECT .* FROM `project_application_approvals`").
			WithArgs(workspaceID, 5).
			WillReturnRows(sqlmock.NewRows([]string{"position"}))
	}
	expectSnapshots()
	expectApplicationReset(mock, application, nil)
	mock.ExpectQuery("SELECT .* FROM `project_applications` WHERE .*id = \\?").
		WithArgs(workspaceID, 3, 5, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "workspace_id", "project_id", "status", "revision",
		}).AddRow(5, workspaceID, 3, "draft", 5))
	expectSnapshots()
	mock.ExpectExec("INSERT INTO `audit_events`").WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()
	err := database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, _, err := resetProjectApplications(
			tx, workspaceID, projectModels.ProjectInfo{ID: 3},
			applicationActor{UserID: 9, Name: "虚构编辑人"},
		)
		return err
	})
	if !errors.Is(err, auditSvc.ErrUnavailable) {
		t.Fatalf("逐单审计不可用时必须阻止整个项目保存事务提交：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
