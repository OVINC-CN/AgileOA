package cpcpersonSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheettest"

	"github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonTask"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

type fixtureActor struct {
	user   userModels.User
	cookie *http.Cookie
}

func personRequest[T any](
	t *testing.T, server *echo.Echo, actor fixtureActor, method, path string, body any, status int,
) T {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(content))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set("X-CSRF-Protection", "1")
	request.AddCookie(actor.cookie)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
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
	var envelope response.Envelope[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func createPersonActor(t *testing.T, tx *gorm.DB, name string, admin bool) fixtureActor {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(strings.Repeat("0", 64)),
		config.Config.User.BcryptCost,
	)
	if err != nil {
		t.Fatal(err)
	}
	user := userModels.User{
		Username: name, DisplayName: "虚构党建验收账号", PasswordHash: string(hash),
		IsEnabled: true, IsAdmin: admin, SessionVersion: 1,
	}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token := "fixture-" + name + strings.Repeat("1", 48)
	digest := sha256.Sum256([]byte(token))
	if err := tx.Create(&userModels.Session{
		UserID: user.ID, TokenHash: hex.EncodeToString(digest[:]), SessionVersion: 1,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	return fixtureActor{user: user, cookie: &http.Cookie{
		Name:     config.Config.User.CookieName,
		Value:    token,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}}
}

func grantPersonActions(
	t *testing.T,
	tx *gorm.DB,
	actor fixtureActor,
	workspaceID string,
	actions ...string,
) {
	t.Helper()
	for _, action := range actions {
		if err := tx.Create(&accessModels.Permission{
			UserID:      actor.user.ID,
			WorkspaceID: workspaceID,
			Action:      action,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func assertEmptyPersonScope(
	t *testing.T,
	server *echo.Echo,
	actor fixtureActor,
	base string,
) {
	t.Helper()
	listed := personRequest[cpcpersonModels.ListData](t, server, actor, "GET", base, nil, 200)
	if listed.Total != 0 || len(listed.Items) != 0 ||
		listed.CanCreate || listed.CanImport || listed.CanExport {
		t.Fatal("无支部范围的账号未返回空名单或显示了范围外操作")
	}
	statistics := personRequest[cpcpersonModels.StatisticsData](
		t, server, actor, "GET", base+"/statistics", nil, 200,
	)
	if statistics.Total != 0 || len(statistics.Stages) != 5 {
		t.Fatal("空读取范围的统计未返回五个阶段的零计数")
	}
	for _, stage := range statistics.Stages {
		if stage.Count != 0 {
			t.Fatal("空读取范围的统计包含其他支部人员")
		}
	}
	for _, path := range []string{"/branches", "/options?field=party_position", "/options?field=status"} {
		options := personRequest[[]cpcpersonModels.Option](
			t,
			server,
			actor,
			"GET",
			base+path,
			nil,
			200,
		)
		if len(options) != 0 {
			t.Fatal("空读取范围的数据选项包含其他支部信息")
		}
	}
	candidates := personRequest[[]cpcpersonModels.Candidate](
		t, server, actor, "GET", base+"/student-options", nil, 200,
	)
	if len(candidates) != 0 {
		t.Fatal("空新增范围的学生候选项包含其他支部信息")
	}
	file := personRequest[cpcpersonModels.FileData](
		t,
		server,
		actor,
		"GET",
		base+"/export",
		nil,
		200,
	)
	if strings.Count(spreadsheettest.Text(t, file.Content), "\n") != 0 {
		t.Fatal("空读取范围的导出未仅保留表头")
	}
}

func TestPersonScopeAndAtomicRecalculation(t *testing.T) {
	// 仅在手动准备的独立空库执行，不建表、不启动服务、不读取真实学生数据。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立党建验收数据库后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_cpc_person_test") {
		t.Fatal("党建验收数据库必须以 _cpc_person_test 结尾")
	}
	tx := db.DB().WithContext(t.Context())
	for _, table := range []string{"users", "workspaces", "students", "cpc_people", "cpc_party_branches"} {
		var count int64
		if err := tx.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("党建验收要求已人工应用全部 SQL 的独立空库")
		}
	}
	const workspaceID = "w7m2k9p4s6n8c3x5"
	const otherWorkspace = "x8n3m0q5t7p9d4y6"
	for _, id := range []string{workspaceID, otherWorkspace} {
		if err := tx.Create(&workspaceModels.Workspace{
			ID: id, Name: "虚构党建空间" + id, IsEnabled: true, Revision: 1,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	timeline := cpctimelineModels.Template{
		WorkspaceID: workspaceID, Name: "虚构时间线", Revision: 1,
	}
	if err := tx.Create(&timeline).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&cpctimelineModels.TemplateNode{
		WorkspaceID: workspaceID, TimelineTemplateID: timeline.ID, NodeID: "node_first",
		Name: "虚构节点", TimeFieldID: "occurred_at", Position: 1,
		FieldDefinitions: `[{"id":"occurred_at","name":"发生时间","type":"date","required":true}]`,
		ValidationRules:  "[]",
	}).Error; err != nil {
		t.Fatal(err)
	}
	first := createPersonActor(t, tx, "fixture.secretary.first", false)
	second := createPersonActor(t, tx, "fixture.secretary.second", false)
	manager := createPersonActor(t, tx, "fixture.manager", false)
	admin := createPersonActor(t, tx, "fixture.admin", true)
	personActions := []string{
		accessConstants.CPCPersonList,
		accessConstants.CPCPersonDetail,
		accessConstants.CPCPersonCreate,
		accessConstants.CPCPersonUpdate,
		accessConstants.CPCPersonDelete,
		accessConstants.CPCPersonImport,
		accessConstants.CPCPersonExport,
		accessConstants.CPCPersonStatistics,
	}
	for _, actor := range []fixtureActor{first, second, manager, admin} {
		grantPersonActions(t, tx, actor, workspaceID, accessConstants.AccessWorkspace)
	}
	for _, actor := range []fixtureActor{first, second, admin} {
		grantPersonActions(t, tx, actor, workspaceID, personActions...)
	}
	grantPersonActions(
		t,
		tx,
		manager,
		workspaceID,
		accessConstants.CPCPartyBranchList,
		accessConstants.CPCPartyBranchUpdate,
		accessConstants.CPCPartyBranchDelete,
		accessConstants.StudentCreate,
		accessConstants.StudentSensitiveHandle,
	)
	grantPersonActions(t, tx, manager, otherWorkspace, personActions...)
	grantPersonActions(t, tx, manager, otherWorkspace,
		accessConstants.AccessWorkspace, accessConstants.CPCPartyBranchList)
	server := httpd.New()
	base := "/v1/workspaces/" + workspaceID + "/cpc-people"
	reorderPath := "/v1/workspaces/" + workspaceID + "/cpc-party-branches/reorder-people"
	personRequest[any](t, server, admin, "POST", reorderPath, nil, 403)
	queued := personRequest[partybranchModels.ReorderData](
		t, server, manager, "POST", reorderPath, nil, 200,
	)
	if queued.Queued {
		t.Fatal("空名单不应创建后台重排任务")
	}
	personRequest[any](t, server, manager, "GET", base, nil, 403)
	grantPersonActions(t, tx, manager, workspaceID, personActions...)

	// 两支部重叠匹配时，必须先取全空间最大编号，不能先按书记范围过滤。
	rule := func(values string) *string {
		content := `{"type":"condition","field":"major","operator":"in","values":` + values + `}`
		return &content
	}
	branch := func(id string, secretary uint64, fullName string, values string) partybranchModels.Branch {
		record := partybranchModels.Branch{
			WorkspaceID: id,
			FullName:    fullName,
			ShortName:   fullName,
			SecretaryID: &secretary,
			AdmissionRule: rule(
				values,
			),
			Revision: 1,
		}
		if err := tx.Create(&record).Error; err != nil {
			t.Fatal(err)
		}
		return record
	}
	branchA := branch(workspaceID, first.user.ID, "虚构甲支部", `["x","y"]`)
	branchB := branch(workspaceID, second.user.ID, "虚构乙支部", `["x"]`)
	branch(otherWorkspace, manager.user.ID, "虚构其他空间支部", `["x","y"]`)
	student := func(number, major string) uint64 {
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
		created := personRequest[studentModels.MutationData](t, server, manager, "POST",
			"/v1/workspaces/"+workspaceID+"/students",
			studentModels.CreateRequest{
				Input: studentModels.Input{
					StudentNo: number, Name: "虚构学生" + number, Major: major, EducationLevel: "硕士",
					Grade: "2026级", Status: "在籍", ClassName: "虚构班级", AdmissionType: "非定向",
					Gender: "女", Ethnicity: "汉族", AccommodationType: "on_campus",
				},
				Sensitive: values,
			}, 200)
		return created.ID
	}
	x, y := student("000001", "x"), student("000002", "y")
	candidates := personRequest[[]cpcpersonModels.Candidate](
		t,
		server,
		first,
		"GET",
		base+"/student-options",
		nil,
		200,
	)
	if len(candidates) != 1 || candidates[0].Value != y ||
		candidates[0].PartyBranchID != branchA.ID {
		t.Fatal("候选项泄露其他支部学生，或未按全空间最大编号确定归属")
	}
	create := func(actor fixtureActor, studentID uint64, position string, status int) cpcpersonModels.Info {
		return personRequest[cpcpersonModels.Info](
			t,
			server,
			actor,
			"POST",
			base,
			cpcpersonModels.Request{
				Input: cpcpersonModels.Input{
					StudentID:     studentID,
					Stage:         "full_member",
					PartyPosition: position, TimelineTemplateID: &timeline.ID,
				},
			},
			status,
		)
	}
	create(first, x, "书记", 403)
	create(admin, x, "", 403)
	personX, personY := create(manager, x, "书记", 200), create(manager, y, "组织委员", 200)
	if personX.PartyBranchID != branchB.ID || personY.PartyBranchID != branchA.ID {
		t.Fatal("自动归属忽略优先级或跨越租户边界")
	}
	if personX.Status != "在编" || personY.Status != "在编" {
		t.Fatal("新增人员未使用默认在编状态")
	}
	assertPersonTimelines(
		t,
		server,
		tx,
		first,
		manager,
		workspaceID,
		otherWorkspace,
		personX,
		personY,
		timeline,
	)
	assertPersonStatuses(t, server, first, base, personY.ID)
	listed := personRequest[cpcpersonModels.ListData](t, server, first, "GET", base, nil, 200)
	if listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].ID != personY.ID {
		t.Fatal("书记列表包含其他支部人员")
	}
	stats := personRequest[cpcpersonModels.StatisticsData](
		t,
		server,
		first,
		"GET",
		base+"/statistics",
		nil,
		200,
	)
	if stats.Total != 1 || len(stats.Stages) != 5 {
		t.Fatal("统计未遵守读取范围")
	}
	options := personRequest[[]cpcpersonModels.Option](
		t,
		server,
		first,
		"GET",
		base+"/options?field=party_position",
		nil,
		200,
	)
	if len(options) != 1 || options[0].Value != "组织委员" {
		t.Fatal("动态职务选项泄露其他支部")
	}
	file := personRequest[cpcpersonModels.FileData](
		t,
		server,
		first,
		"GET",
		base+"/export",
		nil,
		200,
	)
	if strings.Contains(spreadsheettest.Text(t, file.Content), "000001") ||
		!strings.Contains(spreadsheettest.Text(t, file.Content), "000002") {
		t.Fatal("导出未遵守读取范围")
	}
	assertEmptyPersonScope(t, server, admin, base)
	path := fmt.Sprintf("%s/%d", base, personX.ID)
	request := cpcpersonModels.UpdateRequest{
		StudentID: personX.StudentID, Stage: personX.Stage, PartyPosition: personX.PartyPosition,
		Revision: personX.Revision,
	}
	personRequest[any](t, server, first, "GET", path, nil, 404)
	personRequest[any](t, server, first, "PUT", path, request, 404)
	personRequest[any](t, server, first, "DELETE", path+"?revision=1", nil, 404)
	personRequest[any](t, server, manager, "PUT", path, map[string]any{
		"student_id":      x,
		"stage":           "full_member",
		"party_position":  "书记",
		"revision":        1,
		"party_branch_id": branchA.ID,
	}, 400)
	personRequest[any](
		t,
		server,
		manager,
		"GET",
		fmt.Sprintf("/v1/workspaces/%s/cpc-people/%d", otherWorkspace, personX.ID),
		nil,
		404,
	)

	// 查看支部只扩大读取，撤换书记在下一次请求立即生效。
	grantPersonActions(t, tx, admin, workspaceID, accessConstants.CPCPartyBranchList)
	all := personRequest[cpcpersonModels.ListData](t, server, admin, "GET", base, nil, 200)
	if all.Total != 2 || all.Items[0].CanUpdate || all.Items[0].CanDelete {
		t.Fatal("查看范围放宽了写入范围")
	}
	if err := tx.Model(&partybranchModels.Branch{}).
		Where("workspace_id = ? AND id = ?", workspaceID, branchA.ID).
		Update("secretary_id", second.user.ID).
		Error; err != nil {
		t.Fatal(err)
	}
	assertEmptyPersonScope(t, server, first, base)
	if result := personRequest[cpcpersonModels.ListData](
		t,
		server,
		second,
		"GET",
		base,
		nil,
		200,
	); result.Total != 2 {
		t.Fatal("新书记未取得当前支部范围")
	}

	// 直接调用已注册任务处理器，不启动验收服务或队列工作进程。
	content, err := json.Marshal(map[string]any{
		"workspace_id": workspaceID,
	})
	if err != nil {
		t.Fatal(err)
	}
	setRule := func(branchID uint64, field string, values []string) {
		var branch partybranchModels.Branch
		if err := tx.Select("revision").
			Where("workspace_id = ? AND id = ?", workspaceID, branchID).Take(&branch).
			Error; err != nil {
			t.Fatal(err)
		}
		node := partybranchModels.AdmissionRuleNode{
			Type: "condition", Field: field, Operator: "in", Values: values,
		}
		personRequest[partybranchModels.AdmissionRuleInfo](
			t,
			server,
			manager,
			"PUT",
			fmt.Sprintf(
				"/v1/workspaces/%s/cpc-party-branches/%d/admission-rule",
				workspaceID,
				branchID,
			),
			partybranchModels.AdmissionRuleRequest{
				AdmissionRuleChange: partybranchModels.AdmissionRuleChange{Rule: &node},
				Revision:            branch.Revision,
			},
			200,
		)
	}
	setRule(branchA.ID, "education_level", []string{"硕士"})
	var priorityBranch partybranchModels.Branch
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, branchA.ID).
		Take(&priorityBranch).Error; err != nil {
		t.Fatal(err)
	}
	priority := int32(100)
	priorityPath := fmt.Sprintf("/v1/workspaces/%s/cpc-party-branches/%d", workspaceID, branchA.ID)
	updatedBranch := personRequest[partybranchModels.Info](
		t, server, manager, "PUT", priorityPath,
		partybranchModels.Request{
			Input: partybranchModels.Input{
				FullName: priorityBranch.FullName, ShortName: priorityBranch.ShortName,
				SecretaryID: priorityBranch.SecretaryID,
			},
			Priority: &priority, Revision: priorityBranch.Revision,
		}, 200,
	)
	preserved := personRequest[partybranchModels.Info](
		t,
		server,
		manager,
		"PUT",
		priorityPath,
		partybranchModels.Request{
			Input:    updatedBranch.Input,
			Revision: updatedBranch.Revision,
		},
		200,
	)
	if preserved.Priority != priority {
		t.Fatal("旧客户端省略优先级时覆盖了已有设置")
	}
	branches := personRequest[partybranchModels.ListData](
		t, server, manager, "GET", "/v1/workspaces/"+workspaceID+"/cpc-party-branches", nil, 200,
	)
	if len(branches.Items) != 2 || branches.Items[0].ID != branchA.ID ||
		branches.Items[0].Priority != priority {
		t.Fatal("党支部列表未优先按优先级倒序排列")
	}
	unchanged := personRequest[cpcpersonModels.Info](t, server, manager, "GET", path, nil, 200)
	if unchanged.PartyBranchID != branchB.ID || unchanged.Revision != personX.Revision {
		t.Fatal("仅保存准入规则或优先级不应立即重排人员")
	}
	if err := cpcpersonTask.Process(
		t.Context(),
		asynq.NewTask(cpcpersonTask.RecalculateBranches, content),
	); err != nil {
		t.Fatal(err)
	}
	var before []cpcpersonModels.Person
	if err := tx.Where("workspace_id = ?", workspaceID).
		Order("id ASC").
		Find(&before).
		Error; err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || *before[0].PartyBranchID != branchA.ID || before[0].Revision != 2 {
		t.Fatal("后台未按学生学历及优先级重算归属，或错误地继续优先使用较大编号")
	}
	setRule(branchA.ID, "education_level", []string{"博士"})
	setRule(branchB.ID, "student_no", []string{"000003"})
	err = cpcpersonTask.Process(
		t.Context(),
		asynq.NewTask(cpcpersonTask.RecalculateBranches, content),
	)
	if !errors.Is(err, cpcpersonSvc.ErrUnmatched) || !errors.Is(err, asynq.SkipRetry) {
		t.Fatal("未匹配失败没有等待规则修正")
	}
	var after []cpcpersonModels.Person
	if err := tx.Where("workspace_id = ?", workspaceID).
		Order("id ASC").
		Find(&after).
		Error; err != nil {
		t.Fatal(err)
	}
	for index := range before {
		if *after[index].PartyBranchID != *before[index].PartyBranchID ||
			after[index].Revision != before[index].Revision {
			t.Fatal("全量重算失败时出现了部分归属更新")
		}
	}
	var taskAudits int64
	if err := tx.Model(&auditModels.Record{}).
		Where("workspace_id = ? AND action_id = ?", workspaceID, accessConstants.CPCPersonRecalculate).
		Count(&taskAudits).
		Error; err != nil ||
		taskAudits != 0 {
		t.Fatal("后台重算不应生成用户操作审计")
	}

	// 非在编人员保留原归属及版本，无论还能匹配其他支部还是已经无法匹配。
	if err := tx.Model(&cpcpersonModels.Person{}).
		Where("workspace_id = ? AND id = ?", workspaceID, personY.ID).
		Update("status", "转出").Error; err != nil {
		t.Fatal(err)
	}
	var excluded cpcpersonModels.Person
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, personY.ID).
		Take(&excluded).Error; err != nil {
		t.Fatal(err)
	}

	// 关联学生已毕业时，在编人员仍参与重排，不额外按学生档案状态筛选。
	if err := tx.Model(&studentModels.Student{}).
		Where("workspace_id = ? AND id = ?", workspaceID, x).
		Update("status", "毕业").Error; err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		firstStudentNo, secondStudentNo string
		branchID, revision              uint64
	}{
		{"000001", "000002", branchA.ID, before[0].Revision},
		{"000003", "000001", branchB.ID, before[0].Revision + 1},
	} {
		setRule(branchA.ID, "student_no", []string{test.firstStudentNo})
		setRule(branchB.ID, "student_no", []string{test.secondStudentNo})
		if err := cpcpersonTask.Process(
			t.Context(),
			asynq.NewTask(cpcpersonTask.RecalculateBranches, content),
		); err != nil {
			t.Fatalf("非在编人员不应阻断重排：%v", err)
		}
		var people []cpcpersonModels.Person
		if err := tx.Where("workspace_id = ?", workspaceID).
			Order("id ASC").Find(&people).Error; err != nil {
			t.Fatal(err)
		}
		if len(people) != 2 || *people[0].PartyBranchID != test.branchID ||
			people[0].Revision != test.revision {
			t.Fatal("非在编人员或学生档案状态影响了在编人员的正常重排")
		}
		if *people[1].PartyBranchID != *excluded.PartyBranchID ||
			people[1].Revision != excluded.Revision ||
			!people[1].UpdatedAt.Equal(excluded.UpdatedAt) {
			t.Fatal("非在编人员的原归属、版本或更新时间被重排修改")
		}
	}

	// 已入册人员均非在编时不提交任务，不依赖队列是否可用。
	if err := tx.Model(&cpcpersonModels.Person{}).
		Where("workspace_id = ? AND id = ?", workspaceID, personX.ID).
		Update("status", "转出").Error; err != nil {
		t.Fatal(err)
	}
	queued = personRequest[partybranchModels.ReorderData](
		t, server, manager, "POST", reorderPath, nil, 200,
	)
	if queued.Queued {
		t.Fatal("只有非在编人员时不应创建后台重排任务")
	}
	assertOwnedDetailCascades(
		t, server, tx, manager, workspaceID, student("CASCADE-PERSON", "x"), branchA.ID,
	)
}
