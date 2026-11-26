package notificationSvc

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
)

// List 只返回当前账号收到的通知，审计提交后才向调用方返回正文。
func List(
	ctx context.Context,
	query notificationModels.ListQuery,
) (notificationModels.ListData, error) {
	query, result, event := inboxListQuery(query)
	err := accessUtils.Session(ctx, accessConstants.NotificationList, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			// 通知是账号资源，始终以已认证接收人限定查询，不能指定其他用户。
			scope := tx.Model(&notificationModels.Notification{}).
				Where("user_id = ?", principal.User.ID)
			return listInbox(scope, query, &result, event)
		})
	return result, err
}

// Unread 仅统计当前账号的未读消息。
func Unread(ctx context.Context) (notificationModels.UnreadData, error) {
	var result notificationModels.UnreadData
	event := auditModels.Event{ObjectType: auditConstants.ObjectNotification}
	err := accessUtils.Session(ctx, accessConstants.NotificationUnread, event,
		func(tx *gorm.DB, principal accessModels.Principal, _ *auditModels.Event) error {
			scope := tx.Model(&notificationModels.Notification{}).
				Where("user_id = ?", principal.User.ID)
			return unreadCount(scope, &result.UnreadCount)
		})
	return result, err
}

// MarkRead 在账号锁内校验整批归属，变更和审计共用事务。
func MarkRead(
	ctx context.Context,
	request notificationModels.ReadRequest,
) (notificationModels.ReadData, error) {
	var result notificationModels.ReadData
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectNotification,
		Conditions: auditModels.Snapshot{"ids": request.IDs},
	}
	err := accessUtils.Session(ctx, accessConstants.NotificationRead, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			scope := tx.Model(&notificationModels.Notification{}).
				Where("user_id = ?", principal.User.ID)
			return markInboxRead(scope, request, &result, event)
		})
	return result, err
}
