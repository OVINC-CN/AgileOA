package accessUtils

import (
	"context"
	"database/sql"
	"net/http"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// StudentWorkspace 在绑定空间锁内实时复核可信学生身份，业务和审计同事务提交。
func StudentWorkspace(
	ctx context.Context,
	workspaceID, action string,
	event auditModels.Event,
	operation func(*gorm.DB, userModels.StudentIdentity, *auditModels.Event) error,
) error {
	// 自助范围仅开放固定目录，组织账号仍需逐项工作空间授权。
	definition, ok := accessConstants.Lookup(action)
	if !ok || definition.Scope != accessConstants.ScopeSession ||
		!accessConstants.StudentServiceAction(action) {
		return Denied()
	}
	ctx, span := tracing.Tracer.Start(ctx, action)
	defer span.End()
	event.TenantID = &workspaceID
	identity, err := userUtils.Identity(ctx)
	if err != nil {
		return Reject(ctx, action, event, err)
	}
	if identity.Student == nil || identity.Student.WorkspaceID != workspaceID {
		return Reject(ctx, action, event, Denied())
	}
	if db.DB() == nil {
		return Reject(ctx, action, event, unavailable())
	}
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 与档案、项目和授权修改共用空间锁，不重新读取登录凭据。
		workspace, err := lockWorkspace(tx, workspaceID, accessConstants.ReadOnly(action))
		if err != nil {
			return err
		}
		var current struct{ StudentNo, Name, Email, Status string }
		if err := tx.Table("students").Select("student_no", "name", "email", "status").
			Where("workspace_id = ? AND id = ?", workspaceID, identity.Student.StudentID).
			Take(&current).Error; err != nil {
			return err
		}
		student := *identity.Student
		if !workspace.IsEnabled || current.Status != studentConstants.StatusEnrolled ||
			current.StudentNo != student.StudentNo || current.Email != student.Email {
			return &response.BusinessError{
				Status: http.StatusUnauthorized, Message: "学生登录已失效，请重新登录",
			}
		}
		student.Name = current.Name
		if err := operation(tx, student, &event); err != nil {
			return err
		}
		event.Actor = userUtils.StudentActor(student)
		event.Action, event.Result = action, auditConstants.Success
		return auditSvc.Write(tx, event)
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Reject(ctx, action, event, err)
	}
	return nil
}
