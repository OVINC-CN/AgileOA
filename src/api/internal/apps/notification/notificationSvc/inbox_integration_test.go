package notificationSvc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

var _ = flag.String("config", "", "独立通知验收数据库的 YAML 配置")

func inboxRequest[T any](
	t *testing.T,
	server *echo.Echo,
	method, path string,
	body any,
	cookie *http.Cookie,
	status int,
) T {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), method, path, bytes.NewReader(content))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Protection", "1")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	if recorder.Code != status {
		t.Fatalf("通知接口状态错误：%d，%s", recorder.Code, recorder.Body.String())
	}
	var envelope response.Envelope[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func inboxUser(t *testing.T, username, token string) (userModels.User, *http.Cookie) {
	t.Helper()
	user := userModels.User{
		Username: username, DisplayName: "虚构通知用户", PasswordHash: strings.Repeat("0", 60),
		IsEnabled: true, SessionVersion: 1,
	}
	connection := db.DB().WithContext(t.Context())
	if err := connection.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(token))
	if err := connection.Create(&userModels.Session{
		UserID: user.ID, TokenHash: hex.EncodeToString(digest[:]), SessionVersion: 1,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	return user, &http.Cookie{
		Name: config.Config.User.CookieName, Value: token,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
}

func TestPersonalInboxIsolationReadIdempotencyAndAuditRollback(t *testing.T) {
	// 只使用人工安装 SQL 的独立空库，不启动服务、迁移或清理真实数据。
	if db.DB() == nil {
		t.Skip("通过 --config 指定独立通知验收 MySQL 配置后运行")
	}
	dsn, err := mysql.ParseDSN(config.Config.MySQL.DSN)
	if err != nil || !strings.HasSuffix(dsn.DBName, "_notification_test") {
		t.Fatal("通知验收数据库必须以 _notification_test 结尾")
	}
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&userModels.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("通知验收要求独立空数据库")
	}
	user, cookie := inboxUser(t, "fixture.inbox", strings.Repeat("a1", 32))
	other, _ := inboxUser(t, "fixture.other", strings.Repeat("b2", 32))
	delivery := notificationModels.Delivery{
		UserID: user.ID, SourceID: "fixture-result", Title: "重排人员成功", Content: "虚构工作已完成。",
		URL: "/cpc/roster?workspace=w7m2k9p4s6n8c3x5",
	}
	if err := notificationSvc.Deliver(t.Context(), delivery); err != nil {
		t.Fatal(err)
	}
	foreign := delivery
	foreign.UserID = other.ID
	if err := notificationSvc.Deliver(t.Context(), foreign); err != nil {
		t.Fatal(err)
	}
	if err := connection.Model(&auditModels.Record{}).
		Count(&count).
		Error; err != nil ||
		count != 0 {
		t.Fatal("后台投递不应生成用户操作审计")
	}
	server := httpd.New()
	inboxRequest[any](t, server, "GET", "/v1/notifications", nil, nil, 401)
	list := inboxRequest[notificationModels.ListData](
		t, server, "GET", "/v1/notifications?user_id=2", nil, cookie, 200,
	)
	if list.Total != 1 || len(list.Items) != 1 || list.UnreadCount != 1 ||
		list.Items[0].ReadAt != nil || list.Items[0].URL != delivery.URL {
		t.Fatal("通知列表泄露其他账号消息，或读取时提前标记已读")
	}
	var otherNotice notificationModels.Notification
	if err := connection.Where("user_id = ?", other.ID).Take(&otherNotice).Error; err != nil {
		t.Fatal(err)
	}
	ownID := list.Items[0].ID
	inboxRequest[any](t, server, "POST", "/v1/notifications/read",
		notificationModels.ReadRequest{IDs: []uint64{ownID, otherNotice.ID}}, cookie, 404)
	inboxRequest[any](t, server, "POST", "/v1/notifications/read",
		notificationModels.ReadRequest{IDs: []uint64{ownID, ownID}}, cookie, 400)
	for _, changed := range []int64{1, 0} {
		result := inboxRequest[notificationModels.ReadData](
			t,
			server,
			"POST",
			"/v1/notifications/read",
			notificationModels.ReadRequest{IDs: []uint64{ownID}},
			cookie,
			200,
		)
		if result.Changed != changed || result.UnreadCount != 0 {
			t.Fatal("标记已读不幂等或修改了其他账号的通知")
		}
	}
	unread := inboxRequest[notificationModels.ListData](
		t, server, "GET", "/v1/notifications", nil, cookie, 200,
	)
	if unread.Total != 0 || unread.Items == nil || len(unread.Items) != 0 {
		t.Fatal("默认通知列表未排除已读通知")
	}
	all := inboxRequest[notificationModels.ListData](
		t, server, "GET", "/v1/notifications?status=all", nil, cookie, 200,
	)
	if all.Total != 1 || all.Items[0].ReadAt == nil {
		t.Fatal("全部通知未包含已读记录")
	}
	inboxRequest[any](t, server, "GET", "/v1/notifications?status=invalid", nil, cookie, 400)
	var before, after notificationModels.Notification
	if err := connection.First(&before, ownID).Error; err != nil || before.ReadAt == nil {
		t.Fatal(err)
	}
	delivery.URL = "/stu/monthly-work?workspace=w7m2k9p4s6n8c3x5"
	if err := notificationSvc.Deliver(t.Context(), delivery); err != nil {
		t.Fatal(err)
	}
	if err := connection.First(&after, ownID).Error; err != nil || after.ReadAt == nil ||
		!after.ReadAt.Equal(*before.ReadAt) || after.URL != before.URL {
		t.Fatal("重复投递重置了已读时间或跳转链接")
	}
	if err := connection.Model(&notificationModels.Notification{}).
		Count(&count).Error; err != nil || count != 2 {
		t.Fatal("重复投递创建了多条通知")
	}
	checkInboxOrder(t, server, cookie, user.ID)
	checkReadAuditRollback(t, server, cookie, user.ID)
}

func checkInboxOrder(t *testing.T, server *echo.Echo, cookie *http.Cookie, userID uint64) {
	t.Helper()
	now := time.Now().UTC()
	rows := []notificationModels.Notification{
		{UserID: userID, SourceID: "order-old-unread", Title: "旧未读", Content: "正文"},
		{UserID: userID, SourceID: "order-new-unread", Title: "新未读", Content: "正文"},
		{UserID: userID, SourceID: "order-tied-unread", Title: "同时间未读", Content: "正文"},
		{UserID: userID, SourceID: "order-new-read", Title: "新已读", Content: "正文",
			ReadAt: &now},
	}
	if err := db.DB().WithContext(t.Context()).Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	list := inboxRequest[notificationModels.ListData](
		t, server, "GET", "/v1/notifications?status=all&page_size=5", nil, cookie, 200,
	)
	for index, row := range []notificationModels.Notification{rows[2], rows[1], rows[0], rows[3]} {
		if len(list.Items) <= index || list.Items[index].ID != row.ID {
			t.Fatal("通知未按未读优先、组内编号倒序排列")
		}
	}
}

func checkReadAuditRollback(t *testing.T, server *echo.Echo, cookie *http.Cookie, userID uint64) {
	t.Helper()
	if err := notificationSvc.Deliver(t.Context(), notificationModels.Delivery{
		UserID: userID, SourceID: "fixture-rollback", Title: "工作分配通知", Content: "虚构协作工作。",
	}); err != nil {
		t.Fatal(err)
	}
	var notice notificationModels.Notification
	connection := db.DB().WithContext(t.Context())
	if err := connection.Where("user_id = ? AND source_id = ?", userID, "fixture-rollback").
		Take(&notice).Error; err != nil {
		t.Fatal(err)
	}
	const callback = "test:notification-audit-unavailable"
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
	t.Cleanup(func() {
		if err := db.DB().Callback().Create().Remove(callback); err != nil {
			t.Error(err)
		}
	})
	inboxRequest[any](t, server, "POST", "/v1/notifications/read",
		notificationModels.ReadRequest{IDs: []uint64{notice.ID}}, cookie, 503)
	inboxRequest[any](t, server, "GET", "/v1/notifications", nil, cookie, 503)
	if err := connection.First(&notice, notice.ID).Error; err != nil || notice.ReadAt != nil {
		t.Fatal("审计失败未回滚已读状态")
	}
}
