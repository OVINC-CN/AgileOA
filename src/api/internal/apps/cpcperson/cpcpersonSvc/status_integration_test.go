package cpcpersonSvc_test

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
)

func assertPersonStatuses(
	t *testing.T, server *echo.Echo, actor fixtureActor, base string, personID uint64,
) {
	t.Helper()

	// 只修改状态也必须持久化，旧版本不能覆盖新状态。
	path := fmt.Sprintf("%s/%d", base, personID)
	before := personRequest[cpcpersonModels.Info](t, server, actor, "GET", path, nil, 200)
	request := cpcpersonModels.UpdateRequest{
		StudentID: before.StudentID, Stage: before.Stage, Status: "转出",
		PartyPosition: before.PartyPosition, Revision: before.Revision,
	}
	after := personRequest[cpcpersonModels.Info](t, server, actor, "PUT", path, request, 200)
	if after.Status != "转出" || after.Revision != before.Revision+1 ||
		after.StudentStatus != before.StudentStatus {
		t.Fatal("状态单独修改未生效，或覆盖了学生状态")
	}
	personRequest[any](t, server, actor, "PUT", path, request, 409)
	request.Status, request.Revision = "", after.Revision
	preserved := personRequest[cpcpersonModels.Info](t, server, actor, "PUT", path, request, 200)
	if preserved.Status != after.Status || preserved.Revision != after.Revision {
		t.Fatal("旧客户端省略状态时覆盖原值或增加了无变更版本")
	}

	// 新状态按当前支部范围去重枚举，不能混入其他支部的默认状态。
	options := personRequest[[]cpcpersonModels.Option](
		t, server, actor, "GET", base+"/options?field=status", nil, 200,
	)
	if len(options) != 1 || options[0].Value != "转出" || options[0].Label != "转出" {
		t.Fatal("自定义状态未枚举或泄露了其他支部选项")
	}

	// 名单、统计与两类导出共用人员状态，清空及多选遵循同一规则。
	for _, test := range []struct {
		query string
		total int64
	}{
		{"", 1},
		{"?status=" + url.QueryEscape("在编"), 0},
		{"?status=" + url.QueryEscape("转出"), 1},
		{"?status=" + url.QueryEscape("在编") + "&status=" + url.QueryEscape("转出"), 1},
	} {
		listed := personRequest[cpcpersonModels.ListData](
			t, server, actor, "GET", base+test.query, nil, 200,
		)
		statistics := personRequest[cpcpersonModels.StatisticsData](
			t, server, actor, "GET", base+"/statistics"+test.query, nil, 200,
		)
		if listed.Total != test.total || statistics.Total != test.total {
			t.Fatal("人员状态筛选未同时应用于名单及统计")
		}
		for _, format := range []string{"basic", "full"} {
			query := strings.TrimPrefix(test.query, "?")
			file := personRequest[cpcpersonModels.FileData](
				t, server, actor, "GET", base+"/export?format="+format+"&"+query, nil, 200,
			)
			if strings.Contains(
				spreadsheettest.Text(t, file.Content),
				before.StudentNo,
			) != (test.total == 1) ||
				!strings.Contains(spreadsheettest.Text(t, file.Content), "发展阶段,状态,") {
				t.Fatal("导出缺少独立状态列或未使用相同状态筛选")
			}
			rows, err := spreadsheettest.Rows(file.Content)
			if err != nil {
				t.Fatal(err)
			}
			branchColumn := slices.Index(rows[0], "所属党支部")
			shortNameColumn := slices.Index(rows[0], "党支部简称")
			if branchColumn < 0 || shortNameColumn < 0 ||
				(test.total == 1 && (rows[1][branchColumn] != before.PartyBranchName ||
					rows[1][shortNameColumn] != before.PartyBranchShortName)) {
				t.Fatal("基础或完整导出缺少正确的党支部全称及简称")
			}
		}
	}
}
