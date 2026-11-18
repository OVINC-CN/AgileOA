package graduationSvc_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
)

func TestGraduationTenantPermissionsStatisticsAndAudit(t *testing.T) {
	// 验收仅使用独立空库及虚构数据，由用户明确安排后手工执行。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立毕业去向验收数据库后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_graduation_test") {
		t.Fatal("毕业去向验收数据库名称必须以 _graduation_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("毕业去向验收要求独立空数据库")
	}
	_, admin := fixture(t, "fixture.admin", strings.Repeat("a1", 32), true)
	user, cookie := fixture(t, "fixture.graduation", strings.Repeat("b2", 32), false)
	server := httpd.New()
	createWorkspace := func(name string) string {
		return decode[workspaceModels.Info](t, request(t, server, "POST", "/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: name}, admin, 200)).ID
	}
	first, second := createWorkspace("虚构毕业空间甲"), createWorkspace("虚构毕业空间乙")
	base := "/v1/workspaces/" + first + "/graduation-destinations"
	students := []studentModels.Student{
		{
			WorkspaceID: first,
			BasicRecord: studentModels.BasicRecord{
				StudentNo:      "S01",
				Name:           "虚构学生甲",
				EducationLevel: "本科",
				Major:          "虚构专业甲",
				Grade:          "2026级",
			},
		},
		{
			WorkspaceID: first,
			BasicRecord: studentModels.BasicRecord{
				StudentNo:      "S02",
				Name:           "虚构学生乙",
				EducationLevel: "本科",
				Major:          "虚构专业甲",
				Grade:          "2025级",
			},
		},
		{
			WorkspaceID: first,
			BasicRecord: studentModels.BasicRecord{
				StudentNo:      "S03",
				Name:           "虚构学生丙",
				EducationLevel: "硕士",
				Major:          "虚构专业乙",
				Grade:          "2026级",
			},
		},
		{
			WorkspaceID: second,
			BasicRecord: studentModels.BasicRecord{
				StudentNo:      "S01",
				Name:           "其他空间学生",
				EducationLevel: "其他学历",
				Major:          "其他空间专业",
				Grade:          "其他年级",
			},
		},
	}
	for index := range students {
		students[index].Revision = 1
		students[index].GraduationYear = "2026"
		if index == 2 {
			students[index].GraduationYear = "2027"
		}
		students[index].IdentityNumber = "FICTIONAL-PRIVATE"
		if err := connection.Create(&students[index]).Error; err != nil {
			t.Fatal(err)
		}
	}
	idPath := func(index int) string { return fmt.Sprintf("%s/%d", base, students[index].ID) }
	input := func(label, destination string, revision uint64) graduationModels.Request {
		return graduationModels.Request{
			Input: graduationModels.Input{
				GraduationYear: "2026",
				TypeLabel:      label,
				Destination:    destination,
			},
			Revision: revision,
		}
	}
	request(t, server, "GET", base, nil, nil, 401)
	request(t, server, "GET", base, nil, admin, 403)
	request(t, server, "GET", base, nil, cookie, 403)
	grant(t, server, admin, first, user.ID, []string{accessConstants.GraduationList})
	response := request(t, server, "GET", base, nil, cookie, 200)
	list := decode[graduationModels.ListData](t, response)
	if list.Total != 3 || len(list.Items) != 3 ||
		strings.Contains(response.Body.String(), "FICTIONAL-PRIVATE") {
		t.Fatal("列表未限制租户或暴露了敏感字段")
	}
	request(t, server, "GET", base+"/statistics", nil, cookie, 403)
	request(t, server, "GET", base+"/template", nil, cookie, 403)
	request(t, server, "GET", base+"/export", nil, cookie, 403)
	request(t, server, "GET", base+"/export", nil, admin, 403)
	request(t, server, "PUT", idPath(0), input("签约就业", "虚构单位", 1), cookie, 403)
	request(t, server, "DELETE", idPath(0)+"?revision=1", nil, cookie, 403)
	request(t, server, "GET", "/v1/workspaces/"+first+"/students", nil, cookie, 403)
	actions := []string{
		accessConstants.GraduationList,
		accessConstants.GraduationUpdate,
		accessConstants.GraduationDelete,
		accessConstants.GraduationStatistics,
		accessConstants.GraduationImport,
		accessConstants.GraduationExport,
		accessConstants.StudentDelete,
	}
	grant(t, server, admin, first, user.ID, actions)
	request(t, server, "PUT", idPath(3), input("签约就业", "跨空间", 1), cookie, 404)
	request(t, server, "DELETE", idPath(3)+"?revision=1", nil, cookie, 404)
	request(t, server, "PUT", idPath(0), input("\x00", "", 1), cookie, 400)
	request(t, server, "PUT", idPath(0), input(strings.Repeat("字", 101), "", 1), cookie, 400)
	request(t, server, "PUT", idPath(0), graduationModels.Request{
		Input: graduationModels.Input{Type: "unregistered", TypeLabel: "签约就业"}, Revision: 1,
	}, cookie, 400)
	request(t, server, "PUT", idPath(0), map[string]any{
		"type_label": "签约就业", "revision": 1, "workspace_id": second,
	}, cookie, 400)
	request(t, server, "PUT", idPath(0), input("签约就业", "虚构单位", 1), cookie, 200)
	request(t, server, "PUT", idPath(0), input("签约就业", "旧版本覆盖", 1), cookie, 409)
	request(t, server, "PUT", idPath(1), input("", "", 1), cookie, 200)

	// 分母包含所有学生，空类型不保存独立枚举，统一计入待就业。
	statistics := decode[[]graduationModels.Statistic](
		t,
		request(t, server, "GET", base+"/statistics", nil, cookie, 200),
	)
	byMajor := map[string]graduationModels.Statistic{}
	for _, item := range statistics {
		byMajor[item.Value] = item
	}
	if len(statistics) != 2 || byMajor["虚构专业甲"].Total != 2 ||
		byMajor["虚构专业甲"].Employed != 1 || byMajor["虚构专业甲"].EmploymentRate != 0.5 ||
		byMajor["虚构专业乙"].Pending != 1 ||
		byMajor["虚构专业甲"].GraduationYear != "2026" || byMajor["虚构专业乙"].GraduationYear != "2027" {
		t.Fatalf("专业人数或就业率错误：%+v", statistics)
	}
	types := decode[[]graduationModels.Statistic](
		t,
		request(t, server, "GET", base+"/statistics?dimension=type", nil, cookie, 200),
	)
	var pending int64
	for _, item := range types {
		if item.Value == "" {
			pending = item.Total
		}
	}
	if len(types) != 2 || pending != 2 {
		t.Fatalf("未绑定没有合并计入待就业：%+v", types)
	}
	query := url.Values{
		"education_level": {"本科"},
		"major":           {"虚构专业甲"},
		"grade":           {"2026级"},
		"graduation_year": {"2026"},
	}
	filtered := decode[graduationModels.ListData](
		t,
		request(t, server, "GET", base+"?"+query.Encode(), nil, cookie, 200),
	)
	filteredStats := decode[[]graduationModels.Statistic](
		t,
		request(t, server, "GET", base+"/statistics?"+query.Encode(), nil, cookie, 200),
	)
	if filtered.Total != 1 || filtered.Items[0].StudentID != students[0].ID ||
		len(
			filteredStats,
		) != 1 || filteredStats[0].Total != 1 || filteredStats[0].EmploymentRate != 1 {
		t.Fatal("学历、专业和年级筛选未被明细及统计共同应用")
	}
	options := decode[graduationModels.FilterOptions](
		t,
		request(t, server, "GET", base+"/filter-options", nil, cookie, 200),
	)
	if len(options.Types) != 1 || len(options.Majors) != 2 || len(options.GraduationYears) != 2 ||
		!strings.HasPrefix(options.Types[0].Value, graduationConstants.CustomTypePrefix) ||
		strings.Contains(fmt.Sprint(options), "其他空间") {
		t.Fatal("类型枚举未从当前空间学生数据去重获取")
	}
	// 类型筛选与其他条件共同作用于列表和统计；显式空类型只返回待就业。
	query.Set("type", options.Types[0].Value)
	filtered = decode[graduationModels.ListData](
		t,
		request(t, server, "GET", base+"?"+query.Encode(), nil, cookie, 200),
	)
	filteredStats = decode[[]graduationModels.Statistic](
		t,
		request(t, server, "GET", base+"/statistics?"+query.Encode(), nil, cookie, 200),
	)
	if filtered.Total != 1 || filtered.Items[0].StudentID != students[0].ID ||
		len(filteredStats) != 1 || filteredStats[0].Total != 1 || filteredStats[0].Employed != 1 {
		t.Fatal("类型筛选未与其他条件共同作用于明细和统计")
	}
	filtered = decode[graduationModels.ListData](
		t,
		request(t, server, "GET", base+"?type=", nil, cookie, 200),
	)
	filteredStats = decode[[]graduationModels.Statistic](
		t,
		request(t, server, "GET", base+"/statistics?type=", nil, cookie, 200),
	)
	var pendingTotal int64
	for _, item := range filteredStats {
		if item.Employed != 0 || item.Pending != item.Total {
			t.Fatal("待就业类型筛选混入已填写类型的学生")
		}
		pendingTotal += item.Total
	}
	if filtered.Total != 2 || pendingTotal != 2 {
		t.Fatal("空类型筛选未仅返回待就业学生")
	}
	checkGraduationExport(t, server, base, cookie, options.Types[0].Value)

	// 空类型可以与已就业类型并选，多个维度的并集仍受工作空间条件保护。
	multiple := url.Values{
		"education_level": {"本科", "硕士"},
		"major":           {"虚构专业甲", "虚构专业乙"},
		"grade":           {"2025级", "2026级"},
		"graduation_year": {"2026", "2027"},
		"type":            {"", options.Types[0].Value},
	}
	filtered = decode[graduationModels.ListData](t,
		request(t, server, "GET", base+"?"+multiple.Encode(), nil, cookie, 200))
	filteredStats = decode[[]graduationModels.Statistic](t,
		request(t, server, "GET", base+"/statistics?"+multiple.Encode(), nil, cookie, 200))
	var multipleTotal int64
	for _, item := range filteredStats {
		multipleTotal += item.Total
	}
	if filtered.Total != 3 || multipleTotal != filtered.Total {
		t.Fatal("毕业去向多选或空类型组合未同时应用于列表与统计")
	}
	request(t, server, "GET", base+"?type="+strings.Repeat("a", 72), nil, cookie, 400)
	request(t, server, "GET", base+"/statistics?dimension=identity_number", nil, cookie, 400)
	request(t, server, "GET", base+"/statistics?keyword=missing", nil, cookie, 200)
	fields := decode[[]graduationModels.Field](
		t,
		request(t, server, "GET", base+"/fields", nil, cookie, 200),
	)
	if len(fields) != 5 {
		t.Fatal("毕业去向固定字段目录缺失")
	}
	limit := config.Config.Student.MaxOptions
	config.Config.Student.MaxOptions = 1
	request(t, server, "GET", base+"/filter-options", nil, cookie, 400)
	request(t, server, "GET", base+"/statistics", nil, cookie, 400)
	config.Config.Student.MaxOptions = limit

	// 重复和跨空间学号不能导入；清除单独授权并受学生版本保护。
	file := decode[graduationModels.FileData](
		t,
		request(t, server, "GET", base+"/template", nil, cookie, 200),
	)
	if spreadsheettest.Text(t, file.Content) != "学号,毕业年份,类型,毕业去向" {
		t.Fatal("导入空模板表头错误")
	}
	request(t, server, "POST", base+"/import", graduationModels.ImportRequest{
		Content: spreadsheettest.Fixture(
			t,
			"学号,毕业年份,类型,毕业去向\nS03,2027,升学,虚构学校\nS01,2026,签约就业,重复记录\n",
		),
	}, cookie, 400)
	var unchanged studentModels.Student
	if err := connection.Where("workspace_id = ? AND id = ?", first, students[2].ID).
		Take(&unchanged).
		Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.GraduationType != "" || unchanged.Revision != 1 {
		t.Fatal("导入失败未整批回滚")
	}
	request(t, server, "POST", base+"/import", graduationModels.ImportRequest{
		Content: spreadsheettest.Fixture(t, "学号,毕业年份,类型,毕业去向\nS03,2027,升学,虚构学校\n"),
	}, cookie, 200)
	request(t, server, "DELETE", idPath(1)+"?revision=1", nil, cookie, 409)
	request(t, server, "DELETE", idPath(1)+"?revision=2", nil, cookie, 200)
	checkAuditRollback(t, server, cookie, base, first, students[0].ID, students[1].ID)

	// 业务引用继续阻止删除学生；解除引用必须显式操作。
	project := projectModels.Project{
		WorkspaceID: first,
		Name:        "虚构删除保护项目",
		Year:        2026,
		Revision:    1,
	}
	if err := connection.Create(&project).Error; err != nil {
		t.Fatal(err)
	}
	participation := projectModels.Participation{
		WorkspaceID: first,
		ProjectID:   project.ID,
		StudentID:   students[0].ID,
		Revision:    1,
	}
	if err := connection.Create(&participation).Error; err != nil {
		t.Fatal(err)
	}
	studentPath := fmt.Sprintf("/v1/workspaces/%s/students/%d?revision=2", first, students[0].ID)
	request(t, server, "DELETE", studentPath, nil, cookie, 409)
	if err := connection.Where("workspace_id = ? AND id = ?", first, participation.ID).
		Delete(&participation).
		Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "DELETE", studentPath, nil, cookie, 200)
	var retained auditModels.Record
	if err := connection.Where("workspace_id = ? AND action_id = ? AND instance_id = ? AND result_code = ?", first, accessConstants.StudentDelete, fmt.Sprint(students[0].ID), 0).
		Take(&retained).
		Error; err != nil ||
		!strings.Contains(string(retained.InstanceOriginData), "虚构单位") {
		t.Fatal("硬删除没有保留毕业去向与学生身份快照")
	}

	// 模拟入口鉴权之后撤权，Svc 必须在工作空间锁内再次拒绝。
	revoke := true
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if revoke && c.Request().Method == "PUT" && c.Request().URL.Path == idPath(1) {
				revoke = false
				grant(t, server, admin, first, user.ID, []string{})
			}
			return next(c)
		}
	})
	request(t, server, "PUT", idPath(1), input("签约就业", "撤权写入", 3), cookie, 403)
	grant(t, server, admin, first, user.ID, []string{accessConstants.GraduationStatistics})
	request(t, server, "GET", base+"/statistics", nil, cookie, 200)
	statisticsOptions := decode[graduationModels.FilterOptions](
		t,
		request(t, server, "GET", base+"/statistics/filter-options", nil, cookie, 200),
	)
	if len(statisticsOptions.Types) != 1 || statisticsOptions.Types[0].Label != "升学" {
		t.Fatal("仅有统计权限时无法获取类型筛选选项")
	}
	request(t, server, "GET", base, nil, cookie, 403)
	// 导出权限独立于列表，并在进入业务事务后再次复核。
	grant(t, server, admin, first, user.ID, []string{accessConstants.GraduationExport})
	request(t, server, "GET", base+"/export", nil, cookie, 200)
	request(t, server, "GET", base, nil, cookie, 403)
	revokeExport := true
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if revokeExport && c.Request().URL.Path == base+"/export" {
				revokeExport = false
				grant(t, server, admin, first, user.ID, []string{})
			}
			return next(c)
		}
	})
	request(t, server, "GET", base+"/export", nil, cookie, 403)
	grant(t, server, admin, first, user.ID, []string{accessConstants.GraduationExport})
	if err := connection.Model(&workspaceModels.Workspace{}).
		Where("id = ?", first).
		Update("is_enabled", false).
		Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "GET", base+"/statistics", nil, cookie, 403)
	request(t, server, "GET", base+"/export", nil, cookie, 403)
	if err := connection.Model(&userModels.User{}).
		Where("id = ?", user.ID).
		Update("is_enabled", false).
		Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "GET", base+"/statistics", nil, cookie, 401)
}

func checkAuditRollback(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	base, workspaceID string,
	employedID, pendingID uint64,
) {
	t.Helper()
	const callback = "test:graduation-audit-unavailable"
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
	defer func() {
		if err := db.DB().Callback().Create().Remove(callback); err != nil {
			t.Error(err)
		}
	}()
	request(t, server, "GET", base, nil, cookie, 503)
	request(t, server, "GET", base+"/statistics", nil, cookie, 503)
	request(t, server, "GET", base+"/filter-options", nil, cookie, 503)
	request(t, server, "GET", base+"/export", nil, cookie, 503)
	request(t, server, "PUT", fmt.Sprintf("%s/%d", base, employedID), graduationModels.Request{
		Input: graduationModels.Input{TypeLabel: "待就业"}, Revision: 2,
	}, cookie, 503)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/%d?revision=2", base, employedID),
		nil,
		cookie,
		503,
	)
	request(
		t,
		server,
		"POST",
		base+"/import",
		graduationModels.ImportRequest{
			Content: spreadsheettest.Fixture(t, "学号,毕业年份,类型,毕业去向\nS02,2026,签约就业,审计回滚\n"),
		},
		cookie,
		503,
	)
	var employed, pending studentModels.Student
	connection := db.DB().WithContext(t.Context())
	if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, employedID).
		Take(&employed).
		Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, pendingID).
		Take(&pending).
		Error; err != nil {
		t.Fatal(err)
	}
	if employed.GraduationDestination != "虚构单位" || employed.Revision != 2 ||
		pending.GraduationType != "" ||
		pending.Revision != 3 || pending.GraduationYear != "2026" {
		t.Fatal("审计故障没有回滚维护、清除或导入")
	}
}
