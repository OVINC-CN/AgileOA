package userSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

var _ = flag.String("config", "", "独立验收数据库的 YAML 配置")

const testPassword = "Fictional-password-2026"
const temporaryPassword = "Temporary-fixture-password-2026"

func digest(username, password string) string {
	value := sha256.Sum256([]byte(username + ":" + password))
	return hex.EncodeToString(value[:])
}

func send(
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
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(content))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Protection", "1")
	request.Header.Set("X-Request-ID", "untrusted-client-id")
	request.Header.Set("User-Agent", "AgileWorkspace-Auth-Tests")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != status {
		t.Fatalf("%s %s 状态错误：%d，%s", method, path, recorder.Code, recorder.Body.String())
	}
	var envelope response.Envelope[json.RawMessage]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Trace) != 32 || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("账号接口缺少追踪或禁止缓存设置")
	}
	if status >= http.StatusBadRequest &&
		(envelope.ErrorMsg == "" || string(envelope.Data) != "null") {
		t.Fatal("失败响应未阻断业务数据")
	}
	return recorder
}

func login(t *testing.T, server *echo.Echo, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
		Username:       username,
		PasswordSHA256: digest(username, password),
	}, nil, http.StatusOK)
}

func sessionCookie(t *testing.T, result *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	response := result.Result()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode ||
		cookies[0].Path != "/" ||
		cookies[0].Domain != strings.TrimPrefix(config.Config.User.CookieDomain, ".") ||
		cookies[0].MaxAge != int(config.Config.User.SessionTTL/time.Second) ||
		cookies[0].Secure != config.Config.User.CookieSecure {
		t.Fatal("会话 Cookie 属性不正确")
	}
	return cookies[0]
}

func changeRequest(username, current, next string) userModels.ChangePasswordRequest {
	return userModels.ChangePasswordRequest{
		CurrentPasswordSHA256: digest(username, current),
		NewPasswordSHA256:     digest(username, next),
	}
}

func completePasswordChange(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	request userModels.ChangePasswordRequest,
) *http.Cookie {
	t.Helper()
	result := send(
		t,
		server,
		http.MethodPost,
		"/v1/change-password",
		request,
		cookie,
		http.StatusOK,
	)
	updated := sessionCookie(t, result)
	if data[userModels.Info](t, result).PasswordChangeRequired || updated.Value == cookie.Value {
		t.Fatal("改密后应解除限制并旋转会话")
	}
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	return updated
}

func TestUserAuthenticationAndAuditTransactions(t *testing.T) {
	// 只允许独立测试库，绝不清理或迁移真实账号数据。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立 MySQL 验收配置后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_user_test") {
		t.Fatal("验收数据库名称必须以 _user_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var existing int64
	if err := connection.Model(&userModels.User{}).
		Count(&existing).
		Error; err != nil ||
		existing != 0 {
		t.Fatal("验收要求已执行手工 SQL 的空测试数据库")
	}

	// 记录真实 GORM 跨度，最后检查口令及会话没有进入追踪。
	recorder := tracetest.NewSpanRecorder()
	tracing.Provider.RegisterSpanProcessor(recorder)
	t.Cleanup(func() { tracing.Provider.UnregisterSpanProcessor(recorder) })
	originalPassword := config.Config.User.BootstrapPassword
	config.Config.User.BootstrapPassword = ""
	if err := userSvc.Bootstrap(t.Context()); err == nil {
		t.Fatal("首次启动未拒绝空初始密码")
	}
	config.Config.User.BootstrapPassword = temporaryPassword
	t.Cleanup(func() { config.Config.User.BootstrapPassword = originalPassword })
	if err := userSvc.Bootstrap(t.Context()); err != nil {
		t.Fatal(err)
	}
	config.Config.User.BootstrapPassword = ""
	if err := userSvc.Bootstrap(t.Context()); err != nil {
		t.Fatal("后续启动不应再次要求初始密码")
	}
	var admin userModels.User
	if err := connection.Where("username = ?", config.Config.User.BootstrapUsername).
		First(&admin).
		Error; err != nil {
		t.Fatal(err)
	}
	if !admin.IsAdmin || !admin.IsEnabled || !admin.PasswordChangeRequired ||
		bcrypt.CompareHashAndPassword(
			[]byte(admin.PasswordHash),
			[]byte(digest(admin.Username, temporaryPassword)),
		) != nil {
		t.Fatal("首次管理员未按用户名摘要和 bcrypt 创建")
	}

	var initialAudits int64
	if err := connection.Model(&auditModels.Record{}).
		Count(&initialAudits).
		Error; err != nil ||
		initialAudits != 0 {
		t.Fatal("启动没有已登录操作者，不应创建审计")
	}
	// 认证验收同时覆盖 Cookie API 与仅允许 Token 的 MCP 入口。
	originalMCP := config.Config.MCP
	config.Config.MCP.Enabled = true
	config.Config.MCP.MaxResponseBytes = 33554432
	t.Cleanup(func() { config.Config.MCP = originalMCP })
	server := httpd.New()
	firstLogin := login(t, server, admin.Username, temporaryPassword)
	if !data[userModels.Info](t, firstLogin).PasswordChangeRequired {
		t.Fatal("首次登录未返回待改密状态")
	}
	adminCookie := sessionCookie(t, firstLogin)
	send(t, server, http.MethodGet, "/v1/system/users", nil, adminCookie, http.StatusForbidden)
	send(t, server, http.MethodGet, "/v1/workspaces", nil, adminCookie, http.StatusForbidden)
	otherSession := sessionCookie(t, login(t, server, admin.Username, temporaryPassword))
	request := changeRequest(admin.Username, temporaryPassword, testPassword)
	wrongPassword := changeRequest(admin.Username, "wrong-password", testPassword)
	reusedPassword := changeRequest(admin.Username, temporaryPassword, temporaryPassword)
	send(
		t,
		server,
		http.MethodPost,
		"/v1/change-password",
		wrongPassword,
		adminCookie,
		http.StatusBadRequest,
	)
	send(
		t,
		server,
		http.MethodPost,
		"/v1/change-password",
		reusedPassword,
		adminCookie,
		http.StatusBadRequest,
	)
	adminCookie = completePasswordChange(t, server, adminCookie, request)
	send(t, server, http.MethodGet, "/v1/user-info", nil, otherSession, http.StatusUnauthorized)
	send(t, server, http.MethodPost, "/v1/login", map[string]string{
		"username": admin.Username, "password": testPassword,
	}, nil, http.StatusBadRequest)
	send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
		Username: admin.Username, PasswordSHA256: digest(admin.Username, "wrong-password"),
	}, nil, http.StatusUnauthorized)
	send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
		Username: "nonexistent", PasswordSHA256: digest("nonexistent", testPassword),
	}, nil, http.StatusUnauthorized)
	send(t, server, http.MethodGet, "/v1/user-info", nil, nil, http.StatusUnauthorized)
	var unidentified int64
	if err := connection.Model(&auditModels.Record{}).
		Where("user_id = 0 OR (action_id = ? AND result_code = ?)", accessConstants.UserLogin, 401).
		Count(&unidentified).
		Error; err != nil ||
		unidentified != 0 {
		t.Fatal("未确认身份的请求生成了审计")
	}
	infoResponse := send(
		t,
		server,
		http.MethodGet,
		"/v1/user-info",
		nil,
		adminCookie,
		http.StatusOK,
	)
	if data[userModels.Info](t, infoResponse).PasswordChangeRequired ||
		strings.Contains(infoResponse.Body.String(), "password_hash") ||
		strings.Contains(infoResponse.Body.String(), "password_sha256") ||
		strings.Contains(infoResponse.Body.String(), "token") {
		t.Fatal("user-info 暴露了认证凭据")
	}
	var readAuditCount int64
	if err := connection.Model(&auditModels.Record{}).
		Where("action_id = ?", accessConstants.UserInfo).
		Count(&readAuditCount).
		Error; err != nil ||
		readAuditCount != 0 {
		t.Fatal("user-info 被写入审计")
	}
	if len(infoResponse.Header().Get("X-Request-ID")) != 32 {
		t.Fatal("user-info 排除审计后丢失了请求上下文")
	}

	// 写请求的跨站来源与缺失保护头均被拒绝，跨域预检允许明确来源和 Cookie。
	for _, origin := range []string{"", "https://untrusted.example"} {
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/logout",
			nil,
		)
		request.AddCookie(adminCookie)
		if origin != "" {
			request.Header.Set("Origin", origin)
			request.Header.Set("X-CSRF-Protection", "1")
		}
		result := httptest.NewRecorder()
		server.ServeHTTP(result, request)
		if result.Code != http.StatusForbidden {
			t.Fatal("跨站写请求未被拒绝")
		}
	}
	preflight := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodOptions,
		"/v1/login",
		nil,
	)
	preflight.Header.Set("Origin", config.Config.HTTP.CORSAllowOrigins[0])
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "content-type,x-csrf-protection")
	preflightResult := httptest.NewRecorder()
	server.ServeHTTP(preflightResult, preflight)
	if preflightResult.Code != http.StatusNoContent ||
		preflightResult.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("跨域 Cookie 预检未正确启用")
	}

	// 管理员添加账号；重复账号、权限注入和匿名添加均不可成功。
	newUser := userModels.CreateRequest{
		Username:       "fixture.user",
		DisplayName:    "虚构用户",
		PasswordSHA256: digest("fixture.user", temporaryPassword),
	}
	send(t, server, http.MethodPost, "/v1/system/users", newUser, nil, http.StatusUnauthorized)
	send(t, server, http.MethodPost, "/v1/system/users", newUser, adminCookie, http.StatusOK)
	send(t, server, http.MethodPost, "/v1/system/users", newUser, adminCookie, http.StatusConflict)
	send(t, server, http.MethodPost, "/v1/system/users", map[string]any{
		"username":        "injected",
		"display_name":    "权限注入",
		"password_sha256": newUser.PasswordSHA256,
		"is_admin":        true,
	}, adminCookie, http.StatusBadRequest)
	userCookie := sessionCookie(t, login(t, server, newUser.Username, temporaryPassword))
	send(t, server, http.MethodGet, "/v1/workspaces", nil, userCookie, http.StatusForbidden)
	send(t, server, http.MethodPost, "/v1/system/users", newUser, userCookie, http.StatusForbidden)
	var ordinary userModels.User
	if err := connection.Where("username = ?", newUser.Username).
		First(&ordinary).
		Error; err != nil ||
		ordinary.IsAdmin {
		t.Fatal("普通用户被错误授予管理员权限")
	}
	var createAudit auditModels.Record
	if err := connection.Where("action_id = ? AND result_code = ? AND instance_id = ?", accessConstants.UserCreate, 0, strconv.FormatUint(ordinary.ID, 10)).
		First(&createAudit).
		Error; err != nil ||
		createAudit.UserID != admin.ID {
		t.Fatal("账号创建审计未保存操作者和目标快照")
	}
	if string(createAudit.InstanceOriginData) != "null" ||
		!strings.Contains(string(createAudit.InstanceData), ordinary.Username) ||
		!strings.Contains(string(createAudit.InstanceData), "display_name") ||
		strings.Contains(string(createAudit.InstanceData), "password") {
		t.Fatal("账号创建未保存安全的前后快照与字段差异")
	}

	// 现有会话实时反映管理员撤权与账号停用，不依赖前端菜单。
	if err := connection.Model(&admin).Update("is_admin", false).Error; err != nil {
		t.Fatal(err)
	}
	send(t, server, http.MethodPost, "/v1/system/users", newUser, adminCookie, http.StatusForbidden)
	if err := connection.Model(&admin).
		Updates(map[string]any{"is_admin": true, "is_enabled": false}).
		Error; err != nil {
		t.Fatal(err)
	}
	send(
		t,
		server,
		http.MethodGet,
		"/v1/user-info",
		nil,
		adminCookie,
		http.StatusUnauthorized,
	)
	send(
		t,
		server,
		http.MethodPost,
		"/v1/login",
		userModels.LoginRequest{
			Username:       admin.Username,
			PasswordSHA256: digest(admin.Username, testPassword),
		},
		nil,
		http.StatusUnauthorized,
	)
	send(t, server, http.MethodGet, "/v1/user-info", nil, userCookie, http.StatusOK)
	if err := connection.Model(&admin).Update("is_enabled", true).Error; err != nil {
		t.Fatal(err)
	}

	// 审计故障不影响当前用户信息，登录、添加和退出仍阻断并完整回滚。
	var sessionCount int64
	if err := connection.Model(&userModels.Session{}).Count(&sessionCount).Error; err != nil {
		t.Fatal(err)
	}
	callbackName := "test:audit-unavailable"
	if err := db.DB().
		Callback().
		Create().
		Before("gorm:create").
		Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				_ = tx.AddError(errors.New("fictional audit failure"))
			}
		}); err != nil {
		t.Fatal(err)
	}
	removeFailure := func() {
		if err := db.DB().Callback().Create().Remove(callbackName); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(removeFailure)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/user-info",
		nil,
		adminCookie,
		http.StatusOK,
	)
	failedLogin := send(
		t,
		server,
		http.MethodPost,
		"/v1/login",
		userModels.LoginRequest{
			Username:       admin.Username,
			PasswordSHA256: digest(admin.Username, testPassword),
		},
		nil,
		http.StatusServiceUnavailable,
	)
	if failedLogin.Header().Get("Set-Cookie") != "" {
		t.Fatal("审计失败仍返回了会话 Cookie")
	}
	send(
		t,
		server,
		http.MethodPost,
		"/v1/logout",
		nil,
		adminCookie,
		http.StatusServiceUnavailable,
	)
	rollbackUser := userModels.CreateRequest{
		Username:       "rollback.user",
		DisplayName:    "回滚用户",
		PasswordSHA256: digest("rollback.user", testPassword),
	}
	send(
		t,
		server,
		http.MethodPost,
		"/v1/system/users",
		rollbackUser,
		adminCookie,
		http.StatusServiceUnavailable,
	)
	userChange := changeRequest(newUser.Username, temporaryPassword, testPassword)
	failedChange := send(
		t,
		server,
		http.MethodPost,
		"/v1/change-password",
		userChange,
		userCookie,
		http.StatusServiceUnavailable,
	)
	if failedChange.Header().Get("Set-Cookie") != "" {
		t.Fatal("改密审计失败仍更新了会话 Cookie")
	}
	removeFailure()
	var afterSessions, rollbackCount int64
	if err := connection.Model(&userModels.Session{}).Count(&afterSessions).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Model(&userModels.User{}).
		Where("username = ?", rollbackUser.Username).
		Count(&rollbackCount).
		Error; err != nil {
		t.Fatal(err)
	}
	if afterSessions != sessionCount || rollbackCount != 0 {
		t.Fatal("审计故障未回滚业务写入")
	}
	send(t, server, http.MethodGet, "/v1/user-info", nil, adminCookie, http.StatusOK)
	pending := send(t, server, http.MethodGet, "/v1/user-info", nil, userCookie, http.StatusOK)
	if !data[userModels.Info](t, pending).PasswordChangeRequired {
		t.Fatal("审计故障后待改密状态未回滚")
	}
	userCookie = completePasswordChange(t, server, userCookie, userChange)
	send(t, server, http.MethodGet, "/v1/workspaces", nil, userCookie, http.StatusOK)

	// 登录旋转产生逐条删除和创建事件，两条审计必须共享请求编号。
	rotation := send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
		Username:       admin.Username,
		PasswordSHA256: digest(admin.Username, testPassword),
	}, adminCookie, http.StatusOK)
	rotationResponse := rotation.Result()
	rotatedCookies := rotationResponse.Cookies()
	if err := rotationResponse.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if len(rotatedCookies) != 1 || rotatedCookies[0].Value == adminCookie.Value {
		t.Fatal("重新登录未旋转会话")
	}
	send(
		t,
		server,
		http.MethodGet,
		"/v1/user-info",
		nil,
		adminCookie,
		http.StatusUnauthorized,
	)
	adminCookie = rotatedCookies[0]
	var rotationCount int64
	if err := connection.Model(&auditModels.Record{}).
		Where("request_id = ?", requestTrace(t, rotation)).
		Count(&rotationCount).
		Error; err != nil ||
		rotationCount != 2 {
		t.Fatal("会话逐条变更未通过请求编号关联")
	}

	checkManagement(t, server, adminCookie, userCookie)
	checkPasswordLoginPolicy(t, server, adminCookie, userCookie)
	checkStudentLogin(t, server, adminCookie)

	// 过期与主动退出都撤销认证；其他设备会话保持独立。
	if err := connection.Model(&userModels.Session{}).
		Where("user_id = ?", ordinary.ID).
		Update("expires_at", time.Now().UTC().Add(-time.Hour)).
		Error; err != nil {
		t.Fatal(err)
	}
	send(t, server, http.MethodGet, "/v1/user-info", nil, userCookie, http.StatusUnauthorized)
	logoutResult := send(
		t,
		server,
		http.MethodPost,
		"/v1/logout",
		nil,
		adminCookie,
		http.StatusOK,
	)
	if !strings.Contains(logoutResult.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatal("退出未清除 Cookie")
	}
	send(
		t,
		server,
		http.MethodGet,
		"/v1/user-info",
		nil,
		adminCookie,
		http.StatusUnauthorized,
	)

	// 物理外键保护有会话的账号；显式撤销引用后可硬删除且审计快照仍存在。
	if err := connection.Delete(&userModels.User{}, ordinary.ID).Error; err == nil {
		t.Fatal("外键未保护账号删除")
	}
	if err := connection.Where("user_id = ?", ordinary.ID).
		Delete(&userModels.Session{}).
		Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Delete(&userModels.User{}, ordinary.ID).Error; err != nil {
		t.Fatal(err)
	}
	var snapshotCount int64
	if err := connection.Model(&auditModels.Record{}).
		Where("user_id = ?", ordinary.ID).
		Count(&snapshotCount).
		Error; err != nil ||
		snapshotCount == 0 {
		t.Fatal("硬删除丢失了审计身份快照")
	}

	// 追踪中不可包含原始口令、口令摘要或原始会话。
	for _, span := range recorder.Ended() {
		for _, attribute := range span.Attributes() {
			text := attribute.Value.String()
			for _, secret := range []string{
				testPassword, temporaryPassword, digest(admin.Username, testPassword),
				request.CurrentPasswordSHA256, adminCookie.Value, admin.PasswordHash,
			} {
				if strings.Contains(text, secret) {
					t.Fatal("认证凭据进入了 Trace 属性")
				}
			}
		}
	}
}

// requestTrace 从响应追踪头取得审计使用的请求标识，独立于兼容的 X-Request-ID。
func requestTrace(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	parts := strings.Split(recorder.Header().Get("Traceparent"), "-")
	if len(parts) != 4 || len(parts[1]) != 32 {
		t.Fatal("响应缺少有效追踪头")
	}
	return parts[1]
}
