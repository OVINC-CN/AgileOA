package monthlyworkSvc_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"
)

func checkWorkParticipants(
	t *testing.T,
	server *echo.Echo,
	workspaceID, otherWorkspaceID string,
	userID, ownerID, foreignUserID uint64,
	cookie, admin *http.Cookie,
	studentActions []string,
) {
	t.Helper()
	first, firstCookie := fixture(t, "fixture.participant.first", strings.Repeat("e5", 32), false)
	second, secondCookie := fixture(
		t,
		"fixture.participant.second",
		strings.Repeat("f6", 32),
		false,
	)
	disabled, _ := fixture(t, "fixture.participant.disabled", strings.Repeat("07", 32), false)
	readActions := []string{
		accessConstants.AccessWorkspace, accessConstants.MonthlyWorkList,
		accessConstants.CPCMonthlyWorkList,
	}
	grant(t, server, admin, workspaceID, first.ID, readActions)
	grant(t, server, admin, workspaceID, second.ID, []string{accessConstants.AccessWorkspace})
	grant(t, server, admin, workspaceID, disabled.ID, []string{accessConstants.AccessWorkspace})
	connection := db.DB().WithContext(t.Context())
	if err := connection.Model(&userModels.User{}).Where("id = ?", disabled.ID).
		Update("is_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	actions := append(slices.Clone(studentActions),
		accessConstants.CPCMonthlyWorkList, accessConstants.CPCMonthlyWorkCreate,
		accessConstants.CPCMonthlyWorkUpdate, accessConstants.CPCMonthlyWorkDelete,
		accessConstants.CPCMonthlyWorkImport)
	grant(t, server, admin, workspaceID, userID, actions)

	for _, resource := range []string{"monthly-work", "cpc-monthly-work"} {
		deleteAction := accessConstants.MonthlyWorkDelete
		if resource == "cpc-monthly-work" {
			deleteAction = accessConstants.CPCMonthlyWorkDelete
		}
		base := "/v1/workspaces/" + workspaceID + "/" + resource
		input := monthlyworkModels.Input{
			Date: "2022-11-03", WorkItem: "虚构多人工作", OwnerID: ownerID,
			ParticipantIDs: []uint64{second.ID, first.ID, first.ID},
		}
		work := decode[monthlyworkModels.Info](t, request(t, server, "POST", base,
			monthlyworkModels.Request{Input: input}, cookie, 200))
		if !slices.Equal(work.ParticipantIDs, []uint64{first.ID, second.ID}) ||
			len(work.Participants) != 2 || work.Participants[0].Username != first.Username {
			t.Fatal("参与人没有按编号去重排序或身份未完整返回")
		}
		checkAssignmentNotices(t, workspaceID, work, []uint64{ownerID, first.ID, second.ID})
		path := fmt.Sprintf("%s/%d", base, work.ID)
		minePath := base + "?scope=mine&year=2022"
		mine := decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", minePath+"&keyword="+first.Username, nil, firstCookie, 200))
		if mine.Total != 1 || mine.Items[0].ID != work.ID {
			t.Fatal("参与人无法在我的工作中查询或搜索对应记录")
		}
		for _, suffix := range []string{"", "&keyword=" + first.Username} {
			created := decode[monthlyworkModels.ListData](t,
				request(t, server, "GET", minePath+suffix, nil, cookie, 200))
			if created.Total != 0 {
				t.Fatal("创建人或参与人关键词绕过了我的工作范围")
			}
		}
		wrongMonth := decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", minePath+"&month=10", nil, firstCookie, 200))
		if wrongMonth.Total != 0 {
			t.Fatal("参与人查询绕过了月份筛选")
		}
		request(t, server, "GET", minePath, nil, secondCookie, 403)
		request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: work.Input, Revision: work.Revision}, firstCookie, 403)
		for _, id := range []uint64{0, foreignUserID, disabled.ID} {
			invalidInput := input
			invalidInput.ParticipantIDs = []uint64{id}
			request(t, server, "POST", base,
				monthlyworkModels.Request{Input: invalidInput}, cookie, 400)
		}
		assertForeignKey(t, connection.Create(&monthlyworkModels.Participant{
			WorkspaceID: otherWorkspaceID, MonthlyWorkID: work.ID, UserID: first.ID,
		}).Error, 1452)
		assertForeignKey(t, connection.Where("workspace_id = ? AND id = ?", workspaceID, work.ID).
			Delete(&monthlyworkModels.Record{}).Error, 1451)

		// 旧客户端省略或传空值时保留参与人，撤权后的既有指派也可保留。
		omitted := map[string]any{
			"date": work.Date, "work_item": work.WorkItem, "owner_id": work.OwnerID,
			"description": work.Description, "links": work.Links, "revision": work.Revision,
		}
		work = decode[monthlyworkModels.Info](t,
			request(t, server, "PUT", path, omitted, cookie, 200))
		grant(t, server, admin, workspaceID, first.ID, []string{})
		request(t, server, "GET", minePath, nil, firstCookie, 403)
		input = work.Input
		input.ParticipantIDs = nil
		work = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 200))
		if !slices.Equal(work.ParticipantIDs, []uint64{first.ID, second.ID}) {
			t.Fatal("省略字段或参与人撤权导致既有指派丢失")
		}
		checkAssignmentNotices(t, workspaceID, work, nil)
		checkParticipantAuditRollback(t, server, workspaceID, path, work, cookie)
		input.ParticipantIDs = []uint64{second.ID}
		work = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 200))
		input.ParticipantIDs = []uint64{first.ID, second.ID}
		request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 400)
		grant(t, server, admin, workspaceID, first.ID, readActions)
		mine = decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", minePath, nil, firstCookie, 200))
		if mine.Total != 0 {
			t.Fatal("移除参与人后我的工作仍包含对应记录")
		}

		// 负责人同时参与时只返回一条，清空参与人不会丢失本人负责的工作。
		input.OwnerID, input.ParticipantIDs = first.ID, []uint64{first.ID}
		work = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 200))
		mine = decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", minePath, nil, firstCookie, 200))
		if mine.Total != 1 || len(mine.Items) != 1 {
			t.Fatal("负责人同时参与导致我的工作重复")
		}
		checkAssignmentNotices(t, workspaceID, work, []uint64{first.ID})
		input.ParticipantIDs = []uint64{}
		request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision - 1}, cookie, 409)
		work = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 200))
		if work.ParticipantIDs == nil || len(work.ParticipantIDs) != 0 ||
			work.Participants == nil ||
			len(work.Participants) != 0 {
			t.Fatal("空数组未清空参与人或响应返回了 null")
		}
		mine = decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", minePath, nil, firstCookie, 200))
		if mine.Total != 1 {
			t.Fatal("清空参与人后丢失了本人负责的工作")
		}
		input.ParticipantIDs = []uint64{second.ID}
		work = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 200))
		request(
			t,
			server,
			"DELETE",
			fmt.Sprintf("%s?revision=%d", path, work.Revision),
			nil,
			cookie,
			200,
		)
		var count int64
		if err := connection.Model(&monthlyworkModels.Participant{}).
			Where("workspace_id = ? AND monthly_work_id = ?", workspaceID, work.ID).
			Count(&count).Error; err != nil || count != 0 {
			t.Fatal("删除工作未清理记录自有的参与人明细")
		}
		var audit auditModels.Record
		if err := connection.Where("workspace_id = ? AND instance_id = ? AND action_id = ?",
			workspaceID, fmt.Sprint(work.ID), deleteAction).
			Take(&audit).Error; err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(audit.InstanceOriginData), second.Username) {
			t.Fatal("删除工作后审计未保留参与人身份")
		}

		// 新模板支持多人账号，任一账号不可用时整批回滚。
		header := "日期,工作项目,负责人账号,详细说明,外部链接(JSON),参与人账号\n"
		row := fmt.Sprintf("2021-11-03,虚构多人导入,fixture.owner,,[],\"%s,%s\"\n",
			first.Username, second.Username)
		var noticesBefore, noticesAfter int64
		if err := connection.Model(&notificationModels.Notification{}).
			Count(&noticesBefore).Error; err != nil {
			t.Fatal(err)
		}
		request(t, server, "POST", base+"/import", monthlyworkModels.ImportRequest{
			Content: spreadsheettest.Fixture(t, header+row+
				"2021-11-04,无效参与人,fixture.owner,,[],fixture.other\n"),
		}, cookie, 400)
		if err := connection.Model(&notificationModels.Notification{}).
			Count(&noticesAfter).Error; err != nil || noticesAfter != noticesBefore {
			t.Fatal("导入失败未回滚工作分配通知")
		}
		importPath := base + "?scope=mine&year=2021"
		imported := decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", importPath, nil, firstCookie, 200))
		if imported.Total != 0 {
			t.Fatal("参与人导入失败未整批回滚")
		}
		request(t, server, "POST", base+"/import", monthlyworkModels.ImportRequest{
			Content: spreadsheettest.Fixture(t, header+row),
		}, cookie, 200)
		imported = decode[monthlyworkModels.ListData](t,
			request(t, server, "GET", importPath, nil, firstCookie, 200))
		if imported.Total != 1 || len(imported.Items[0].Participants) != 2 {
			t.Fatal("导入的多人参与工作未出现在参与人的工作列表")
		}
		checkAssignmentNotices(
			t,
			workspaceID,
			imported.Items[0],
			[]uint64{ownerID, first.ID, second.ID},
		)
	}
}

func checkAssignmentNotices(
	t *testing.T,
	workspaceID string,
	work monthlyworkModels.Info,
	want []uint64,
) {
	t.Helper()
	var notices []notificationModels.Notification
	if err := db.DB().WithContext(t.Context()).
		Where("source_id = ?", fmt.Sprintf("monthly-work:%s:%d:%d", workspaceID, work.ID, work.Revision)).
		Order("user_id ASC").Find(&notices).Error; err != nil {
		t.Fatal(err)
	}
	ids := make([]uint64, 0, len(notices))
	for _, notice := range notices {
		ids = append(ids, notice.UserID)
		if notice.Title != "工作分配通知" || !strings.Contains(notice.Content, work.WorkItem) ||
			!strings.Contains(notice.Content, work.Date) {
			t.Fatal("工作分配通知缺少工作项目或日期")
		}
	}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("工作分配通知遗漏或重复发送：%v", ids)
	}
}

func checkParticipantAuditRollback(
	t *testing.T,
	server *echo.Echo,
	workspaceID, path string,
	work monthlyworkModels.Info,
	cookie *http.Cookie,
) {
	t.Helper()
	const callback = "test:monthly-work-participant-audit-unavailable"
	if err := db.DB().
		Callback().
		Create().
		Before("gorm:create").
		Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				_ = tx.AddError(gorm.ErrInvalidData)
			}
		}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.DB().Callback().Create().Remove(callback); err != nil {
			t.Error(err)
		}
	}()
	input := work.Input
	input.ParticipantIDs = []uint64{}
	input.OwnerID = work.ParticipantIDs[len(work.ParticipantIDs)-1]
	request(t, server, "PUT", path,
		monthlyworkModels.Request{Input: input, Revision: work.Revision}, cookie, 503)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s?revision=%d", path, work.Revision),
		nil,
		cookie,
		503,
	)
	var ids []uint64
	if err := db.DB().WithContext(t.Context()).Model(&monthlyworkModels.Participant{}).
		Where("workspace_id = ? AND monthly_work_id = ?", workspaceID, work.ID).
		Order("user_id ASC").Pluck("user_id", &ids).Error; err != nil ||
		!slices.Equal(ids, work.ParticipantIDs) {
		t.Fatal("审计失败未回滚参与人的修改或删除")
	}
	var noticeCount int64
	if err := db.DB().WithContext(t.Context()).Model(&notificationModels.Notification{}).
		Where("source_id = ?", fmt.Sprintf("monthly-work:%s:%d:%d", workspaceID, work.ID, work.Revision+1)).
		Count(&noticeCount).Error; err != nil || noticeCount != 0 {
		t.Fatal("审计失败未回滚负责人重新分配的通知")
	}
}
