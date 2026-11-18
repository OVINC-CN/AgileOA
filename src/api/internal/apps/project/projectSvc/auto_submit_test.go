package projectSvc

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
)

func TestDraftAutoSubmitDeadline(t *testing.T) {
	deadline := time.Date(2026, 12, 31, 15, 59, 0, 0, time.UTC)
	later := deadline.Add(time.Hour)
	earlier := deadline.Add(-time.Hour)
	for _, item := range []struct {
		name    string
		current *time.Time
		now     time.Time
		want    bool
	}{
		{"截止前不提交", &deadline, deadline.Add(-time.Microsecond), false},
		{"截止时刻不提交", &deadline, deadline, false},
		{"严格晚于截止时间", &deadline, deadline.Add(time.Microsecond), true},
		{"延后截止时间使旧任务失效", &later, later.Add(time.Hour), false},
		{"提前截止时间使旧任务失效", &earlier, deadline.Add(time.Hour), false},
		{"清空截止时间使旧任务失效", nil, deadline.Add(time.Hour), false},
	} {
		t.Run(item.name, func(t *testing.T) {
			if draftAutoSubmitDue(item.current, deadline, item.now) != item.want {
				t.Fatal("自动提交未按最新截止时间及严格时间边界执行")
			}
		})
	}
}

func TestOpenProjectRequiresDeadline(t *testing.T) {
	deadline := time.Date(2026, 12, 31, 23, 59, 0, 123456789, time.FixedZone("UTC+8", 8*3600))
	for _, item := range []struct {
		name     string
		open     bool
		deadline *time.Time
		wantErr  bool
	}{
		{"开放时缺少截止时间", true, nil, true},
		{"关闭时无需截止时间", false, nil, false},
		{"开放时填写截止时间", true, &deadline, false},
	} {
		t.Run(item.name, func(t *testing.T) {
			input, err := cleanProject(projectModels.ProjectInput{
				Name: "虚构项目", Year: 2026,
				AllowSelfApply: item.open, SelfApplyDeadline: item.deadline,
			})
			if (err != nil) != item.wantErr {
				t.Fatalf("截止时间必填校验不符合预期：%v", err)
			}
			if input.SelfApplyDeadline != nil &&
				(input.SelfApplyDeadline.Location() != time.UTC ||
					input.SelfApplyDeadline.Nanosecond()%1000 != 0) {
				t.Fatal("截止时间未按数据库精度转换为 UTC")
			}
		})
	}
}

func autoSubmitDatabase(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn: connection, SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return database.WithContext(t.Context()), mock
}

func expectAutoSubmitLock(mock sqlmock.Sqlmock, value projectModels.AutoSubmitPayload) {
	// 每次处理都先取得同一空间排他锁，随后读取当前项目及草稿状态。
	mock.ExpectQuery("SELECT .* FROM `workspaces` WHERE id = \\? LIMIT \\? FOR UPDATE").
		WithArgs(value.WorkspaceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_enabled"}).AddRow(value.WorkspaceID, true))
	mock.ExpectQuery("SELECT .* FROM `projects` WHERE workspace_id = \\? AND id = \\? LIMIT \\?").
		WithArgs(value.WorkspaceID, value.ProjectID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "self_apply_deadline"}).
			AddRow(value.ProjectID, value.Deadline))
}

func expectAutoSubmitChanges(
	mock sqlmock.Sqlmock,
	value projectModels.AutoSubmitPayload,
	id uint64,
) {
	mock.ExpectQuery("SELECT .* FROM `project_applications` WHERE .*status = \\?").
		WithArgs(value.WorkspaceID, value.ProjectID, id, projectConstants.ApplicationDraft, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "student_id", "revision", "status"}).
			AddRow(id, 7, 4, projectConstants.ApplicationDraft))
	mock.ExpectQuery("SELECT .* FROM `project_approval_steps`").
		WithArgs(value.WorkspaceID, value.ProjectID).
		WillReturnRows(sqlmock.NewRows([]string{"position", "kind", "sign_mode", "allow_edit", "reviewer_label"}).
			AddRow(1, projectConstants.ApproverApplicant, projectConstants.SignAny, true, "申报人"))
	mock.ExpectQuery("SELECT .* FROM `students`").
		WithArgs(value.WorkspaceID, 7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"class_name", "student_no", "name"}).
			AddRow("虚构班级", "0007", "虚构学生"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `project_application_detail_attachments`").
		WithArgs(value.WorkspaceID, id, "ready").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("DELETE FROM `project_application_nodes`").
		WithArgs(value.WorkspaceID, id).WillReturnResult(sqlmock.NewResult(0, 1))
	for _, position := range []int{0, 1} {
		mock.ExpectExec("INSERT INTO `project_application_nodes`").
			WithArgs(value.WorkspaceID, id, position, 0, projectConstants.SignAny,
				true, projectConstants.ApproverApplicant, "", nil, nil, "申报人").
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("UPDATE `project_applications` SET .*status.* WHERE .*status.*revision").
		WithArgs(1, 5, projectConstants.ApplicationPending,
			value.WorkspaceID, value.ProjectID, id, projectConstants.ApplicationDraft, 4).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestRepeatedAutoSubmitOnlyChangesDraftOnce(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	value := projectModels.AutoSubmitPayload{
		WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3,
		Deadline: time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond),
	}
	for _, first := range []bool{true, false} {
		mock.ExpectBegin()
		expectAutoSubmitLock(mock, value)
		if first {
			expectAutoSubmitChanges(mock, value, 5)
			mock.ExpectExec("INSERT INTO `project_application_history`").
				WithArgs(value.WorkspaceID, 5, 0, projectConstants.ApplicationSubmit,
					"system", 0, "系统", "截止时间已过，系统自动提交草稿申报").
				WillReturnResult(sqlmock.NewResult(1, 1))
			expectAutoSubmitNotice(mock, value, 5, nil)
		} else {
			mock.ExpectQuery("SELECT .* FROM `project_applications` WHERE .*status = \\?").
				WithArgs(value.WorkspaceID, value.ProjectID, 5, projectConstants.ApplicationDraft, 1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))
		}
		mock.ExpectCommit()
		err := database.Transaction(func(tx *gorm.DB) error {
			changed, notices, err := autoSubmitDraft(tx, value, 5)
			if err == nil && changed != first {
				t.Fatal("重复任务不应修改已提交申报或创建系统流转记录")
			}
			if err == nil && (len(notices.students) == 1) != first {
				t.Fatal("自动提交只应在首次进入审批时投递一次学生通知")
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAutoSubmitHistoryFailureRollsBackWorkflow(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	value := projectModels.AutoSubmitPayload{
		WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3,
		Deadline: time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond),
	}
	mock.ExpectBegin()
	expectAutoSubmitLock(mock, value)
	expectAutoSubmitChanges(mock, value, 5)
	failure := errors.New("history write failed")
	mock.ExpectExec("INSERT INTO `project_application_history`").WillReturnError(failure)
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		_, _, err := autoSubmitDraft(tx, value, 5)
		return err
	})
	if !errors.Is(err, failure) {
		t.Fatalf("流转记录失败时应回滚状态和流程快照：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectAutoSubmitNotice(
	mock sqlmock.Sqlmock,
	value projectModels.AutoSubmitPayload,
	id uint64,
	failure error,
) {
	// 复用首级节点及学生身份解析，申报人节点也必须投递学生通知。
	mock.ExpectQuery("SELECT .* FROM `project_application_nodes`").
		WithArgs(value.WorkspaceID, id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"position", "kind", "sign_mode", "reviewer_label"}).
			AddRow(1, projectConstants.ApproverApplicant, projectConstants.SignAny, "申报人"))
	insert := mock.ExpectExec("INSERT INTO `student_notifications`")
	if failure != nil {
		insert.WillReturnError(failure)
	} else {
		insert.WillReturnResult(sqlmock.NewResult(1, 1))
	}
}

func TestAutoSubmitNotificationFailureRollsBackWorkflow(t *testing.T) {
	database, mock := autoSubmitDatabase(t)
	value := projectModels.AutoSubmitPayload{
		WorkspaceID: "w7m2k9p4s6n8c3x5", ProjectID: 3,
		Deadline: time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond),
	}
	mock.ExpectBegin()
	expectAutoSubmitLock(mock, value)
	expectAutoSubmitChanges(mock, value, 5)
	mock.ExpectExec("INSERT INTO `project_application_history`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	failure := errors.New("notification write failed")
	expectAutoSubmitNotice(mock, value, 5, failure)
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		_, _, err := autoSubmitDraft(tx, value, 5)
		return err
	})
	if !errors.Is(err, failure) {
		t.Fatalf("投递失败时应回滚自动提交状态和流转记录：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
