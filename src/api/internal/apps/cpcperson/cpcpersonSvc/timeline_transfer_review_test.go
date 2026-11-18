package cpcpersonSvc

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestTransferringTimelineRejectsNodeWritesBeforeLoadingNodes(t *testing.T) {
	tx, mock := timelineTestDB(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("SELECT .* FROM cpc_people AS p").
		WithArgs(workspaceID, 7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "party_branch_id", "timeline_template_id", "timeline_status",
			"timeline_transfer_target_id", "revision",
		}).AddRow(7, 1, 2, "transferring", 3, 4))
	_, _, err := loadTimelineNodeForUpdate(tx, personAccess{actions: []string{
		accessConstants.CPCPersonUpdate, accessConstants.CPCPartyBranchUpdate,
	}}, workspaceID, 7, "node_first", 1, 1)
	var failure *response.BusinessError
	if !errors.As(err, &failure) || failure.Status != http.StatusConflict {
		t.Fatalf("转移中仍可写入节点：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineRefreshPreservesPendingTransfer(t *testing.T) {
	tx, mock := timelineTestDB(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("SELECT .* FROM `cpc_people`").
		WithArgs(workspaceID, 7, 2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "timeline_status"}).
			AddRow(7, "transferring"))
	if err := cpcpersonUtils.RefreshTimelineStatus(
		tx, workspaceID, accessConstants.StudentUpdate, 7, 2, nil,
	); err != nil {
		t.Fatalf("引用重算覆盖了待审核状态：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimelineTransferReviewRemovalIsTenantScopedAndAudited(t *testing.T) {
	tx, mock := timelineTestDB(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_transfer_reviews`").
		WithArgs(workspaceID, 7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"workspace_id",
			"person_id",
			"source_template_id",
			"target_template_id",
			"requested_by_id",
		}).AddRow(9, workspaceID, 7, 2, 3, 42))
	mock.ExpectExec("DELETE FROM `cpc_person_timeline_transfer_reviews` WHERE workspace_id = \\? AND id = \\?").
		WithArgs(workspaceID, 9).
		WillReturnResult(sqlmock.NewResult(0, 1))
	audit := event(workspaceID, 7)
	if _, err := removeTimelineTransferReview(tx, workspaceID, 7, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.Conditions["transfer_review_id"] != uint64(9) ||
		audit.Conditions["target_template_id"] != uint64(3) ||
		audit.Conditions["requested_by_id"] != uint64(42) {
		t.Fatal("移除审核记录前未保存审核编号和目标")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransferCancellationRestoresActualStatus(t *testing.T) {
	data := cpcpersonModels.TimelineData{Status: cpcpersonUtils.TimelineTransferring}
	if timelineActualStatus(data) != cpcpersonUtils.TimelineNormal {
		t.Fatal("无异常时间线取消转移后未恢复正常")
	}
	data.Nodes = []cpcpersonModels.TimelineNodeInfo{{
		Issues: []cpcpersonModels.TimelineIssue{{RuleID: "rule_first"}},
	}}
	if timelineActualStatus(data) != cpcpersonUtils.TimelineAbnormal {
		t.Fatal("转移期间引用变化产生的异常在取消时被隐藏")
	}
	status := cpcpersonUtils.TimelineTransferring
	if cpcpersonUtils.RequireTimelineWritable(&status) == nil {
		t.Fatal("转移中未阻止修改人员和附件")
	}
}
