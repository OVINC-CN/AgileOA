package userSvc_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

func data[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope response.Envelope[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func checkManagement(t *testing.T, server *echo.Echo, adminCookie, ordinaryCookie *http.Cookie) {
	t.Helper()
	connection := db.DB().WithContext(t.Context())
	var admin userModels.User
	if err := connection.Where("username = ?", config.Config.User.BootstrapUsername).
		First(&admin).
		Error; err != nil {
		t.Fatal(err)
	}
	// 全部统计和列表来自真实数据库，分页、状态和关键词由服务端处理。
	stats := data[userModels.StatisticsData](
		t,
		send(
			t,
			server,
			http.MethodGet,
			"/v1/system/users/statistics",
			nil,
			adminCookie,
			http.StatusOK,
		),
	)
	if stats.Total < 2 || len(stats.Days) != config.Config.User.ChartDays || stats.Logins < 2 {
		t.Fatal("账号统计或真实登录趋势无效")
	}
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/users?page=1&page_size=1",
		nil,
		nil,
		http.StatusUnauthorized,
	)
	send(t, server, http.MethodGet, "/v1/system/users", nil, ordinaryCookie, http.StatusForbidden)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/users?page_size=100000",
		nil,
		adminCookie,
		http.StatusBadRequest,
	)
	created := data[userModels.Info](
		t,
		send(
			t,
			server,
			http.MethodPost,
			"/v1/system/users",
			userModels.CreateRequest{
				Username:       "managed.fixture",
				DisplayName:    "管理验收用户",
				Email:          "managed@example.com",
				PasswordSHA256: digest("managed.fixture", temporaryPassword),
			},
			adminCookie,
			http.StatusOK,
		),
	)
	if created.Email != "managed@example.com" {
		t.Fatal("创建用户未返回邮箱")
	}
	path := "/v1/system/users/" + strconv.FormatUint(created.ID, 10)
	cookie := sessionCookie(t, login(t, server, created.Username, temporaryPassword))
	checkManagementAuditFailure(t, server, adminCookie, created.ID)
	list := data[userModels.ListData](
		t,
		send(
			t,
			server,
			http.MethodGet,
			"/v1/system/users?keyword=managed@example.com&page=1&page_size=1",
			nil,
			adminCookie,
			http.StatusOK,
		),
	)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].LastLoginAt == nil ||
		list.Items[0].SessionCount != 1 || !list.Items[0].PasswordChangeRequired ||
		list.Items[0].Email != created.Email {
		t.Fatal("用户分页目录或登录关联无效")
	}
	cookie = completePasswordChange(
		t,
		server,
		cookie,
		changeRequest(created.Username, temporaryPassword, testPassword),
	)
	cookie = checkAPITokens(t, server, adminCookie, cookie, created)
	checkEmailLoginUsername(t, server, adminCookie, created)
	enabled, disabled := true, false
	request := userModels.UpdateRequest{DisplayName: "更新后的用户", IsEnabled: &disabled}
	send(t, server, http.MethodPut, path, request, ordinaryCookie, http.StatusForbidden)
	unchanged := data[userModels.Info](
		t,
		send(t, server, http.MethodPut, path, request, adminCookie, http.StatusOK),
	)
	if unchanged.Email != created.Email {
		t.Fatal("省略邮箱参数时应保留原值")
	}
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	request.IsEnabled = &enabled
	email := "updated@example.com"
	request.Email = &email
	updated := data[userModels.Info](
		t,
		send(t, server, http.MethodPut, path, request, adminCookie, http.StatusOK),
	)
	if updated.Email != email {
		t.Fatal("编辑用户未保存邮箱")
	}
	email = ""
	cleared := data[userModels.Info](
		t,
		send(t, server, http.MethodPut, path, request, adminCookie, http.StatusOK),
	)
	if cleared.Email != "" {
		t.Fatal("空字符串未清空邮箱")
	}
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	beforeReset := login(t, server, created.Username, testPassword)
	if data[userModels.Info](t, beforeReset).PasswordChangeRequired {
		t.Fatal("常规编辑或重新启用不应触发改密")
	}
	cookie = sessionCookie(t, beforeReset)
	const resetPassword = "Updated-fictional-password-2026"
	request.PasswordSHA256 = digest(created.Username, resetPassword)
	send(t, server, http.MethodPut, path, request, adminCookie, http.StatusOK)
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	send(
		t,
		server,
		http.MethodPost,
		"/v1/login",
		userModels.LoginRequest{
			Username:       created.Username,
			PasswordSHA256: digest(created.Username, testPassword),
		},
		nil,
		http.StatusUnauthorized,
	)
	afterReset := login(t, server, created.Username, resetPassword)
	if !data[userModels.Info](t, afterReset).PasswordChangeRequired {
		t.Fatal("管理员重置后未要求改密")
	}
	resetCookie := sessionCookie(t, afterReset)
	send(t, server, http.MethodGet, "/v1/workspaces", nil, resetCookie, http.StatusForbidden)
	completePasswordChange(
		t,
		server,
		resetCookie,
		changeRequest(created.Username, resetPassword, testPassword),
	)
	// 当前操作者无法停用或删除自身；引用保护需要单独显式撤销。
	self := "/v1/system/users/" + strconv.FormatUint(admin.ID, 10)
	send(
		t,
		server,
		http.MethodPut,
		self,
		userModels.UpdateRequest{DisplayName: admin.DisplayName, IsEnabled: &disabled},
		adminCookie,
		http.StatusConflict,
	)
	send(t, server, http.MethodDelete, self, nil, adminCookie, http.StatusConflict)
	send(t, server, http.MethodDelete, path, nil, adminCookie, http.StatusConflict)
	send(t, server, http.MethodDelete, path+"/sessions", nil, adminCookie, http.StatusOK)
	send(t, server, http.MethodDelete, path, nil, adminCookie, http.StatusOK)
	send(t, server, http.MethodPut, path, request, adminCookie, http.StatusNotFound)
	// 普通审计读取也留痕，详情保留硬删除前快照。
	auditList := data[auditModels.ListData](
		t,
		send(
			t,
			server,
			http.MethodGet,
			"/v1/system/audit/logs?action_id=user.delete&instance_id="+strconv.FormatUint(
				created.ID,
				10,
			),
			nil,
			adminCookie,
			http.StatusOK,
		),
	)
	var record auditModels.Summary
	for _, item := range auditList.Items {
		if item.InstanceID == strconv.FormatUint(created.ID, 10) && item.ResultCode == 0 {
			record = item
			break
		}
	}
	if record.ID == 0 || len(record.EventDate) != 10 {
		t.Fatal("审计查询未定位硬删除事件或日期无效")
	}
	detailPath := "/v1/system/audit/logs/" + record.EventID + "?event_date=" + record.EventDate
	detailResponse := send(t, server, http.MethodGet, detailPath, nil, adminCookie, http.StatusOK)
	detail := data[auditModels.Detail](t, detailResponse)
	var before map[string]any
	if err := json.Unmarshal(detail.InstanceOriginData, &before); err != nil {
		t.Fatal(err)
	}
	if before["username"] != created.Username || string(detail.InstanceData) != "null" {
		t.Fatal("硬删除前后快照未保留")
	}
	if strings.Contains(detailResponse.Body.String(), "password_hash") ||
		strings.Contains(detailResponse.Body.String(), "token_hash") {
		t.Fatal("审计详情暴露认证凭据")
	}
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/audit/logs",
		nil,
		ordinaryCookie,
		http.StatusForbidden,
	)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/audit/logs?start_time=1793664000000&end_time=1767225600000",
		nil,
		adminCookie,
		http.StatusBadRequest,
	)
	send(t, server, http.MethodGet, "/v1/system/audit/statistics", nil, adminCookie, http.StatusOK)
	var reads int64
	if err := connection.Model(&auditModels.Record{}).
		Where("action_id IN ?", []string{accessConstants.AuditRead, accessConstants.AuditDetail, accessConstants.AuditStatistics}).
		Count(&reads).
		Error; err != nil ||
		reads < 3 {
		t.Fatal("审计查询自身未记录")
	}
	checkAuditFilterDefaults(t, server, adminCookie, admin)
	checkAuditSourceFilters(t, server, adminCookie, admin)
	checkAuditWorkspaceScope(t, server, adminCookie, ordinaryCookie, admin)
}

func checkAuditWorkspaceScope(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	ordinaryCookie *http.Cookie,
	admin userModels.User,
) {
	t.Helper()
	original := config.Config.Audit
	t.Cleanup(func() { config.Config.Audit = original })
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	config.Config.Audit.SnapshotKeyID = "fixture"
	config.Config.Audit.SnapshotKeys = map[string]string{
		"fixture": base64.StdEncoding.EncodeToString(key),
	}
	ctx := auditUtils.WithRequest(
		t.Context(),
		auditModels.Request{Method: "test", Path: "audit-isolation"},
	)
	ctx, span := tracing.Tracer.Start(ctx, "test.audit.isolation")
	defer span.End()
	tenantID := "t4f8a2n6k9c3p5s7"
	deletedID := "d4f8a2n6k9c3p5s7"
	// 停用空间及没有工作空间实体的历史审计同样可由系统管理员查询。
	if err := db.DB().WithContext(ctx).Create(&workspaceModels.Workspace{
		ID: tenantID, Name: "虚构停用空间", Revision: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []*string{nil, &tenantID, &deletedID} {
		action := accessConstants.UserUpdate
		if tenant != nil {
			action = accessConstants.CPCPersonUpdate
		}
		if err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return auditSvc.Write(
				tx,
				auditModels.Event{
					TenantID: tenant,
					Actor: auditModels.Actor{
						ID:          admin.ID,
						Username:    admin.Username,
						DisplayName: admin.DisplayName,
						IsAdmin:     true,
					},
					Action:         action,
					ObjectType:     "user",
					ObjectID:       "fixture-isolation",
					SensitiveAfter: auditModels.Snapshot{"contact": "fictional-sensitive-contact"},
					Result:         "success",
				},
			)
		}); err != nil {
			t.Fatal(err)
		}
	}
	var records []auditModels.Record
	if err := db.DB().
		WithContext(ctx).
		Where("instance_id = ?", "fixture-isolation").
		Find(&records).
		Error; err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		date := record.EventDate[:10]
		path := "/v1/system/audit/logs/" + record.EventID + "?event_date=" + date
		send(t, server, http.MethodGet, path, nil, ordinaryCookie, http.StatusForbidden)
		response := send(t, server, http.MethodGet, path, nil, cookie, http.StatusOK)
		detail := data[auditModels.Detail](t, response)
		if !detail.HasSensitiveSnapshot ||
			strings.Contains(response.Body.String(), "fictional-sensitive-contact") ||
			strings.Contains(response.Body.String(), "sensitive_key_id") ||
			strings.Contains(response.Body.String(), `"sensitive_snapshot":`) {
			t.Fatal("普通审计入口暴露敏感快照")
		}
	}
	for _, scenario := range []struct {
		name   string
		ids    []string
		action string
		total  int64
	}{
		{name: "all", total: 3},
		{name: "system", ids: []string{"system"}, total: 1},
		{name: "disabled", ids: []string{tenantID}, total: 1},
		{name: "deleted", ids: []string{deletedID}, total: 1},
		{name: "multiple", ids: []string{tenantID, deletedID}, total: 2},
		{name: "mixed", ids: []string{"system", deletedID}, total: 2},
		{name: "workspace-action", ids: []string{tenantID}, action: accessConstants.CPCPersonUpdate, total: 1},
		{name: "other-action", ids: []string{tenantID}, action: accessConstants.UserUpdate},
		{name: "missing", ids: []string{"m4f8a2n6k9c3p5s7"}},
	} {
		t.Run("audit-workspace-"+scenario.name, func(t *testing.T) {
			// 列表、总数及趋势应用相同空间条件，分页不影响统计或空间选项。
			parameters := url.Values{"instance_id": {"fixture-isolation"}, "page_size": {"1"}}
			for _, id := range scenario.ids {
				parameters.Add("workspace_id", id)
			}
			if scenario.action != "" {
				parameters.Set("action_id", scenario.action)
			}
			list := data[auditModels.ListData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/logs?"+parameters.Encode(), nil, cookie, http.StatusOK))
			stats := data[auditModels.StatisticsData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/statistics?"+parameters.Encode(), nil, cookie, http.StatusOK))
			if list.Total != scenario.total || stats.Total != scenario.total ||
				stats.Success != scenario.total || stats.Failure != 0 || stats.Denied != 0 ||
				len(list.Items) != int(min(scenario.total, 1)) {
				t.Fatal("审计列表、分页或统计未使用相同工作空间条件")
			}
			var total int64
			for _, day := range stats.Days {
				total += day.Success + day.Failure
			}
			if total != scenario.total {
				t.Fatal("审计趋势未应用工作空间筛选")
			}
			for _, item := range list.Items {
				id := item.WorkspaceID
				if len(scenario.ids) > 0 && !slices.Contains(scenario.ids, id) {
					t.Fatal("指定空间查询返回其他空间审计")
				}
			}
			for _, id := range []string{"system", tenantID, deletedID} {
				if !slices.ContainsFunc(list.Workspaces, func(option auditModels.Option) bool {
					return option.Value == id
				}) {
					t.Fatal("空间选项未保留全局、停用或已删除空间")
				}
			}
			if !slices.ContainsFunc(list.Actions, func(option auditModels.Option) bool {
				return option.Value == accessConstants.CPCPersonUpdate
			}) {
				t.Fatal("审计操作目录缺少业务操作")
			}
		})
	}
	for _, endpoint := range []string{"logs", "statistics"} {
		path := "/v1/system/audit/" + endpoint + "?workspace_id=" + tenantID
		send(t, server, http.MethodGet, path, nil, ordinaryCookie, http.StatusForbidden)
		send(t, server, http.MethodGet, path, nil, nil, http.StatusUnauthorized)
	}
}

func checkManagementAuditFailure(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	targetID uint64,
) {
	t.Helper()
	connection := db.DB().WithContext(t.Context())
	temporary := data[userModels.Info](
		t,
		send(
			t,
			server,
			http.MethodPost,
			"/v1/system/users",
			userModels.CreateRequest{
				Username:       "rollback.delete",
				DisplayName:    "删除回滚验收",
				PasswordSHA256: digest("rollback.delete", testPassword),
			},
			cookie,
			http.StatusOK,
		),
	)
	var before userModels.User
	if err := connection.First(&before, targetID).Error; err != nil {
		t.Fatal(err)
	}
	var beforeSessions int64
	if err := connection.Model(&userModels.Session{}).
		Where("user_id = ?", targetID).
		Count(&beforeSessions).
		Error; err != nil {
		t.Fatal(err)
	}
	const callbackName = "test:management-audit-unavailable"
	if err := db.DB().
		Callback().
		Create().
		Before("gorm:create").
		Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				_ = tx.AddError(gorm.ErrInvalidData)
			}
		}); err != nil {
		t.Fatal(err)
	}
	active := true
	cleanup := func() {
		if active {
			active = false
			if err := db.DB().Callback().Create().Remove(callbackName); err != nil {
				t.Error(err)
			}
		}
	}
	defer cleanup()
	enabled := true
	passwordLoginEnabled := false
	path := "/v1/system/users/" + strconv.FormatUint(targetID, 10)
	send(t, server, http.MethodGet, "/v1/system/users", nil, cookie, http.StatusServiceUnavailable)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/users/statistics",
		nil,
		cookie,
		http.StatusServiceUnavailable,
	)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/audit/logs",
		nil,
		cookie,
		http.StatusServiceUnavailable,
	)
	send(
		t,
		server,
		http.MethodGet,
		"/v1/system/audit/statistics",
		nil,
		cookie,
		http.StatusServiceUnavailable,
	)
	send(
		t,
		server,
		http.MethodPut,
		path,
		userModels.UpdateRequest{
			DisplayName:          "不应保存的名称",
			IsEnabled:            &enabled,
			PasswordLoginEnabled: &passwordLoginEnabled,
		},
		cookie,
		http.StatusServiceUnavailable,
	)
	send(t, server, http.MethodDelete, path+"/sessions", nil, cookie, http.StatusServiceUnavailable)
	send(
		t,
		server,
		http.MethodDelete,
		"/v1/system/users/"+strconv.FormatUint(temporary.ID, 10),
		nil,
		cookie,
		http.StatusServiceUnavailable,
	)
	cleanup()
	var after userModels.User
	var afterSessions, remaining int64
	if err := connection.First(&after, targetID).Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Model(&userModels.Session{}).
		Where("user_id = ?", targetID).
		Count(&afterSessions).
		Error; err != nil {
		t.Fatal(err)
	}
	if err := connection.Model(&userModels.User{}).
		Where("id = ?", temporary.ID).
		Count(&remaining).
		Error; err != nil {
		t.Fatal(err)
	}
	if after.DisplayName != before.DisplayName || afterSessions != beforeSessions ||
		after.PasswordLoginEnabled != before.PasswordLoginEnabled ||
		after.PasswordHash != before.PasswordHash ||
		after.PasswordChangeRequired != before.PasswordChangeRequired ||
		after.SessionVersion != before.SessionVersion ||
		remaining != 1 {
		t.Fatal("审计故障没有完整回滚更新、会话撤销或用户删除")
	}
	send(
		t,
		server,
		http.MethodDelete,
		"/v1/system/users/"+strconv.FormatUint(temporary.ID, 10),
		nil,
		cookie,
		http.StatusOK,
	)
}
