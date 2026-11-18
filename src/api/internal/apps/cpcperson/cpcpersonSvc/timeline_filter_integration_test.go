package cpcpersonSvc_test

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
)

func assertTimelineFieldFilters(
	t *testing.T, server *echo.Echo, first, manager fixtureActor,
	base, otherWorkspace string, nodeID uint64,
) {
	t.Helper()

	// 字段目录沿用名单权限，不要求模板管理权限且不跨空间提供元数据。
	fields := personRequest[[]cpcpersonModels.TimelineFilterField](
		t, server, first, "GET", base+"/timeline-fields", nil, 200,
	)
	if len(fields) != 1 || fields[0].NodeID != nodeID || fields[0].FieldID != "occurred_at" {
		t.Fatal("未按可读人员绑定模板提供时间线字段目录")
	}
	if !strings.Contains(fields[0].Label, " / 1-虚构节点 / ") {
		t.Fatal("时间线筛选字段名称未使用模板内顺序号")
	}
	otherFields := personRequest[[]cpcpersonModels.TimelineFilterField](
		t, server, manager, "GET",
		"/v1/workspaces/"+otherWorkspace+"/cpc-people/timeline-fields", nil, 200,
	)
	if len(otherFields) != 0 {
		t.Fatal("时间线字段目录泄露了其他空间模板")
	}
	query := func(operator, value string) string {
		condition := cpcpersonModels.TimelineFilter{
			NodeID: nodeID, FieldID: "occurred_at", Operator: operator, Value: value,
		}
		encoded, err := json.Marshal(condition)
		if err != nil {
			t.Fatal(err)
		}
		return "timeline_filter=" + url.QueryEscape(string(encoded))
	}

	// 日期上下限包含边界，已保存的待处理字段也参与筛选，统计和导出保持一致。
	for _, test := range []struct {
		query string
		total int64
	}{
		{query("gte", "2026-11-06") + "&" + query("lte", "2026-11-06"), 1},
		{query("gt", "2026-11-06"), 0},
		{query("empty", ""), 0},
		{query("not_empty", ""), 1},
	} {
		listed := personRequest[cpcpersonModels.ListData](
			t, server, first, "GET", base+"?"+test.query, nil, 200,
		)
		stats := personRequest[cpcpersonModels.StatisticsData](
			t, server, first, "GET", base+"/statistics?"+test.query, nil, 200,
		)
		if listed.Total != test.total || stats.Total != test.total {
			t.Fatal("时间线字段筛选未同时应用于名单及统计")
		}
		for _, format := range []string{"basic", "full"} {
			file := personRequest[cpcpersonModels.FileData](
				t, server, first, "GET", base+"/export?format="+format+"&"+test.query, nil, 200,
			)
			rows, err := spreadsheettest.Rows(file.Content)
			if err != nil || int64(len(rows)-1) != test.total {
				t.Fatal("时间线字段筛选未应用于人员导出")
			}
		}
	}

	// 未填写人员只匹配空值条件，不能混入有日期的范围结果。
	all := personRequest[cpcpersonModels.ListData](
		t, server, manager, "GET", base+"?"+query("empty", ""), nil, 200,
	)
	if all.Total != 1 {
		t.Fatal("空值条件未匹配同模板内尚未填写的人员")
	}
	personRequest[any](t, server, first, "GET", base+"?"+query("contains", "2026"), nil, 400)
	personRequest[any](t, server, first, "GET", base+"?"+query("eq", "2026-02-30"), nil, 400)
	personRequest[any](t, server, manager, "GET",
		"/v1/workspaces/"+otherWorkspace+"/cpc-people?"+query("empty", ""), nil, 400)
}
