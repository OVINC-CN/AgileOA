package userSvc

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ChangePassword 验证临时密码，并在同一事务内更新密码、旋转会话和保存审计。
func ChangePassword(
	ctx context.Context,
	request userModels.ChangePasswordRequest,
) (userModels.Info, string, error) {
	if !passwordDigestPattern.MatchString(request.CurrentPasswordSHA256) ||
		!passwordDigestPattern.MatchString(request.NewPasswordSHA256) {
		err := Reject(ctx, accessConstants.UserChangePassword, http.StatusBadRequest)
		return userModels.Info{}, "", err
	}

	var result userModels.Info
	newToken := randomToken()
	err := authenticated(
		ctx,
		accessConstants.UserChangePassword,
		func(tx *gorm.DB, user userModels.User, session userModels.Session) (auditModels.Event, error) {
			event := accountEvent(user)
			if !user.PasswordLoginEnabled || !user.PasswordChangeRequired {
				return event, conflict("当前无需更换临时密码")
			}
			if err := bcrypt.CompareHashAndPassword(
				[]byte(user.PasswordHash),
				[]byte(request.CurrentPasswordSHA256),
			); err != nil {
				return event, &response.BusinessError{
					Status:  http.StatusBadRequest,
					Message: "当前密码错误",
				}
			}
			if request.NewPasswordSHA256 == request.CurrentPasswordSHA256 {
				return event, &response.BusinessError{
					Status:  http.StatusBadRequest,
					Message: "新密码不能与当前密码相同",
				}
			}

			// 账号版本作废其他会话，当前会话也必须更换令牌，避免旧 Cookie 获得业务权限。
			hash, err := bcrypt.GenerateFromPassword(
				[]byte(request.NewPasswordSHA256),
				config.Config.User.BcryptCost,
			)
			if err != nil {
				return event, err
			}
			event.Before = accountSnapshot(user)
			user.PasswordChangeRequired = false
			user.SessionVersion++
			if err := tx.Model(&user).Updates(map[string]any{
				"password_hash":            string(hash),
				"password_change_required": user.PasswordChangeRequired,
				"session_version":          user.SessionVersion,
			}).Error; err != nil {
				return event, err
			}
			expiresAt := time.Now().UTC().Add(config.Config.User.SessionTTL)
			if session.ID == 0 {
				// Token 认证没有 Cookie 会话，改密后建立新会话以保持同样的响应语义。
				session = userModels.Session{
					UserID: user.ID, TokenHash: tokenHash(newToken),
					SessionVersion: user.SessionVersion, ExpiresAt: expiresAt,
				}
				if err := tx.Create(&session).Error; err != nil {
					return event, err
				}
			} else {
				if err := tx.Model(&session).Updates(map[string]any{
					"token_hash":      tokenHash(newToken),
					"session_version": user.SessionVersion,
					"expires_at":      expiresAt,
				}).Error; err != nil {
					return event, err
				}
			}
			event.After = accountSnapshot(user)
			result = info(user)
			return event, nil
		},
	)
	if err != nil {
		return userModels.Info{}, "", err
	}
	return result, newToken, nil
}
