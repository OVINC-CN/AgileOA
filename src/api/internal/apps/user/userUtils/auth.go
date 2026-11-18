package userUtils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Authenticate 仅供认证中间件验证 Cookie 或 API Token，并返回不含原始凭据的身份。
func Authenticate(
	ctx context.Context,
	token string,
	isAPIToken bool,
) (userModels.Identity, error) {
	// 学生 Cookie 使用独立前缀与会话表，不能作为管理账号或 MCP 凭据使用。
	if !isAPIToken && strings.HasPrefix(token, userConstants.StudentSessionPrefix) {
		return authenticateStudent(ctx, token)
	}
	deny := func() (userModels.Identity, error) {
		return userModels.Identity{}, &response.BusinessError{
			Status:  http.StatusUnauthorized,
			Message: "请先登录",
		}
	}
	value := token
	if isAPIToken {
		if !strings.HasPrefix(token, userConstants.APITokenPrefix) {
			return deny()
		}
		value = strings.TrimPrefix(token, userConstants.APITokenPrefix)
	}
	if len(value) != userConstants.SessionBytes*2 {
		return deny()
	}
	if _, err := hex.DecodeString(value); err != nil {
		return deny()
	}
	hash := sha256.Sum256([]byte(token))
	digest := hex.EncodeToString(hash[:])
	// 一次关联查询验证凭据和账号状态，Cookie 另校验会话版本及有效期。
	var identity userModels.Identity
	if db.DB() == nil {
		return userModels.Identity{}, &response.BusinessError{
			Status: http.StatusServiceUnavailable, Message: "账号服务暂时不可用",
		}
	}
	query := db.DB().WithContext(ctx).Model(&userModels.User{}).Where("users.is_enabled = ?", true)
	if isAPIToken {
		// API Token 的有效性不依赖网页登录会话版本。
		query = query.Select("users.*").
			Joins("JOIN user_api_tokens ON user_api_tokens.user_id = users.id").
			Where("user_api_tokens.token_hash = ?", digest)
	} else {
		query = query.Select([]string{
			"users.*",
			"user_sessions.id AS session_id",
			"user_sessions.user_id AS session_user_id",
			"user_sessions.expires_at AS session_expires_at",
			"user_sessions.created_at AS session_created_at",
			"user_sessions.updated_at AS session_updated_at",
		}).
			Joins("JOIN user_sessions ON user_sessions.user_id = users.id").
			Where("user_sessions.token_hash = ? AND user_sessions.expires_at > ?", digest, time.Now().UTC()).
			Where("user_sessions.session_version = users.session_version")
	}
	if err := query.Take(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return deny()
		}
		return userModels.Identity{}, err
	}
	user := identity.User
	auditUtils.SetActor(
		ctx,
		auditModels.Actor{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			IsAdmin:     user.IsAdmin,
		},
	)
	// 上下文只保留已验证身份和会话业务字段，不继续传递口令或凭据摘要。
	identity.User.PasswordHash = ""
	identity.IsAPIToken = isAPIToken
	return identity, nil
}

// CheckAction 统一检查账号状态、Cookie 待改密限制和系统管理员范围。
func CheckAction(identity userModels.Identity, action string) error {
	user := identity.User
	if !user.IsEnabled {
		return &response.BusinessError{Status: http.StatusUnauthorized, Message: "请先登录"}
	}
	// 待改密限制仅适用于网页登录会话，已有 API Token 继续按业务权限访问。
	if !identity.IsAPIToken && user.PasswordLoginEnabled && user.PasswordChangeRequired &&
		action != accessConstants.UserInfo &&
		action != accessConstants.UserLogout && action != accessConstants.UserChangePassword {
		return &response.BusinessError{
			Status:  http.StatusForbidden,
			Message: "请先修改密码",
		}
	}
	definition, ok := accessConstants.Lookup(action)
	if !ok || definition.Scope == accessConstants.ScopePublic {
		return &response.BusinessError{
			Status:  http.StatusForbidden,
			Message: response.MessageForbidden,
		}
	}
	if definition.Scope == accessConstants.ScopeSystem && !user.IsAdmin {
		return &response.BusinessError{
			Status:  http.StatusForbidden,
			Message: "仅系统管理员可以访问系统管理",
		}
	}
	return nil
}
