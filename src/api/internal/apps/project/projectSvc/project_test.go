package projectSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentcos"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
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
	if status >= 400 {
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

// TestProjectTenantScoresAndAttachments 只在 CI 指定的独立空库及本机虚构 COS 和 STS 中验收。
func TestProjectTenantScoresAndAttachments(t *testing.T) {
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立项目验收数据库后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_project_test") {
		t.Fatal("项目验收数据库必须以 _project_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("项目验收要求独立空数据库")
	}
	objects := startObjectStore(t)
	_, admin := fixture(t, "fixture.admin", strings.Repeat("a1", 32), true)
	user, cookie := fixture(t, "fixture.project", strings.Repeat("b2", 32), false)
	server := httpd.New()
	createWorkspace := func(name string) workspaceModels.Info {
		return decode[workspaceModels.Info](
			t,
			request(
				t,
				server,
				"POST",
				"/v1/system/workspaces",
				workspaceModels.CreateRequest{Name: name},
				admin,
				200,
			),
		)
	}
	first, second := createWorkspace("虚构项目空间甲"), createWorkspace("虚构项目空间乙")
	actions := make([]string, 0, 20)
	actions = append(actions,
		accessConstants.AccessWorkspace,
		accessConstants.ProjectList,
		accessConstants.ProjectCreate,
		accessConstants.ProjectUpdate,
		accessConstants.ProjectDelete,
		accessConstants.ProjectStatistics,
		accessConstants.ProjectImport,
		accessConstants.ParticipationList,
		accessConstants.ParticipationCreate,
		accessConstants.ParticipationUpdate,
		accessConstants.ParticipationDelete,
		accessConstants.ParticipationImport,
		accessConstants.ProjectAttachmentRead,
		accessConstants.ProjectAttachmentUpload,
		accessConstants.ProjectAttachmentDelete,
		accessConstants.StudentCreate,
		accessConstants.StudentSensitiveHandle,
		accessConstants.StudentDelete,
	)
	grant(t, server, admin, first.ID, user.ID, actions)
	grant(t, server, admin, second.ID, user.ID, actions)
	base := "/v1/workspaces/" + first.ID
	otherBase := "/v1/workspaces/" + second.ID
	// 列表及导入授权不能替代导出，管理员也不能绕过显式授权。
	for _, resource := range []string{"projects", "project-participations"} {
		request(t, server, "GET", base+"/"+resource+"/export", nil, cookie, 403)
		request(t, server, "GET", base+"/"+resource+"/export", nil, admin, 403)
	}
	actions = append(actions, accessConstants.ProjectExport, accessConstants.ParticipationExport)
	grant(t, server, admin, first.ID, user.ID, actions)
	request(t, server, "GET", base+"/projects", nil, admin, 403)
	addProject := func(path, name string, year int) projectModels.ProjectInfo {
		return decode[projectModels.ProjectInfo](
			t,
			request(t, server, "POST", path+"/projects", projectModels.ProjectRequest{
				ProjectInput: projectModels.ProjectInput{
					Name:        name,
					Year:        year,
					Description: "虚构说明",
					DetailNames: []string{"虚构子项", "无分数子项", strconv.Itoa(year) + "年专属子项"},
				},
			}, cookie, 200),
		)
	}
	project := addProject(base, "虚构项目甲", 2025)
	nextYear := addProject(base, project.Name, 2026)
	other := addProject(otherBase, "虚构项目乙", 2026)
	// 尚未参与评分的配置也可选择，名称按项目及归属年份隔离。
	namesPath := base + "/project-participations/detail-names?project_id=" + strconvID(project.ID)
	names := decode[[]projectModels.NameOption](
		t, request(t, server, "GET", namesPath, nil, cookie, 200),
	)
	if len(names) != 3 || !slices.ContainsFunc(names, func(option projectModels.NameOption) bool {
		return option.Value == "2025年专属子项" && option.Label == option.Value
	}) || slices.ContainsFunc(names, func(option projectModels.NameOption) bool {
		return option.Value == "2026年专属子项"
	}) {
		t.Fatal("子项选项未从当前年度项目配置读取")
	}
	request(t, server, "GET", base+"/project-participations/detail-names", nil, cookie, 400)
	request(t, server, "GET", namesPath+"&project_id=0", nil, cookie, 400)
	request(t, server, "GET", otherBase+"/project-participations/detail-names?project_id="+
		strconvID(project.ID), nil, cookie, 404)
	// 两个列表按各自权限枚举当前空间的完整名称和年份，不混入其他空间。
	for _, resource := range []string{"projects", "project-participations"} {
		filters := decode[projectModels.FilterOptionsData](
			t,
			request(t, server, "GET", base+"/"+resource+"/filter-options", nil, cookie, 200),
		)
		if len(filters.Names) != 1 || filters.Names[0].Value != project.Name ||
			len(filters.Years) != 2 ||
			filters.Years[0].Value != "2026" ||
			filters.Years[1].Value != "2025" {
			t.Fatal("筛选枚举混入其他空间或没有完整去重")
		}
		request(t, server, "GET", base+"/"+resource+"/filter-options", nil, admin, 403)
	}
	exact := decode[projectModels.ProjectList](
		t,
		request(
			t,
			server,
			"GET",
			base+"/projects?project_name="+url.QueryEscape(project.Name),
			nil,
			cookie,
			200,
		),
	)
	partial := decode[projectModels.ProjectList](
		t,
		request(
			t,
			server,
			"GET",
			base+"/projects?project_name="+url.QueryEscape("虚构"),
			nil,
			cookie,
			200,
		),
	)
	if exact.Total != 2 || partial.Total != 0 ||
		exact.Items[0].ID != project.ID || exact.Items[1].ID != nextYear.ID {
		t.Fatal("枚举项目名称未按完整名称精确匹配，或项目未按归属年份正序排列")
	}
	multiple := url.Values{
		"project_name": {other.Name, project.Name},
		"year":         {"2025", "2026"},
	}
	exact = decode[projectModels.ProjectList](t, request(t, server, "GET",
		base+"/projects?"+multiple.Encode(), nil, cookie, 200))
	if exact.Total != 2 {
		t.Fatal("多选项目名称与年份未取并集，或混入其他空间项目")
	}

	addStudent := func(path, number, educationLevel, major, grade string) studentModels.MutationData {
		values := []studentModels.FieldValue{}
		for _, field := range studentConstants.Fields {
			if field.Group == studentConstants.Sensitive {
				value := ""
				if field.Key == "identity_number" {
					value = "00000020000102001X"
				}
				values = append(values, studentModels.FieldValue{Field: field.Key, Value: value})
			}
		}
		return decode[studentModels.MutationData](
			t,
			request(
				t,
				server,
				"POST",
				path+"/students",
				studentModels.CreateRequest{
					Input: studentModels.Input{
						StudentNo: number, Name: "虚构项目学生", Grade: grade, Status: "在籍",
						EducationLevel: educationLevel, Major: major, ClassName: "虚构班级",
						AdmissionType: "非定向", Gender: "女", Ethnicity: "汉族",
						AccommodationType: "on_campus",
					},
					Sensitive: values,
				},
				cookie,
				200,
			),
		)
	}
	student := addStudent(base, "00000001", "硕士", "虚构专业甲", "2025级")
	student2 := addStudent(base, "00000002", "博士", "虚构专业乙", "2026级")
	otherStudent := addStudent(otherBase, "00000003", "学士", "其他空间专业", "2024级")
	rank := int32(3)
	input := projectModels.ParticipationInput{
		ParticipationFields: projectModels.ParticipationFields{
			ProjectID: project.ID,
			StudentID: student.ID,
			Rank:      &rank,
			Score:     score(90.123456),
		},
		Details: []projectModels.ScoreItem{
			{Name: "虚构子项", Description: strings.Repeat("虚构详情", 100), Score: score(5.123456)},
			{Name: "无分数子项"},
		},
	}
	createParticipation := func(input projectModels.ParticipationInput, status int) *httptest.ResponseRecorder {
		return request(
			t,
			server,
			"POST",
			base+"/project-participations",
			projectModels.ParticipationRequest{ParticipationInput: input},
			cookie,
			status,
		)
	}
	for _, invalidRank := range []int32{0, -1} {
		invalidInput := input
		invalidInput.Rank = &invalidRank
		createParticipation(invalidInput, 400)
	}
	for _, name := range []string{"未配置子项", "2026年专属子项"} {
		invalidInput := input
		invalidInput.Details = []projectModels.ScoreItem{{Name: name}}
		createParticipation(invalidInput, 400)
	}
	record := decode[projectModels.ParticipationInfo](t, createParticipation(input, 200))
	if record.Rank == nil || *record.Rank != 3 ||
		record.Score == nil || *record.Score != 90.123456 || record.Year != 2025 ||
		len(record.Details) != 2 ||
		record.Details[1].Score != nil {
		t.Fatal("排名、总分、项目年份、空子分数或长详情未独立保留")
	}
	names = decode[[]projectModels.NameOption](
		t,
		request(t, server, "GET", namesPath, nil, cookie, 200),
	)
	if len(names) != 3 {
		t.Fatal("保存评分后丢失未使用的子项配置")
	}
	invalidInput := input
	invalidInput.Details = []projectModels.ScoreItem{{Name: "未配置子项"}}
	request(t, server, "PUT", base+"/project-participations/"+strconvID(record.ID),
		projectModels.ParticipationRequest{
			ParticipationInput: invalidInput, Revision: record.Revision,
		}, cookie, 400)
	configured := project.ProjectInput
	configured.DetailNames = []string{"2025年专属子项"}
	request(
		t,
		server,
		"PUT",
		base+"/projects/"+strconvID(project.ID),
		projectModels.ProjectRequest{
			ProjectInput: configured,
			Revision:     project.Revision,
		},
		cookie,
		409,
	)
	// 配置修改失败时项目版本及原始配置必须一并回滚。
	unchanged := decode[projectModels.ProjectList](t, request(
		t,
		server,
		"GET",
		base+"/projects?project_name="+url.QueryEscape(
			project.Name,
		)+"&year=2025",
		nil,
		cookie,
		200,
	))
	if len(unchanged.Items) != 1 || unchanged.Items[0].Revision != project.Revision ||
		!slices.Equal(unchanged.Items[0].DetailNames, project.DetailNames) {
		t.Fatal("移除已引用子项没有回滚配置及版本")
	}
	if err := connection.Create(
		&projectModels.Detail{
			WorkspaceID: second.ID, ParticipationID: record.ID,
			ProjectID: project.ID, Name: "跨租户非法子项",
		},
	).Error; err == nil {
		t.Fatal("独立子项表未通过复合外键保护租户")
	}
	for _, detail := range []projectModels.Detail{
		{
			WorkspaceID: first.ID, ParticipationID: record.ID, ProjectID: project.ID,
			SortOrder: 2, Name: "未配置子项",
		},
		{
			WorkspaceID: first.ID, ParticipationID: record.ID, ProjectID: nextYear.ID,
			SortOrder: 2, Name: "2026年专属子项",
		},
	} {
		if err := connection.Create(&detail).Error; err == nil {
			t.Fatal("评分明细未通过外键保护所属项目及子项配置")
		}
	}
	request(
		t,
		server,
		"GET",
		fmt.Sprintf("%s/project-participations/%d", otherBase, record.ID),
		nil,
		cookie,
		404,
	)
	createParticipation(input, 409)
	input.StudentID = student2.ID
	input.Rank = nil
	input.Score = nil
	unscored := decode[projectModels.ParticipationInfo](t, createParticipation(input, 200))
	if unscored.Rank != nil {
		t.Fatal("未填写的排名没有保持空值")
	}
	// 替换完整评分时可以切换到配置相同名称的项目，明细关联随事务更新。
	for _, projectID := range []uint64{nextYear.ID, project.ID} {
		moved := projectModels.ParticipationInput{
			ParticipationFields: unscored.ParticipationFields,
			Details:             unscored.Details,
		}
		moved.ProjectID = projectID
		unscored = decode[projectModels.ParticipationInfo](t, request(t, server, "PUT",
			base+"/project-participations/"+strconvID(unscored.ID),
			projectModels.ParticipationRequest{
				ParticipationInput: moved, Revision: unscored.Revision,
			}, cookie, 200))
		if unscored.ProjectID != projectID || len(unscored.Details) != 2 {
			t.Fatal("切换项目未保留评分或同步明细关联")
		}
	}
	input.StudentID, input.ProjectID = student.ID, nextYear.ID
	nextYearRecord := decode[projectModels.ParticipationInfo](t, createParticipation(input, 200))
	input.StudentID = otherStudent.ID
	createParticipation(input, 400)
	input.StudentID, input.ProjectID = student.ID, other.ID
	createParticipation(input, 400)
	request(
		t,
		server,
		"PUT",
		fmt.Sprintf("%s/projects/%d", base, other.ID),
		projectModels.ProjectRequest{ProjectInput: project.ProjectInput, Revision: 1},
		cookie,
		404,
	)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/projects/%d?revision=1", base, other.ID),
		nil,
		cookie,
		404,
	)
	list := decode[projectModels.ParticipationList](
		t,
		request(
			t,
			server,
			"GET",
			base+"/project-participations?student=00000001",
			nil,
			cookie,
			200,
		),
	)
	if list.Total != 2 || list.Items[0].ID != record.ID || list.Items[1].ID != nextYearRecord.ID {
		t.Fatal("学生筛选或参与记录按归属年份正序排列不正确")
	}
	query := fmt.Sprintf("%s/project-participations?project_id=%d&year=2025", base, project.ID)
	list = decode[projectModels.ParticipationList](
		t,
		request(t, server, "GET", query, nil, cookie, 200),
	)
	if list.Total != 2 || list.Items[0].ID != unscored.ID || list.Items[0].Score != nil ||
		list.Items[0].Rank != nil || list.Items[1].Rank == nil || *list.Items[1].Rank != 3 ||
		list.Items[1].ID != record.ID || list.Items[1].Score == nil {
		t.Fatal("参与记录未按排名正序排列，或排名、空分数语义发生变化")
	}

	// 排名优先于编号；排名相同时编号倒序，清空排名后重新排在已填写值前。
	largerRank := int32(4)
	for _, rank := range []*int32{&largerRank, record.Rank, nil} {
		unscored.Rank = rank
		unscored = decode[projectModels.ParticipationInfo](t, request(
			t, server, "PUT", fmt.Sprintf("%s/project-participations/%d", base, unscored.ID),
			projectModels.ParticipationRequest{
				ParticipationInput: projectModels.ParticipationInput{
					ParticipationFields: unscored.ParticipationFields,
					Details:             unscored.Details,
				},
				Revision: unscored.Revision,
			}, cookie, 200,
		))
		ordered := decode[projectModels.ParticipationList](t, request(
			t, server, "GET", query, nil, cookie, 200,
		))
		want := []uint64{unscored.ID, record.ID}
		if rank != nil && *rank > *record.Rank {
			want = []uint64{record.ID, unscored.ID}
		}
		if len(ordered.Items) != 2 || ordered.Items[0].ID != want[0] ||
			ordered.Items[1].ID != want[1] {
			t.Fatal("参与记录未按排名正序及相同排名下的编号逆序排列")
		}
	}
	stats := decode[[]projectModels.Statistic](
		t,
		request(t, server, "GET", base+"/projects/statistics", nil, cookie, 200),
	)
	if len(stats) != 2 || stats[0].Year != 2025 || stats[0].Count != 2 || stats[1].Count != 1 {
		t.Fatal("统计没有按项目年份计人数")
	}
	checkDetailSearch(t, server, base, cookie, project, other)
	input = projectModels.ParticipationInput{
		ParticipationFields: record.ParticipationFields,
		Details:             record.Details,
	}
	input.Rank, input.Score, input.Details[0].Score = nil, nil, score(0)
	updated := decode[projectModels.ParticipationInfo](
		t,
		request(
			t,
			server,
			"PUT",
			fmt.Sprintf("%s/project-participations/%d", base, record.ID),
			projectModels.ParticipationRequest{
				ParticipationInput: input,
				Revision:           record.Revision,
			},
			cookie,
			200,
		),
	)
	if updated.Rank != nil || updated.Score != nil ||
		updated.Details[0].Score == nil || *updated.Details[0].Score != 0 {
		t.Fatal("清空排名、空总分或零子分数未被保存")
	}
	request(
		t,
		server,
		"PUT",
		fmt.Sprintf("%s/project-participations/%d", base, record.ID),
		projectModels.ParticipationRequest{ParticipationInput: input, Revision: record.Revision},
		cookie,
		409,
	)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/projects/%d?revision=1", base, project.ID),
		nil,
		cookie,
		409,
	)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/students/%d?revision=1", base, student.ID),
		nil,
		cookie,
		409,
	)
	file := uploadFile(t, server, base, project.ID, cookie, 200)
	if file.Category != "project" || file.State != "ready" || !objects.contains(first.ID) {
		t.Fatal("文件没有使用工作空间及项目原始文件名路径上传到私有存储")
	}
	projectList := decode[projectModels.ProjectList](t, request(
		t,
		server,
		"GET",
		base+"/projects?project_name="+url.QueryEscape(
			project.Name,
		)+"&year="+strconv.Itoa(
			project.Year,
		),
		nil,
		cookie,
		200,
	))
	if len(projectList.Items) != 1 || len(projectList.Items[0].AttachmentIDs) != 1 ||
		projectList.Items[0].AttachmentIDs[0] != file.ID {
		t.Fatal("上传后项目未直接返回所属附件")
	}
	project = projectList.Items[0]
	checkProjectExports(t, server, base, cookie, project, projectModels.ParticipationInput{
		ParticipationFields: projectModels.ParticipationFields{
			ProjectID: nextYear.ID, StudentID: student2.ID, Score: score(0),
		},
	})
	for _, resource := range []string{"projects", "project-participations"} {
		request(t, server, "GET", otherBase+"/"+resource+"/export", nil, cookie, 403)
	}
	var storedFile attachmentModels.ProjectAttachment
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, file.ID).
		Take(&storedFile).
		Error; err != nil {
		t.Fatal(err)
	}
	objectPattern := regexp.MustCompile(fmt.Sprintf(
		`^workspaces/%s/projects/%d/[0-9a-f]{10}/%s$`,
		regexp.QuoteMeta(first.ID),
		project.ID,
		regexp.QuoteMeta(file.Filename),
	))
	if !objectPattern.MatchString(storedFile.ObjectKey) || storedFile.ProjectID != project.ID {
		t.Fatal("项目附件表未保存预期的 COS 路径或所属项目")
	}
	expectedObject := storedFile.ObjectKey

	// 同名上传也占用独立名额，达到上限时不得写入新对象。
	func() {
		maxAttachments := config.Config.Project.MaxAttachments
		config.Config.Project.MaxAttachments = 1
		defer func() { config.Config.Project.MaxAttachments = maxAttachments }()
		before := objects.writes.Load()
		uploadNamedFile(t, server, base, project.ID, "其他文件.pdf", cookie, 409)
		uploadNamedFile(t, server, base, project.ID, file.Filename, cookie, 409)
		if objects.writes.Load() != before {
			t.Fatal("附件已达上限时仍上传了新对象")
		}
	}()

	// 同名文件生成独立编号和随机路径，删除其中一个不影响原附件。
	sameName := uploadNamedFile(t, server, base, project.ID, file.Filename, cookie, 200)
	var sameNameStored attachmentModels.ProjectAttachment
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, sameName.ID).
		Take(&sameNameStored).Error; err != nil {
		t.Fatal(err)
	}
	if sameName.ID == file.ID || sameName.Filename != file.Filename ||
		!objectPattern.MatchString(sameNameStored.ObjectKey) ||
		sameNameStored.ObjectKey == expectedObject || sameNameStored.ProjectID != project.ID {
		t.Fatal("同名上传未生成独立的附件编号、随机路径或所属项目")
	}
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, file.ID).
		Take(&storedFile).Error; err != nil {
		t.Fatal(err)
	}
	if storedFile.ObjectKey != expectedObject || storedFile.Revision != file.Revision ||
		storedFile.State != "ready" {
		t.Fatal("同名上传改变了原附件的路径、版本或状态")
	}
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, sameName.ID, sameName.Revision),
		nil,
		cookie,
		200,
	)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, file.ID, file.Revision-1),
		nil,
		cookie,
		409,
	)
	download := request(
		t, server, "GET", fmt.Sprintf("%s/attachments/%d/download", base, file.ID),
		nil, cookie, 200,
	)
	link := decode[attachmentModels.URLData](t, download)
	target, err := url.Parse(link.URL)
	if err != nil || target.Host != "cdn.example.com" || target.Path != "/"+expectedObject ||
		target.Query().Get("eo-sign") == "" ||
		link.Filename != file.Filename || download.Header().Get("Location") != "" ||
		download.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("下载没有返回已授权的文件名及短时 CDN 地址")
	}
	request(
		t, server, "GET", fmt.Sprintf("%s/attachments/%d/download", otherBase, file.ID),
		nil, cookie, 404,
	)
	uploadFile(t, server, otherBase, project.ID, cookie, 400)
	checkProjectAuditRollback(t, server, base, cookie, project, updated, objects)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, file.ID, file.Revision),
		nil,
		cookie,
		200,
	)
	if objects.contains(first.ID) {
		t.Fatal("显式删除没有清理存储对象")
	}
	// 删除故障保留已审计的删除意图，刷新版本后显式重试，不恢复为可下载状态。
	retryFile := uploadFile(t, server, base, project.ID, cookie, 200)
	objects.failDelete.Store(true)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, retryFile.ID, retryFile.Revision),
		nil,
		cookie,
		503,
	)
	var pending attachmentModels.ProjectAttachment
	if err := connection.Where("workspace_id = ? AND id = ?", first.ID, retryFile.ID).
		Take(&pending).
		Error; err != nil ||
		pending.State != "deleting" {
		t.Fatal("删除故障没有保留待重试记录")
	}
	objects.failDelete.Store(false)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, pending.ID, pending.Revision),
		nil,
		cookie,
		200,
	)
	// 参与记录删除直接清理全部评分子项，并保留删除前的完整明细。
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/project-participations/%d?revision=%d", base, updated.ID, updated.Revision),
		nil,
		cookie,
		200,
	)
	var retained int64
	if err := connection.Model(&projectModels.Detail{}).
		Where("workspace_id = ? AND participation_id = ?", first.ID, updated.ID).
		Count(&retained).
		Error; err != nil || retained != 0 {
		t.Fatal("删除参与记录未级联清理评分子项")
	}
	var deletion auditModels.Record
	if err := connection.Where(
		"workspace_id = ? AND instance_id = ? AND action_id = ? AND result_code = ?",
		first.ID, strconvID(updated.ID), accessConstants.ParticipationDelete, 0,
	).Take(&deletion).Error; err != nil {
		t.Fatal(err)
	}
	var snapshot projectModels.ParticipationInfo
	if err := json.Unmarshal(deletion.InstanceOriginData, &snapshot); err != nil ||
		snapshot.StudentID != updated.StudentID || len(snapshot.Details) == 0 ||
		!reflect.DeepEqual(snapshot.Details, updated.Details) {
		t.Fatal("级联删除丢失评分子项或学生身份快照")
	}
	checkProjectDetailConfiguration(t, server, base, cookie)
	checkProjectImport(t, server, base, cookie)
	checkApprovalUserWorkspaceAccess(t, server, first.ID, second.ID, admin, cookie)
	// 入口检查之后撤权，Svc 必须在工作空间锁内重新鉴权。
	revoke := true
	revokePath := "/v1/workspaces/:workspace_id/projects"
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if revoke && c.Path() == revokePath &&
				c.Request().Method == "GET" {
				revoke = false
				grant(t, server, admin, first.ID, user.ID, []string{})
			}
			return next(c)
		}
	})
	request(t, server, "GET", base+"/projects", nil, cookie, 403)
	request(
		t,
		server,
		"POST",
		base+"/project-participations",
		projectModels.ParticipationRequest{ParticipationInput: input},
		cookie,
		403,
	)
	grant(t, server, admin, first.ID, user.ID, []string{accessConstants.ProjectStatistics})
	request(t, server, "GET", base+"/projects/statistics", nil, cookie, 200)
	request(t, server, "GET", base+"/projects", nil, cookie, 403)
	for _, item := range []struct{ action, resource, other string }{
		{accessConstants.ProjectExport, "projects", "project-participations"},
		{accessConstants.ParticipationExport, "project-participations", "projects"},
	} {
		// 各类导出独立工作，不要求列表权限，也不获得另一类导出权限。
		grant(t, server, admin, first.ID, user.ID, []string{item.action})
		request(t, server, "GET", base+"/"+item.resource+"/export", nil, cookie, 200)
		request(t, server, "GET", base+"/"+item.other+"/export", nil, cookie, 403)
		request(t, server, "GET", base+"/"+item.resource, nil, cookie, 403)
		revokePath = "/v1/workspaces/:workspace_id/" + item.resource + "/export"
		revoke = true
		request(t, server, "GET", base+"/"+item.resource+"/export", nil, cookie, 403)
	}
}

func score(value float64) *float64  { return &value }
func strconvID(value uint64) string { return fmt.Sprint(value) }

type objectStore struct {
	mu         sync.Mutex
	objects    map[string][]byte
	types      map[string]string
	failDelete atomic.Bool
	writes     atomic.Int64
}

func startObjectStore(t *testing.T) *objectStore {
	t.Helper()
	originalCOS, originalSTS, originalConfig := tencentcos.Client, tencentcos.STSClient, config.Config.COS
	config.Config.COS.Enabled = true
	config.Config.COS.Bucket = "fixture-project-1250000000"
	config.Config.COS.Region = "ap-guangzhou"
	t.Cleanup(func() {
		tencentcos.Client, tencentcos.STSClient, config.Config.COS = originalCOS, originalSTS, originalConfig
	})
	store := &objectStore{objects: map[string][]byte{}, types: map[string]string{}}
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && r.Header.Get("X-TC-Action") == "GetFederationToken" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{
					"Credentials": map[string]string{
						"TmpSecretId":  "fixture-id",
						"TmpSecretKey": "fixture-key",
						"Token":        "fixture-token",
					},
					"ExpiredTime": time.Now().Add(time.Hour).Unix(), "RequestId": "fixture-request",
				}})
				return
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			if r.Header.Get("Authorization") == "" || r.Header.Get("X-Cos-Acl") != "" {
				w.WriteHeader(403)
				return
			}
			switch r.Method {
			case "PUT":
				data, err := io.ReadAll(r.Body)
				if err != nil {
					w.WriteHeader(400)
					return
				}
				store.objects[r.URL.Path] = data
				store.types[r.URL.Path] = r.Header.Get("Content-Type")
				store.writes.Add(1)
				w.Header().Set("ETag", `"fixture"`)
				w.WriteHeader(200)
			case "HEAD":
				data, ok := store.objects[r.URL.Path]
				if !ok {
					w.WriteHeader(404)
					return
				}
				w.Header().Set("Content-Length", strconv.Itoa(len(data)))
				w.Header().Set("Content-Type", store.types[r.URL.Path])
				w.WriteHeader(200)
			case "DELETE":
				if store.failDelete.Load() {
					w.WriteHeader(503)
					return
				}
				delete(store.objects, r.URL.Path)
				w.WriteHeader(204)
			default:
				w.WriteHeader(405)
			}
		}),
	)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	tencentcos.Client = cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID: "fixture-id", SecretKey: "fixture-key", Transport: server.Client().Transport,
		},
	})
	tencentcos.Client.DisableURLCheck()
	tencentcos.Client.Conf.RetryOpt.Count = 1
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint, clientProfile.HttpProfile.Scheme = endpoint.Host, "http"
	tencentcos.STSClient, err = sts.NewClient(common.NewCredential("fixture-id", "fixture-key"),
		"ap-guangzhou", clientProfile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return store
}
func (store *objectStore) contains(workspaceID string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	for path, data := range store.objects {
		if strings.Contains(path, "/workspaces/"+workspaceID+"/projects/") && len(data) > 0 {
			return true
		}
	}
	return false
}

func uploadFile(
	t *testing.T,
	server *echo.Echo,
	base string,
	projectID uint64,
	cookie *http.Cookie,
	status int,
) attachmentModels.Info {
	return uploadNamedFile(
		t,
		server,
		base,
		projectID,
		fmt.Sprintf("虚构项目说明-%d.txt", uploadSequence.Add(1)),
		cookie,
		status,
	)
}

var uploadSequence atomic.Uint64

func uploadNamedFile(
	t *testing.T,
	server *echo.Echo,
	base string,
	projectID uint64,
	filename string,
	cookie *http.Cookie,
	status int,
) attachmentModels.Info {
	t.Helper()
	const content = "%PDF-1.7\nfictional project attachment\n%%EOF"
	path := fmt.Sprintf("%s/projects/%d/attachments", base, projectID)
	recorder := request(t, server, "POST", path, attachmentModels.UploadRequest{
		Filename: filename, Size: int64(len(content)), ContentType: "application/pdf",
	}, cookie, status)
	if status != 200 {
		return attachmentModels.Info{}
	}
	prepared := decode[attachmentModels.UploadData](t, recorder)
	client := cos.NewClient(tencentcos.Client.BaseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID: prepared.Upload.TmpSecretID, SecretKey: prepared.Upload.TmpSecretKey,
			SessionToken: prepared.Upload.SecurityToken,
		},
	})
	client.DisableURLCheck()
	_, err := client.Object.Put(
		t.Context(),
		prepared.Upload.Key,
		strings.NewReader(content),
		&cos.ObjectPutOptions{
			ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
				ContentType: prepared.Upload.ContentType,
			},
		},
	)
	if err != nil {
		t.Fatal("虚构 COS 直传失败")
	}
	return decode[attachmentModels.Info](t, request(
		t,
		server,
		"POST",
		path+"/complete",
		attachmentModels.CompleteUploadRequest{
			UploadID: prepared.UploadID,
			Revision: prepared.Revision,
		},
		cookie,
		200,
	))
}

func checkProjectAuditRollback(
	t *testing.T,
	server *echo.Echo,
	base string,
	cookie *http.Cookie,
	project projectModels.ProjectInfo,
	participation projectModels.ParticipationInfo,
	store *objectStore,
) {
	t.Helper()
	owned := decode[projectModels.ProjectInfo](t, request(
		t, server, "POST", base+"/projects", projectModels.ProjectRequest{
			ProjectInput: projectModels.ProjectInput{
				Name: "虚构级联回滚项目", Year: 2026, DetailNames: []string{"自有配置"},
			},
		}, cookie, 200,
	))
	anotherFile := uploadFile(t, server, base, project.ID, cookie, 200)
	before := store.writes.Load()
	const callback = "test:project-audit-unavailable"
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
	request(
		t,
		server,
		"POST",
		base+"/projects",
		projectModels.ProjectRequest{
			ProjectInput: projectModels.ProjectInput{
				Name: "虚构回滚项目", Year: 2026, DetailNames: []string{"回滚新增子项"},
			},
		},
		cookie,
		503,
	)
	request(t, server, "GET", base+"/projects", nil, cookie, 503)
	request(t, server, "GET", base+"/projects/filter-options", nil, cookie, 503)
	request(t, server, "GET", base+"/projects/export", nil, cookie, 503)
	request(t, server, "GET", base+"/project-participations/export", nil, cookie, 503)
	// 审计失败时，父记录及数据库级联删除的明细必须全部回滚。
	request(t, server, "DELETE", base+"/projects/"+strconvID(owned.ID)+"?revision=1",
		nil, cookie, 503)
	request(t, server, "DELETE", fmt.Sprintf(
		"%s/project-participations/%d?revision=%d", base, participation.ID, participation.Revision,
	), nil, cookie, 503)
	request(
		t,
		server,
		"GET",
		fmt.Sprintf("%s/attachments/%d/download", base, project.AttachmentIDs[0]),
		nil,
		cookie,
		503,
	)
	request(
		t,
		server,
		"PUT",
		fmt.Sprintf("%s/projects/%d", base, project.ID),
		projectModels.ProjectRequest{
			ProjectInput: projectModels.ProjectInput{
				Name:        "虚构回滚修改",
				Year:        2026,
				DetailNames: append(slices.Clone(project.DetailNames), "回滚新增子项"),
			},
			Revision: project.Revision,
		},
		cookie,
		503,
	)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, anotherFile.ID, anotherFile.Revision),
		nil,
		cookie,
		503,
	)
	uploadFile(t, server, base, project.ID, cookie, 503)
	if store.writes.Load() != before {
		t.Fatal("审计未提交即上传了对象")
	}
	if err := db.DB().Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	workspaceID := strings.TrimPrefix(base, "/v1/workspaces/")
	var option projectModels.DetailOption
	if err := db.DB().WithContext(t.Context()).Where(
		"workspace_id = ? AND project_id = ?", workspaceID, owned.ID,
	).Take(&option).Error; err != nil || option.Name != "自有配置" {
		t.Fatal("审计故障没有回滚项目自有配置的级联删除")
	}
	unchanged := decode[projectModels.ParticipationInfo](t, request(
		t, server, "GET", base+"/project-participations/"+strconvID(participation.ID),
		nil, cookie, 200,
	))
	if unchanged.Revision != participation.Revision ||
		!reflect.DeepEqual(unchanged.Details, participation.Details) {
		t.Fatal("审计故障没有回滚参与记录及评分子项的级联删除")
	}
	request(t, server, "DELETE", base+"/projects/"+strconvID(owned.ID)+"?revision=1",
		nil, cookie, 200)
	request(
		t,
		server,
		"DELETE",
		fmt.Sprintf("%s/attachments/%d?revision=%d", base, anotherFile.ID, anotherFile.Revision),
		nil,
		cookie,
		200,
	)
	var count int64
	if err := db.DB().
		WithContext(t.Context()).
		Model(&projectModels.Project{}).
		Where("workspace_id = ? AND name IN ?", strings.TrimPrefix(base, "/v1/workspaces/"), []string{"虚构回滚项目", "虚构回滚修改"}).
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("审计故障没有回滚业务")
	}
	if err := db.DB().WithContext(t.Context()).Model(&projectModels.DetailOption{}).
		Where("workspace_id = ? AND name = ?",
			strings.TrimPrefix(base, "/v1/workspaces/"),
			"回滚新增子项").
		Count(&count).Error; err != nil || count != 0 {
		t.Fatal("审计故障没有回滚新增子项配置")
	}
}

func checkProjectDetailConfiguration(
	t *testing.T,
	server *echo.Echo,
	base string,
	cookie *http.Cookie,
) {
	t.Helper()
	input := projectModels.ProjectInput{
		Name: "虚构配置管理项目", Year: 2026, DetailNames: []string{" 配置子项 ", "未使用子项"},
	}
	for _, names := range [][]string{{"重复", " 重复 "}, {""}, {strings.Repeat("长", 101)}} {
		invalid := input
		invalid.DetailNames = names
		request(t, server, "POST", base+"/projects",
			projectModels.ProjectRequest{ProjectInput: invalid}, cookie, 400)
	}
	created := decode[projectModels.ProjectInfo](t, request(t, server, "POST", base+"/projects",
		projectModels.ProjectRequest{ProjectInput: input}, cookie, 200))
	if !slices.Contains(created.DetailNames, "配置子项") {
		t.Fatal("项目配置没有清理名称两端空白")
	}
	path := base + "/projects/" + strconvID(created.ID)
	request(t, server, "DELETE", path+"?revision=2", nil, cookie, 409)
	input.DetailNames = []string{"新配置子项"}
	updated := decode[projectModels.ProjectInfo](t, request(t, server, "PUT", path,
		projectModels.ProjectRequest{
			ProjectInput: input,
			Revision:     created.Revision,
		}, cookie, 200))
	if !slices.Equal(updated.DetailNames, input.DetailNames) || updated.Revision != 2 {
		t.Fatal("项目修改未替换未使用配置或推进并发版本")
	}
	input.DetailNames = []string{}
	request(t, server, "PUT", path,
		projectModels.ProjectRequest{ProjectInput: input, Revision: created.Revision}, cookie, 409)
	updated = decode[projectModels.ProjectInfo](t, request(t, server, "PUT", path,
		projectModels.ProjectRequest{ProjectInput: input, Revision: updated.Revision}, cookie, 200))
	request(t, server, "DELETE", path+"?revision="+strconvID(updated.Revision), nil, cookie, 200)
	var count int64
	if err := db.DB().WithContext(t.Context()).Model(&projectModels.DetailOption{}).
		Where("workspace_id = ? AND project_id = ?",
			strings.TrimPrefix(base, "/v1/workspaces/"), created.ID).
		Count(&count).Error; err != nil || count != 0 {
		t.Fatal("删除项目未级联清理未使用的子项配置")
	}
}

func checkProjectImport(t *testing.T, server *echo.Echo, base string, cookie *http.Cookie) {
	t.Helper()
	template := decode[projectModels.FileData](t,
		request(t, server, "GET", base+"/projects/template", nil, cookie, 200))
	rows, err := spreadsheettest.Rows(template.Content)
	if err != nil || len(rows) == 0 ||
		!slices.Equal(rows[0], []string{"项目名称", "归属年份", "说明", "子项名称(JSON)"}) {
		t.Fatalf("奖项导入模板必须仅包含四列基础资料：%v", err)
	}
	request(t, server, "GET", base+"/project-participations/template", nil, cookie, 200)
	for _, column := range []string{"审批流(JSON)", "分数计算(JSON)"} {
		content := "项目名称,归属年份,说明,子项名称(JSON)," + column + "\n" +
			"虚构配置导入项目,2026,,[],[]\n"
		request(t, server, "POST", base+"/projects/import", projectModels.ImportRequest{
			Content: spreadsheettest.Fixture(t, content),
		}, cookie, 400)
	}
	request(
		t,
		server,
		"POST",
		base+"/projects/import",
		projectModels.ImportRequest{
			Content: spreadsheettest.Fixture(t,
				"项目名称,归属年份,说明,子项名称(JSON)\n虚构导入项目,2026,说明,\"[\"\"导入子项\"\"]\"\n"),
		},
		cookie,
		200,
	)
	projects := decode[projectModels.ProjectList](t, request(t, server, "GET",
		base+"/projects?project_name="+url.QueryEscape("虚构导入项目"), nil, cookie, 200))
	if len(projects.Items) != 1 || projects.Items[0].AllowSelfApply ||
		projects.Items[0].SelfApplyDeadline != nil || len(projects.Items[0].ApprovalSteps) != 0 ||
		projects.Items[0].ScoreCalculation != nil {
		t.Fatal("基础奖项导入不应包含审批设置或分数计算配置")
	}
	// 导入评分不得创建项目中未配置的名称，失败后仍可导入同一学生。
	request(t, server, "POST", base+"/project-participations/import",
		projectModels.ImportRequest{Content: spreadsheettest.Fixture(t,
			`项目名称,归属年份,学号,排名,总分,评分子项(JSON)
虚构导入项目,2026,00000001,1,,"[{""name"":""未配置子项"",""description"":"""",""score"":null}]"
`)}, cookie, 400)
	request(
		t,
		server,
		"POST",
		base+"/project-participations/import",
		projectModels.ImportRequest{
			Content: spreadsheettest.Fixture(
				t,
				"项目名称,归属年份,学号,排名,总分,评分子项(JSON)\n虚构导入项目,2026,00000001,1,,[]\n",
			),
		},
		cookie,
		200,
	)
	imported := decode[projectModels.ParticipationList](t, request(
		t, server, "GET", base+"/project-participations?project_name="+url.QueryEscape("虚构导入项目"),
		nil, cookie, 200,
	))
	if imported.Total != 1 || imported.Items[0].Rank == nil || *imported.Items[0].Rank != 1 ||
		imported.Items[0].Score != nil {
		t.Fatal("导入未独立保留排名与空总分")
	}
	request(
		t,
		server,
		"POST",
		base+"/projects/import",
		projectModels.ImportRequest{
			Content: spreadsheettest.Fixture(t,
				"项目名称,归属年份,说明,子项名称(JSON)\n虚构整批回滚,2026,说明,[]\n虚构导入项目,2026,重复,[]\n"),
		},
		cookie,
		409,
	)
	var count int64
	if err := db.DB().
		WithContext(t.Context()).
		Model(&projectModels.Project{}).
		Where("workspace_id = ? AND name = ?", strings.TrimPrefix(base, "/v1/workspaces/"), "虚构整批回滚").
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("项目导入没有整批回滚")
	}
}

func checkProjectExports(
	t *testing.T,
	server *echo.Echo,
	base string,
	cookie *http.Cookie,
	project projectModels.ProjectInfo,
	withoutDetails projectModels.ParticipationInput,
) {
	t.Helper()
	rank := int32(2)
	withoutDetails.Rank = &rank
	request(t, server, "POST", base+"/project-participations", projectModels.ParticipationRequest{
		ParticipationInput: withoutDetails,
	}, cookie, 200)
	export := func(resource, parameters string) [][]string {
		t.Helper()
		file := decode[projectModels.FileData](t, request(
			t, server, "GET", base+"/"+resource+"/export?"+parameters, nil, cookie, 200,
		))
		expectedName := "奖项.xlsx"
		if resource == "project-participations" {
			expectedName = "获奖学生.xlsx"
			parameters, err := url.ParseQuery(parameters)
			if err != nil {
				t.Fatal(err)
			}
			if parameters.Get("format") == "full" {
				expectedName = "获奖明细.xlsx"
			}
		}
		if file.Filename != expectedName || !strings.HasSuffix(file.ContentType, ".sheet") {
			t.Fatal("导出文件名或 XLSX 类型不符合契约")
		}
		rows, err := spreadsheettest.Rows(file.Content)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	// 多选筛选导出全部结果，忽略列表页码且不混入其他空间项目。
	parameters := url.Values{
		"project_name": {project.Name, "不存在的项目"},
		"year":         {"2025", "2026"}, "page": {"2"}, "page_size": {"1"},
	}
	projects := export("projects", parameters.Encode())
	if len(projects) != 3 || len(projects[0]) != 7 ||
		projects[1][0] != project.Name || projects[1][1] != "2025" ||
		projects[1][2] != project.Description || projects[1][3] != "1" ||
		projects[2][1] != "2026" || projects[0][4] != "子项名称(JSON)" ||
		projects[0][5] != "允许自行申报" || projects[0][6] != "自行申报截止时间" {
		t.Fatal("项目导出未复用筛选、排序或完整字段")
	}
	var exportedNames []string
	if err := json.Unmarshal([]byte(projects[1][4]), &exportedNames); err != nil ||
		!slices.Equal(exportedNames, project.DetailNames) {
		t.Fatal("项目导出丢失子项配置或未使用名称")
	}
	rows := export("project-participations", parameters.Encode())
	if len(rows) != 5 || len(rows[0]) != 10 || rows[1][1] != "2025" ||
		rows[1][3] != "00000002" || rows[1][5] != "博士" || rows[1][6] != "虚构专业乙" ||
		rows[1][7] != "2026级" || rows[1][8] != "" || rows[1][9] != "" ||
		rows[4][1] != "2026" || rows[4][8] != "2" || rows[4][9] != "0" ||
		rows[0][8] != "排名" || rows[0][9] != "总分" {
		t.Fatal("基础导出混入子项或丢失无子项记录、排名、零总分及列表排序")
	}

	// 学生多选筛选在列表、统计和两种导出中一致生效，不混入其他空间的学生枚举。
	filters := decode[projectModels.FilterOptionsData](t, request(
		t, server, "GET", base+"/project-participations/filter-options", nil, cookie, 200,
	))
	if len(filters.EducationLevels) != 2 || len(filters.Majors) != 2 || len(filters.Grades) != 2 {
		t.Fatal("获奖学生筛选枚举未按当前空间完整去重")
	}
	cohort := url.Values{
		"project_name":    {project.Name},
		"year":            {"2025", "2026"},
		"education_level": {"硕士", "不存在的学历"},
		"major":           {"虚构专业甲", "其他空间专业"},
		"grade":           {"2025级", "2024级"},
	}
	for _, format := range []string{"basic", "full"} {
		cohort.Set("format", format)
		filtered := export("project-participations", cohort.Encode())
		if len(filtered) != 3 || filtered[1][3] != "00000001" ||
			filtered[2][3] != "00000001" || filtered[1][5] != "硕士" ||
			filtered[1][6] != "虚构专业甲" || filtered[1][7] != "2025级" {
			t.Fatal("获奖学生和明细导出未复用全部学生多选条件或基础字段")
		}
	}
	list := decode[projectModels.ParticipationList](t, request(
		t, server, "GET", base+"/project-participations?"+cohort.Encode(), nil, cookie, 200,
	))
	stats := decode[[]projectModels.Statistic](t, request(
		t, server, "GET", base+"/projects/statistics?"+cohort.Encode(), nil, cookie, 200,
	))
	if list.Total != 2 || list.Items[0].EducationLevel != "硕士" ||
		list.Items[0].Major != "虚构专业甲" || list.Items[0].Grade != "2025级" ||
		len(stats) != 2 || stats[0].Count != 1 || stats[1].Count != 1 {
		t.Fatal("列表和统计未共用学生多选筛选")
	}
	cohort.Set("major", "虚构专业乙")
	if len(export("project-participations", cohort.Encode())) != 1 {
		t.Fatal("不同学生字段的筛选没有取交集")
	}

	parameters.Set("format", "full")
	rows = export("project-participations", parameters.Encode())
	if len(rows) != 5 || len(rows[0]) != 14 || rows[4][8] != "2" ||
		strings.Join(rows[4][10:], "") != "" {
		t.Fatal("完整明细未保持每条获奖记录一行或无子项的空单元格")
	}

	// 子项关键词只筛参与记录，同一学生的其他子项也必须完整导出。
	parameters.Set("student", "00000001")
	parameters.Set("year", "2025")
	parameters.Set("detail_keyword", "虚构子项")
	rows = export("project-participations", parameters.Encode())
	if len(rows) != 2 || rows[1][3] != "00000001" || rows[1][8] != "" || rows[1][9] != "" ||
		rows[0][10] != project.Name+"-虚构子项" || rows[1][10] != "0" ||
		rows[0][11] != project.Name+"-虚构子项-说明" || rows[1][11] != strings.Repeat("虚构详情", 100) ||
		rows[0][12] != project.Name+"-无分数子项" || rows[1][12] != "" {
		t.Fatal("明细未将全部评分子项写成独立列，或丢失前导零、空值及零分")
	}
	var audit auditModels.Record
	if err := db.DB().WithContext(t.Context()).
		Where("workspace_id = ? AND action_id = ? AND result_code = ?",
			strings.TrimPrefix(base, "/v1/workspaces/"),
			accessConstants.ParticipationExport, 0).
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

	// 双精度分数保持原值，空结果仍提供固定表头。
	parameters.Set("student", "00000002")
	rows = export("project-participations", parameters.Encode())
	if len(rows) != 2 || rows[1][10] != "5.123456" {
		t.Fatal("明细导出丢失双精度分数")
	}
	parameters.Set("student", "不存在的学生")
	if rows := export("project-participations", parameters.Encode()); len(rows) != 1 ||
		!slices.Equal(rows[0], []string{
			"项目名称", "归属年份", "项目说明", "学号", "姓名", "学历", "专业", "年级", "排名", "总分",
		}) {
		t.Fatal("空结果丢失完整明细表头")
	}

	// 同一获奖记录的子项不能重名，分数列与说明列的拼接冲突仍须独立保留。
	duplicateProject := decode[projectModels.ProjectInfo](t, request(
		t, server, "POST", base+"/projects", projectModels.ProjectRequest{
			ProjectInput: projectModels.ProjectInput{
				Name: "虚构重名子项项目", Year: 2026, DetailNames: []string{"重复", "重复-说明"},
			},
		}, cookie, 200,
	))
	duplicateInput := projectModels.ParticipationRequest{
		ParticipationInput: projectModels.ParticipationInput{
			ParticipationFields: projectModels.ParticipationFields{
				ProjectID: duplicateProject.ID,
				StudentID: withoutDetails.StudentID,
			},
			Details: []projectModels.ScoreItem{{Name: "重复"}, {Name: " 重复 "}},
		},
	}
	request(t, server, "POST", base+"/project-participations", duplicateInput, cookie, 400)
	duplicateInput.Details = []projectModels.ScoreItem{
		{Name: "重复", Description: "说明甲", Score: score(0)},
		{Name: "重复-说明", Description: "说明丙"},
	}
	request(t, server, "POST", base+"/project-participations", duplicateInput, cookie, 200)
	rows = export("project-participations", url.Values{
		"project_id": {strconvID(duplicateProject.ID)}, "format": {"full"},
	}.Encode())
	if len(rows) != 2 || len(rows[0]) != 14 || rows[1][10] != "0" || rows[1][11] != "说明甲" ||
		rows[1][12] != "" || rows[1][13] != "说明丙" ||
		rows[0][12] != duplicateProject.Name+"-重复-说明（2）" {
		t.Fatal("唯一子项的分数列和说明列名称冲突导致数据覆盖")
	}
	request(
		t,
		server,
		"GET",
		base+"/project-participations/export?detail_keyword=子项",
		nil,
		cookie,
		400,
	)
	request(t, server, "GET", base+"/projects/export?year=1899", nil, cookie, 400)
	request(
		t,
		server,
		"GET",
		base+"/project-participations/export?format=invalid",
		nil,
		cookie,
		400,
	)

	// 行数按获奖记录计数，子项变成列后不消耗额外的数据行额度。
	func() {
		maxRows, maxBytes := config.Config.Project.MaxExportRows, config.Config.Project.MaxFileBytes
		defer func() {
			config.Config.Project.MaxExportRows, config.Config.Project.MaxFileBytes = maxRows, maxBytes
		}()
		config.Config.Project.MaxExportRows = 1
		path := fmt.Sprintf(
			"%s/project-participations/export?project_id=%d&year=2025",
			base,
			project.ID,
		)
		request(t, server, "GET", path, nil, cookie, 400)
		request(t, server, "GET", base+"/projects/export", nil, cookie, 400)
		request(t, server, "GET", path+"&student=00000001&format=full", nil, cookie, 200)
		config.Config.Project.MaxExportRows = maxRows
		config.Config.Project.MaxFileBytes = 1
		request(t, server, "GET", base+"/projects/export", nil, cookie, 400)
	}()
}

func checkDetailSearch(
	t *testing.T,
	server *echo.Echo,
	base string,
	cookie *http.Cookie,
	project, other projectModels.ProjectInfo,
) {
	t.Helper()
	path := base + "/project-participations"
	for _, parameters := range []string{
		"detail_keyword=详情",
		"project_name=" + url.QueryEscape(project.Name) + "&detail_keyword=详情",
		"year=2025&detail_keyword=详情",
		"keyword=" + url.QueryEscape(project.Name) + "&year=2025&detail_keyword=详情",
	} {
		request(t, server, "GET", path+"?"+parameters, nil, cookie, 400)
	}

	for _, test := range []struct {
		keyword string
		count   int64
	}{
		{"构子", 2}, {"详情", 2}, {"项", 2}, {"不存在的子项", 0}, {"%", 0},
	} {
		parameters := url.Values{
			"project_name":   {"不存在的项目", project.Name},
			"year":           {"1900", "2025"},
			"detail_keyword": {test.keyword},
		}
		result := decode[projectModels.ParticipationList](
			t,
			request(t, server, "GET", path+"?"+parameters.Encode(), nil, cookie, 200),
		)
		if result.Total != test.count || int64(len(result.Items)) != test.count {
			t.Fatal("子项包含匹配结果或去重数量不正确")
		}
		for _, item := range result.Items {
			if item.ProjectID != project.ID || item.Year != 2025 {
				t.Fatal("子项搜索混入其他项目或年份")
			}
		}
	}
	for _, parameters := range []url.Values{
		{"project_id": {fmt.Sprint(other.ID)}, "year": {"2026"}, "detail_keyword": {"详情"}},
		{"project_name": {other.Name}, "year": {"2026"}, "detail_keyword": {"详情"}},
		{"project_id": {fmt.Sprint(project.ID)}, "year": {"2026"}, "detail_keyword": {"详情"}},
	} {
		result := decode[projectModels.ParticipationList](
			t,
			request(t, server, "GET", path+"?"+parameters.Encode(), nil, cookie, 200),
		)
		if result.Total != 0 {
			t.Fatal("子项搜索越过工作空间或项目归属年份边界")
		}
	}
}
