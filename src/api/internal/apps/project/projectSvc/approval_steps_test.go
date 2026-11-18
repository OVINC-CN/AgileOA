package projectSvc

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
)

func TestApprovalIdentitySnapshotsPersistFormattedNames(t *testing.T) {
	for _, item := range []struct {
		name, kind, query, number, displayName, label string
	}{
		{
			name:        "组织用户",
			kind:        projectConstants.ApproverUser,
			query:       "SELECT .* FROM `users`",
			number:      "admin",
			displayName: "系统管理员",
			label:       "系统管理员(admin)",
		},
		{
			name:        "学生用户",
			kind:        projectConstants.ApproverStudent,
			query:       "SELECT .* FROM `students`",
			number:      "0007",
			displayName: "学生 · 用户(甲)",
			label:       "学生 · 用户(甲)(0007)",
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			// 使用虚构数据库连接，直接核对两类快照实际写入的字段。
			connection, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = connection.Close() })
			database, err := gorm.Open(mysql.New(mysql.Config{
				Conn: connection, SkipInitializeWithVersion: true,
			}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			tx := database.WithContext(t.Context())
			workspaceID := "w7m2k9p4s6n8c3x5"
			projectID, applicationID, reviewerID := uint64(3), uint64(5), uint64(7)
			step := projectModels.ApprovalStepInput{
				Kind: item.kind, SignMode: projectConstants.SignAny,
			}
			var studentID, userID any
			if item.kind == projectConstants.ApproverStudent {
				step.StudentIDs, studentID = []uint64{reviewerID}, reviewerID
			} else {
				step.UserIDs, userID = []uint64{reviewerID}, reviewerID
			}
			expectIdentity := func() {
				query := mock.ExpectQuery(item.query)
				if item.kind == projectConstants.ApproverStudent {
					query.WithArgs(workspaceID, reviewerID, studentConstants.StatusEnrolled, 1)
				} else {
					query.WithArgs(reviewerID, true, 1)
				}
				query.WillReturnRows(sqlmock.NewRows([]string{"number", "name"}).
					AddRow(item.number, item.displayName))
			}

			// 配置保存直接写入新格式，不能靠读取时转换掩盖旧数据。
			mock.ExpectQuery("SELECT .* FROM `project_approval_steps`").
				WithArgs(workspaceID, projectID).
				WillReturnRows(sqlmock.NewRows([]string{"reviewer_label"}))
			mock.ExpectExec("DELETE FROM `project_approval_steps`").
				WithArgs(workspaceID, projectID).WillReturnResult(sqlmock.NewResult(0, 0))
			expectIdentity()
			mock.ExpectExec("INSERT INTO `project_approval_steps`").
				WithArgs(workspaceID, projectID, 1, 0, projectConstants.SignAny, true, item.kind,
					"", studentID, userID, item.label).
				WillReturnResult(sqlmock.NewResult(0, 1))
			steps := []projectModels.ApprovalStepInput{step}
			if err := saveApprovalSteps(tx, workspaceID, projectID, steps); err != nil {
				t.Fatal(err)
			}

			// 正式提交复制的节点同样保存新格式，申报人占位名称保持业务含义。
			mock.ExpectExec("INSERT INTO `project_application_nodes`").
				WithArgs(workspaceID, applicationID, 0, 0, projectConstants.SignAny,
					true, projectConstants.ApproverApplicant, "", nil, nil, "申报人").
				WillReturnResult(sqlmock.NewResult(0, 1))
			expectIdentity()
			mock.ExpectExec("INSERT INTO `project_application_nodes`").
				WithArgs(workspaceID, applicationID, 1, 0, projectConstants.SignAny, true, item.kind,
					"", studentID, userID, item.label).
				WillReturnResult(sqlmock.NewResult(0, 1))
			if err := initializeApplicationNodes(
				tx,
				workspaceID,
				applicationID,
				steps,
			); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApprovalWorkflowComparisonIgnoresDisplayAndTargetOrder(t *testing.T) {
	before := []projectModels.ApprovalStepInput{
		{Kind: projectConstants.ApproverUser, SignMode: projectConstants.SignAll,
			UserIDs: []uint64{7, 9}, ReviewerLabel: "原显示名称"},
		{Kind: projectConstants.ApproverStudent, StudentIDs: []uint64{3, 5}},
		{Kind: projectConstants.ApproverClassPosition, ClassPositions: []string{"班长", "团支书"}},
	}
	after := []projectModels.ApprovalStepInput{
		{Kind: projectConstants.ApproverUser, SignMode: projectConstants.SignAll,
			UserIDs: []uint64{9, 7}, ReviewerLabel: "新显示名称"},
		{Kind: projectConstants.ApproverStudent, SignMode: projectConstants.SignAny,
			StudentIDs: []uint64{5, 3}},
		{Kind: projectConstants.ApproverClassPosition, ClassPositions: []string{"团支书", "班长"}},
	}
	if !sameApprovalSteps(before, after) || before[0].UserIDs[0] != 7 {
		t.Fatal("显示名称及同级多选顺序不应触发重置，比较也不能改写原始配置")
	}
	for _, change := range []func([]projectModels.ApprovalStepInput){
		func(steps []projectModels.ApprovalStepInput) { steps[0].SignMode = projectConstants.SignAny },
		func(steps []projectModels.ApprovalStepInput) { steps[0].UserIDs = []uint64{7, 8} },
		func(steps []projectModels.ApprovalStepInput) { steps[1].StudentIDs = []uint64{3} },
		func(steps []projectModels.ApprovalStepInput) { steps[2].ClassPositions = []string{"班长"} },
		func(steps []projectModels.ApprovalStepInput) { steps[0], steps[1] = steps[1], steps[0] },
	} {
		changed := append([]projectModels.ApprovalStepInput(nil), after...)
		change(changed)
		if sameApprovalSteps(before, changed) {
			t.Fatal("签署方式、处理人集合及级别顺序的变化必须触发重置")
		}
	}
	if sameApprovalSteps(before, before[:2]) {
		t.Fatal("移除审批级别必须触发重置")
	}
}
