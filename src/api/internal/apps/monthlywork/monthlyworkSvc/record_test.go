package monthlyworkSvc_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
)

// TestMonthlyWorkTenantOwnerAndAuditBoundaries 仅在独立空验收库内使用虚构数据。
func TestMonthlyWorkTenantOwnerAndAuditBoundaries(t *testing.T) {
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立月度工作验收数据库后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_monthly_work_test") {
		t.Fatal("月度工作验收数据库必须以 _monthly_work_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("月度工作验收要求独立空数据库")
	}
	_, admin := fixture(t, "fixture.admin", strings.Repeat("a1", 32), true)
	user, cookie := fixture(t, "fixture.work", strings.Repeat("b2", 32), false)
	owner, ownerCookie := fixture(t, "fixture.owner", strings.Repeat("c3", 32), false)
	otherOwner, _ := fixture(t, "fixture.other", strings.Repeat("d4", 32), false)
	server := httpd.New()
	createWorkspace := func(name string) workspaceModels.Info {
		return decode[workspaceModels.Info](t, request(t, server, "POST", "/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: name}, admin, 200))
	}
	first, second := createWorkspace("虚构月度工作甲"), createWorkspace("虚构月度工作乙")
	actions := []string{
		accessConstants.AccessWorkspace, accessConstants.MonthlyWorkList,
		accessConstants.MonthlyWorkCreate, accessConstants.MonthlyWorkUpdate,
		accessConstants.MonthlyWorkDelete, accessConstants.MonthlyWorkImport,
		accessConstants.MonthlyWorkAttachmentRead, accessConstants.MonthlyWorkAttachmentUpload,
		accessConstants.MonthlyWorkAttachmentDelete, accessConstants.ProjectAttachmentRead,
	}
	grant(t, server, admin, first.ID, user.ID, actions)
	grant(t, server, admin, second.ID, user.ID, actions)
	grant(t, server, admin, first.ID, owner.ID, []string{accessConstants.AccessWorkspace})
	grant(t, server, admin, second.ID, otherOwner.ID, []string{accessConstants.AccessWorkspace})
	base, otherBase := "/v1/workspaces/"+first.ID, "/v1/workspaces/"+second.ID
	request(t, server, "GET", base+"/monthly-work?year=2026", nil, admin, 403)
	request(t, server, "GET", base+"/monthly-work?year=2026", nil, ownerCookie, 403)
	options := decode[[]monthlyworkModels.OwnerOption](t,
		request(t, server, "GET", base+"/monthly-work/owners", nil, cookie, 200))
	if len(options) != 2 {
		t.Fatal("负责人选项未限定为当前空间的可用用户")
	}
	for _, option := range options {
		if option.Value == otherOwner.ID {
			t.Fatal("负责人选项泄露其他空间账号")
		}
	}
	input := monthlyworkModels.Input{
		Date: "2026-11-03", WorkItem: "虚构工作项目", OwnerID: owner.ID,
		Description: "虚构多行说明\n第二行",
		Links: []monthlyworkModels.Link{
			{Name: "工作材料", URL: "https://example.com/work?signature=fictional-signature"},
			{Name: "会议记录", URL: "https://example.com/notes"},
		},
	}
	invalidOwner := input
	invalidOwner.OwnerID = otherOwner.ID
	request(
		t,
		server,
		"POST",
		base+"/monthly-work",
		monthlyworkModels.Request{Input: invalidOwner},
		cookie,
		400,
	)
	record := decode[monthlyworkModels.Info](t, request(t, server, "POST", base+"/monthly-work",
		monthlyworkModels.Request{Input: input}, cookie, 200))
	if record.OwnerID != owner.ID || record.OwnerUsername != owner.Username ||
		len(record.Links) != 2 {
		t.Fatal("负责人关联或多个链接未完整保存")
	}
	path := fmt.Sprintf("%s/monthly-work/%d", base, record.ID)
	input.OwnerID = otherOwner.ID
	other := decode[monthlyworkModels.Info](t, request(t, server, "POST", otherBase+"/monthly-work",
		monthlyworkModels.Request{Input: input}, cookie, 200))
	input.OwnerID = owner.ID
	listed := decode[monthlyworkModels.ListData](
		t,
		request(
			t,
			server,
			"GET",
			base+"/monthly-work?year=2026&month=11&keyword=fixture.owner",
			nil,
			cookie,
			200,
		),
	)
	if listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].ID != record.ID {
		t.Fatal("月份或负责人关键词查询越过租户边界")
	}
	request(t, server, "PUT", fmt.Sprintf("%s/monthly-work/%d", otherBase, record.ID),
		monthlyworkModels.Request{Input: input, Revision: record.Revision}, cookie, 404)

	for _, query := range []string{"?year=invalid", "?year=0", "?year=10000", "?year=2026&month=13", "?year=2026&month=0"} {
		request(t, server, "GET", base+"/monthly-work"+query, nil, cookie, 400)
	}

	// 负责人和系统管理员身份都不隐含修改权限，撤权后下一次请求立即拒绝。
	request(
		t,
		server,
		"PUT",
		path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision},
		ownerCookie,
		403,
	)
	if err := connection.Where(
		"workspace_id = ? AND user_id = ? AND action = ?",
		first.ID,
		user.ID,
		accessConstants.MonthlyWorkUpdate,
	).Delete(&accessModels.Permission{}).Error; err != nil {
		t.Fatal(err)
	}
	request(
		t,
		server,
		"PUT",
		path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision},
		cookie,
		403,
	)
	grant(t, server, admin, first.ID, user.ID, actions)

	// 用户外键和复合租户附件外键均保留删除保护。
	if err := connection.Where("user_id = ?", owner.ID).
		Delete(&userModels.Session{}).
		Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Where("user_id = ?", owner.ID).
		Delete(&accessModels.Permission{}).
		Error; err != nil {
		t.Fatal(err)
	}
	assertForeignKey(t, connection.Where("id = ?", owner.ID).Delete(&userModels.User{}).Error, 1451)
	grant(t, server, admin, first.ID, owner.ID, []string{accessConstants.AccessWorkspace})
	newFile := func(workspaceID string, workID uint64, hash string) attachmentModels.MonthlyWorkAttachment {
		return attachmentModels.MonthlyWorkAttachment{
			MonthlyWorkID: workID,
			Record: attachmentModels.Record{
				WorkspaceID: workspaceID, Filename: "虚构说明.pdf",
				ObjectKey: "fictional/" + hash, ObjectHash: strings.Repeat(hash, 64), Size: 1,
				State: attachmentConstants.Ready, Revision: 1,
			},
		}
	}
	filesToCreate := []attachmentModels.MonthlyWorkAttachment{
		newFile(first.ID, record.ID, "a"),
		newFile(second.ID, other.ID, "b"),
	}
	if err := connection.Create(&filesToCreate).Error; err != nil {
		t.Fatal(err)
	}
	file, foreign := filesToCreate[0], filesToCreate[1]
	crossTenant := newFile(first.ID, other.ID, "c")
	assertForeignKey(t, connection.Create(&crossTenant).Error, 1452)
	record = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision}, cookie, 200))
	if len(record.AttachmentIDs) != 1 || record.AttachmentIDs[0] != file.ID {
		t.Fatal("更新工作说明时未保留所属附件，或返回了其他空间的附件")
	}
	request(
		t,
		server,
		"DELETE",
		path+fmt.Sprintf("?revision=%d", record.Revision),
		nil,
		cookie,
		409,
	)
	withFiles := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 200))
	if len(withFiles.Items) != 1 || len(withFiles.Items[0].Attachments) != 1 ||
		withFiles.Items[0].Attachments[0].Filename != file.Filename {
		t.Fatal("工作列表未返回当前记录所属附件的文件名")
	}
	assertForeignKey(t, connection.Where("workspace_id = ? AND id = ?", first.ID, record.ID).
		Delete(&monthlyworkModels.Record{}).Error, 1451)
	request(
		t,
		server,
		"PUT",
		path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision - 1},
		cookie,
		409,
	)
	files := decode[attachmentModels.ListData](t,
		request(t, server, "GET", otherBase+"/attachments/monthly-work", nil, cookie, 200))
	if files.Total != 1 || files.Items[0].ID != foreign.ID ||
		files.Items[0].MonthlyWorkID != other.ID {
		t.Fatal("附件列表未隔离租户或类别")
	}
	projects := decode[attachmentModels.ListData](t,
		request(t, server, "GET", base+"/attachments/project", nil, cookie, 200))
	if projects.Total != 0 {
		t.Fatal("项目附件目录暴露了月度工作附件")
	}
	if err := connection.Where(
		"workspace_id = ? AND user_id = ? AND action = ?",
		first.ID,
		user.ID,
		accessConstants.MonthlyWorkAttachmentRead,
	).Delete(&accessModels.Permission{}).Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "GET", base+"/attachments/monthly-work", nil, cookie, 403)
	hiddenFiles := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 200))
	if len(hiddenFiles.Items) != 1 || len(hiddenFiles.Items[0].Attachments) != 0 {
		t.Fatal("撤销附件读取权限后仍返回了文件名")
	}
	record = decode[monthlyworkModels.Info](t, request(t, server, "PUT", path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision}, cookie, 200))
	if len(record.Attachments) != 0 || len(record.AttachmentIDs) != 1 ||
		record.AttachmentIDs[0] != file.ID {
		t.Fatal("无附件读取权限时编辑工作泄露了文件名或修改了附件归属")
	}
	grant(t, server, admin, first.ID, user.ID, actions)

	// 普通审计不复制外部地址中的签名参数，仍可通过摘要观察地址变化。
	var audit auditModels.Record
	if err := connection.Where(
		"workspace_id = ? AND instance_id = ? AND action_id = ?",
		first.ID,
		fmt.Sprint(record.ID),
		accessConstants.MonthlyWorkCreate,
	).Take(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(audit.InstanceData), "fictional-signature") ||
		!strings.Contains(string(audit.InstanceData), "url_hash") {
		t.Fatal("审计包含签名参数或缺少链接摘要")
	}
	const callback = "test:monthly-work-audit-unavailable"
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
	t.Cleanup(func() { _ = db.DB().Callback().Create().Remove(callback) })
	input.WorkItem = "不应提交的修改"
	request(
		t,
		server,
		"PUT",
		path,
		monthlyworkModels.Request{Input: input, Revision: record.Revision},
		cookie,
		503,
	)
	request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 503)
	var persisted monthlyworkModels.Record
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, record.ID).
		Take(&persisted).
		Error; err != nil {
		t.Fatal(err)
	}
	if persisted.WorkItem != record.WorkItem || persisted.Revision != record.Revision {
		t.Fatal("审计故障没有回滚工作修改")
	}
	if err := db.DB().Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}

	// 导入先解析负责人账号，任何一行无效均回滚之前的新增。
	content := "日期,工作项目,负责人账号,详细说明,外部链接(JSON)\n2026-11-01,虚构导入,fixture.owner,,[]\n2026-02-30,无效日期,fixture.owner,,[]\n"
	request(
		t,
		server,
		"POST",
		base+"/monthly-work/import",
		monthlyworkModels.ImportRequest{Content: spreadsheettest.Fixture(t, content)},
		cookie,
		400,
	)
	if err := connection.Model(&monthlyworkModels.Record{}).
		Where("workspace_id = ?", first.ID).
		Count(&count).
		Error; err != nil ||
		count != 1 {
		t.Fatal("导入失败未整批回滚")
	}

	// 删除虚构文件记录后才允许删除工作，历史审计继续保留。
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, file.ID).
		Delete(&attachmentModels.MonthlyWorkAttachment{}).
		Error; err != nil {
		t.Fatal(err)
	}
	request(
		t,
		server,
		"DELETE",
		path+fmt.Sprintf("?revision=%d", record.Revision),
		nil,
		cookie,
		200,
	)
	if err := connection.Model(&auditModels.Record{}).
		Where("workspace_id = ? AND instance_id = ? AND action_id = ?",
			first.ID, fmt.Sprint(record.ID), accessConstants.MonthlyWorkDelete).
		Count(&count).
		Error; err != nil ||
		count != 1 {
		t.Fatal("硬删除后未保留审计")
	}
	// 全年读取不受默认每页条数限制，月份可选，年份筛选不跨空间。
	rows := config.Config.HTTP.DefaultPageSize + 1
	for index := range rows {
		month := 1
		if index%2 == 0 {
			month = 11
		}
		input.Date = fmt.Sprintf("2026-%02d-01", month)
		input.WorkItem = fmt.Sprintf("虚构全年工作%d", index)
		request(
			t,
			server,
			"POST",
			base+"/monthly-work",
			monthlyworkModels.Request{Input: input},
			cookie,
			200,
		)
	}
	input.Date = "2025-12-31"
	request(
		t,
		server,
		"POST",
		base+"/monthly-work",
		monthlyworkModels.Request{Input: input},
		cookie,
		200,
	)
	allResponse := request(t, server, "GET", base+"/monthly-work?year=2026", nil, cookie, 200)
	all := decode[monthlyworkModels.ListData](t, allResponse)
	if all.Total != int64(rows) || len(all.Items) != rows {
		t.Fatal("全年记录仍被分页或混入其他年份及空间")
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(allResponse.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload.Data["page"]; exists {
		t.Fatal("月度工作仍返回分页字段")
	}
	if _, exists := payload.Data["page_size"]; exists {
		t.Fatal("月度工作仍返回每页条数")
	}
	monthly := decode[monthlyworkModels.ListData](t,
		request(t, server, "GET", base+"/monthly-work?year=2026&month=11", nil, cookie, 200))
	if monthly.Total != int64((rows+1)/2) {
		t.Fatal("可选月份筛选没有限制在所选年份内")
	}
	filters := decode[monthlyworkModels.FilterOptionsData](t,
		request(t, server, "GET", base+"/monthly-work/filter-options", nil, cookie, 200))
	if len(filters.Years) != 2 || filters.Years[0].Value != "2026" ||
		filters.Years[1].Value != "2025" {
		t.Fatal("年份枚举未完整去重或排序")
	}

	checkWorkCategoryBoundaries(t, server, first.ID, user.ID, cookie, admin, input, actions)
	checkWorkParticipants(
		t, server, first.ID, second.ID, user.ID, owner.ID, otherOwner.ID, cookie, admin, actions,
	)
}

func assertForeignKey(t *testing.T, err error, number uint16) {
	t.Helper()
	var database *mysql.MySQLError
	if !errors.As(err, &database) || database.Number != number {
		t.Fatalf("外键保护错误：%v", err)
	}
}
