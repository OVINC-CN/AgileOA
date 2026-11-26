package auditSvc

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func TestNumericResultCodes(t *testing.T) {
	for _, scenario := range []struct {
		result  string
		code    string
		want    int
		invalid bool
	}{
		{result: auditConstants.Success, want: 0},
		{result: auditConstants.Failure, code: "http_403", want: 403},
		{result: auditConstants.Failure, code: "10001", want: 10001},
		{result: auditConstants.Failure, code: "-2147483648", want: -2147483648},
		{result: auditConstants.Failure, code: "access_denied", invalid: true},
		{result: auditConstants.Failure, code: "0", invalid: true},
		{result: auditConstants.Failure, code: "2147483648", invalid: true},
	} {
		code, err := resultCode(
			auditModels.Event{Result: scenario.result, ErrorCode: scenario.code},
		)
		if (err != nil) != scenario.invalid || (!scenario.invalid && code != scenario.want) {
			t.Fatalf("数字结果码转换错误：%q，%d，%v", scenario.code, code, err)
		}
	}
}

func TestMigratedSensitiveContextRetainsOriginalAAD(t *testing.T) {
	// MySQL JSON 的空白、斜线和 Unicode 转义不能改变原加密上下文。
	original, err := json.Marshal([]any{
		nil, "old-request", "student.delete", "student", "<虚构/对象>", "2026-11-03",
	})
	if err != nil {
		t.Fatal(err)
	}
	context := []byte(`[null, "old-request", "student.delete", "student", "<虚构/对象>", "2026-11-03"]`)
	associated, err := SensitiveAssociatedData(context)
	if err != nil || !bytes.Equal(original, associated) {
		t.Fatal("回填的私有上下文未恢复原 Go JSON 编码")
	}
	for _, invalid := range []string{`[]`, `[null,1,"a","b","c","d"]`, `[null,null,"a","b","c","d"]`} {
		if _, err := SensitiveAssociatedData([]byte(invalid)); err == nil {
			t.Fatal("无效加密关联上下文被接受")
		}
	}
}

func TestExemptQueriesNeverRequireAuditPersistence(t *testing.T) {
	for _, scenario := range []struct {
		action string
		actor  auditModels.Actor
	}{
		{accessConstants.UserInfo, auditModels.Actor{ID: 1}},
		{accessConstants.NotificationUnread, auditModels.Actor{ID: 1}},
		{accessConstants.StudentNotificationUnread, auditModels.Actor{StudentID: 1}},
	} {
		t.Run(scenario.action, func(t *testing.T) {
			event := auditModels.Event{Actor: scenario.actor, Action: scenario.action}
			if err := Write(nil, event); err != nil {
				t.Fatal("免审计查询的成功响应依赖审计存储")
			}
			event.ErrorCode = "http_403"
			if err := Failure(t.Context(), event); err != nil {
				t.Fatal("免审计查询的失败响应依赖审计存储")
			}
		})
	}
	for _, action := range []string{
		accessConstants.UserCreate, accessConstants.NotificationList,
		accessConstants.NotificationRead, accessConstants.StudentNotificationList,
		accessConstants.StudentNotificationRead,
	} {
		event := auditModels.Event{Actor: auditModels.Actor{ID: 1}, Action: action}
		if !errors.Is(Write(nil, event), ErrUnavailable) {
			t.Fatalf("操作 %s 仍需审计", action)
		}
	}
}

func TestSensitiveSnapshotsAreEncryptedAndBoundToObject(t *testing.T) {
	// 仅使用临时虚构密钥，测试不读取真实业务快照。
	original := config.Config.Audit
	t.Cleanup(func() { config.Config.Audit = original })
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	config.Config.Audit.SnapshotKeyID = "fixture"
	config.Config.Audit.SnapshotKeys = map[string]string{
		"fixture": base64.StdEncoding.EncodeToString(key),
	}
	event := auditModels.Event{
		Action:          accessConstants.UserCreate,
		ObjectType:      "fixture",
		ObjectID:        "123",
		SensitiveBefore: auditModels.Snapshot{"contact": "fictional-before"},
		SensitiveAfter:  auditModels.Snapshot{"contact": "fictional-after"},
	}
	ciphertext, keyID, err := encryptSensitive(event, "request", "2026-11-03")
	if err != nil || keyID != "fixture" || bytes.Contains(ciphertext, []byte("fictional-after")) {
		t.Fatal("敏感快照未使用独立密钥加密")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		t.Fatal(err)
	}
	aad, err := json.Marshal(
		[]any{nil, "request", event.Action, event.ObjectType, event.ObjectID, "2026-11-03"},
	)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := aead.Open(nil, nil, ciphertext, aad)
	if err != nil || !bytes.Contains(plaintext, []byte("fictional-before")) ||
		!bytes.Contains(plaintext, []byte("diff")) {
		t.Fatal("敏感快照前后值和差异未完整保留")
	}
	if _, err := aead.Open(nil, nil, ciphertext, []byte("another-object")); err == nil {
		t.Fatal("敏感快照可以被替换到其他对象")
	}
	config.Config.Audit.SnapshotKeyID = "missing"
	if _, _, err := encryptSensitive(
		event,
		"request",
		"2026-11-03",
	); !errors.Is(
		err,
		ErrUnavailable,
	) {
		t.Fatal("缺少独立密钥时没有阻断敏感审计")
	}
}

func TestCredentialsCannotEnterAnySnapshot(t *testing.T) {
	// 普通和加密快照共用凭据检查，不能把加密审计当作保存口令的渠道。
	for _, field := range []string{
		"password", "password_sha256", "token_hash", "session_token", "cookie", "signing_key",
		"signature", "secret", "client_secret", "secretary_token", "secretary_secret",
		"secretary_signing_key", "secretary_password", "secretary_signature",
	} {
		t.Run(field, func(t *testing.T) {
			_, err := encode(map[string]any{"items": []any{map[string]string{field: "fictional"}}})
			if !errors.Is(err, ErrUnavailable) {
				t.Fatal("嵌套凭据进入了审计")
			}
		})
	}
	if _, err := encode(auditModels.Snapshot{"id": 1, "display_name": "虚构用户"}); err != nil {
		t.Fatal("固定字段目录中的公开快照被错误拒绝")
	}
	if _, err := encode(
		auditModels.Snapshot{"url": "https://cdn.example.com/file?X-Amz-Signature=fictional"},
	); !errors.Is(
		err,
		ErrUnavailable,
	) {
		t.Fatal("CDN URL 签名进入了审计")
	}
}

func TestPasswordPolicySnapshotsAndChangesAreNotCredentials(t *testing.T) {
	// 登录方式和改密要求属于账号状态，创建、修改及删除的差异都应保留。
	before := auditModels.Snapshot{
		"id":                       1,
		"password_login_enabled":   true,
		"password_change_required": true,
	}
	after := auditModels.Snapshot{
		"id":                       1,
		"password_login_enabled":   false,
		"password_change_required": false,
	}
	for _, value := range []any{
		before,
		after,
		diff(nil, before),
		diff(before, after),
		diff(before, nil),
		map[string]any{"items": []any{before, after}},
	} {
		if _, err := encode(value); err != nil {
			t.Fatalf("账号密码策略或其差异被误判为凭据：%v", err)
		}
	}
}

func TestPasswordPolicyFieldsCannotContainCredentials(t *testing.T) {
	// 只允许两个固定字段的布尔状态和布尔差异，不能借用状态字段保存凭据。
	for _, field := range []string{"password_login_enabled", "password_change_required"} {
		t.Run(field, func(t *testing.T) {
			for _, value := range []any{
				"fictional",
				1,
				nil,
				[]any{true},
				map[string]any{"password_sha256": "fictional"},
				map[string]any{"before": true},
				map[string]any{"before": false, "after": "fictional"},
				map[string]any{"before": "fictional", "after": true},
				map[string]any{"before": false, "after": []any{true}},
				map[string]any{"before": true, "after": false, "extra": "fictional"},
			} {
				if _, err := encode(
					auditModels.Snapshot{field: value},
				); !errors.Is(
					err,
					ErrUnavailable,
				) {
					t.Fatal("账号状态字段中的非布尔内容绕过了凭据检查")
				}
			}
		})
	}
	for _, field := range []string{
		"password",
		"password_hash",
		"password_sha256",
		"token",
		"password_login_enabled_hash",
		"password_change_required_token",
	} {
		if _, err := encode(auditModels.Snapshot{field: true}); !errors.Is(err, ErrUnavailable) {
			t.Fatal("凭据字段因使用布尔值绕过了检查")
		}
	}
}

func TestSecretarySnapshotsAndChangesAreNotCredentials(t *testing.T) {
	// 创建、修改、清空和删除均保存书记字段，差异快照也必须能通过校验。
	// #nosec G101 -- secretary_* 是书记身份字段，测试仅使用虚构账号和姓名。
	before := auditModels.Snapshot{
		"id": 1, "full_name": "虚构党支部全称", "short_name": "虚构党支部",
		"major": "", "secretary_id": 2, "secretary_name": "虚构书记",
		"secretary_username": "fixture.secretary",
	}
	after := auditModels.Snapshot{
		"id": 1, "full_name": "虚构党支部全称", "short_name": "虚构党支部",
		"major": "", "secretary_id": nil, "secretary_name": "", "secretary_username": "",
	}
	for _, value := range []any{before, after, diff(nil, before), diff(before, after), diff(before, nil)} {
		if _, err := encode(value); err != nil {
			t.Fatalf("书记身份或其差异被误判为凭据：%v", err)
		}
	}
	// #nosec G101 -- 使用虚构令牌地址验证拦截规则，不包含可用凭据。
	if _, err := encode(auditModels.Snapshot{
		"secretary_name": "https://example.com/?token=fictional",
	}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("书记字段中的签名或令牌地址绕过了凭据检查")
	}
}
