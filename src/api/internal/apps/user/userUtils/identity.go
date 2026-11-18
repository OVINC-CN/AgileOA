package userUtils

import (
	"context"
	"errors"
	"net/http"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

type identityKey struct{}

// WithIdentity 仅供认证中间件写入已验证的身份，原始凭据不进入业务上下文。
func WithIdentity(ctx context.Context, identity userModels.Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

// Identity 读取认证中间件确认的身份，缺失身份时默认拒绝。
func Identity(ctx context.Context) (userModels.Identity, error) {
	identity, ok := ctx.Value(identityKey{}).(userModels.Identity)
	if !ok || (identity.User.ID == 0 &&
		(identity.Student == nil || identity.Student.StudentID == 0)) {
		return userModels.Identity{}, &response.BusinessError{
			Status: http.StatusUnauthorized, Message: "请先登录",
		}
	}
	return identity, nil
}

// Require 在业务事务内按可信账号编号复核状态与权限，不再次读取或验证凭据。
func Require(tx *gorm.DB, action string) (userModels.User, error) {
	// 身份只能来自认证中间件，后台任务不能借用用户身份。
	identity, err := Identity(tx.Statement.Context)
	if err != nil {
		return userModels.User{}, err
	}
	if identity.Student != nil {
		return userModels.User{}, &response.BusinessError{
			Status: http.StatusForbidden, Message: "学生登录仅可访问学生服务",
		}
	}
	// 查询共享账号状态，写操作预先取得排他锁，避免事务中途升级锁。
	strength := "UPDATE"
	if accessConstants.ReadOnly(action) {
		strength = "SHARE"
	}
	var user userModels.User
	if err := tx.Clauses(clause.Locking{Strength: strength}).
		First(&user, identity.User.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, &response.BusinessError{Status: http.StatusUnauthorized, Message: "请先登录"}
		}
		return user, err
	}
	if user.SessionVersion != identity.User.SessionVersion {
		return user, &response.BusinessError{Status: http.StatusUnauthorized, Message: "请先登录"}
	}
	identity.User = user
	return user, CheckAction(identity, action)
}
