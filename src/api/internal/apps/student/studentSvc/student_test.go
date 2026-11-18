package studentSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

const (
	religionExportPath    = "/export?format=religion"
	fixtureIdentityNumber = "00000020000102001X"
	fixtureAcademicHeader = "年级,状态,学历,专业,班级,录取类型,民族,住宿类型"
	fixtureAcademicValues = "2026级,自定义状态,博士,虚构专业,2026级虚构班,非定向,汉族,on_campus"
)

func request(
	t *testing.T,
	server *echo.Echo,
	method, path string,
	body any,
	cookie *http.Cookie,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(content))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	if recorder.Code != status {
		t.Fatalf(
			"%s %s：期望 %d，实际 %d：%s",
			method,
			path,
			status,
			recorder.Code,
			recorder.Body.String(),
		)
	}
	if status != 200 {
		var envelope response.Envelope[any]
		if err := json.Unmarshal(
			recorder.Body.Bytes(),
			&envelope,
		); err != nil ||
			envelope.Data != nil {
			t.Fatal("失败响应包含业务数据")
		}
	}
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope response.Envelope[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func fixture(t *testing.T, username, token string, admin bool) (userModels.User, *http.Cookie) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(strings.Repeat("0", 64)),
		config.Config.User.BcryptCost,
	)
	if err != nil {
		t.Fatal(err)
	}
	user := userModels.User{
		Username:       username,
		DisplayName:    "虚构验收账号",
		PasswordHash:   string(hash),
		IsAdmin:        admin,
		IsEnabled:      true,
		SessionVersion: 1,
	}
	connection := db.DB().WithContext(t.Context())
	if err := connection.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(token))
	session := userModels.Session{
		UserID:         user.ID,
		SessionVersion: 1,
		TokenHash:      hex.EncodeToString(digest[:]),
		ExpiresAt:      time.Now().Add(time.Hour).UTC(),
	}
	if err := connection.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	return user, &http.Cookie{
		Name:     config.Config.User.CookieName,
		Value:    token,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func grant(
	t *testing.T,
	server *echo.Echo,
	admin *http.Cookie,
	workspaceID string,
	userID uint64,
	actions []string,
) {
	t.Helper()
	path := fmt.Sprintf("/v1/system/access/workspaces/%s/users/%d/actions", workspaceID, userID)
	current := decode[accessModels.ActionsData](t, request(t, server, "GET", path, nil, admin, 200))
	request(
		t,
		server,
		"PUT",
		path,
		accessModels.UpdateActionsRequest{Actions: actions, Revision: current.Revision},
		admin,
		200,
	)
}

func addStudent(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path, number, name, gender string,
) studentModels.MutationData {
	t.Helper()
	return decode[studentModels.MutationData](
		t,
		request(t, server, "POST", path, studentCreateRequest(number, name, gender), cookie, 200),
	)
}

func studentCreateRequest(number, name, gender string) studentModels.CreateRequest {
	values := []studentModels.FieldValue{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive {
			value := ""
			if field.Key == "identity_number" {
				value = fixtureIdentityNumber
			}
			values = append(values, studentModels.FieldValue{Field: field.Key, Value: value})
		}
	}
	return studentModels.CreateRequest{
		Input: studentModels.Input{
			StudentNo:                number,
			Name:                     name,
			Grade:                    "2026级",
			Status:                   "自定义状态",
			EducationLevel:           "博士",
			Gender:                   gender,
			Ethnicity:                "汉族",
			Major:                    "虚构专业",
			ClassName:                "2026级虚构班",
			AdmissionType:            "非定向",
			ClassPosition:            "班长",
			ArrivalMonth:             "2026-09",
			PreviousSchoolOrEmployer: "虚构大学",
			PreviousGraduationMonth:  "2026-06",
			Advisor:                  "虚构导师",
			Phone:                    "FICTIONAL-PHONE-001",
			BirthDate:                "1999-12-31",
			AccommodationType:        "off_campus",
			Origin:                   "虚构户籍市",
			NativePlace:              "虚构籍贯县",
		},
		Sensitive: values,
	}
}

func updateSensitive(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path string,
	student studentModels.MutationData,
	values ...studentModels.FieldValue,
) studentModels.MutationData {
	t.Helper()

	current := decode[studentModels.SensitiveData](
		t,
		request(
			t,
			server,
			"GET",
			fmt.Sprintf("%s/%d/sensitive", path, student.ID),
			nil,
			cookie,
			200,
		),
	)
	for index, item := range current.Values {
		for _, value := range values {
			if value.Field == item.Field {
				current.Values[index].Value = value.Value
			}
		}
	}
	return decode[studentModels.MutationData](
		t,
		request(
			t,
			server,
			"PUT",
			fmt.Sprintf("%s/%d/sensitive", path, student.ID),
			studentModels.SensitiveRequest{Revision: student.Revision, Values: current.Values},
			cookie,
			200,
		),
	)
}

func TestStudentWorkspaceSensitiveInformationAndAudit(t *testing.T) {
	// 验收只使用独立空库、虚构数据和 DML 账号，不迁移或使用真实学生资料。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立学生验收数据库后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_student_test") {
		t.Fatal("学生验收数据库名称必须以 _student_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("学生验收要求独立空数据库")
	}
	_, admin := fixture(t, "fixture.admin", strings.Repeat("a1", 32), true)
	user, cookie := fixture(t, "fixture.student", strings.Repeat("b2", 32), false)
	server := httpd.New()
	first := decode[workspaceModels.Info](
		t,
		request(
			t,
			server,
			"POST",
			"/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: "虚构学生空间甲"},
			admin,
			200,
		),
	)
	second := decode[workspaceModels.Info](
		t,
		request(
			t,
			server,
			"POST",
			"/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: "虚构学生空间乙"},
			admin,
			200,
		),
	)
	path := "/v1/workspaces/" + first.ID + "/students"
	otherPath := "/v1/workspaces/" + second.ID + "/students"
	request(t, server, "GET", path, nil, nil, 401)
	request(t, server, "GET", path, nil, admin, 403)
	request(t, server, "GET", path, nil, cookie, 403)
	// 基础字段枚举复用列表权限；详情、统计、导出和处理敏感信息仍分别授权。
	grant(t, server, admin, first.ID, user.ID, []string{accessConstants.StudentList})
	request(t, server, "GET", path+"/options?field=gender", nil, cookie, 200)
	for _, suffix := range []string{"/1", "/1/sensitive", "/statistics", "/sensitive-options?field=has_religion", "/export"} {
		request(t, server, "GET", path+suffix, nil, cookie, 403)
	}
	actions := make([]string, 0, 10)
	actions = append(
		actions,
		accessConstants.AccessWorkspace,
		accessConstants.StudentList,
		accessConstants.StudentDetail,
		accessConstants.StudentCreate,
		accessConstants.StudentUpdate,
		accessConstants.StudentDelete,
		accessConstants.StudentStatistics,
		accessConstants.StudentImport,
		accessConstants.StudentExport,
	)
	full := append(slices.Clone(actions), accessConstants.StudentSensitiveHandle)
	grant(t, server, admin, first.ID, user.ID, actions)
	request(t, server, "POST", path,
		studentCreateRequest("00000001", "虚构学生甲", "女"), cookie, 403)
	grant(t, server, admin, first.ID, user.ID, full)
	student := addStudent(t, server, cookie, path, "00000001", "虚构学生甲", "女")
	idPath := fmt.Sprintf("%s/%d", path, student.ID)
	initial := decode[studentModels.Info](t, request(t, server, "GET", idPath, nil, cookie, 200))
	if initial.BirthDate != "2000-01-02" {
		t.Fatal("出生日期未由身份证号码生成，或采用了请求传入的值")
	}
	grant(t, server, admin, first.ID, user.ID, actions)
	request(t, server, "GET", idPath+"/sensitive", nil, cookie, 403)
	request(t, server, "GET", path+"/export?format=off_campus", nil, cookie, 403)
	request(
		t,
		server,
		"PUT",
		idPath+"/sensitive",
		studentModels.SensitiveRequest{
			Revision: student.Revision,
			Values:   []studentModels.FieldValue{{Field: "has_religion", Value: "no"}},
		},
		cookie,
		403,
	)
	grant(t, server, admin, first.ID, user.ID, full)
	request(t, server, "PUT", idPath+"/sensitive", studentModels.SensitiveRequest{
		Revision: student.Revision,
		Values:   []studentModels.FieldValue{{Field: "home_address", Value: "不得部分覆盖"}},
	}, cookie, 400)
	student = updateSensitive(t, server, cookie, path, student,
		studentModels.FieldValue{Field: "identity_number", Value: fixtureIdentityNumber},
		studentModels.FieldValue{Field: "home_address", Value: "虚构家庭地址1号"},
		studentModels.FieldValue{Field: "has_religion", Value: "no"},
		studentModels.FieldValue{Field: "only_child", Value: "yes"})
	grant(t, server, admin, first.ID, user.ID, actions)
	for _, suffix := range []string{"", fmt.Sprintf("/%d", student.ID)} {
		recorder := request(t, server, "GET", path+suffix, nil, cookie, 200)
		for _, hidden := range []string{"identity_number", "home_address", "only_child", "single_parent", "orphan", "student_loan", "registered_poverty", "has_religion", "religion_category", "religion_identification", "religion_start_year", "family_religion", "medical_history", "special_circumstances"} {
			if strings.Contains(recorder.Body.String(), hidden) {
				t.Fatal("基础接口暴露敏感字段")
			}
		}
	}
	info := decode[studentModels.Info](t, request(t, server, "GET", idPath, nil, cookie, 200))
	if info.Grade != "2026级" || info.EducationLevel != "博士" || info.Gender != "女" ||
		info.Status != "自定义状态" ||
		info.ArrivalMonth != "2026-09" ||
		info.PreviousSchoolOrEmployer != "虚构大学" ||
		info.PreviousGraduationMonth != "2026-06" ||
		info.Phone != "FICTIONAL-PHONE-001" ||
		info.BirthDate != "2000-01-02" ||
		info.Origin != "虚构户籍市" || info.NativePlace != "虚构籍贯县" ||
		info.AccommodationType != "off_campus" {
		t.Fatal("用户明确的基础字段没有正确返回")
	}
	// 合并保存没有敏感权限时整笔拒绝，不能先写基础字段。
	unauthorized := info.Input
	unauthorized.Name = "不得写入的虚构姓名"
	sensitiveValues := []studentModels.FieldValue{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive {
			sensitiveValues = append(sensitiveValues, studentModels.FieldValue{Field: field.Key})
		}
	}
	request(t, server, "PUT", idPath, studentModels.UpdateRequest{
		Input: unauthorized, Revision: student.Revision, Sensitive: sensitiveValues,
	}, cookie, 403)
	unchanged := decode[studentModels.Info](t, request(t, server, "GET", idPath, nil, cookie, 200))
	if unchanged.Name != info.Name || unchanged.Revision != student.Revision {
		t.Fatal("无敏感权限时合并更新部分写入了基础信息")
	}
	grant(t, server, admin, first.ID, user.ID, full)
	privateBefore := decode[studentModels.SensitiveData](
		t,
		request(t, server, "GET", idPath+"/sensitive", nil, cookie, 200),
	)
	combinedInput := info.Input
	combinedInput.Name = "虚构学生合并修改"
	combinedInput.NativePlace = "虚构新籍贯县"
	for index, value := range privateBefore.Values {
		switch value.Field {
		case "identity_number":
			privateBefore.Values[index].Value = "00000020010229001X"
		case "religion_category":
			privateBefore.Values[index].Value = "虚构宗教类别"
		case "religion_identification":
			privateBefore.Values[index].Value = "疑似"
		case "religion_start_year":
			privateBefore.Values[index].Value = "2020年"
		case "family_religion":
			privateBefore.Values[index].Value = "虚构家庭成员信仰情况"
		}
	}
	student = decode[studentModels.MutationData](
		t,
		request(t, server, "PUT", idPath, studentModels.UpdateRequest{
			Input: combinedInput, Revision: student.Revision, Sensitive: privateBefore.Values,
		}, cookie, 200),
	)
	if student.Revision != info.Revision+1 {
		t.Fatal("基础和敏感信息合并保存未共用一次版本递增")
	}
	var combinedStored studentModels.Student
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, student.ID).
		Take(&combinedStored).
		Error; err != nil {
		t.Fatal(err)
	}
	if combinedStored.Name != combinedInput.Name || combinedStored.ReligionCategory != "虚构宗教类别" ||
		combinedStored.NativePlace != combinedInput.NativePlace || combinedStored.Origin != info.Origin ||
		combinedStored.BirthDate != "" ||
		combinedStored.ReligionStartYear != "2020年" ||
		combinedStored.FamilyReligion != "虚构家庭成员信仰情况" {
		t.Fatal("合并修改未同时保存基础字段及宗教信仰文本字段")
	}
	request(t, server, "GET", path+"/sensitive-options?field=religion_category", nil, cookie, 200)
	request(t, server, "GET", path+"/sensitive-options?field=religion_start_year", nil, cookie, 400)
	grant(t, server, admin, first.ID, user.ID, actions)
	// 敏感字段枚举和筛选共用权限，基础筛选及统计不额外要求敏感权限。
	request(t, server, "GET", path+"/sensitive-options?field=has_religion", nil, cookie, 403)
	checkSensitiveFilters(t, server, cookie, path, 403)
	statistics := decode[studentModels.StatisticsData](
		t,
		request(
			t,
			server,
			"GET",
			path+"/statistics?position_status=assigned&accommodation_type=off_campus&status=自定义状态",
			nil,
			cookie,
			200,
		),
	)
	if statistics.Total != 1 || statistics.ClassCount != 1 || statistics.ReligiousCount != 0 ||
		statistics.OffCampusCount != 1 {
		t.Fatal("四项统计筛选或文本枚举转换错误")
	}
	grant(t, server, admin, first.ID, user.ID, full)
	checkSensitiveFilters(t, server, cookie, path, 200)
	filtered := decode[studentModels.ListData](
		t,
		request(
			t,
			server,
			"GET",
			path+"?has_religion=no&accommodation_type=off_campus&status=自定义状态",
			nil,
			cookie,
			200,
		),
	)
	if filtered.Total != 1 {
		t.Fatal("明细和统计筛选不一致")
	}

	// 多个维度各自取并集，固定枚举仍转换存储代码，列表与统计使用相同条件。
	multiple := "position_status=unassigned&position_status=assigned" +
		"&accommodation_type=on_campus&accommodation_type=off_campus" +
		"&has_religion=yes&has_religion=no&status=不存在&status=自定义状态"
	filtered = decode[studentModels.ListData](t,
		request(t, server, "GET", path+"?"+multiple, nil, cookie, 200))
	statistics = decode[studentModels.StatisticsData](t,
		request(t, server, "GET", path+"/statistics?"+multiple, nil, cookie, 200))
	if filtered.Total != 1 || filtered.Items[0].ID != student.ID || statistics.Total != 1 {
		t.Fatal("多选筛选未匹配任一选中值，或明细与统计条件不一致")
	}
	exported := decode[studentModels.FileData](t,
		request(t, server, "GET", path+"/export?"+multiple, nil, cookie, 200))
	exportRows, err := spreadsheettest.Rows(exported.Content)
	if err != nil || len(exportRows) != 2 || exportRows[1][0] != combinedInput.StudentNo {
		t.Fatal("导出未使用与列表相同的多选条件")
	}
	request(t, server, "GET", path+"?has_religion=no&has_religion=invalid", nil, cookie, 400)
	var stored studentModels.Student
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, student.ID).
		Take(&stored).
		Error; err != nil {
		t.Fatal(err)
	}
	if stored.Gender != "女" || stored.Phone != "FICTIONAL-PHONE-001" ||
		stored.Status != "自定义状态" ||
		stored.HasReligion != studentConstants.FlagNo ||
		stored.AccommodationType != studentConstants.AccommodationOffCampus ||
		stored.BirthDate != "" {
		t.Fatal("明文宽表或 VARCHAR 文本落库不正确")
	}
	request(
		t,
		server,
		"PUT",
		idPath,
		studentModels.UpdateRequest{Input: info.Input, Revision: 1},
		cookie,
		409,
	)
	// 同一学号在其他空间独立存在，所有对象查询和写入继续包含租户条件。
	grant(t, server, admin, second.ID, user.ID, full)
	other := addStudent(t, server, cookie, otherPath, "00000001", "虚构学生乙", "男")
	other = updateSensitive(
		t,
		server,
		cookie,
		otherPath,
		other,
		studentModels.FieldValue{Field: "has_religion", Value: "yes"},
	)
	otherInput := info.Input
	otherInput.Name, otherInput.Gender = "虚构学生乙", "男"
	otherInput.Grade, otherInput.EducationLevel = "2025级", "硕士"
	otherInput.Status = "其他空间状态"
	request(
		t,
		server,
		"PUT",
		fmt.Sprintf("%s/%d", otherPath, other.ID),
		studentModels.UpdateRequest{Input: otherInput, Revision: other.Revision},
		cookie,
		200,
	)
	// 其他空间的敏感权限不能用于当前空间的列表、统计或导出筛选。
	grant(t, server, admin, first.ID, user.ID, actions)
	for _, suffix := range []string{"", "/statistics", "/export"} {
		request(t, server, "GET", path+suffix+"?has_religion=no", nil, cookie, 403)
		request(t, server, "GET", otherPath+suffix+"?has_religion=yes", nil, cookie, 200)
	}
	grant(t, server, admin, first.ID, user.ID, full)
	for field, expected := range map[string]string{
		"grade":           "2026级",
		"status":          "自定义状态",
		"education_level": "博士",
	} {
		values := decode[[]studentModels.Option](
			t,
			request(t, server, "GET", path+"/options?field="+field, nil, cookie, 200),
		)
		if len(values) != 1 || values[0].Value != expected {
			t.Fatal("年级、状态或学历枚举未返回当前空间的独立字段")
		}
	}

	request(t, server, "GET", fmt.Sprintf("%s/%d", path, other.ID), nil, cookie, 404)
	request(
		t,
		server,
		"PUT",
		fmt.Sprintf("%s/%d", path, other.ID),
		studentModels.UpdateRequest{Input: info.Input, Revision: 1},
		cookie,
		404,
	)
	request(t, server, "DELETE", fmt.Sprintf("%s/%d?revision=1", path, other.ID), nil, cookie, 404)
	options := decode[[]studentModels.Option](
		t,
		request(t, server, "GET", path+"/options?field=gender", nil, cookie, 200),
	)
	if len(options) != 1 || options[0].Value != "女" {
		t.Fatal("基础枚举混入其他空间")
	}
	// 处理敏感信息一个权限同时支持查看、编辑和敏感枚举，不需要学生修改权限。
	grant(t, server, admin, first.ID, user.ID, []string{accessConstants.StudentSensitiveHandle})
	request(t, server, "GET", path+"/options?field=gender", nil, cookie, 403)
	request(t, server, "GET", path+religionExportPath, nil, cookie, 403)
	private := decode[studentModels.SensitiveData](
		t,
		request(t, server, "GET", idPath+"/sensitive", nil, cookie, 200),
	)
	if len(private.Values) != 14 {
		t.Fatal("统一敏感信息目录不是指定的十四个字段")
	}
	options = decode[[]studentModels.Option](
		t,
		request(t, server, "GET", path+"/sensitive-options?field=has_religion", nil, cookie, 200),
	)
	if len(options) != 1 || options[0].Value != "no" {
		t.Fatal("敏感枚举没有复用处理敏感信息权限")
	}
	student = updateSensitive(
		t,
		server,
		cookie,
		path,
		student,
		studentModels.FieldValue{Field: "identity_number", Value: fixtureIdentityNumber},
		studentModels.FieldValue{Field: "home_address", Value: "虚构家庭地址2号"},
		studentModels.FieldValue{Field: "only_child", Value: ""},
		studentModels.FieldValue{Field: "has_religion", Value: "yes"},
	)
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, student.ID).
		Take(&stored).
		Error; err != nil {
		t.Fatal(err)
	}
	if stored.OnlyChild != studentConstants.EnumUnknown ||
		stored.IdentityNumber != fixtureIdentityNumber || stored.BirthDate != "2000-01-02" {
		t.Fatal("全量敏感更新未清空指定值或丢失未修改字段")
	}
	request(t, server, "GET", fmt.Sprintf("%s/%d/sensitive", path, other.ID), nil, cookie, 404)
	request(
		t,
		server,
		"PUT",
		idPath+"/sensitive",
		studentModels.SensitiveRequest{
			Revision: student.Revision,
			Values:   []studentModels.FieldValue{{Field: "gender", Value: "男"}},
		},
		cookie,
		400,
	)
	grant(t, server, admin, first.ID, user.ID, actions)
	checkTransfers(t, server, admin, cookie, first.ID, user.ID, path, actions, full)
	grant(t, server, admin, first.ID, user.ID, full)
	checkAuditRollback(t, server, cookie, path, first.ID, student)
	grant(t, server, admin, first.ID, user.ID, []string{accessConstants.StudentDetail})
	request(t, server, "GET", idPath, nil, cookie, 200)
	request(t, server, "PUT", idPath, studentModels.UpdateRequest{
		Revision:         student.Revision,
		OffCampusDetails: &studentModels.OffCampusValues{},
	}, cookie, 403)
	grant(t, server, admin, first.ID, user.ID, full)
	student = checkOffCampus(t, server, cookie, path, student, other.ID)
	checkStudentDeleteCascade(t, server, cookie, path, first.ID)
	grant(t, server, admin, first.ID, user.ID, actions)
	// 在入口鉴权之后撤权，Svc 在空间锁内再次复核并阻断操作。
	revoke := true
	revokePath := "/v1/workspaces/:workspace_id/students"
	remaining := []string{}
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if revoke && c.Path() == revokePath &&
				c.Request().Method == "GET" {
				revoke = false
				grant(t, server, admin, first.ID, user.ID, remaining)
			}
			return next(c)
		}
	})
	request(t, server, "GET", path, nil, cookie, 403)
	grant(t, server, admin, first.ID, user.ID, full)
	revoke, revokePath, remaining = true, revokePath+"/export", actions
	request(t, server, "GET", path+religionExportPath, nil, cookie, 403)
	// 入口鉴权后撤销敏感权限，三个筛选入口仍在事务内拒绝请求。
	for _, suffix := range []string{"", "/statistics", "/export"} {
		grant(t, server, admin, first.ID, user.ID, full)
		revoke, revokePath, remaining = true,
			"/v1/workspaces/:workspace_id/students"+suffix, actions
		request(t, server, "GET", path+suffix+"?has_religion=no", nil, cookie, 403)
	}
	grant(t, server, admin, first.ID, user.ID, full)
	revoke, revokePath, remaining = true, "/v1/workspaces/:workspace_id/students/:id",
		[]string{accessConstants.AccessWorkspace, accessConstants.StudentUpdate}
	request(t, server, "GET", idPath, nil, cookie, 403)
	request(t, server, "DELETE", "/v1/system/workspaces/"+first.ID, nil, admin, 409)
	grant(t, server, admin, first.ID, user.ID, full)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s?revision=%d", idPath, student.Revision),
		nil,
		cookie,
		200,
	)
	if err := connection.Model(&studentModels.Student{}).
		Where("workspace_id = ? AND id = ?", first.ID, student.ID).
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("学生未被硬删除")
	}
	if err := connection.Model(&auditModels.Record{}).
		Where("workspace_id = ? AND resource_type_id = ? AND instance_id = ?", first.ID, "student", fmt.Sprint(student.ID)).
		Count(&count).
		Error; err != nil ||
		count == 0 {
		t.Fatal("硬删除丢失审计和身份快照")
	}
	if err := connection.Model(&workspaceModels.Workspace{}).
		Where("id = ?", first.ID).
		Update("is_enabled", false).
		Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "GET", path, nil, cookie, 403)
	if err := connection.Model(&userModels.User{}).
		Where("id = ?", user.ID).
		Update("is_enabled", false).
		Error; err != nil {
		t.Fatal(err)
	}
	request(t, server, "GET", path, nil, cookie, 401)
}

func checkSensitiveFilters(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path string,
	status int,
) {
	t.Helper()
	for _, suffix := range []string{"", "/statistics", "/export"} {
		// 所有可枚举的敏感字段都必须同时保护列表、统计及默认基础导出。
		for _, field := range studentConstants.Fields {
			if field.Group != studentConstants.Sensitive || !field.Enumerable {
				continue
			}
			query := url.Values{field.Key: {field.Example}}.Encode()
			request(t, server, "GET", path+suffix+"?"+query, nil, cookie, status)
		}

		// 重复参数、全部枚举值及编码后的条件仍按敏感字段校验权限。
		for _, query := range []string{
			"has_religion=no&has_religion=yes",
			"has_religion=unknown&has_religion=yes&has_religion=no",
			"%68as_religion=%20no%20&has_religion=&gender=女",
			"religion_category=虚构宗教类别&religion_identification=疑似",
		} {
			request(t, server, "GET", path+suffix+"?"+query, nil, cookie, status)
		}

		// 未筛选、基础筛选及归一化后为空的敏感条件保持可用。
		for _, query := range []string{
			"",
			"position_status=assigned&gender=女",
			"status=自定义状态&has_religion=&has_religion=%20%09",
		} {
			request(t, server, "GET", path+suffix+"?"+query, nil, cookie, 200)
		}
	}
}

func checkOffCampus(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path string,
	student studentModels.MutationData,
	otherID uint64,
) studentModels.MutationData {
	t.Helper()
	studentPath := fmt.Sprintf("%s/%d", path, student.ID)
	initial := decode[studentModels.DetailData](
		t, request(t, server, "GET", studentPath, nil, cookie, 200),
	)
	if initial.Revision != student.Revision || initial.OffCampusDetails == nil ||
		*initial.OffCampusDetails != (studentModels.OffCampusValues{}) {
		t.Fatal("未填写的校外住宿明细应返回空字段及学生版本")
	}
	otherPath := fmt.Sprintf("%s/%d", path, otherID)
	request(t, server, "GET", otherPath, nil, cookie, 404)
	request(t, server, "GET", path+"/options?field=advisor_no", nil, cookie, 400)

	details := studentModels.OffCampusValues{
		OffCampusFields: studentModels.OffCampusFields{
			Reason: "虚构原因", Category: "虚构住宿类别", RentalMethod: "虚构租住方式",
			LandlordName: "虚构房东", LandlordPhone: "00001", RoommateName: "虚构合租人",
			RoommateUnit: "虚构单位", RoommatePhone: "00002", Address: "虚构校外地址\n虚构房间",
		},
		StartDate: "2026-09-01",
		EndDate:   "2027-06-30",
	}
	info := initial.Info
	info.Guardian1Name, info.Guardian1Relationship, info.Guardian1Phone = "虚构家长甲", "父亲", "00005"
	info.Guardian2Name, info.Guardian2Relationship, info.Guardian2Phone = "虚构家长乙", "母亲", "00006"
	advisorNo, advisorPhone := "00003", "00004"
	save := studentModels.UpdateRequest{
		Input:            info.Input,
		Revision:         student.Revision,
		OffCampusDetails: &details,
		AdvisorNo:        &advisorNo,
		AdvisorPhone:     &advisorPhone,
	}
	request(t, server, "PUT", otherPath, save, cookie, 404)
	details.EndDate = "2026-08-31"
	request(t, server, "PUT", studentPath, save, cookie, 400)
	details.EndDate = "2027-06-30"
	student = decode[studentModels.MutationData](
		t, request(t, server, "PUT", studentPath, save, cookie, 200),
	)
	request(t, server, "PUT", studentPath, save, cookie, 409)
	current := decode[studentModels.DetailData](
		t, request(t, server, "GET", studentPath, nil, cookie, 200),
	)
	if current.Revision != student.Revision || current.OffCampusDetails == nil ||
		*current.OffCampusDetails != details {
		t.Fatal("校外住宿明细或开始和结束日期未正确保存")
	}
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s?revision=%d", studentPath, student.Revision+1),
		nil,
		cookie,
		409,
	)

	// 校外住宿报表保留 Excel 表头、顺序和无来源列，单列合并起止日期。
	report := decode[studentModels.FileData](t, request(t, server, "GET",
		path+"/export?format=off_campus&keyword=00000001", nil, cookie, 200))
	rows, err := spreadsheettest.Rows(report.Content)
	header := []string{
		"序号", "学院", "提交日期", "姓名", "性别", "民族", "专业", "班级", "学号", "本人手机号",
		"电子邮箱/QQ/微信", "现住宿舍号", "校外住宿原因", "校外住宿起止时间", "校外住宿类别",
		"租住方式", "房东姓名", "房东联系电话", "合租人（若有）姓名", "合租人（若有）单位",
		"合租人（若有）联系电话", "校外居住地详细地址", "家长称谓及姓名", "家长联系电话",
		"家庭详细住址", "日常管理措施", "是否已办理退宿手续", "未办理退宿原因",
		"是否有家人或者伴侣同住", "导师姓名", "导师工号", "导师电话",
	}
	if err != nil || report.Filename != "校外住宿明细表.xlsx" || len(rows) != 2 ||
		!slices.Equal(rows[0], header) || rows[1][3] != info.Name ||
		rows[1][13] != "2026-09-01 至 2027-06-30" ||
		rows[1][21] != details.Address || rows[1][22] != "父亲 虚构家长甲" || rows[1][23] != "00005" ||
		rows[1][24] != "虚构家庭地址2号" ||
		rows[1][30] != advisorNo || rows[1][31] != advisorPhone {
		t.Fatal("校外住宿明细导出的表头、租户、日期或字段映射不一致")
	}
	for _, column := range []int{1, 2, 25, 26, 27, 28} {
		if rows[1][column] != "" {
			t.Fatal("校外住宿明细未保留无数据来源的空列")
		}
	}

	info.AccommodationType = "on_campus"
	changeType := studentModels.UpdateRequest{Input: info.Input, Revision: student.Revision}
	request(t, server, "PUT", studentPath, changeType, cookie, 409)
	changeType.OffCampusDetails = &studentModels.OffCampusValues{}
	student = decode[studentModels.MutationData](
		t, request(t, server, "PUT", studentPath, changeType, cookie, 200),
	)
	current = decode[studentModels.DetailData](
		t,
		request(t, server, "GET", studentPath, nil, cookie, 200),
	)
	if current.AdvisorNo != advisorNo || current.AdvisorPhone != advisorPhone {
		t.Fatal("旧请求省略导师字段时丢失已保存的值")
	}
	if current.ArrivalMonth != "2026-09" || current.PreviousSchoolOrEmployer != "虚构大学" ||
		current.PreviousGraduationMonth != "2026-06" {
		t.Fatal("旧请求省略来校或既往教育字段时丢失已保存的值")
	}
	if current.OffCampusDetails != nil {
		t.Fatal("校内学生的详情不应返回校外住宿明细")
	}
	changeType.Revision = student.Revision
	changeType.OffCampusDetails = &details
	request(t, server, "PUT", studentPath, changeType, cookie, 409)
	return student
}

func checkTransfers(
	t *testing.T,
	server *echo.Echo,
	admin, cookie *http.Cookie,
	workspaceID string,
	userID uint64,
	path string,
	actions, full []string,
) {
	t.Helper()
	file := decode[studentModels.FileData](
		t,
		request(t, server, "GET", path+"/template?include_sensitive=true", nil, cookie, 200),
	)
	if !strings.Contains(spreadsheettest.Text(t, file.Content), "身份证号码") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "是否有宗教信仰") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "枚举字段：") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "出国联培（joint_abroad）") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "宗教信仰起始年份") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "姓名,头像,") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "户籍所在地（省市）,籍贯,") {
		t.Fatal("完整模板缺少住宿枚举说明、头像、籍贯或敏感列")
	}
	if strings.Contains(spreadsheettest.Text(t, file.Content), ",出生日期,") {
		t.Fatal("导入模板包含只读出生日期列")
	}
	grant(t, server, admin, workspaceID, userID, full)
	avatarKey := "workspaces/" + workspaceID + "/students/avatars/虚构头像.png"
	imported := decode[studentModels.ImportData](
		t,
		request(
			t,
			server,
			"POST",
			path+"/import",
			studentModels.ImportRequest{
				Content: spreadsheettest.Fixture(t, "学号,姓名,性别,头像,户籍所在地（省市）,籍贯,"+
					fixtureAcademicHeader+",身份证号码\n"+
					"00000002,虚构导入学生,自填性别,"+avatarKey+",虚构导入户籍,虚构导入籍贯,"+
					fixtureAcademicValues+","+fixtureIdentityNumber+"\n"),
			},
			cookie,
			200,
		),
	)
	if imported.Created != 1 {
		t.Fatal("基础模板没有导入")
	}
	var withAvatar studentModels.Student
	if err := db.DB().WithContext(t.Context()).
		Where("workspace_id = ? AND student_no = ?", workspaceID, "00000002").
		Take(&withAvatar).Error; err != nil || withAvatar.Avatar != avatarKey ||
		withAvatar.Origin != "虚构导入户籍" || withAvatar.NativePlace != "虚构导入籍贯" {
		t.Fatal("导入未独立保存户籍和籍贯，或未原样保存头像的 COS 对象键")
	}
	request(t, server, "POST", path+"/import", studentModels.ImportRequest{
		Content: spreadsheettest.Fixture(
			t,
			"学号,姓名,头像,"+fixtureAcademicHeader+",性别,身份证号码\n"+
				"CROSS-AVATAR,虚构学生,workspaces/w7m2k9p4s6n8c3x5/students/avatars/照片.png,"+
				fixtureAcademicValues+",女,"+fixtureIdentityNumber+"\n",
		),
	}, cookie, 400)
	options := decode[[]studentModels.Option](
		t,
		request(t, server, "GET", path+"/options?field=accommodation_type", nil, cookie, 200),
	)
	unknown := false
	for _, option := range options {
		if option.Value == "unknown" && option.Label == "未知" {
			unknown = true
		}
	}
	if !unknown {
		t.Fatal("未填写的住宿类型没有显示为未知")
	}
	grant(t, server, admin, workspaceID, userID, actions)
	request(
		t,
		server,
		"POST",
		path+"/import",
		studentModels.ImportRequest{
			Content: spreadsheettest.Fixture(t,
				"学号,姓名,身份证号码,"+fixtureAcademicHeader+",性别\n"+
					"00000003,虚构学生,"+fixtureIdentityNumber+","+fixtureAcademicValues+",女\n"),
		},
		cookie,
		403,
	)
	request(t, server, "GET", path+"/export?include_sensitive=true", nil, cookie, 403)
	request(t, server, "GET", path+religionExportPath+"&include_sensitive=false", nil, cookie, 403)
	grant(t, server, admin, workspaceID, userID, full)
	request(
		t,
		server,
		"POST",
		path+"/import",
		studentModels.ImportRequest{
			Content: spreadsheettest.Fixture(t,
				"学号,姓名,"+fixtureAcademicHeader+",性别,身份证号码\n"+
					"00000004,整批回滚学生,"+fixtureAcademicValues+",女,"+fixtureIdentityNumber+"\n"+
					"00000001,重复学生,"+fixtureAcademicValues+",女,"+fixtureIdentityNumber+"\n"),
		},
		cookie,
		409,
	)
	var count int64
	if err := db.DB().
		WithContext(t.Context()).
		Model(&studentModels.Student{}).
		Where("workspace_id = ? AND student_no = ?", workspaceID, "00000004").
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("批量导入没有整批回滚")
	}
	grant(t, server, admin, workspaceID, userID, actions)
	ordinary := decode[studentModels.FileData](
		t,
		request(t, server, "GET", path+"/export", nil, cookie, 200),
	)
	if strings.Contains(spreadsheettest.Text(t, ordinary.Content), "身份证号码") ||
		strings.Contains(spreadsheettest.Text(t, ordinary.Content), "是否独生子女") {
		t.Fatal("基础导出包含敏感列")
	}
	if !strings.Contains(spreadsheettest.Text(t, ordinary.Content), avatarKey) ||
		!strings.Contains(spreadsheettest.Text(t, ordinary.Content), "虚构导入户籍,虚构导入籍贯") {
		t.Fatal("基础导出未保留可再次导入的头像对象键、户籍或籍贯")
	}
	grant(t, server, admin, workspaceID, userID, full)
	request(
		t,
		server,
		"POST",
		path+"/import",
		studentModels.ImportRequest{
			Content: spreadsheettest.Fixture(
				t,
				"学号,姓名,身份证号码,是否有宗教信仰,出生日期,"+fixtureAcademicHeader+",性别\n"+
					"00000003,虚构敏感导入,00000020040229001x,否,2000-01-01,"+fixtureAcademicValues+",女\n"+
					"00000004,筛选外学生,"+fixtureIdentityNumber+",是,2000-01-01,"+fixtureAcademicValues+",女\n",
			),
		},
		cookie,
		200,
	)
	sensitive := decode[studentModels.FileData](
		t,
		request(t, server, "GET", path+"/export?include_sensitive=true", nil, cookie, 200),
	)
	if !strings.Contains(spreadsheettest.Text(t, sensitive.Content), fixtureIdentityNumber) ||
		!strings.Contains(spreadsheettest.Text(t, sensitive.Content), "2004-02-29") {
		t.Fatal("统一授权后的敏感导出缺少证件号码或导入后的派生出生日期")
	}
	report := decode[studentModels.FileData](t, request(
		t,
		server,
		"GET",
		path+religionExportPath+"&keyword=虚构&page=2&page_size=1",
		nil,
		cookie,
		200,
	))
	rows, err := spreadsheettest.Rows(report.Content)
	header := []string{
		"序号", "姓名", "性别", "出生年月", "民族", "籍贯", "身份证号码", "政治面貌",
		"入学年份", "专业", "家庭详细住址", "联系电话", "XJ类别", "XJ标识",
		"XJ起始年份", "家庭成员XJ情况",
	}
	row := []string{
		"1", "虚构学生合并修改", "女", "2000-01", "汉族", "虚构新籍贯县", fixtureIdentityNumber, "",
		"2026", "虚构专业", "虚构家庭地址2号", "FICTIONAL-PHONE-001", "虚构宗教类别", "疑似",
		"2020年", "虚构家庭成员信仰情况",
	}
	if err != nil || report.Filename != "XJ 明细表.xlsx" || len(rows) != 2 ||
		!slices.Equal(rows[0], header) || !slices.Equal(rows[1], row) {
		t.Fatal("宗教信仰明细的租户、关键词、信教状态或字段映射错误")
	}
	empty := decode[studentModels.FileData](t, request(
		t,
		server,
		"GET",
		path+religionExportPath+"&has_religion=no",
		nil,
		cookie,
		200,
	))
	if spreadsheettest.Text(t, empty.Content) != strings.Join(header, ",") {
		t.Fatal("无宗教信仰的筛选条件应只导出表头")
	}
	grant(t, server, admin, workspaceID, userID, actions)
}

func checkAuditRollback(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path, workspaceID string,
	student studentModels.MutationData,
) {
	t.Helper()
	connection := db.DB().WithContext(t.Context())
	var before studentModels.Student
	if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, student.ID).
		Take(&before).
		Error; err != nil {
		t.Fatal(err)
	}
	const callback = "test:student-audit-unavailable"
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
	request(
		t,
		server,
		"POST",
		path,
		studentCreateRequest("AUDIT-ROLLBACK", "虚构回滚学生", "女"),
		cookie,
		503,
	)
	request(t, server, "GET", path, nil, cookie, 503)
	request(t, server, "GET", path+religionExportPath, nil, cookie, 503)
	request(t, server, "GET", path+"/export?format=off_campus", nil, cookie, 503)
	detailPath := fmt.Sprintf("%s/%d", path, student.ID)
	private := []studentModels.FieldValue{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive {
			value := ""
			if field.Key == "identity_number" {
				value = fixtureIdentityNumber
			}
			private = append(private, studentModels.FieldValue{Field: field.Key, Value: value})
		}
	}
	input := studentCreateRequest(before.StudentNo, "合并回滚的虚构学生", before.Gender).Input
	request(t, server, "PUT", detailPath, studentModels.UpdateRequest{
		Input:     input,
		Revision:  student.Revision,
		Sensitive: private,
		OffCampusDetails: &studentModels.OffCampusValues{
			OffCampusFields: studentModels.OffCampusFields{Reason: "虚构回滚明细"},
		},
	}, cookie, 503)

	request(
		t,
		server,
		"POST",
		path+"/import",
		studentModels.ImportRequest{
			Content: spreadsheettest.Fixture(t, "学号,姓名,"+fixtureAcademicHeader+",性别,身份证号码\n"+
				"AUDIT-IMPORT,虚构导入回滚,"+fixtureAcademicValues+",女,"+fixtureIdentityNumber+"\n"),
		},
		cookie,
		503,
	)
	if err := db.DB().Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var after studentModels.Student
	if err := connection.Where("workspace_id = ? AND id = ?", workspaceID, student.ID).
		Take(&after).
		Error; err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("审计故障没有回滚合并更新的全部基础与敏感字段")
	}
	detail := decode[studentModels.DetailData](
		t, request(t, server, "GET", detailPath, nil, cookie, 200),
	)
	if detail.Revision != student.Revision || detail.OffCampusDetails == nil ||
		detail.OffCampusDetails.Reason != "" {
		t.Fatal("审计故障没有回滚校外住宿明细")
	}
	var count int64
	if err := db.DB().
		WithContext(t.Context()).
		Model(&studentModels.Student{}).
		Where("workspace_id = ? AND student_no IN ?", workspaceID, []string{"AUDIT-ROLLBACK", "AUDIT-IMPORT"}).
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("审计故障没有回滚学生写入")
	}
}
