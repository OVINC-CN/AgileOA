package userSvc_test

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestDisabledPasswordLoginRejectsBeforeCaptcha(t *testing.T) {
	// 关闭后直接拒绝，不依赖验证码服务或数据库，也不签发会话。
	original := config.Config.User
	t.Cleanup(func() { config.Config.User = original })
	config.Config.User.PasswordLoginEnabled = false
	config.Config.User.Captcha.Enabled = true
	for _, username := range []string{config.Config.User.BootstrapUsername, "nonexistent.fixture"} {
		info, token, err := userSvc.Login(
			t.Context(),
			userModels.LoginRequest{
				Username:       username,
				PasswordSHA256: digest(username, testPassword),
			},
			"127.0.0.1",
		)
		var failure *response.BusinessError
		if !errors.As(err, &failure) || failure.Status != http.StatusUnauthorized ||
			failure.Message != "用户名或密码错误" || info.ID != 0 || token != "" {
			t.Fatalf("关闭密码登录后未统一拒绝登录：%v", err)
		}
	}
}

func checkPasswordLoginPolicy(
	t *testing.T,
	server *echo.Echo,
	adminCookie, ordinaryCookie *http.Cookie,
) {
	t.Helper()
	t.Run("密码登录开关", func(t *testing.T) {
		// 关闭密码登录不需要初始密码，普通账号仍不能创建用户。
		enabled, disabled := true, false
		request := userModels.CreateRequest{
			Username:             "mail.only.fixture",
			DisplayName:          "仅邮件登录用户",
			Email:                "mail.only@example.com",
			PasswordLoginEnabled: &disabled,
		}
		send(t, server, http.MethodPost, "/v1/system/users", request,
			ordinaryCookie, http.StatusForbidden)
		created := data[userModels.Info](t, send(
			t, server, http.MethodPost, "/v1/system/users", request, adminCookie, http.StatusOK,
		))
		if created.PasswordLoginEnabled || created.PasswordChangeRequired {
			t.Fatal("关闭密码登录的账号不应要求改密")
		}
		var stored userModels.User
		connection := db.DB().WithContext(t.Context())
		if err := connection.First(&stored, created.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.PasswordHash != "" || stored.PasswordChangeRequired ||
			stored.PasswordLoginEnabled {
			t.Fatal("关闭密码登录时不应保存口令或待改密状态")
		}
		path := "/v1/system/users/" + strconv.FormatUint(created.ID, 10)
		list := data[userModels.ListData](t, send(
			t, server, http.MethodGet, "/v1/system/users?keyword="+created.Username,
			nil, adminCookie, http.StatusOK,
		))
		if len(list.Items) != 1 || list.Items[0].PasswordLoginEnabled {
			t.Fatal("用户列表未返回密码登录设置")
		}

		// 密码登录统一拒绝，编辑省略开关时保留禁用状态。
		send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
			Username: created.Username, PasswordSHA256: digest(created.Username, temporaryPassword),
		}, nil, http.StatusUnauthorized)
		update := userModels.UpdateRequest{DisplayName: created.DisplayName, IsEnabled: &enabled}
		unchanged := data[userModels.Info](t, send(
			t, server, http.MethodPut, path, update, adminCookie, http.StatusOK,
		))
		if unchanged.PasswordLoginEnabled || unchanged.PasswordChangeRequired {
			t.Fatal("普通编辑不应启用密码登录或要求改密")
		}

		// 邮件链接完成核验后直接访问业务入口，不进入强制改密流程。
		var emailCookie *http.Cookie
		if db.RDB() != nil {
			original := config.Config.User.EmailLogin.Enabled
			config.Config.User.EmailLogin.Enabled = true
			defer func() { config.Config.User.EmailLogin.Enabled = original }()
			result := verifyEmailLogin(
				t, server, seedEmailLogin(t, created.ID), created.Username, http.StatusOK,
			)
			value := data[userModels.EmailLoginData](t, result)
			if value.User == nil || value.User.PasswordChangeRequired ||
				value.User.PasswordLoginEnabled {
				t.Fatal("仅邮件登录账号被错误要求修改密码")
			}
			emailCookie = sessionCookie(t, result)
			send(t, server, http.MethodGet, "/v1/workspaces", nil, emailCookie, http.StatusOK)
		}

		// 无需改密的账号可配置 Token，重新启用密码登录后已有 Token 保持有效。
		key := make([]byte, 32)
		_, _ = rand.Read(key)
		originalKey := config.Config.User.APITokenKey
		config.Config.User.APITokenKey = base64.StdEncoding.EncodeToString(key)
		defer func() { config.Config.User.APITokenKey = originalKey }()
		token := data[userModels.APITokenData](t, send(
			t, server, http.MethodPut, path+"/api-token", nil, adminCookie, http.StatusOK,
		))
		update.PasswordLoginEnabled = &enabled
		send(t, server, http.MethodPut, path, update, ordinaryCookie, http.StatusForbidden)
		send(t, server, http.MethodPut, path, update, adminCookie, http.StatusBadRequest)
		update.PasswordSHA256 = digest(created.Username, temporaryPassword)
		updated := data[userModels.Info](t, send(
			t, server, http.MethodPut, path, update, adminCookie, http.StatusOK,
		))
		if !updated.PasswordLoginEnabled || !updated.PasswordChangeRequired {
			t.Fatal("重新启用密码登录未要求修改临时密码")
		}

		// 全局开关只阻止新的密码登录，账号配置、已有会话和 Token 均保留。
		originalPasswordLogin := config.Config.User.PasswordLoginEnabled
		defer func() { config.Config.User.PasswordLoginEnabled = originalPasswordLogin }()
		config.Config.User.PasswordLoginEnabled = false
		denied := send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
			Username: created.Username, PasswordSHA256: digest(created.Username, temporaryPassword),
		}, nil, http.StatusUnauthorized)
		if denied.Header().Get("Set-Cookie") != "" {
			t.Fatal("全局关闭密码登录后仍签发了会话 Cookie")
		}
		send(t, server, http.MethodGet, "/v1/user-info", nil, adminCookie, http.StatusOK)
		assertAPITokenActive(t, server, adminCookie, path+"/api-token", token.Token)
		if db.RDB() != nil {
			result := verifyEmailLogin(
				t, server, seedEmailLogin(t, created.ID), created.Username, http.StatusOK,
			)
			value := data[userModels.EmailLoginData](t, result)
			if value.User == nil || !value.User.PasswordLoginEnabled ||
				!value.User.PasswordChangeRequired {
				t.Fatal("全局关闭密码登录影响了邮件登录或账号设置")
			}
			sessionCookie(t, result)
		}
		config.Config.User.PasswordLoginEnabled = originalPasswordLogin
		if emailCookie != nil {
			send(
				t,
				server,
				http.MethodGet,
				"/v1/user-info",
				nil,
				emailCookie,
				http.StatusUnauthorized,
			)
		}

		// 待改密期间关闭密码登录也必须解除限制，并使旧密码和会话失效。
		passwordCookie := sessionCookie(t, login(t, server, created.Username, temporaryPassword))
		update.PasswordLoginEnabled = &disabled
		send(t, server, http.MethodPut, path, update, adminCookie, http.StatusBadRequest)
		update.PasswordSHA256 = ""
		updated = data[userModels.Info](t, send(
			t, server, http.MethodPut, path, update, adminCookie, http.StatusOK,
		))
		if updated.PasswordLoginEnabled || updated.PasswordChangeRequired {
			t.Fatal("关闭密码登录未清除待改密状态")
		}
		assertAPITokenActive(t, server, adminCookie, path+"/api-token", token.Token)
		send(
			t,
			server,
			http.MethodGet,
			"/v1/user-info",
			nil,
			passwordCookie,
			http.StatusUnauthorized,
		)
		send(t, server, http.MethodPost, "/v1/login", userModels.LoginRequest{
			Username: created.Username, PasswordSHA256: digest(created.Username, temporaryPassword),
		}, nil, http.StatusUnauthorized)
		send(t, server, http.MethodDelete, path+"/sessions", nil, adminCookie, http.StatusOK)
		send(t, server, http.MethodDelete, path, nil, adminCookie, http.StatusOK)
	})
}
