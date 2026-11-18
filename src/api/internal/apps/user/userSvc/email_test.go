package userSvc

import (
	"testing"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

func TestLoginEmailTemplateData(t *testing.T) {
	// 云端模板仅包含六位验证码及有效分钟数，并保留验证码前导零。
	original := config.Config.User.EmailLogin
	t.Cleanup(func() { config.Config.User.EmailLogin = original })
	config.Config.User.EmailLogin.TTL = 7 * time.Minute
	data := loginEmailTemplateData("012345")
	if len(data) != 2 || data["LOGIN_CODE"] != "012345" || data["TTL_MINUTES"] != "7" {
		t.Fatal("邮件模板变量未保留数字验证码或仍然包含链接参数")
	}
}

func TestEmailLoginChallengeIsolationAndAttemptLimit(t *testing.T) {
	if db.RDB() == nil {
		t.Skip("配置独立 Redis 后运行验证码隔离与错误次数验收")
	}
	original := config.Config.User.EmailLogin.MaxAttempts
	config.Config.User.EmailLogin.MaxAttempts = 3
	t.Cleanup(func() { config.Config.User.EmailLogin.MaxAttempts = original })
	ctx := t.Context()
	challengeID := randomToken()
	hash := tokenHash(challengeID)
	otherID := randomToken()
	otherHash := tokenHash(otherID)
	record := userModels.EmailLogin{
		StudentID: 17, WorkspaceID: randomToken()[:16], WorkspaceName: "验证码隔离验收",
		StudentNo: "000001", Email: "student@example.com",
		CodeHash:  tokenHash(challengeID + ":012345"),
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	other := record
	other.StudentID++
	other.CodeHash = tokenHash(otherID + ":012345")
	t.Cleanup(func() {
		discardEmailLoginChallenge(ctx, record, hash)
		discardEmailLoginChallenge(ctx, other, otherHash)
	})
	for _, item := range []struct {
		record userModels.EmailLogin
		hash   string
	}{{record: record, hash: hash}, {record: other, hash: otherHash}} {
		if err := reserveStudentLoginEmail(ctx, item.record, item.hash); err != nil {
			t.Fatal(err)
		}
		if err := cacheEmailLogin(ctx, item.record, item.hash); err != nil {
			t.Fatal(err)
		}
	}
	_, payload, err := readEmailLogin(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	_, otherPayload, err := readEmailLogin(ctx, otherHash)
	if err != nil {
		t.Fatal(err)
	}
	// 同一个数字验证码在不同身份下独立使用，替换校验标识不能匹配其摘要。
	request := userModels.EmailLoginRequest{ChallengeID: otherID, Code: "012345"}
	if err := verifyEmailLoginCode(ctx, record, hash, payload, request); err == nil {
		t.Fatal("不同校验标识不得复用验证码摘要")
	}
	request.ChallengeID = challengeID
	if err := verifyEmailLoginCode(ctx, record, hash, payload, request); err != nil {
		t.Fatal("包含前导零的正确验证码应可使用")
	}
	before := db.RDB().PTTL(ctx, userConstants.EmailLoginChallengePrefix+otherHash).Val()
	request.Code = "999999"
	// 第一次跨标识校验已计一次错误；之后两次错误达到上限。
	for range 2 {
		if err := verifyEmailLoginCode(ctx, record, hash, payload, request); err == nil {
			t.Fatal("错误验证码不得通过")
		}
	}
	if _, _, err := readEmailLogin(ctx, hash); err == nil {
		t.Fatal("达到错误次数上限后必须作废验证码")
	}
	ttl, err := db.RDB().PTTL(ctx, userConstants.EmailLoginChallengePrefix+otherHash).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl > before || ttl <= 0 {
		t.Fatal("校验失败不得延长验证码有效期或影响其他身份")
	}
	request.ChallengeID = otherID
	request.Code = "012345"
	if err := verifyEmailLoginCode(ctx, other, otherHash, otherPayload, request); err != nil {
		t.Fatal("其他身份的相同验证码应保持有效")
	}
	consumed, err := consumeEmailLoginScript.Run(
		ctx, db.RDB(), emailLoginConsumeKeys(other, otherHash), otherHash, otherPayload,
	).Int()
	if err != nil || consumed != 1 {
		t.Fatal("正确验证码应能原子消费")
	}
	if err := verifyEmailLoginCode(ctx, other, otherHash, otherPayload, request); err == nil {
		t.Fatal("已消费的验证码不得重放")
	}
}
