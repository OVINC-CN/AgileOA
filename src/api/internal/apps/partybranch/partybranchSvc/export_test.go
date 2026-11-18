package partybranchSvc

import (
	"reflect"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheettest"
)

func TestBranchExportPreservesTextZeroAndEmptyValues(t *testing.T) {
	name, username := "虚构管理员", "000001"
	items := []partybranchModels.Info{
		{
			Input: partybranchModels.Input{
				ShortName: "=虚构简称", FullName: "虚构党支部全称",
			},
			Priority: -1, SecretaryName: name, SecretaryUsername: username,
		},
		{Input: partybranchModels.Input{ShortName: "虚构简称二", FullName: "虚构全称二"}},
	}
	file, err := exportFile(items)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := spreadsheettest.Rows(file.Content)
	if err != nil {
		t.Fatal(err)
	}
	if file.Filename != "party-branches.xlsx" || file.ContentType != spreadsheet.ContentType ||
		!reflect.DeepEqual(rows, [][]string{
			{"优先级", "党支部简称", "党支部全称", "管理员姓名", "管理员账号"},
			{"-1", "=虚构简称", "虚构党支部全称", "虚构管理员", "000001"},
			{"0", "虚构简称二", "虚构全称二", "", ""},
		}) {
		t.Fatal("党支部导出丢失文本、零或空值，或文件契约错误")
	}
	empty, err := exportFile(nil)
	if err != nil {
		t.Fatal(err)
	}
	emptyRows, err := spreadsheettest.Rows(empty.Content)
	if err != nil || len(emptyRows) != 1 || !reflect.DeepEqual(emptyRows[0], rows[0]) {
		t.Fatal("无匹配记录时未保留完整表头")
	}
}

func TestBranchExportRejectsWholeFileBeyondLimits(t *testing.T) {
	limits := config.Config.PartyBranch
	t.Cleanup(func() { config.Config.PartyBranch = limits })
	items := []partybranchModels.Info{{}, {}}
	config.Config.PartyBranch.MaxExportRows = len(items)
	config.Config.PartyBranch.MaxFileBytes = 1
	if result, err := exportFile(items); err == nil || result.Content != "" {
		t.Fatal("超过文件大小上限时未拒绝整个文件")
	}
	config.Config.PartyBranch.MaxFileBytes = limits.MaxFileBytes
	config.Config.PartyBranch.MaxExportRows = len(items) - 1
	if result, err := exportFile(items); err == nil || result.Content != "" {
		t.Fatal("超过行数上限时返回了截断数据")
	}
}
