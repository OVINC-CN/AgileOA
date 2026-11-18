package userSvc_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
)

const fixtureEmailCode = "012345"

func checkEmailLoginUsername(
	t *testing.T,
	server *echo.Echo,
	adminCookie *http.Cookie,
	account userModels.Info,
) {
	t.Helper()
	t.Run("邮件链接用户名核验", func(t *testing.T) {
		if db.RDB() == nil {
			t.Skip("配置独立 Redis 后运行邮件链接验收")
		}
		original := config.Config.User.EmailLogin.Enabled
		config.Config.User.EmailLogin.Enabled = true
		defer func() { config.Config.User.EmailLogin.Enabled = original }()
		connection := db.DB().WithContext(t.Context())
		verified := func(want bool) {
			t.Helper()
			var user userModels.User
			if err := connection.First(&user, account.ID).Error; err != nil {
				t.Fatal(err)
			}
			if user.EmailLoginVerified != want {
				t.Fatal("邮箱用户名核验状态错误")
			}
		}
		update := func(email *string) {
			t.Helper()
			enabled := true
			send(
				t, server, http.MethodPut,
				"/v1/system/users/"+strconv.FormatUint(account.ID, 10),
				userModels.UpdateRequest{
					DisplayName: account.DisplayName, Email: email, IsEnabled: &enabled,
				},
				adminCookie, http.StatusOK,
			)
		}
		prompt := func(code string) {
			t.Helper()
			var before, after int64
			if err := connection.Model(&userModels.Session{}).
				Where("user_id = ?", account.ID).Count(&before).Error; err != nil {
				t.Fatal(err)
			}
			result := verifyEmailLogin(t, server, code, "", http.StatusOK)
			value := data[userModels.EmailLoginData](t, result)
			if !value.UsernameRequired || value.User != nil ||
				result.Header().Get("Set-Cookie") != "" {
				t.Fatal("待核验时不得公开账号或设置会话 Cookie")
			}
			if err := connection.Model(&userModels.Session{}).
				Where("user_id = ?", account.ID).
				Count(&after).
				Error; err != nil || before != after {
				t.Fatal("待核验时不得创建会话")
			}
			verified(false)
		}
		complete := func(code, username string) {
			t.Helper()
			result := verifyEmailLogin(t, server, code, username, http.StatusOK)
			value := data[userModels.EmailLoginData](t, result)
			if value.UsernameRequired || value.User == nil || value.User.ID != account.ID {
				t.Fatal("核验成功未返回登录账号")
			}
			sessionCookie(t, result)
			verified(true)
			verifyEmailLogin(t, server, code, username, http.StatusUnauthorized)
		}

		// 密码登录不代替首次邮件核验；错误用户名不消费链接，正确用户名才建立会话。
		verified(false)
		code := seedEmailLogin(t, account.ID)
		prompt(code)
		wrong := verifyEmailLogin(t, server, code, "wrong.fixture", http.StatusUnauthorized)
		if wrong.Header().Get("Set-Cookie") != "" {
			t.Fatal("错误用户名不得设置会话 Cookie")
		}
		prompt(code)
		complete(code, account.Username)
		complete(seedEmailLogin(t, account.ID), "")

		// 普通编辑和相同邮箱保留核验；实际改动及清空后重新填写均须再次核验。
		update(nil)
		verified(true)
		same := " " + account.Email + " "
		update(&same)
		verified(true)
		oldCode := seedEmailLogin(t, account.ID)
		changed := "updated.managed@example.com"
		update(&changed)
		verified(false)
		verifyEmailLogin(t, server, oldCode, account.Username, http.StatusUnauthorized)
		code = seedEmailLogin(t, account.ID)
		prompt(code)
		complete(code, account.Username)
		empty := ""
		update(&empty)
		verified(false)
		update(&account.Email)
		verified(false)
		code = seedEmailLogin(t, account.ID)
		prompt(code)

		// 审计写入失败必须回滚核验状态和会话，已消费的登录码仍不可重放。
		const callbackName = "test:email-login-audit-unavailable"
		if err := connection.Callback().Create().Before("gorm:create").
			Register(callbackName, func(tx *gorm.DB) {
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
			failed := verifyEmailLogin(
				t, server, code, account.Username, http.StatusServiceUnavailable,
			)
			if failed.Header().Get("Set-Cookie") != "" {
				t.Fatal("审计故障不得设置会话 Cookie")
			}
		}()
		verified(false)
		verifyEmailLogin(t, server, code, account.Username, http.StatusUnauthorized)
		code = seedEmailLogin(t, account.ID)
		prompt(code)
		complete(code, account.Username)
	})
}

func seedEmailLogin(t *testing.T, userID uint64) string {
	t.Helper()
	// 仅为独立验收账号写入短时凭据，省去邮件 API 外部依赖，不启动任何服务。
	var user userModels.User
	if err := db.DB().WithContext(t.Context()).First(&user, userID).Error; err != nil {
		t.Fatal(err)
	}
	value := make([]byte, userConstants.SessionBytes)
	_, _ = rand.Read(value)
	code := hex.EncodeToString(value)
	hash := sha256.Sum256([]byte(code))
	now := time.Now().UTC()
	payload, err := json.Marshal(userModels.EmailLogin{
		CodeHash: emailCodeDigest(code, fixtureEmailCode),
		UserID:   user.ID, Email: user.Email, SessionVersion: user.SessionVersion,
		CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	codeKey := userConstants.EmailLoginChallengePrefix + hex.EncodeToString(hash[:])
	userKey := userConstants.EmailLoginUserPrefix + strconv.FormatUint(user.ID, 10)
	pipeline := db.RDB().TxPipeline()
	pipeline.Set(t.Context(), codeKey, payload, time.Minute)
	pipeline.Set(t.Context(), userKey, hex.EncodeToString(hash[:]), time.Minute)
	if _, err := pipeline.Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.WithoutCancel(t.Context()), config.Config.Redis.WriteTimeout,
		)
		defer cancel()
		if err := db.RDB().Del(ctx, codeKey, userKey).Err(); err != nil {
			t.Error(err)
		}
	})
	return code
}

func verifyEmailLogin(
	t *testing.T,
	server *echo.Echo,
	challengeID, username string,
	status int,
) *httptest.ResponseRecorder {
	t.Helper()
	return send(
		t, server, http.MethodPost, "/v1/login-email-verify",
		userModels.EmailLoginRequest{
			ChallengeID: challengeID, Code: fixtureEmailCode, Username: username,
		}, nil, status,
	)
}

func emailCodeDigest(challengeID, code string) string {
	digest := sha256.Sum256([]byte(challengeID + ":" + code))
	return hex.EncodeToString(digest[:])
}
