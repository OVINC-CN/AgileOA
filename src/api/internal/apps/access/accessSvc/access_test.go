package accessSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

var _ = flag.String("config", "", "独立权限验收数据库的 YAML 配置")

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
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	if recorder.Code != status {
		t.Fatalf(
			"%s %s：期望 %d，实际 %d，%s",
			method,
			path,
			status,
			recorder.Code,
			recorder.Body.String(),
		)
	}
	var envelope response.Envelope[json.RawMessage]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Trace) != 32 || (status >= 400 && string(envelope.Data) != "null") {
		t.Fatal("请求缺少追踪或失败响应泄露业务数据")
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

func fixtureUser(t *testing.T, username, token string, admin bool) (userModels.User, *http.Cookie) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(
		[]byte(strings.Repeat("0", 64)),
		config.Config.User.BcryptCost,
	)
	if err != nil {
		t.Fatal(err)
	}
	user := userModels.User{
		Username: username, DisplayName: "虚构验收账号", PasswordHash: string(hash),
		IsAdmin: admin, IsEnabled: true, SessionVersion: 1,
	}
	connection := db.DB().WithContext(t.Context())
	if err := connection.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if token == "" {
		return user, nil
	}
	digest := sha256.Sum256([]byte(token))
	session := userModels.Session{
		UserID: user.ID, SessionVersion: 1, TokenHash: hex.EncodeToString(digest[:]),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := connection.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	return user, &http.Cookie{
		Name: config.Config.User.CookieName, Value: token,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
}

func permissionPath(workspaceID string, userID uint64) string {
	return "/v1/system/access/workspaces/" + workspaceID + "/users/" + strconv.FormatUint(
		userID,
		10,
	) + "/actions"
}

func replaceActions(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	workspaceID string,
	userID uint64,
	actions []string,
	status int,
) {
	t.Helper()
	path := permissionPath(workspaceID, userID)
	current := decode[accessModels.ActionsData](
		t,
		request(t, server, http.MethodGet, path, nil, cookie, 200),
	)
	request(
		t,
		server,
		http.MethodPut,
		path,
		accessModels.UpdateActionsRequest{
			Actions:  actions,
			Revision: current.Revision,
		},
		cookie,
		status,
	)
}

func createWorkspace(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	name string,
) workspaceModels.Info {
	t.Helper()
	workspace := decode[workspaceModels.Info](
		t,
		request(t, server, http.MethodPost, "/v1/system/workspaces",
			workspaceModels.CreateRequest{Name: name, Description: "仅用于独立数据库的虚构数据"}, cookie, 200),
	)
	if !workspaceUtils.ValidID(workspace.ID) {
		t.Fatal("工作空间编号不是 16 位字母数字混合随机串")
	}
	return workspace
}

func TestWorkspacePermissionsAndAuditTransactions(t *testing.T) {
	// 只接受已人工安装 SQL 的空验收库，不迁移、清理或使用真实业务数据。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立权限验收 MySQL 配置后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_access_test") {
		t.Fatal("验收数据库名称必须以 _access_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("权限验收要求独立的空测试数据库")
	}
	_, adminCookie := fixtureUser(t, "fixture.admin", strings.Repeat("a1", 32), true)
	user, cookie := fixtureUser(t, "fixture.user", strings.Repeat("b2", 32), false)
	other, _ := fixtureUser(t, "fixture.other", "", false)
	server := httpd.New()
	first := createWorkspace(t, server, adminCookie, "虚构空间甲")
	second := createWorkspace(t, server, adminCookie, "虚构空间乙")
	request(t, server, http.MethodGet, "/v1/system/workspaces/enum", nil, nil, 401)
	request(t, server, http.MethodGet, "/v1/system/workspaces/enum", nil, cookie, 403)
	options := decode[[]workspaceModels.EnumOption](t, request(t, server, http.MethodGet,
		"/v1/system/workspaces/enum?page_size=1", nil, adminCookie, 200))
	if len(options) != 2 {
		t.Fatal("工作空间枚举被分页或没有返回全部空间")
	}
	path := "/v1/workspaces/" + first.ID
	managePath := "/v1/system/workspaces/" + first.ID
	request(t, server, http.MethodGet, "/v1/workspaces", nil, nil, 401)
	request(t, server, http.MethodGet, "/v1/system/workspaces", nil, cookie, 403)
	request(
		t,
		server,
		http.MethodPost,
		"/v1/system/workspaces",
		workspaceModels.CreateRequest{Name: "越权"},
		cookie,
		403,
	)
	request(t, server, http.MethodGet, "/v1/system/access/actions", nil, cookie, 403)
	request(t, server, http.MethodGet, permissionPath(first.ID, user.ID), nil, cookie, 403)
	request(t, server, http.MethodGet, "/v1/workspaces/1", nil, adminCookie, 400)
	request(t, server, http.MethodGet, path, nil, cookie, 403)
	request(t, server, http.MethodGet, path, nil, adminCookie, 403)
	list := decode[workspaceModels.ListData](
		t,
		request(t, server, http.MethodGet, "/v1/workspaces", nil, cookie, 200),
	)
	if list.Total != 0 {
		t.Fatal("无权限账号可见工作空间")
	}
	checkAssignmentWorkspaceAccess(t, server, adminCookie, cookie, first.ID, user.ID)
	checkPermissionUserList(t, server, adminCookie, first.ID, second.ID, user.ID, other.ID)

	// 未登记、重复、系统管理操作及空值均不能成为有效授权。
	for _, actions := range [][]string{{"unknown.read"}, {accessConstants.UserCreate},
		{accessConstants.AccessWorkspace, accessConstants.AccessWorkspace}, nil} {
		replaceActions(t, server, adminCookie, first.ID, user.ID, actions, 400)
	}
	if err := connection.Create(&accessModels.Permission{
		UserID:      user.ID,
		WorkspaceID: second.ID,
		Action:      accessConstants.UserInfo,
	}).Error; err != nil {
		t.Fatal(err)
	}
	list = decode[workspaceModels.ListData](
		t,
		request(t, server, http.MethodGet, "/v1/workspaces", nil, cookie, 200),
	)
	if list.Total != 0 {
		t.Fatal("反向列表未严格要求 access_workspace")
	}
	replaceActions(t, server, adminCookie, second.ID, user.ID, []string{}, 200)
	replaceActions(
		t,
		server,
		adminCookie,
		first.ID,
		user.ID,
		[]string{accessConstants.AccessWorkspace},
		200,
	)
	replaceActions(
		t,
		server,
		adminCookie,
		second.ID,
		other.ID,
		[]string{accessConstants.AccessWorkspace},
		200,
	)
	list = decode[workspaceModels.ListData](
		t,
		request(t, server, http.MethodGet, "/v1/workspaces?user_id=1", nil, cookie, 200),
	)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != first.ID {
		t.Fatal("用户工作空间列表出现跨用户或跨空间授权")
	}
	request(t, server, http.MethodGet, path, nil, cookie, 200)
	request(t, server, http.MethodGet, "/v1/workspaces/"+second.ID, nil, cookie, 403)
	request(t, server, http.MethodDelete, managePath, nil, cookie, 403)
	request(t, server, http.MethodDelete, managePath, nil, adminCookie, 409)
	request(
		t,
		server,
		http.MethodDelete,
		"/v1/system/users/"+strconv.FormatUint(other.ID, 10),
		nil,
		adminCookie,
		409,
	)

	// 撤权立即生效，旧页面不能使用旧版本重新授予刚撤销的权限。
	old := decode[accessModels.ActionsData](
		t,
		request(
			t,
			server,
			http.MethodGet,
			permissionPath(first.ID, user.ID),
			nil,
			adminCookie,
			200,
		),
	)
	replaceActions(t, server, adminCookie, first.ID, user.ID, []string{}, 200)
	request(t, server, http.MethodGet, path, nil, cookie, 403)
	request(
		t,
		server,
		http.MethodPut,
		permissionPath(first.ID, user.ID),
		accessModels.UpdateActionsRequest{
			Actions: []string{accessConstants.AccessWorkspace}, Revision: old.Revision,
		},
		adminCookie,
		409,
	)
	replaceActions(
		t,
		server,
		adminCookie,
		first.ID,
		user.ID,
		[]string{accessConstants.AccessWorkspace},
		200,
	)

	// 模拟入口中间件检查后发生撤权，业务事务必须在空间锁内再次拒绝。
	revokeBeforeBusiness := true
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if revokeBeforeBusiness && c.Request().Method == http.MethodGet &&
				c.Request().URL.Path == path {
				revokeBeforeBusiness = false
				replaceActions(t, server, adminCookie, first.ID, user.ID, []string{}, 200)
			}
			return next(c)
		}
	})
	request(t, server, http.MethodGet, path, nil, cookie, 403)
	replaceActions(
		t,
		server,
		adminCookie,
		first.ID,
		user.ID,
		[]string{accessConstants.AccessWorkspace},
		200,
	)
	checkDisabledWorkspace(t, server, adminCookie, cookie, first.ID)
	checkAuditRollback(t, server, adminCookie, cookie, first.ID, user.ID)

	// 显式解除权限后才能删除；删除后仍保留空间随机编号 与操作快照。
	replaceActions(t, server, adminCookie, first.ID, user.ID, []string{}, 200)
	replaceActions(t, server, adminCookie, second.ID, other.ID, []string{}, 200)
	request(t, server, http.MethodDelete, managePath, nil, adminCookie, 200)
	request(t, server, http.MethodDelete, "/v1/system/workspaces/"+second.ID, nil, adminCookie, 200)
	request(
		t,
		server,
		http.MethodDelete,
		"/v1/system/users/"+strconv.FormatUint(other.ID, 10),
		nil,
		adminCookie,
		200,
	)
	if err := connection.Model(&auditModels.Record{}).
		Where("workspace_id = ?", first.ID).
		Count(&count).
		Error; err != nil ||
		count == 0 {
		t.Fatal("工作空间删除导致历史审计丢失")
	}
	request(t, server, http.MethodPut, "/v1/system/users/"+strconv.FormatUint(user.ID, 10),
		map[string]any{"display_name": user.DisplayName, "is_enabled": false}, adminCookie, 200)
	request(t, server, http.MethodGet, "/v1/workspaces", nil, cookie, 401)
}

func checkDisabledWorkspace(t *testing.T, server *echo.Echo, admin, user *http.Cookie, id string) {
	t.Helper()
	for _, enabled := range []bool{false, true} {
		path := "/v1/system/workspaces/" + id
		current := decode[workspaceModels.Info](
			t,
			request(t, server, http.MethodGet, path, nil, admin, 200),
		)
		request(t, server, http.MethodPut, path, workspaceModels.UpdateRequest{
			CreateRequest: workspaceModels.CreateRequest{
				Name:        current.Name,
				Description: current.Description,
			},
			IsEnabled: &enabled,
			Revision:  current.Revision,
		}, admin, 200)
		status := http.StatusForbidden
		if enabled {
			status = http.StatusOK
		}
		request(t, server, http.MethodGet, "/v1/workspaces/"+id, nil, user, status)
	}
}

func checkAuditRollback(
	t *testing.T,
	server *echo.Echo,
	admin, user *http.Cookie,
	workspaceID string,
	userID uint64,
) {
	t.Helper()
	path := permissionPath(workspaceID, userID)
	before := decode[accessModels.ActionsData](
		t,
		request(t, server, http.MethodGet, path, nil, admin, 200),
	)
	const callbackName = "test:access-audit-unavailable"
	seen := 0
	if err := db.DB().
		Callback().
		Create().
		Before("gorm:create").
		Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				seen++
				if seen == 2 {
					_ = tx.AddError(gorm.ErrInvalidData)
				}
			}
		}); err != nil {
		t.Fatal(err)
	}
	request(t, server, http.MethodPut, path, accessModels.UpdateActionsRequest{
		Actions: []string{}, Revision: before.Revision,
	}, admin, 503)
	if err := db.DB().Callback().Create().Remove(callbackName); err != nil {
		t.Fatal(err)
	}
	after := decode[accessModels.ActionsData](
		t,
		request(t, server, http.MethodGet, path, nil, admin, 200),
	)
	if after.Revision != before.Revision || len(after.Actions) != 1 || seen < 3 {
		t.Fatal("逐条授权审计中途失败未回滚权限与工作空间版本")
	}

	// 审计完全不可用时，读取和创建都必须阻断，创建的数据不能提交。
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
	request(t, server, http.MethodGet, "/v1/workspaces/"+workspaceID, nil, user, 503)
	request(t, server, http.MethodGet, "/v1/workspaces", nil, user, 503)
	request(t, server, http.MethodGet, "/v1/system/workspaces/enum", nil, admin, 503)
	request(
		t,
		server,
		http.MethodPost,
		"/v1/system/workspaces",
		workspaceModels.CreateRequest{Name: "虚构回滚空间"},
		admin,
		503,
	)
	if err := db.DB().Callback().Create().Remove(callbackName); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.DB().WithContext(t.Context()).Model(&workspaceModels.Workspace{}).
		Where("name = ?", "虚构回滚空间").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("工作空间创建未随审计故障回滚")
	}
}
