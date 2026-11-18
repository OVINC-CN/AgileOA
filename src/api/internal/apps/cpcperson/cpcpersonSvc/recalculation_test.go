package cpcpersonSvc

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func recalculationRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "party_branch_id", "revision", "student_no", "name", "new_branch_id",
		"party_branch_short_name", "new_branch_short_name",
	})
}

func expectRecalculationRows(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("SELECT .* FROM `workspaces` WHERE id = \\? LIMIT \\? FOR UPDATE").
		WithArgs(workspaceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "is_enabled"}).
			AddRow(workspaceID, "虚构工作空间", true))
	mock.ExpectQuery("SELECT .* FROM `cpc_party_branches` WHERE workspace_id = \\? "+
		"AND admission_rule IS NOT NULL ORDER BY priority DESC, id DESC LIMIT \\?").
		WithArgs(workspaceID, config.Config.CPCPerson.MaxOptions+1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "admission_rule"}).AddRow(
			2, `{"type":"condition","field":"education_level","operator":"in","values":["硕士"]}`,
		))

	// 仅匹配同空间在编人员，两个党支部关联不能过滤掉未匹配的在编人员。
	mock.ExpectQuery("SELECT matched.id, .*"+
		regexp.QuoteMeta("COALESCE(previous_branch.short_name, '-') AS party_branch_short_name, ")+
		"next_branch.short_name AS new_branch_short_name FROM \\(SELECT p.id, .* "+
		"FROM cpc_people AS p "+
		"JOIN students AS s ON s.workspace_id = p.workspace_id AND s.id = p.student_id "+
		"WHERE p.workspace_id = \\? AND p.status = \\?\\) AS matched "+
		"LEFT JOIN cpc_party_branches AS previous_branch "+
		"ON previous_branch.workspace_id = matched.workspace_id "+
		"AND previous_branch.id = matched.party_branch_id "+
		"LEFT JOIN cpc_party_branches AS next_branch "+
		"ON next_branch.workspace_id = matched.workspace_id "+
		"AND next_branch.id = matched.new_branch_id ORDER BY matched.id ASC").
		WithArgs("硕士", 2, workspaceID, "在编").
		WillReturnRows(rows)
}

func expectBranchChange(mock sqlmock.Sqlmock, id uint64, affected int64) {
	mock.ExpectExec(regexp.QuoteMeta(
		"UPDATE `cpc_people` SET `party_branch_id`=?,`revision`=? "+
			"WHERE workspace_id = ? AND id = ? AND revision = ?",
	)).WithArgs(2, 2, "w7m2k9p4s6n8c3x5", id, 1).
		WillReturnResult(sqlmock.NewResult(0, affected))
}

func TestRecalculationNotifiesRequesterForEachChangedPerson(t *testing.T) {
	database, mock := timelineTestDB(t)
	mock.ExpectBegin()
	expectRecalculationRows(mock, recalculationRows().
		AddRow(42, 1, 1, "001", "虚构学生甲", 2, "原党支部", "新党支部").
		AddRow(43, 2, 1, "002", "虚构学生乙", 2, "新党支部", "新党支部").
		AddRow(44, 1, 1, "003", "虚构学生丙", 2, "原党支部", "新党支部"))
	for index, person := range []struct {
		id              uint64
		studentNo, name string
	}{
		{42, "001", "虚构学生甲"},
		{44, "003", "虚构学生丙"},
	} {
		expectBranchChange(mock, person.id, 1)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `notifications`")).
			WithArgs(
				7,
				fmt.Sprintf("fixture-reorder:person:%d", person.id),
				"人员党支部调整",
				fmt.Sprintf(
					"工作空间“虚构工作空间”的人员党支部已调整。\n学号：%s\n姓名：%s"+
						"\n调整前党支部：原党支部\n调整后党支部：新党支部",
					person.studentNo, person.name,
				),
				"/cpc/roster?workspace=w7m2k9p4s6n8c3x5",
				nil,
			).
			WillReturnResult(sqlmock.NewResult(int64(index+1), 1))
	}
	mock.ExpectCommit()
	var changes []branchRecalculation
	var notifications []notificationModels.Notification
	err := database.Transaction(func(tx *gorm.DB) error {
		var err error
		changes, notifications, err = recalculateBranches(
			tx, "w7m2k9p4s6n8c3x5", 7, "fixture-reorder",
		)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || changes[0].ID != 42 || changes[1].ID != 44 ||
		changes[0].StudentNo != "001" || changes[0].Name != "虚构学生甲" ||
		changes[0].PartyBranchID != 1 || *changes[0].NewBranchID != 2 ||
		changes[0].PartyBranchShortName != "原党支部" ||
		changes[0].NewBranchShortName != "新党支部" ||
		len(notifications) != 2 || notifications[0].UserID != 7 || notifications[1].UserID != 7 ||
		notifications[0].SourceID == notifications[1].SourceID {
		t.Fatal("变更快照或逐人通知不完整，或向未变更人员发送了明细通知")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecalculationNotificationFailureRollsBackAllChanges(t *testing.T) {
	database, mock := timelineTestDB(t)
	mock.ExpectBegin()
	expectRecalculationRows(mock, recalculationRows().
		AddRow(42, 1, 1, "001", "虚构学生甲", 2, "原党支部", "新党支部").
		AddRow(44, 1, 1, "003", "虚构学生丙", 2, "原党支部", "新党支部"))
	failure := errors.New("notification write unavailable")
	for index, id := range []uint64{42, 44} {
		expectBranchChange(mock, id, 1)
		insert := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `notifications`")).
			WithArgs(7, fmt.Sprintf("fixture-reorder:person:%d", id), "人员党支部调整",
				sqlmock.AnyArg(), "/cpc/roster?workspace=w7m2k9p4s6n8c3x5", nil)
		if index == 0 {
			insert.WillReturnResult(sqlmock.NewResult(1, 1))
		} else {
			insert.WillReturnError(failure)
		}
	}
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		changes, notifications, err := recalculateBranches(
			tx, "w7m2k9p4s6n8c3x5", 7, "fixture-reorder",
		)
		if len(changes) != 0 || len(notifications) != 0 {
			t.Fatal("通知保存失败后仍返回了可记录成功日志的变更或通知")
		}
		return err
	})
	if !errors.Is(err, failure) {
		t.Fatalf("明细通知保存失败时未回滚全部归属变更：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecalculationSkipsUnchangedAndLegacyNotifications(t *testing.T) {
	for _, test := range []struct {
		name, source string
		recipientID  uint64
		previousID   uint64
		wantChanges  int
	}{
		{"归属未变更", "fixture-reorder", 7, 2, 0},
		{"历史任务无接收人", "fixture-reorder", 0, 1, 1},
		{"历史任务无来源", "", 7, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock := timelineTestDB(t)
			mock.ExpectBegin()
			expectRecalculationRows(mock, recalculationRows().AddRow(
				42, test.previousID, 1, "001", "虚构学生甲", 2, "原党支部", "新党支部",
			))
			if test.wantChanges > 0 {
				expectBranchChange(mock, 42, 1)
			}
			mock.ExpectCommit()
			err := database.Transaction(func(tx *gorm.DB) error {
				changes, notifications, err := recalculateBranches(
					tx, "w7m2k9p4s6n8c3x5", test.recipientID, test.source,
				)
				if len(changes) != test.wantChanges || len(notifications) != 0 {
					t.Fatal("未变更人员或历史任务错误地产生了明细通知")
				}
				return err
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

func TestRecalculationUnmatchedPersonPreventsWrites(t *testing.T) {
	database, mock := timelineTestDB(t)
	mock.ExpectBegin()
	expectRecalculationRows(mock, recalculationRows().
		AddRow(42, 1, 1, "001", "虚构学生甲", 2, "原党支部", "新党支部").
		AddRow(44, 1, 1, "003", "虚构学生丙", nil, "原党支部", nil))
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		changes, notifications, err := recalculateBranches(
			tx, "w7m2k9p4s6n8c3x5", 7, "fixture-reorder",
		)
		if len(changes) != 0 || len(notifications) != 0 {
			t.Fatal("存在未匹配人员时仍返回了可记录成功日志的变更或通知")
		}
		return err
	})
	if !errors.Is(err, ErrUnmatched) {
		t.Fatalf("存在未匹配人员时未阻断归属修改及明细通知：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
