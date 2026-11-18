package partybranchSvc

import (
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestBranchTextAndSecretaryValidation(t *testing.T) {
	t.Parallel()
	input := partybranchModels.Input{
		FullName: strings.Repeat("中", 200), ShortName: "虚构党支部",
	}
	if _, err := clean(input); err != nil {
		t.Fatal("有效中文边界长度及未配置书记被拒绝")
	}
	for _, change := range []func(*partybranchModels.Input){
		func(value *partybranchModels.Input) { value.FullName += "中" },
		func(value *partybranchModels.Input) { value.FullName = " " },
		func(value *partybranchModels.Input) { value.ShortName = strings.Repeat("中", 101) },
		func(value *partybranchModels.Input) { value.ShortName = "虚构\x00党支部" },
		func(value *partybranchModels.Input) { value.SecretaryID = new(uint64) },
	} {
		changed := input
		change(&changed)
		if _, err := clean(changed); err == nil {
			t.Fatal("接受了无效文本或零值书记编号")
		}
	}
}

func TestXLSXRejectsArbitraryColumnsAndImportLimits(t *testing.T) {
	content := "\ufeff党支部全称,党支部简称,党支部书记账号\n虚构全称,虚构简称,\n"
	rows, err := workbookRows(spreadsheettest.Fixture(t, content))
	if err != nil || len(rows) != 1 || len(rows[0]) != 4 || rows[0][2] != "" || rows[0][3] != "" {
		t.Fatal("带 BOM 且未配置书记的合法模板被拒绝")
	}
	priorityRows, err := workbookRows(
		spreadsheettest.Fixture(t, "党支部全称,党支部简称,党支部书记账号,优先级\n虚构全称,虚构简称,,100\n"),
	)
	if err != nil || len(priorityRows) != 1 || priorityRows[0][3] != "100" {
		t.Fatal("新模板优先级列丢失")
	}
	for _, changed := range []string{
		strings.Replace(content, "党支部全称", "id", 1),
		"党支部全称,党支部简称,党支部书记账号\n",
		strings.Replace(content, "虚构简称,", "虚构简称,,unexpected", 1),
		"党支部全称,党支部简称,专业,党支部书记账号\n虚构全称,虚构简称,会计,\n",
	} {
		if _, err := workbookRows(spreadsheettest.Fixture(t, changed)); err == nil {
			t.Fatal("任意字段、空导入或错误列数未被拒绝")
		}
	}
	limits := config.Config.PartyBranch
	t.Cleanup(func() { config.Config.PartyBranch = limits })
	config.Config.PartyBranch.MaxImportRows = 1
	if _, err := workbookRows(spreadsheettest.Fixture(t, content+"虚构全称二,虚构简称二,\n")); err == nil {
		t.Fatal("超过配置导入行数的请求未被拒绝")
	}
	config.Config.PartyBranch.MaxFileBytes = len(content) - 1
	if _, err := workbookRows(spreadsheettest.Fixture(t, content)); err == nil {
		t.Fatal("超过配置字节数的请求未被拒绝")
	}
}
