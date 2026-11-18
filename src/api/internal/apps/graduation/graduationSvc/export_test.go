package graduationSvc_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"
)

func checkGraduationExport(
	t *testing.T,
	server *echo.Echo,
	base string,
	cookie *http.Cookie,
	employedType string,
) {
	t.Helper()
	export := func(parameters url.Values) [][]string {
		t.Helper()
		file := decode[graduationModels.FileData](t, request(
			t, server, "GET", base+"/export?"+parameters.Encode(), nil, cookie, 200,
		))
		if file.Filename != "毕业去向.xlsx" || file.ContentType != spreadsheet.ContentType {
			t.Fatal("毕业去向导出文件不符合 XLSX 契约")
		}
		text := spreadsheettest.Text(t, file.Content)
		if strings.Contains(text, "其他空间") || strings.Contains(text, "FICTIONAL-PRIVATE") {
			t.Fatal("导出混入其他空间或学生敏感字段")
		}
		rows, err := spreadsheettest.Rows(file.Content)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	// 导出全部匹配结果，不受列表页码及页大小限制。
	parameters := url.Values{"page": {"2"}, "page_size": {"1"}}
	rows := export(parameters)
	if len(rows) != 4 || rows[1][0] != "S03" || rows[3][0] != "S01" ||
		!slices.Equal(rows[0], []string{
			"学号", "姓名", "学历", "专业", "年级", "毕业年份", "类型代码", "类型", "毕业去向",
		}) {
		t.Fatal("毕业去向导出未保持全部结果、固定字段或学生编号倒序")
	}
	parameters = url.Values{
		"education_level": {"本科", "硕士"}, "major": {"虚构专业甲"},
		"grade": {"2026级"}, "graduation_year": {"2026", "2027"}, "type": {employedType},
	}
	rows = export(parameters)
	if len(rows) != 2 || rows[1][0] != "S01" || rows[1][7] != "签约就业" || rows[1][8] != "虚构单位" {
		t.Fatal("毕业去向导出未复用全部多选筛选")
	}
	rows = export(url.Values{"type": {""}})
	if len(rows) != 3 || rows[1][7] != "待就业" || rows[2][7] != "待就业" || rows[1][8] != "" {
		t.Fatal("显式空类型未仅导出待就业学生，或空值被替换")
	}
	var audit auditModels.Record
	workspaceID := strings.TrimSuffix(
		strings.TrimPrefix(base, "/v1/workspaces/"),
		"/graduation-destinations",
	)
	if err := db.DB().WithContext(t.Context()).
		Where("workspace_id = ? AND action_id = ? AND result_code = ?",
			workspaceID, accessConstants.GraduationExport, 0).
		Order("id DESC").Take(&audit).Error; err != nil {
		t.Fatal(err)
	}
	var extend auditModels.Extend
	if err := json.Unmarshal(audit.ExtendData, &extend); err != nil {
		t.Fatal(err)
	}
	if audit.RequestID == "" || audit.UserID == 0 || extend.Method != http.MethodGet ||
		extend.Path == "" {
		t.Fatal("导出审计未保存操作者、追踪或安全路由信息")
	}
	if rows := export(url.Values{"keyword": {"不存在的学生"}}); len(rows) != 1 {
		t.Fatal("无匹配记录未返回空表格")
	}
	request(t, server, "GET", base+"/export?graduation_year=1899", nil, cookie, 400)

	// 任何上限失败都拒绝整个文件，不返回截断数据。
	maxRows, maxBytes := config.Config.Student.MaxExportRows, config.Config.Student.MaxFileBytes
	defer func() {
		config.Config.Student.MaxExportRows, config.Config.Student.MaxFileBytes = maxRows, maxBytes
	}()
	config.Config.Student.MaxExportRows = 1
	request(t, server, "GET", base+"/export", nil, cookie, 400)
	config.Config.Student.MaxExportRows = maxRows
	config.Config.Student.MaxFileBytes = 1
	request(t, server, "GET", base+"/export", nil, cookie, 400)
}
