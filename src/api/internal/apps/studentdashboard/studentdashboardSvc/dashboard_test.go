package studentdashboardSvc

import (
	"reflect"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardModels"
)

func TestSummaryGroupsClassesAndMajorsWithoutLosingHistoricalEmptyValues(t *testing.T) {
	t.Parallel()
	data := summarize([]majorCount{
		{ClassName: "二班", Major: "审计学", Count: 3},
		{ClassName: " 一班 ", Major: " 会计学 ", Count: 2},
		{ClassName: "一班", Major: "会计学", Count: 1},
		{ClassName: "一班", Major: "审计学", Count: 1},
		{ClassName: "  ", Major: "", Count: 1},
	}, nil)
	if !reflect.DeepEqual(data.Majors, []string{"", "会计学", "审计学"}) {
		t.Fatalf("专业列未去重或排序：%v", data.Majors)
	}
	if len(data.Items) != 3 || data.Items[0].ClassName != "" ||
		data.Items[1].ClassName != "一班" || data.Items[2].ClassName != "二班" {
		t.Fatalf("班级顺序或历史空班级错误：%+v", data.Items)
	}
	if data.Items[1].MajorCounts["会计学"] != 3 || data.Items[1].Total != 4 ||
		data.Items[2].MajorCounts["会计学"] != 0 {
		t.Fatalf("跨专业班级人数错误：%+v", data.Items)
	}
	want := studentdashboardModels.Counts{
		MajorCounts: map[string]int64{"": 1, "会计学": 3, "审计学": 4},
		Total:       8,
	}
	if !reflect.DeepEqual(data.Totals, want) {
		t.Fatalf("各专业合计与总人数不一致：%+v", data.Totals)
	}
	empty := summarize(nil, nil)
	if empty.Items == nil || empty.Majors == nil || empty.Totals.MajorCounts == nil ||
		empty.Totals.Total != 0 {
		t.Fatal("空仪表必须返回空数组、空人数映射及零总数")
	}
}

func TestMonitorsExcludeDeputiesAndDeduplicateCombinedPositions(t *testing.T) {
	t.Parallel()
	data := summarize(
		[]majorCount{{ClassName: "一班", Major: "会计学", Count: 6}},
		[]monitorName{
			{ClassName: " 一班 ", ClassPosition: "班长", Name: " 虚构学生甲 "},
			{ClassName: "一班", ClassPosition: "班长兼学习委员", Name: "虚构学生甲"},
			{ClassName: "一班", ClassPosition: "学习委员、班长", Name: "虚构学生乙"},
			{ClassName: "一班", ClassPosition: "副班长", Name: "虚构副职"},
			{ClassName: "一班", ClassPosition: "班长助理", Name: "虚构助理"},
			{ClassName: "一班", ClassPosition: "班长", Name: "  "},
			{ClassName: "其他班级", ClassPosition: "班长", Name: "范围外姓名"},
		},
	)
	if data.Items[0].Monitors != "虚构学生乙,虚构学生甲" {
		t.Fatalf("班长识别、班级隔离或姓名去重错误：%q", data.Items[0].Monitors)
	}
}

func TestExportKeepsClassFirstTotalAndMonitorsLastWithCompleteTotals(t *testing.T) {
	t.Parallel()
	data := summarize(
		[]majorCount{
			{ClassName: "一班", Major: "会计学", Count: 2},
			{ClassName: "二班", Major: "审计学", Count: 1},
		},
		[]monitorName{{ClassName: "一班", ClassPosition: "班长", Name: "虚构班长"}},
	)
	want := [][]string{
		{"班级", "会计学", "审计学", "总数", "班长"},
		{"一班", "2", "0", "2", "虚构班长"},
		{"二班", "0", "1", "1", ""},
		{"合计", "2", "1", "3", ""},
	}
	if records := exportRecords(data); !reflect.DeepEqual(records, want) {
		t.Fatalf("导出列顺序、零值、班长或合计错误：%v", records)
	}
	wantEmpty := [][]string{{"班级", "总数", "班长"}, {"合计", "0", ""}}
	if records := exportRecords(summarize(nil, nil)); !reflect.DeepEqual(records, wantEmpty) {
		t.Fatalf("空导出未保留表头及零总数：%v", records)
	}
}
