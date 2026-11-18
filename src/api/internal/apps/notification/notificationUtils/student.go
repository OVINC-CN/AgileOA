package notificationUtils

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// CreateStudent 在业务事务内按空间、学生及来源幂等投递，不重置已读状态。
func CreateStudent(
	tx *gorm.DB,
	value notificationModels.StudentDelivery,
) (notificationModels.StudentNotification, bool, error) {
	// 消息字段沿用组织通知的固定长度和站内链接校验。
	if !workspaceUtils.ValidID(value.WorkspaceID) || !ValidDelivery(notificationModels.Delivery{
		UserID: value.StudentID, SourceID: value.SourceID,
		Title: value.Title, Content: value.Content, URL: value.URL,
	}) {
		return notificationModels.StudentNotification{}, false, errors.New(
			"invalid student notification delivery",
		)
	}
	if _, transaction := tx.Statement.ConnPool.(gorm.TxCommitter); !transaction {
		return notificationModels.StudentNotification{}, false, errors.New(
			"student notification requires a transaction",
		)
	}

	// 复合外键保证学生属于通知空间；重复投递保留原内容与首次已读时间。
	notification := notificationModels.StudentNotification{
		WorkspaceID: value.WorkspaceID, StudentID: value.StudentID, SourceID: value.SourceID,
		Title: value.Title, Content: value.Content, URL: value.URL,
	}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification)
	return notification, created.RowsAffected > 0, created.Error
}

// LogStudentCreated 仅在业务与审计事务提交后记录实际新增的学生通知，不记录正文。
func LogStudentCreated(
	ctx context.Context,
	notifications []notificationModels.StudentNotification,
) {
	for _, notification := range notifications {
		logger.Logger.WithContext(ctx).Infof(
			"Student notification delivered; id: %d; workspace: %s; student: %d; source: %s",
			notification.ID, notification.WorkspaceID, notification.StudentID, notification.SourceID,
		)
	}
}
