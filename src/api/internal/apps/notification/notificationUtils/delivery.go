package notificationUtils

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// ValidDelivery 检查内部业务消息的固定字段和长度。
func ValidDelivery(value notificationModels.Delivery) bool {
	return value.UserID > 0 && strings.TrimSpace(value.SourceID) != "" &&
		utf8.RuneCountInString(value.SourceID) <= notificationConstants.SourceMaxLength &&
		strings.TrimSpace(value.Title) != "" &&
		utf8.RuneCountInString(value.Title) <= notificationConstants.TitleMaxLength &&
		strings.TrimSpace(value.Content) != "" &&
		utf8.RuneCountInString(value.Content) <= notificationConstants.ContentMaxLength &&
		ValidURL(value.URL)
}

// Create 在调用方业务事务内幂等创建通知，不提前写成功日志或单独提交。
func Create(
	tx *gorm.DB,
	value notificationModels.Delivery,
) (notificationModels.Notification, bool, error) {
	if !ValidDelivery(value) {
		return notificationModels.Notification{}, false, errors.New("invalid notification delivery")
	}
	if _, transaction := tx.Statement.ConnPool.(gorm.TxCommitter); !transaction {
		return notificationModels.Notification{}, false, errors.New(
			"notification requires a transaction",
		)
	}
	notification := notificationModels.Notification{
		UserID: value.UserID, SourceID: value.SourceID,
		Title: value.Title, Content: value.Content, URL: value.URL,
	}
	// 同一来源的重复投递不覆盖原内容及已读时间。
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification)
	return notification, created.RowsAffected > 0, created.Error
}

// LogCreated 由调用方在事务提交成功后记录实际新建通知，不写正文。
func LogCreated(ctx context.Context, notifications []notificationModels.Notification) {
	for _, notification := range notifications {
		logger.Logger.WithContext(ctx).Infof(
			"Notification delivered; id: %d; user: %d; source: %s",
			notification.ID, notification.UserID, notification.SourceID,
		)
	}
}
