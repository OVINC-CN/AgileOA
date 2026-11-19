package monthlyworkSvc_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
)

func checkWorkCategoryBoundaries(
	t *testing.T,
	server *echo.Echo,
	workspaceID string,
	userID, otherOwnerID uint64,
	cookie, admin *http.Cookie,
	input monthlyworkModels.Input,
	studentActions []string,
) {
	t.Helper()
	base := "/v1/workspaces/" + workspaceID
	cpcPath := base + "/cpc-monthly-work"

	// 学生工作权限和管理员身份均不隐含党建权限。
	request(t, server, "GET", cpcPath+"?year=2024", nil, cookie, 403)
	request(t, server, "POST", cpcPath, monthlyworkModels.Request{Input: input}, cookie, 403)
	request(t, server, "GET", cpcPath+"?year=2024", nil, admin, 403)
	cpcActions := []string{
		accessConstants.AccessWorkspace,
		accessConstants.CPCMonthlyWorkList,
		accessConstants.CPCMonthlyWorkCreate,
		accessConstants.CPCMonthlyWorkUpdate,
		accessConstants.CPCMonthlyWorkDelete,
		accessConstants.CPCMonthlyWorkImport,
		accessConstants.CPCMonthlyWorkAttachmentRead,
		accessConstants.CPCMonthlyWorkAttachmentUpload,
		accessConstants.CPCMonthlyWorkAttachmentDelete,
	}
	grant(t, server, admin, workspaceID, userID, cpcActions)
	request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 403)
	request(t, server, "GET", base+"/attachments/monthly-work", nil, cookie, 403)
	allActions := append(append([]string{}, studentActions...), cpcActions[1:]...)
	grant(t, server, admin, workspaceID, userID, allActions)

	// 同表保存不同类别，不允许通过另一类接口修改或删除。
	input.Date, input.WorkItem = "2024-11-03", "虚构党建月度工作"
	cpc := decode[monthlyworkModels.Info](t, request(
		t, server, "POST", cpcPath, monthlyworkModels.Request{Input: input}, cookie, 200,
	))
	connection := db.DB().WithContext(t.Context())
	var persisted monthlyworkModels.Record
	if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, cpc.ID).
		Take(&persisted).
		Error; err != nil || persisted.Category != monthlyworkConstants.CategoryCPC {
		t.Fatal("党建工作未在共用表中保存固定类别")
	}
	student := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 200)).Items[0]
	for _, target := range []struct {
		Path string
		Item monthlyworkModels.Info
	}{
		{Path: base + "/monthly-work", Item: cpc},
		{Path: cpcPath, Item: student},
	} {
		path := fmt.Sprintf("%s/%d", target.Path, target.Item.ID)
		request(t, server, "PUT", path,
			monthlyworkModels.Request{Input: input, Revision: target.Item.Revision}, cookie, 404)
		request(t, server, "DELETE", path+fmt.Sprintf("?revision=%d", target.Item.Revision),
			nil, cookie, 404)
	}
	studentList := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2024", nil, cookie, 200))
	cpcList := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", cpcPath+"?year=2024", nil, cookie, 200))
	if studentList.Total != 0 || cpcList.Total != 1 || cpcList.Items[0].ID != cpc.ID {
		t.Fatal("两类月度工作列表未按类别隔离")
	}
	filters := decode[monthlyworkModels.FilterOptionsData](t,
		request(t, server, "GET", cpcPath+"/filter-options", nil, cookie, 200))
	if len(filters.Years) != 1 || filters.Years[0].Value != "2024" {
		t.Fatal("党建年份枚举包含学生工作年份")
	}

	// 导入继承路径类别，模板仍保持原有的固定字段。
	request(t, server, "GET", cpcPath+"/template", nil, cookie, 200)
	request(t, server, "POST", cpcPath+"/import", monthlyworkModels.ImportRequest{
		Content: spreadsheettest.Fixture(t, "日期,工作项目,负责人账号,详细说明,外部链接(JSON)\n"+
			"2023-11-03,虚构党建导入,fixture.work,,[]\n"),
	}, cookie, 200)
	studentList = decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2023", nil, cookie, 200))
	cpcList = decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", cpcPath+"?year=2023", nil, cookie, 200))
	if studentList.Total != 0 || cpcList.Total != 1 {
		t.Fatal("党建导入的记录混入学生工作")
	}

	// 两类附件共用原表，只能从所属类别的目录和工作列表读取。
	files := make([]attachmentModels.MonthlyWorkAttachment, 0, 2)
	for index, item := range []monthlyworkModels.Info{student, cpc} {
		files = append(files, attachmentModels.MonthlyWorkAttachment{
			MonthlyWorkID: item.ID,
			Record: attachmentModels.Record{
				WorkspaceID: workspaceID,
				ObjectHash:  strings.Repeat(fmt.Sprint(index+3), 64),
				ObjectKey:   fmt.Sprintf("fictional/category/%d", index), Filename: "虚构类别附件.pdf",
				Size: 1, State: attachmentConstants.Ready, Revision: 1,
			},
		})
	}
	if err := connection.Create(&files).Error; err != nil {
		t.Fatal(err)
	}
	for index, resource := range []string{"monthly-work", "cpc-monthly-work"} {
		attachments := decode[attachmentModels.ListData](t,
			request(t, server, "GET", base+"/attachments/"+resource, nil, cookie, 200))
		if attachments.Total != 1 || attachments.Items[0].ID != files[index].ID {
			t.Fatal("附件目录越过工作类别边界")
		}
		wrongOwner := files[1-index].MonthlyWorkID
		foreignFiles := decode[attachmentModels.ListData](t, request(t, server, "GET",
			fmt.Sprintf("%s/attachments/%s?monthly_work_id=%d", base, resource, wrongOwner),
			nil, cookie, 200))
		if foreignFiles.Total != 0 {
			t.Fatal("指定工作编号绕过了附件类别隔离")
		}

		// 旧范围参数不能扩大可见范围，年份选项和附件同样过滤他人工作。
		item := []monthlyworkModels.Info{student, cpc}[index]
		workPath := base + "/" + resource
		hiddenInput := item.Input
		hiddenInput.OwnerID, hiddenInput.Date = otherOwnerID, "1902-11-03"
		request(t, server, "POST", workPath,
			monthlyworkModels.Request{Input: hiddenInput}, cookie, 200)
		for _, query := range []string{"", "&scope=all", "&scope=mine&scope=all"} {
			hidden := decode[monthlyworkModels.ListData](t,
				request(t, server, "GET", workPath+"?year=1902"+query, nil, cookie, 200))
			if hidden.Total != 0 || len(hidden.Items) != 0 {
				t.Fatal("列表包含了本人录入但由他人负责的记录")
			}
		}
		years := decode[monthlyworkModels.FilterOptionsData](t,
			request(t, server, "GET", workPath+"/filter-options", nil, cookie, 200))
		for _, year := range years.Years {
			if year.Value == "1902" {
				t.Fatal("年份选项包含仅他人可见的工作年份")
			}
		}
		ownInput := item.Input
		ownInput.OwnerID, ownInput.Date = userID, "1901-11-03"
		own := decode[monthlyworkModels.Info](t, request(t, server, "POST", workPath,
			monthlyworkModels.Request{Input: ownInput}, cookie, 200))
		multiple := decode[monthlyworkModels.ListData](t, request(t, server, "GET",
			workPath+"?year=1901&year=1902&month=1&month=11&keyword=虚构", nil, cookie, 200))
		if multiple.Total != 1 || multiple.Items[0].ID != own.ID {
			t.Fatal("年份和月份多选未取并集，或越过工作范围与类别边界")
		}
		unmatched := decode[monthlyworkModels.ListData](t, request(t, server, "GET",
			workPath+"?year=1901&year=1902&scope=all&keyword=fixture.owner", nil, cookie, 200))
		if unmatched.Total != 0 {
			t.Fatal("负责人关键词绕过了工作可见范围")
		}
		request(t, server, "DELETE",
			fmt.Sprintf("%s/%s/%d?revision=%d", base, resource, own.ID, own.Revision),
			nil, cookie, 200)
	}
	cpcList = decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", cpcPath+"?year=2024", nil, cookie, 200))
	if len(cpcList.Items[0].Attachments) != 1 || cpcList.Items[0].Attachments[0].ID != files[1].ID {
		t.Fatal("党建工作列表未正确关联所属附件")
	}
	grant(t, server, admin, workspaceID, userID, studentActions)
	request(t, server, "GET", cpcPath+"?year=2024", nil, cookie, 403)
	request(t, server, "GET", base+"/attachments/cpc-monthly-work", nil, cookie, 403)
}
