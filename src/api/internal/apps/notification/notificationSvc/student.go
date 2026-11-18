package notificationSvc

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
)

// StudentList 只返回绑定空间内当前学生的通知，审计提交后才返回正文。
func StudentList(
	ctx context.Context,
	workspaceID string,
	query notificationModels.ListQuery,
) (notificationModels.ListData, error) {
	query, result, event := inboxListQuery(query)
	err := accessUtils.StudentWorkspace(ctx, workspaceID, accessConstants.StudentNotificationList,
		event,
		func(tx *gorm.DB, student userModels.StudentIdentity, event *auditModels.Event) error {
			return listInbox(studentInbox(tx, student), query, &result, event)
		})
	return result, err
}

// StudentUnread 只统计当前学生的未读数量，不读取通知正文。
func StudentUnread(
	ctx context.Context,
	workspaceID string,
) (notificationModels.UnreadData, error) {
	var result notificationModels.UnreadData
	event := auditModels.Event{ObjectType: auditConstants.ObjectNotification}
	err := accessUtils.StudentWorkspace(ctx, workspaceID, accessConstants.StudentNotificationUnread,
		event,
		func(tx *gorm.DB, student userModels.StudentIdentity, event *auditModels.Event) error {
			if err := unreadCount(studentInbox(tx, student), &result.UnreadCount); err != nil {
				return err
			}
			event.ReturnedCount = result.UnreadCount
			return nil
		})
	return result, err
}

// StudentMarkRead 在空间锁内复核学生身份，整批变更和审计共用事务。
func StudentMarkRead(
	ctx context.Context,
	workspaceID string,
	request notificationModels.ReadRequest,
) (notificationModels.ReadData, error) {
	var result notificationModels.ReadData
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectNotification,
		Conditions: auditModels.Snapshot{"ids": request.IDs},
	}
	err := accessUtils.StudentWorkspace(ctx, workspaceID, accessConstants.StudentNotificationRead,
		event,
		func(tx *gorm.DB, student userModels.StudentIdentity, event *auditModels.Event) error {
			return markInboxRead(studentInbox(tx, student), request, &result, event)
		})
	return result, err
}

func studentInbox(tx *gorm.DB, student userModels.StudentIdentity) *gorm.DB {
	// 学生编号和空间均来自事务内复核的身份，忽略请求中的接收人参数。
	return tx.Model(&notificationModels.StudentNotification{}).
		Where("workspace_id = ? AND student_id = ?", student.WorkspaceID, student.StudentID)
}
