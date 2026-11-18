package cpcpersonSvc_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"
)

func assertPersonTimelineView(
	t *testing.T,
	server *echo.Echo,
	tx *gorm.DB,
	first, manager fixtureActor,
	workspaceID, otherWorkspace string,
	person cpcpersonModels.Info,
	data cpcpersonModels.TimelineData,
) {
	t.Helper()
	base := "/v1/workspaces/" + workspaceID + "/cpc-people"
	viewPath := fmt.Sprintf("%s/timeline-view/%d", base, *data.TemplateID)
	nodePath := fmt.Sprintf("%s/nodes/%d", viewPath, data.Nodes[0].NumericID)

	// 列表权限提供可读人员实际绑定的模板与目录，不要求模板管理权限。
	options := personRequest[[]cpcpersonModels.Option](
		t, server, first, "GET", base+"/timeline-templates/list", nil, 200,
	)
	if len(options) != 1 || options[0].Label != data.TemplateName {
		t.Fatal("时间线视图未按人员列表权限提供已绑定模板")
	}
	view := personRequest[cpcpersonModels.TimelineViewData](
		t, server, first, "GET", viewPath, nil, 200,
	)
	if len(view.Nodes) != 1 || view.Nodes[0].NumericID != data.Nodes[0].NumericID {
		t.Fatal("时间线视图未返回模板节点的完整有序目录")
	}
	list := personRequest[cpcpersonModels.TimelineViewListData](
		t, server, first, "GET", nodePath, nil, 200,
	)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].Person.ID != person.ID ||
		list.Items[0].Status != "completed" || list.Items[0].StatusLabel != "已完成" ||
		list.Items[0].Values["occurred_at"] != "2026-11-06" {
		t.Fatal("时间线视图未按书记范围提供独立节点状态和字段值")
	}

	// 全范围统计不受分页影响，普通条件和字段条件同时应用于名单和统计。
	all := personRequest[cpcpersonModels.TimelineViewListData](
		t, server, manager, "GET", nodePath+"?page_size=1", nil, 200,
	)
	stats := personRequest[cpcpersonModels.TimelineViewStatisticsData](
		t, server, manager, "GET", nodePath+"/statistics?page_size=1", nil, 200,
	)
	older := personRequest[cpcpersonModels.TimelineViewListData](
		t, server, manager, "GET", nodePath+"?page=2&page_size=1", nil, 200,
	)
	if all.Total != 2 || len(all.Items) != 1 || len(older.Items) != 1 ||
		all.Items[0].Person.ID <= older.Items[0].Person.ID {
		t.Fatal("时间线节点名单未在分页前按人员编号倒序排列")
	}
	if stats.Total != 2 || stats.Completed != 1 || stats.Pending != 1 || stats.Skipped != 0 {
		t.Fatal("时间线节点统计未沿用读取范围，或错误按分页计算人数")
	}

	// 节点状态多选取并集，列表、统计和导出始终限定同一模板和节点。
	for _, test := range []struct {
		statuses []string
		total    int64
	}{
		{[]string{"pending"}, 1},
		{[]string{"skipped"}, 0},
		{[]string{"completed"}, 1},
		{[]string{"pending", "completed"}, 2},
		{[]string{"pending", "skipped"}, 1},
	} {
		params := url.Values{"node_status": test.statuses}
		filtered := personRequest[cpcpersonModels.TimelineViewListData](
			t, server, manager, "GET", nodePath+"?"+params.Encode(), nil, 200,
		)
		counts := personRequest[cpcpersonModels.TimelineViewStatisticsData](
			t, server, manager, "GET", nodePath+"/statistics?"+params.Encode(), nil, 200,
		)
		params.Set("timeline_template_id", fmt.Sprint(*data.TemplateID))
		params.Set("timeline_node_id", fmt.Sprint(data.Nodes[0].NumericID))
		file := personRequest[cpcpersonModels.FileData](
			t, server, manager, "GET", base+"/export?"+params.Encode(), nil, 200,
		)
		rows, err := spreadsheettest.Rows(file.Content)
		if err != nil {
			t.Fatal(err)
		}
		if filtered.Total != test.total || counts.Total != test.total ||
			int64(len(rows)-1) != test.total {
			t.Fatal("节点状态未同时应用于名单、统计与导出")
		}
	}
	personRequest[any](t, server, manager, "GET", nodePath+"?node_status=abnormal", nil, 400)
	personRequest[any](t, server, manager, "GET", base+"/export?node_status=pending", nil, 400)
	condition := fmt.Sprintf(
		`{"node_id":%d,"field_id":"occurred_at","operator":"eq","value":"2026-11-06"}`,
		data.Nodes[0].NumericID,
	)
	query := "keyword=" + url.QueryEscape(person.StudentNo) +
		"&status=" + url.QueryEscape(person.Status) +
		"&timeline_filter=" + url.QueryEscape(condition)
	filtered := personRequest[cpcpersonModels.TimelineViewListData](
		t, server, manager, "GET", nodePath+"?"+query, nil, 200,
	)
	stats = personRequest[cpcpersonModels.TimelineViewStatisticsData](
		t, server, manager, "GET", nodePath+"/statistics?"+query, nil, 200,
	)
	if filtered.Total != 1 || stats.Total != 1 || stats.Completed != 1 {
		t.Fatal("时间线节点名单和统计未完整复用人员筛选条件")
	}
	filtered = personRequest[cpcpersonModels.TimelineViewListData](
		t, server, first, "GET", nodePath+"?keyword=fixture.missing", nil, 200,
	)
	if filtered.Total != 0 || len(filtered.Items) != 0 {
		t.Fatal("时间线节点空筛选不应保留旧人员结果")
	}

	// 统计独立授权，撤销统计权限后仍可读取节点名单。
	if err := tx.Where("workspace_id = ? AND user_id = ? AND action = ?",
		workspaceID, first.user.ID, accessConstants.CPCPersonStatistics).
		Delete(&accessModels.Permission{}).Error; err != nil {
		t.Fatal(err)
	}
	personRequest[any](t, server, first, "GET", nodePath+"/statistics", nil, 403)
	personRequest[cpcpersonModels.TimelineViewListData](
		t, server, first, "GET", nodePath, nil, 200,
	)
	grantPersonActions(t, tx, first, workspaceID, accessConstants.CPCPersonStatistics)

	// 其他空间不能读取模板定义或节点填写值。
	otherBase := "/v1/workspaces/" + otherWorkspace + "/cpc-people"
	otherOptions := personRequest[[]cpcpersonModels.Option](
		t, server, manager, "GET", otherBase+"/timeline-templates/list", nil, 200,
	)
	if len(otherOptions) != 0 {
		t.Fatal("时间线模板选项泄露了其他空间的绑定")
	}
	otherView := fmt.Sprintf("%s/timeline-view/%d", otherBase, *data.TemplateID)
	personRequest[any](t, server, manager, "GET", otherView, nil, 404)
	personRequest[any](t, server, manager, "GET",
		fmt.Sprintf("%s/nodes/%d", otherView, data.Nodes[0].NumericID), nil, 404)
}
