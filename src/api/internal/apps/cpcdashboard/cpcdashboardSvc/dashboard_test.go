package cpcdashboardSvc

import (
	"reflect"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcdashboard/cpcdashboardModels"
)

func TestSummaryPreservesEmptyBranchesAndOnlyCountsMembers(t *testing.T) {
	t.Parallel()
	data := summarize(
		[]branch{{ID: 3, ShortName: "虚构空支部"}, {ID: 1, ShortName: "虚构支部"}},
		[]stageCount{
			{PartyBranchID: 1, Stage: "full_member", Count: 2},
			{PartyBranchID: 1, Stage: "probationary_member", Count: 1},
			{PartyBranchID: 1, Stage: "activist", Count: 4},
			{PartyBranchID: 99, Stage: "full_member", Count: 100},
		},
		nil,
		nil,
	)
	if len(data.Items) != 2 || data.Items[0].PartyBranchID != 3 || data.Items[0].Total != 0 {
		t.Fatalf("空支部或支部排序未保留：%+v", data.Items)
	}
	want := cpcdashboardModels.Counts{FullMembers: 2, ProbationaryMembers: 1, Total: 3}
	if data.Items[1].Counts != want || data.Totals != want {
		t.Fatalf("合计包含其他发展阶段或范围外支部：%+v", data)
	}
	if empty := summarize(nil, nil, nil, nil); empty.Items == nil || empty.Totals.Total != 0 {
		t.Fatal("空仪表必须返回空数组及零合计")
	}
}

func TestSummaryDeduplicatesNamesAndSeparatesSecretaryFromDeputy(t *testing.T) {
	t.Parallel()
	data := summarize(
		[]branch{{ID: 1, ShortName: "虚构支部"}},
		nil,
		[]personName{
			{PartyBranchID: 1, PartyPosition: "支部书记", Name: " 虚构学生甲 "},
			{PartyBranchID: 1, PartyPosition: "书记", Name: "虚构学生甲"},
			{PartyBranchID: 1, PartyPosition: "副书记", Name: "虚构副职"},
			{PartyBranchID: 1, PartyPosition: "组织委员、宣传委员兼纪律委员", Name: "虚构学生乙"},
			{PartyBranchID: 1, PartyPosition: "纪检委员", Name: "虚构学生乙"},
			{PartyBranchID: 99, PartyPosition: "书记", Name: "范围外学生"},
		},
		[]advisorName{
			{PartyBranchID: 1, Advisor: "虚构导师甲"},
			{PartyBranchID: 1, Advisor: " 虚构导师乙 "},
			{PartyBranchID: 1, Advisor: "虚构导师甲"},
			{PartyBranchID: 1, Advisor: "   "},
			{PartyBranchID: 99, Advisor: "范围外导师"},
		},
	)
	item := data.Items[0]
	if item.Secretaries != "虚构学生甲" || item.OrganizationMembers != "虚构学生乙" ||
		item.PublicityMembers != "虚构学生乙" || item.DisciplineMembers != "虚构学生乙" {
		t.Fatalf("支委职务识别或姓名去重错误：%+v", item)
	}
	if item.Advisors != "虚构导师乙,虚构导师甲" {
		t.Fatalf("导师姓名未去重、排序或使用英文逗号：%q", item.Advisors)
	}
}

func TestExportKeepsAdvisorLastAndIncludesTotals(t *testing.T) {
	t.Parallel()
	data := cpcdashboardModels.Data{
		Items: []cpcdashboardModels.BranchSummary{{
			PartyBranchID: 1, PartyBranchName: "虚构支部", Advisors: "导师甲,导师乙",
			Counts: cpcdashboardModels.Counts{FullMembers: 2, ProbationaryMembers: 0, Total: 2},
		}},
		Totals: cpcdashboardModels.Counts{FullMembers: 2, ProbationaryMembers: 0, Total: 2},
	}
	records := exportRecords(data)
	if len(records) != 3 || records[0][8] != "导师姓名" || records[1][8] != "导师甲,导师乙" {
		t.Fatalf("导出列顺序或导师姓名被改变：%v", records)
	}
	if !reflect.DeepEqual(records[2], []string{"合计", "2", "0", "2", "", "", "", "", ""}) {
		t.Fatalf("导出未保留零或人数合计：%v", records[2])
	}
}
