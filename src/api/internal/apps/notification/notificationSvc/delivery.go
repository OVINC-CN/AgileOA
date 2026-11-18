package notificationSvc

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

// Deliver 按接收人和来源幂等投递；后台不生成用户操作审计，提交成功后记录操作日志。
func Deliver(ctx context.Context, value notificationModels.Delivery) error {
	// 后台参数必须完整；账号已硬删除时不重新创建账号或通知。
	if !notificationUtils.ValidDelivery(value) {
		return errors.New("invalid notification delivery")
	}
	if db.DB() == nil {
		return errors.New("database is unavailable")
	}
	var notification notificationModels.Notification
	var created bool
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 账号锁与已读接口保持一致，重复投递不会重置已读时间。
		var user userModels.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, value.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var err error
		notification, created, err = notificationUtils.Create(tx, value)
		return err
	})

	// 仅在确实创建且事务已提交后记录成功，不写通知正文。
	if err == nil && created {
		notificationUtils.LogCreated(ctx, []notificationModels.Notification{notification})
	}
	return err
}
