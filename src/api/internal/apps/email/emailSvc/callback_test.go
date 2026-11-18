package emailSvc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestPrepareCallbackKeepsFixedFieldsAndRemovesCredentials(t *testing.T) {
	t.Parallel()

	// 大小写别名及扩展字段兼容腾讯云协议；未知来源字段和链接凭据不落库。
	var request emailModels.CallbackRequest
	err := json.Unmarshal([]byte(`{
		"event":"click","email":"Recipient@Example.COM","bulkid":"message-1",
		"timestamp":1794508800,"eventid":5,"templateId":123,
		"link":"https://name:password@example.com/login/email?code=private#secret",
		"ip":"192.0.2.1","X-Tencentcloudses-Cb-Custom":"private-extension"
	}`), &request)
	if err != nil {
		t.Fatal(err)
	}
	callback, err := prepareCallback(request)
	if err != nil {
		t.Fatal(err)
	}
	if callback.Email != "recipient@example.com" || callback.BulkID != "message-1" ||
		callback.TemplateID != 123 || callback.Link != "https://example.com/login/email" {
		t.Fatal("回调未保留协议字段或未规范化邮箱及链接")
	}
	encoded, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password", "private", "secret", "192.0.2.1"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatal("回调保存了链接凭据、IP 或未知扩展字段")
		}
	}

	// 相同内容保持幂等，不同发生时间的事件分别落库。
	again, err := prepareCallback(callback.CallbackRequest)
	if err != nil || again.EventKey != callback.EventKey {
		t.Fatal("同一回调未生成稳定摘要")
	}
	request.Timestamp++
	next, err := prepareCallback(request)
	if err != nil || next.EventKey == callback.EventKey {
		t.Fatal("不同时间的重复打开或点击被错误合并")
	}
}

func TestPrepareCallbackRejectsInvalidEvents(t *testing.T) {
	t.Parallel()
	for _, change := range []func(*emailModels.CallbackRequest){
		func(r *emailModels.CallbackRequest) { r.Event = "unknown" },
		func(r *emailModels.CallbackRequest) { r.EventID = 4 },
		func(r *emailModels.CallbackRequest) { r.Timestamp = 0 },
		func(r *emailModels.CallbackRequest) { r.Email = "姓名 <recipient@example.com>" },
		func(r *emailModels.CallbackRequest) { r.BulkID = "" },
		func(r *emailModels.CallbackRequest) { r.Link = "javascript:alert(1)" },
	} {
		request := callbackRequest()
		change(&request)
		if _, err := prepareCallback(request); err == nil {
			t.Fatal("邮件回调接受了无效字段")
		}
	}
}

func TestCallbackTransactionUsesConfiguredEvents(t *testing.T) {
	original := config.Config.Email.AbnormalEvents
	t.Cleanup(func() { config.Config.Email.AbnormalEvents = original })
	duplicateError := &mysql.MySQLError{Number: 1062, Message: "duplicate"}
	storageError := errors.New("storage unavailable")
	for _, scenario := range []struct {
		name           string
		event          string
		configured     []string
		callbackError  error
		blacklistError error
		blacklistWrite bool
		created        bool
		blocked        bool
		failed         bool
	}{
		{
			name: "丢弃拉黑", event: "dropped", configured: []string{"dropped"},
			blacklistWrite: true, created: true, blocked: true,
		},
		{
			name: "退信拉黑", event: "bounce", configured: []string{"bounce"},
			blacklistWrite: true, created: true, blocked: true,
		},
		{
			name: "举报拉黑", event: "spamreport", configured: []string{"spamreport"},
			blacklistWrite: true, created: true, blocked: true,
		},
		{
			name: "退订拉黑", event: "unsubscribe", configured: []string{"unsubscribe"},
			blacklistWrite: true, created: true, blocked: true,
		},
		{name: "未配置的退信只落库", event: "bounce", configured: []string{}, created: true},
		{
			name: "正常事件只落库", event: "delivered", configured: []string{"bounce"},
			created: true,
		},
		{
			name: "以配置为准", event: "delivered", configured: []string{"delivered"},
			blacklistWrite: true, created: true, blocked: true,
		},
		{
			name: "重复回调直接成功", event: "bounce", configured: []string{"bounce"},
			callbackError: duplicateError,
		},
		{
			name: "已拉黑邮箱仍保存新事件", event: "bounce", configured: []string{"bounce"},
			blacklistWrite: true, blacklistError: duplicateError, created: true,
		},
		{
			name: "拉黑失败回滚回调", event: "bounce", configured: []string{"bounce"},
			blacklistWrite: true, blacklistError: storageError, failed: true,
		},
		{
			name: "落库失败不拉黑", event: "bounce", configured: []string{"bounce"},
			callbackError: storageError, failed: true,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// 用数据库协议替身检查提交、回滚和去重，不连接或迁移真实数据库。
			connection, mock := callbackDatabase(t)
			config.Config.Email.AbnormalEvents = scenario.configured
			request := callbackRequest()
			request.Event = scenario.event
			callback, err := prepareCallback(request)
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectBegin()
			insert := mock.ExpectExec("INSERT INTO `email_callbacks`")
			if scenario.callbackError != nil {
				insert.WillReturnError(scenario.callbackError)
			} else {
				insert.WillReturnResult(sqlmock.NewResult(7, 1))
			}
			if scenario.blacklistWrite {
				insert := mock.ExpectExec("INSERT INTO `email_blacklists`")
				if scenario.blacklistError != nil {
					insert.WillReturnError(scenario.blacklistError)
				} else {
					insert.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			if scenario.failed {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			var created, blocked bool
			err = connection.WithContext(t.Context()).Transaction(func(tx *gorm.DB) error {
				var err error
				created, blocked, err = saveCallback(tx, &callback)
				return err
			})
			if (err != nil) != scenario.failed || created != scenario.created ||
				blocked != scenario.blocked {
				t.Fatalf("回调事务结果不符合预期：created=%t blocked=%t err=%v", created, blocked, err)
			}
		})
	}
}

func callbackRequest() emailModels.CallbackRequest {
	return emailModels.CallbackRequest{
		Event: "bounce", Email: "recipient@example.com", BulkID: "message-1", Timestamp: 1794508800,
	}
}

func callbackDatabase(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = connection.Close()
	})
	database, err := gorm.Open(
		gormmysql.New(gormmysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		&gorm.Config{DisableAutomaticPing: true, Logger: gormlogger.Discard},
	)
	if err != nil {
		t.Fatal(err)
	}
	return database, mock
}
