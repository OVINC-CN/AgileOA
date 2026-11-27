package userSvc_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func sendBearer(
	t *testing.T,
	server *echo.Echo,
	method, path, token string,
	cookie *http.Cookie,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	request.Header.Set(userConstants.CSRFHeader, userConstants.CSRFValue)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	server.ServeHTTP(result, request)
	if result.Code != status {
		t.Fatalf("%s %s 状态错误：%d", method, path, result.Code)
	}
	return result
}

func sendMCP(
	t *testing.T,
	server *echo.Echo,
	token string,
	cookie *http.Cookie,
	method string,
	params any,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()
	content, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": method, "params": params,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/v1/mcp", bytes.NewReader(content),
	)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Set(echo.HeaderAccept, "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-11-25")
	request.Header.Set(userConstants.CSRFHeader, userConstants.CSRFValue)
	if token != "" {
		request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	result := httptest.NewRecorder()
	server.ServeHTTP(result, request)
	if result.Code != status {
		t.Fatalf("MCP %s 状态错误：%d", method, result.Code)
	}
	return result
}

func sendMCPList(
	t *testing.T,
	server *echo.Echo,
	token string,
	cookie *http.Cookie,
	status int,
) {
	t.Helper()
	result := sendMCP(t, server, token, cookie, "tools/list", map[string]any{}, status)
	if status != http.StatusOK {
		return
	}
	var wire struct {
		Result struct {
			Tools []json.RawMessage `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(
		result.Body.Bytes(),
		&wire,
	); err != nil ||
		len(wire.Result.Tools) == 0 {
		t.Fatal("有效 API Token 未返回 MCP 工具目录")
	}
}

func checkAPITokens(
	t *testing.T,
	server *echo.Echo,
	adminCookie, cookie *http.Cookie,
	user userModels.Info,
) *http.Cookie {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	original := config.Config.User.APITokenKey
	config.Config.User.APITokenKey = base64.StdEncoding.EncodeToString(key)
	defer func() { config.Config.User.APITokenKey = original }()
	path := "/v1/system/users/" + strconv.FormatUint(user.ID, 10)
	tokenPath := path + "/api-token"
	issue := func() userModels.APITokenData {
		return data[userModels.APITokenData](t,
			send(t, server, http.MethodPut, tokenPath, nil, adminCookie, http.StatusOK))
	}

	// 管理操作仅限系统管理员，生成后可反复查看，普通目录不包含凭据。
	send(t, server, http.MethodPut, tokenPath, nil, cookie, http.StatusForbidden)
	first := issue()
	if !strings.HasPrefix(first.Token, userConstants.APITokenPrefix) ||
		first.CreatedAt == nil || first.UpdatedAt == nil {
		t.Fatal("API Token 生成结果无效")
	}
	for range 2 {
		viewed := data[userModels.APITokenData](t,
			send(t, server, http.MethodGet, tokenPath, nil, adminCookie, http.StatusOK))
		if viewed.Token != first.Token {
			t.Fatal("再次查看没有返回原始凭据")
		}
	}
	var stored userModels.APIToken
	connection := db.DB().WithContext(t.Context())
	if err := connection.First(&stored, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash == first.Token || strings.Contains(string(stored.Ciphertext), first.Token) {
		t.Fatal("数据库保存了明文凭据")
	}

	// MCP 只读取 Token，其他接口只读取 Cookie，混合凭据也不跨入口回退。
	sendBearer(t, server, http.MethodGet, "/v1/user-info", first.Token, nil,
		http.StatusUnauthorized)
	sendMCPList(t, server, first.Token, nil, http.StatusOK)
	sendMCPList(t, server, "", cookie, http.StatusUnauthorized)
	sendMCPList(t, server, "invalid", adminCookie, http.StatusUnauthorized)
	sendMCPList(t, server, cookie.Value, nil, http.StatusUnauthorized)
	info := data[userModels.Info](t,
		sendBearer(t, server, http.MethodGet, "/v1/user-info", "invalid", adminCookie,
			http.StatusOK))
	if !info.IsAdmin || info.ID == user.ID {
		t.Fatal("普通 API 未使用 Cookie 所属账号")
	}
	sendBearer(t, server, http.MethodGet, "/v1/system/users", first.Token, adminCookie,
		http.StatusOK)
	checkAuthenticationQueryCount(t, server, cookie, first.Token)
	checkMCPReadPermissions(t, server, adminCookie, first.Token, user.ID)
	checkAPITokenWorkspace(t, server, adminCookie, cookie, first.Token, user.ID)

	// 重置立即作废旧值；审计故障时查看被阻断，重置、撤销和退出全部回滚。
	second := issue()
	if second.Token == first.Token {
		t.Fatal("重置未替换凭据")
	}
	sendMCPList(t, server, first.Token, nil, http.StatusUnauthorized)
	checkAPITokenAuditRollback(t, server, adminCookie, cookie, tokenPath, path, second.Token)
	sendMCPList(t, server, second.Token, nil, http.StatusOK)
	send(t, server, http.MethodDelete, path+"/sessions", nil, adminCookie, http.StatusOK)
	sendMCPList(t, server, second.Token, nil, http.StatusUnauthorized)
	assertNoAPIToken(t, user.ID)
	cookie = sessionCookie(t, login(t, server, user.Username, testPassword))
	third := issue()
	send(t, server, http.MethodPost, "/v1/logout", nil, cookie, http.StatusOK)
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	assertAPITokenActive(t, server, adminCookie, tokenPath, third.Token)
	cookie = sessionCookie(t, login(t, server, user.Username, testPassword))
	sendBearer(t, server, http.MethodPost, "/v1/logout", third.Token, nil,
		http.StatusUnauthorized)
	assertAPITokenActive(t, server, adminCookie, tokenPath, third.Token)

	// 重置及完成临时密码修改只作废 Cookie，会话版本变化不影响已有 Token。
	enabled, disabled := true, false
	update := userModels.UpdateRequest{
		DisplayName: user.DisplayName, IsEnabled: &enabled,
		PasswordSHA256: digest(user.Username, temporaryPassword),
	}
	send(t, server, http.MethodPut, path, update, adminCookie, http.StatusOK)
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	assertAPITokenActive(t, server, adminCookie, tokenPath, third.Token)
	cookie = sessionCookie(t, login(t, server, user.Username, temporaryPassword))
	send(t, server, http.MethodGet, "/v1/workspaces", nil, cookie, http.StatusForbidden)
	cookie = completePasswordChange(
		t, server, cookie, changeRequest(user.Username, temporaryPassword, testPassword),
	)
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusOK)
	assertAPITokenActive(t, server, adminCookie, tokenPath, third.Token)

	// 停用时删除 Token，重新启用账号不能恢复已撤销的凭据。
	update.PasswordSHA256 = ""
	update.IsEnabled = &disabled
	send(t, server, http.MethodPut, path, update, adminCookie, http.StatusOK)
	send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusUnauthorized)
	sendMCPList(t, server, third.Token, nil, http.StatusUnauthorized)
	assertNoAPIToken(t, user.ID)
	update.IsEnabled = &enabled
	send(t, server, http.MethodPut, path, update, adminCookie, http.StatusOK)
	sendMCPList(t, server, third.Token, nil, http.StatusUnauthorized)
	assertNoAPIToken(t, user.ID)
	cookie = sessionCookie(t, login(t, server, user.Username, testPassword))
	fourth := issue()
	sendMCPList(t, server, fourth.Token, nil, http.StatusOK)
	send(t, server, http.MethodDelete, tokenPath, nil, adminCookie, http.StatusOK)
	sendMCPList(t, server, fourth.Token, nil, http.StatusUnauthorized)
	assertNoAPIToken(t, user.ID)

	// 查看及删除 Token 不允许泄露普通审计中的认证信息。
	var records []auditModels.Record
	if err := connection.Where("resource_type_id = ? AND instance_id = ?",
		"user_api_token", strconv.FormatUint(user.ID, 10)).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("API Token 操作缺少审计")
	}
	for _, record := range records {
		for _, snapshot := range [][]byte{record.InstanceOriginData, record.InstanceData, record.ExtendData, record.SnapshotUserInfo} {
			for _, secret := range []string{
				first.Token, second.Token, third.Token, fourth.Token, stored.TokenHash,
			} {
				if strings.Contains(string(snapshot), secret) {
					t.Fatal("认证凭据进入普通审计")
				}
			}
		}
	}
	return cookie
}

func assertAPITokenActive(
	t *testing.T,
	server *echo.Echo,
	adminCookie *http.Cookie,
	tokenPath, token string,
) {
	t.Helper()
	viewed := data[userModels.APITokenData](t, send(
		t, server, http.MethodGet, tokenPath, nil, adminCookie, http.StatusOK,
	))
	if viewed.Token != token {
		t.Fatal("已有 API Token 被意外替换或删除")
	}

	// 实际调用业务查询，覆盖入口及事务内的待改密检查。
	result := sendMCP(t, server, token, nil, "tools/call", map[string]any{
		"name": "get_workspaces", "arguments": map[string]any{},
	}, http.StatusOK)
	var wire struct {
		Result struct {
			IsError bool `json:"isError"`
			Meta    struct {
				HTTPStatus int `json:"http_status"`
			} `json:"_meta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Result.IsError || wire.Result.Meta.HTTPStatus != http.StatusOK {
		t.Fatal("已有 API Token 无法继续查询业务接口")
	}
}

func checkAuthenticationQueryCount(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	token string,
) {
	t.Helper()
	for _, useToken := range []bool{false, true} {
		// 分别记录 Cookie 和 MCP Token 的完整请求，均只允许一次认证查询。
		recorder := tracetest.NewSpanRecorder()
		tracing.Provider.RegisterSpanProcessor(recorder)
		if useToken {
			sendMCPList(t, server, token, nil, http.StatusOK)
		} else {
			send(t, server, http.MethodGet, "/v1/user-info", nil, cookie, http.StatusOK)
		}
		tracing.Provider.UnregisterSpanProcessor(recorder)
		queries := 0
		for _, span := range recorder.Ended() {
			if strings.HasPrefix(span.Name(), "select ") {
				queries++
			}
		}
		if queries != 1 {
			t.Fatalf("认证重复查询：Token=%t，查询数=%d", useToken, queries)
		}
	}
}

func assertNoAPIToken(t *testing.T, userID uint64) {
	t.Helper()
	var count int64
	if err := db.DB().WithContext(t.Context()).Model(&userModels.APIToken{}).
		Where("user_id = ?", userID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("撤销凭据后仍保留 API Token")
	}
}

func checkMCPReadPermissions(
	t *testing.T,
	server *echo.Echo,
	adminCookie *http.Cookie,
	token string,
	userID uint64,
) {
	t.Helper()
	for _, test := range []struct {
		name   string
		status int
	}{
		{"get_user_info", http.StatusOK},
		{"get_system_access_actions", http.StatusForbidden},
	} {
		// 管理员 Cookie 不会改变普通账号 Token 对应的身份或权限。
		result := sendMCP(t, server, token, adminCookie, "tools/call", map[string]any{
			"name": test.name, "arguments": map[string]any{},
		}, http.StatusOK)
		var wire struct {
			Result struct {
				IsError           bool                               `json:"isError"`
				Meta              map[string]int                     `json:"_meta"`
				StructuredContent response.Envelope[json.RawMessage] `json:"structuredContent"`
			} `json:"result"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &wire); err != nil ||
			wire.Result.Meta["http_status"] != test.status ||
			wire.Result.IsError != (test.status == http.StatusForbidden) {
			t.Fatalf("工具 %s 未保留原接口权限结果", test.name)
		}
		envelope := wire.Result.StructuredContent
		if test.status == http.StatusForbidden {
			if envelope.ErrorMsg == "" || string(envelope.Data) != "null" {
				t.Fatal("系统工具未阻断未授权账号")
			}
			continue
		}
		var info userModels.Info
		if err := json.Unmarshal(envelope.Data, &info); err != nil ||
			info.ID != userID || info.IsAdmin {
			t.Fatal("当前用户工具未返回 Token 所属账号")
		}
	}
}

func checkAPITokenWorkspace(
	t *testing.T,
	server *echo.Echo,
	adminCookie, cookie *http.Cookie,
	token string,
	userID uint64,
) {
	t.Helper()
	connection := db.DB().WithContext(t.Context())
	workspace := workspaceModels.Workspace{
		ID: "a1p2i3t4o5k6e7n8", Name: "Token 验收空间", IsEnabled: true, Revision: 1,
	}
	if err := connection.Create(&workspace).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Where("workspace_id = ?", workspace.ID).
			Delete(&accessModels.Permission{}).Error; err != nil {
			t.Error(err)
		}
		if err := connection.Delete(&workspace).Error; err != nil {
			t.Error(err)
		}
	}()
	path := "/v1/workspaces/" + workspace.ID + "/students/fields"
	checkSource := func(result *httptest.ResponseRecorder, source string) {
		var record auditModels.Record
		expected := uint8(auditConstants.AccessWeb)
		if source == auditConstants.SourceMCP {
			expected = auditConstants.AccessMCP
		}
		if err := connection.Select("access_type").Where("request_id = ? AND action_id = ?",
			requestTrace(t, result), accessConstants.StudentList).
			Take(&record).Error; err != nil || record.AccessType != expected {
			t.Fatal("原接口审计未保存请求入口来源")
		}
	}
	query := func(cookie *http.Cookie, status int) {
		result := sendMCP(t, server, token, cookie, "tools/call", map[string]any{
			"name":      "get_workspace_students_fields",
			"arguments": map[string]any{"workspace_id": workspace.ID},
		}, http.StatusOK)
		var wire struct {
			Result struct {
				IsError bool `json:"isError"`
				Meta    struct {
					HTTPStatus int `json:"http_status"`
				} `json:"_meta"`
				StructuredContent response.Envelope[json.RawMessage] `json:"structuredContent"`
			} `json:"result"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Result.Meta.HTTPStatus != status ||
			wire.Result.IsError != (status >= http.StatusBadRequest) ||
			len(wire.Result.StructuredContent.Trace) != 32 {
			t.Fatal("MCP 工具未保留原接口的鉴权结果或响应")
		}
		checkSource(result, auditConstants.SourceMCP)
	}
	query(nil, http.StatusForbidden)
	permission := accessModels.Permission{
		UserID: userID, WorkspaceID: workspace.ID, Action: accessConstants.StudentList,
	}
	if err := connection.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	result := send(t, server, http.MethodGet, path, nil, cookie, http.StatusOK)
	checkSource(result, auditConstants.SourceWeb)
	query(nil, http.StatusOK)
	// 管理员 Cookie 没有空间权限，仍应使用 Token 所属账号的授权。
	query(adminCookie, http.StatusOK)
	if err := connection.Delete(&permission).Error; err != nil {
		t.Fatal(err)
	}
	query(nil, http.StatusForbidden)
}

func checkAPITokenAuditRollback(
	t *testing.T,
	server *echo.Echo,
	adminCookie, cookie *http.Cookie,
	tokenPath, userPath, token string,
) {
	t.Helper()
	const callback = "test:api-token-audit-failure"
	if err := db.DB().Callback().Create().Before("gorm:create").
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
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		send(t, server, method, tokenPath, nil, adminCookie, http.StatusServiceUnavailable)
	}
	send(t, server, http.MethodDelete, userPath+"/sessions", nil, adminCookie,
		http.StatusServiceUnavailable)
	disabled := false
	send(t, server, http.MethodPut, userPath, userModels.UpdateRequest{
		DisplayName: "Token 验收用户", IsEnabled: &disabled,
	}, adminCookie, http.StatusServiceUnavailable)
	send(t, server, http.MethodPost, "/v1/logout", nil, cookie, http.StatusServiceUnavailable)
	sendMCPList(t, server, token, nil, http.StatusOK)
}
