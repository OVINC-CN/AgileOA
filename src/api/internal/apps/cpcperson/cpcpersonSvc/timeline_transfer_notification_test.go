package cpcpersonSvc

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func expectTimelineReviewers(mock sqlmock.Sqlmock, ids ...uint64) {
	rows := sqlmock.NewRows([]string{"id"})
	for _, id := range ids {
		rows.AddRow(id)
	}
	// 接收人必须同时具备当前空间审核授权、启用账号及对应支部或全空间编辑范围。
	mock.ExpectQuery("SELECT .* FROM `users` JOIN access_permissions AS reviewer .*"+
		"reviewer.workspace_id = \\? AND reviewer.action = \\? .*users.is_enabled = \\? .*"+
		"EXISTS .*workspace_id = \\? AND user_id = users.id AND action = \\? .*"+
		"EXISTS .*workspace_id = \\? AND id = \\? AND secretary_id = users.id").
		WithArgs("w7m2k9p4s6n8c3x5", accessConstants.CPCPersonTimelineTransferReview, true,
			"w7m2k9p4s6n8c3x5", accessConstants.CPCPartyBranchUpdate, "w7m2k9p4s6n8c3x5", 2).
		WillReturnRows(rows)
}

func TestTimelineTransferNotificationRequiresAvailableReviewer(t *testing.T) {
	tx, mock := timelineTestDB(t)
	expectTimelineReviewers(mock)
	_, err := notifyTimelineTransferReview(tx, "w7m2k9p4s6n8c3x5",
		cpcpersonModels.TimelineTransferReview{ID: 9},
		cpcpersonModels.Info{PartyBranchID: 2}, "来源模板", "目标模板")
	var failure *response.BusinessError
	if !errors.As(err, &failure) || failure.Status != http.StatusBadRequest {
		t.Fatalf("没有可审批账号时仍发起了无接收人的申请：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineTransferNotificationFailureRollsBackTransaction(t *testing.T) {
	database, mock := timelineTestDB(t)
	mock.ExpectBegin()
	expectTimelineReviewers(mock, 4, 5)
	mock.ExpectQuery("SELECT `name` FROM `workspaces`").
		WithArgs("w7m2k9p4s6n8c3x5", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("虚构工作空间"))
	for index, id := range []uint64{4, 5} {
		insert := mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `notifications`")).
			WithArgs(id, "cpc-person-timeline-transfer:w7m2k9p4s6n8c3x5:9",
				"时间线转移待审核", sqlmock.AnyArg(),
				"/cpc/roster?person_id=42&timeline_mode=review&workspace=w7m2k9p4s6n8c3x5", nil)
		if index == 0 {
			insert.WillReturnResult(sqlmock.NewResult(10, 1))
		} else {
			insert.WillReturnError(errors.New("notification write unavailable"))
		}
	}
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		_, err := notifyTimelineTransferReview(tx, "w7m2k9p4s6n8c3x5",
			cpcpersonModels.TimelineTransferReview{ID: 9},
			cpcpersonModels.Info{ID: 42, PartyBranchID: 2}, "来源模板", "目标模板")
		return err
	})
	if err == nil {
		t.Fatal("部分审批通知写入失败时未回滚业务事务")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineTransferResultsNotifyStoredRequester(t *testing.T) {
	for _, test := range []struct{ result, title, reason string }{
		{cpcpersonConstants.TimelineTransferApproved, "时间线转移审核通过", ""},
		{cpcpersonConstants.TimelineTransferRejected, "时间线转移已驳回", "材料不完整\n请补充申请材料"},
	} {
		t.Run(test.result, func(t *testing.T) {
			database, mock := timelineTestDB(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `name` FROM `workspaces`").
				WithArgs("w7m2k9p4s6n8c3x5", 1).
				WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("虚构工作空间"))
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `notifications`")).
				WithArgs(42, "cpc-person-timeline-transfer-result:w7m2k9p4s6n8c3x5:9",
					test.title, sqlmock.AnyArg(),
					"/cpc/roster?person_id=42&workspace=w7m2k9p4s6n8c3x5", nil).
				WillReturnResult(sqlmock.NewResult(10, 1))
			mock.ExpectCommit()
			var notifications []notificationModels.Notification
			err := database.Transaction(func(tx *gorm.DB) error {
				var err error
				notifications, err = notifyTimelineTransferResult(tx, "w7m2k9p4s6n8c3x5",
					cpcpersonModels.TimelineTransferReview{ID: 9, RequestedByID: 42},
					cpcpersonModels.Info{ID: 42}, "目标模板", test.result, test.reason)
				return err
			})
			if err != nil || len(notifications) != 1 || notifications[0].UserID != 42 {
				t.Fatalf("审核结果未发送给申请中保存的提交人：%v", err)
			}
			if test.result == cpcpersonConstants.TimelineTransferRejected {
				if !strings.Contains(notifications[0].Content, "审批意见："+test.reason) {
					t.Fatal("驳回结果通知未包含完整审批意见")
				}
			} else if strings.Contains(notifications[0].Content, "审批意见：") {
				t.Fatal("审核通过通知不应包含驳回审批意见")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
