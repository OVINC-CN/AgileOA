package userSvc_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
)

func checkStudentLogin(t *testing.T, server *echo.Echo, admin *http.Cookie) {
	t.Helper()
	t.Run("绑定空间的在籍学生登录", func(t *testing.T) {
		if db.RDB() == nil {
			t.Skip("配置独立 Redis 后运行学生邮件登录验收")
		}
		original := config.Config.User.EmailLogin.Enabled
		config.Config.User.EmailLogin.Enabled = true
		defer func() { config.Config.User.EmailLogin.Enabled = original }()
		connection := db.DB().WithContext(t.Context())
		createSpace := func(name string) workspaceModels.Info {
			return data[workspaceModels.Info](t, send(
				t,
				server,
				http.MethodPost,
				"/v1/system/workspaces",
				workspaceModels.CreateRequest{Name: name},
				admin,
				http.StatusOK,
			))
		}
		space := createSpace("学生邮件登录验收")
		other := createSpace("学生邮件登录隔离验收")
		send(t, server, http.MethodPost, "/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: space.Name}, admin, http.StatusConflict)
		enabled := true
		send(t, server, http.MethodPut, "/v1/system/workspaces/"+other.ID,
			workspaceModels.UpdateRequest{
				CreateRequest: workspaceModels.CreateRequest{Name: space.Name},
				IsEnabled:     &enabled, Revision: other.Revision,
			}, admin, http.StatusConflict)

		// 相同学号可以存在于不同空间，学生身份不创建全局账号或授权。
		student := studentModels.Student{
			WorkspaceID:    space.ID,
			IdentityNumber: "00000020000102001X", MedicalHistory: "虚构敏感健康记录",
			GraduationYear: "2027", GraduationType: "employment",
			GraduationTypeLabel: "签约就业", GraduationDestination: "虚构就业单位",
			BasicRecord: studentModels.BasicRecord{
				StudentNo: "00000001", Name: "虚构在籍学生", Email: "student@example.com",
				Status: studentConstants.StatusEnrolled, Revision: 1,
				Major: "虚构专业", Advisor: "虚构导师",
				Grade: "2026级", EducationLevel: "博士", ClassName: "2026级一班",
				ClassPosition: "班长", AdmissionType: "非定向", ArrivalMonth: "2026-09",
				AccommodationType: studentConstants.AccommodationOffCampus,
			},
		}
		if err := connection.Create(&student).Error; err != nil {
			t.Fatal(err)
		}
		target := student
		target.ID = 0
		target.WorkspaceID = other.ID
		target.Email = "other@example.com"
		if err := connection.Create(&target).Error; err != nil {
			t.Fatal(err)
		}
		if err := connection.Create(&studentModels.StudentOffCampusDetail{
			WorkspaceID: space.ID, StudentID: student.ID,
			OffCampusFields: studentModels.OffCampusFields{Reason: "校外实习", LandlordName: "虚构房东"},
		}).Error; err != nil {
			t.Fatal(err)
		}
		request := userModels.SendStudentLoginEmailRequest{
			WorkspaceName: space.Name, StudentNo: student.StudentNo, Email: student.Email,
		}
		for _, mismatch := range []userModels.SendStudentLoginEmailRequest{
			{WorkspaceName: "不存在的空间", StudentNo: student.StudentNo, Email: student.Email},
			{WorkspaceName: other.Name, StudentNo: student.StudentNo, Email: student.Email},
			{WorkspaceName: space.Name, StudentNo: "00000002", Email: student.Email},
			{WorkspaceName: space.Name, StudentNo: student.StudentNo, Email: "Student@example.com"},
		} {
			result := send(t, server, http.MethodPost, "/v1/student-login-email",
				mismatch, nil, http.StatusUnauthorized)
			if !strings.Contains(result.Body.String(), "用户不存在") {
				t.Fatal("三项不匹配应统一提示用户不存在")
			}
		}

		code := seedStudentEmailLogin(t, space, student)
		login := verifyEmailLogin(t, server, code, "", http.StatusOK)
		result := data[userModels.EmailLoginData](t, login)
		if result.UsernameRequired || result.User == nil ||
			result.User.IdentityType != userConstants.IdentityStudent ||
			result.User.WorkspaceID != space.ID || result.User.Username != student.StudentNo ||
			result.User.IsAdmin || result.User.PasswordChangeRequired {
			t.Fatal("学生登录应直接返回绑定空间的独立学生身份")
		}
		cookie := sessionCookie(t, login)
		if !strings.HasPrefix(cookie.Value, userConstants.StudentSessionPrefix) {
			t.Fatal("学生 Cookie 缺少独立身份前缀")
		}
		verifyEmailLogin(t, server, code, "", http.StatusUnauthorized)
		send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusOK)
		path := "/v1/workspaces/" + space.ID + "/student-service"
		profileResponse := send(t, server, http.MethodGet, path+"?student_id="+
			strconv.FormatUint(target.ID, 10), nil, cookie, http.StatusOK)
		profile := data[studentModels.ProfileData](t, profileResponse)
		if profile != (studentModels.ProfileData{
			ID:             student.ID,
			StudentNo:      student.StudentNo,
			Name:           student.Name,
			Grade:          student.Grade,
			Status:         student.Status,
			EducationLevel: student.EducationLevel,
			Major:          student.Major,
			ClassName:      student.ClassName,
			ClassPosition:  student.ClassPosition,
		}) {
			t.Fatal("个人信息未按可信身份返回学籍信息")
		}
		fields := map[string]bool{
			"id":              true,
			"student_no":      true,
			"name":            true,
			"grade":           true,
			"status":          true,
			"education_level": true,
			"major":           true,
			"class_name":      true,
			"class_position":  true,
		}
		profileFields := data[map[string]json.RawMessage](t, profileResponse)
		if len(profileFields) != len(fields) {
			t.Fatal("个人信息接口未严格返回学生编号及十个学籍字段")
		}
		for field := range profileFields {
			if !fields[field] {
				t.Fatalf("个人信息接口返回了学籍目录外的字段：%s", field)
			}
		}
		send(t, server, http.MethodGet, path, nil, admin, http.StatusForbidden)
		checkStudentNotifications(t, server, cookie, admin, student, target)
		for _, path := range []string{
			"/v1/workspaces", "/v1/system/users", "/v1/notifications/unread",
			"/v1/workspaces/" + other.ID + "/student-service",
			"/v1/workspaces/" + space.ID + "/students",
			"/v1/workspaces/" + space.ID + "/cpc-people",
		} {
			send(t, server, http.MethodGet, path, nil, cookie, http.StatusForbidden)
		}
		sendMCPList(t, server, cookie.Value, cookie, http.StatusUnauthorized)
		forged := &http.Cookie{
			Name:     cookie.Name,
			Value:    strings.TrimPrefix(cookie.Value, userConstants.StudentSessionPrefix),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}
		send(t, server, http.MethodGet, "/v1/user-info", nil, forged, http.StatusUnauthorized)

		// 在籍状态在发信、链接兑换和已登录请求三个入口实时生效。
		code = seedStudentEmailLogin(t, space, student)
		update := func(column string, value any) {
			t.Helper()
			if err := connection.Model(&studentModels.Student{}).
				Where("workspace_id = ? AND id = ?", space.ID, student.ID).
				Update(column, value).Error; err != nil {
				t.Fatal(err)
			}
		}
		update("status", "毕业")
		send(
			t,
			server,
			http.MethodPost,
			"/v1/student-login-email",
			request,
			nil,
			http.StatusUnauthorized,
		)
		verifyEmailLogin(t, server, code, "", http.StatusUnauthorized)
		send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
		update("status", studentConstants.StatusEnrolled)
		update("email", "changed@example.com")
		verifyEmailLogin(t, server, code, "", http.StatusUnauthorized)
		send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
		update("email", student.Email)
		if err := connection.Model(&workspaceModels.Workspace{}).Where("id = ?", space.ID).
			Update("is_enabled", false).Error; err != nil {
			t.Fatal(err)
		}
		verifyEmailLogin(t, server, code, "", http.StatusUnauthorized)
		send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
		if err := connection.Model(&workspaceModels.Workspace{}).Where("id = ?", space.ID).
			Update("is_enabled", true).Error; err != nil {
			t.Fatal(err)
		}

		checkStudentLoginAuditRollback(t, server, space, student)
		send(t, server, http.MethodPost, "/v1/logout", nil, cookie, http.StatusOK)
		send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	})
}

func seedStudentEmailLogin(
	t *testing.T,
	workspace workspaceModels.Info,
	student studentModels.Student,
) string {
	t.Helper()
	// 只在独立测试库配套的 Redis 写入虚构链接，不调用真实邮件服务。
	value := make([]byte, userConstants.SessionBytes)
	_, _ = rand.Read(value)
	code := hex.EncodeToString(value)
	digest := sha256.Sum256([]byte(code))
	hash := hex.EncodeToString(digest[:])
	payload, err := json.Marshal(userModels.EmailLogin{
		CodeHash:  emailCodeDigest(code, fixtureEmailCode),
		StudentID: student.ID, WorkspaceID: workspace.ID, WorkspaceName: workspace.Name,
		StudentNo: student.StudentNo, Email: student.Email,
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	codeKey := userConstants.EmailLoginChallengePrefix + hash
	indexKey := userConstants.StudentEmailLoginPrefix + workspace.ID + ":" +
		strconv.FormatUint(student.ID, 10)
	pipeline := db.RDB().TxPipeline()
	pipeline.Set(t.Context(), codeKey, payload, time.Minute)
	pipeline.Set(t.Context(), indexKey, hash, time.Minute)
	if _, err := pipeline.Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.WithoutCancel(t.Context()),
			config.Config.Redis.WriteTimeout,
		)
		defer cancel()
		if err := db.RDB().Del(ctx, codeKey, indexKey).Err(); err != nil {
			t.Error(err)
		}
	})
	return code
}

func checkStudentLoginAuditRollback(
	t *testing.T,
	server *echo.Echo,
	workspace workspaceModels.Info,
	student studentModels.Student,
) {
	t.Helper()
	connection := db.DB().WithContext(t.Context())
	var before, after int64
	if err := connection.Model(&userModels.StudentSession{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	code := seedStudentEmailLogin(t, workspace, student)
	const callbackName = "test:student-login-audit-unavailable"
	if err := connection.Callback().Create().Before("gorm:create").Register(callbackName,
		func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				_ = tx.AddError(gorm.ErrInvalidData)
			}
		}); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if err := connection.Callback().Create().Remove(callbackName); err != nil {
				t.Error(err)
			}
		}()
		result := verifyEmailLogin(t, server, code, "", http.StatusServiceUnavailable)
		if result.Header().Get("Set-Cookie") != "" {
			t.Fatal("审计失败不得签发学生 Cookie")
		}
	}()
	if err := connection.Model(&userModels.StudentSession{}).
		Count(&after).
		Error; err != nil ||
		before != after {
		t.Fatal("审计失败没有回滚学生会话")
	}
	verifyEmailLogin(t, server, code, "", http.StatusUnauthorized)
}
