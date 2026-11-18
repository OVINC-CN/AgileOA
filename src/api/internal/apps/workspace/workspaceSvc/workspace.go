package workspaceSvc

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 仅允许系统管理员查询全部工作空间，不将管理权限授予普通用户。
func List(
	ctx context.Context,
	query workspaceModels.ListQuery,
) (workspaceModels.ListData, error) {
	result := workspaceModels.ListData{
		Items: []workspaceModels.Info{}, Page: query.Page, PageSize: query.PageSize,
	}
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectWorkspace,
		Conditions: auditModels.Snapshot{
			"keyword": query.Keyword, "status": query.Statuses,
			"page": query.Page, "page_size": query.PageSize,
		},
	}
	err := accessUtils.System(ctx, accessConstants.WorkspaceList, event,
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			scope := tx.Model(&workspaceModels.Workspace{})
			if query.Keyword != "" {
				scope = scope.Where("name LIKE ?", "%"+query.Keyword+"%")
			}
			if len(query.Statuses) > 0 {
				states := make([]bool, 0, len(query.Statuses))
				for _, status := range query.Statuses {
					states = append(states, status == "enabled")
				}
				scope = scope.Where("is_enabled IN ?", states)
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			var workspaces []workspaceModels.Workspace
			if err := scope.Order("id DESC").
				Offset((query.Page - 1) * query.PageSize).
				Limit(query.PageSize).
				Find(&workspaces).
				Error; err != nil {
				return err
			}
			for _, workspace := range workspaces {
				result.Items = append(result.Items, workspaceUtils.Info(workspace))
				event.ReturnedIDs = append(event.ReturnedIDs, workspace.ID)
			}
			event.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// Enum 仅向系统管理员一次返回完整工作空间枚举，包含停用空间。
func Enum(ctx context.Context) ([]workspaceModels.EnumOption, error) {
	result := []workspaceModels.EnumOption{}
	event := auditModels.Event{ObjectType: auditConstants.ObjectWorkspace}
	err := accessUtils.System(ctx, accessConstants.WorkspaceEnum, event,
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			var workspaces []workspaceModels.Workspace
			if err := tx.Select("id,name,is_enabled").
				Order("name ASC, id ASC").
				Find(&workspaces).
				Error; err != nil {
				return err
			}
			for _, workspace := range workspaces {
				result = append(result, workspaceModels.EnumOption{
					Value: workspace.ID, Label: workspace.Name, IsEnabled: workspace.IsEnabled,
				})
				event.ReturnedIDs = append(event.ReturnedIDs, workspace.ID)
			}
			event.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Create 由系统管理员初始化工作空间，不自动授予业务权限。
func Create(
	ctx context.Context,
	request workspaceModels.CreateRequest,
) (workspaceModels.Info, error) {
	var result workspaceModels.Info
	event := auditModels.Event{ObjectType: auditConstants.ObjectWorkspace}
	err := accessUtils.System(ctx, accessConstants.WorkspaceCreate, event,
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			if !valid(request) {
				return invalid()
			}
			id, err := newID()
			if err != nil {
				return err
			}
			workspace := workspaceModels.Workspace{
				ID: id,
				Name: strings.TrimSpace(
					request.Name,
				),
				Description: strings.TrimSpace(request.Description),
				IsEnabled:   true,
				Revision:    1,
			}
			if err := tx.Create(&workspace).Error; err != nil {
				return workspaceWriteError(err)
			}
			// 响应和审计读取数据库自动维护的时间。
			if err := tx.Where("id = ?", workspace.ID).Take(&workspace).Error; err != nil {
				return err
			}
			event.TenantID = &workspace.ID
			event.ObjectID = workspace.ID
			event.After = snapshot(workspace)
			result = workspaceUtils.Info(workspace)
			return nil
		})
	return result, err
}

// Detail 在工作空间锁内返回系统管理信息。
func Detail(ctx context.Context, id string) (workspaceModels.Info, error) {
	var result workspaceModels.Info
	err := accessUtils.System(ctx, accessConstants.WorkspaceDetail, workspaceEvent(id),
		func(_ *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			result = workspaceUtils.Info(principal.Workspace)
			event.ReturnedIDs = []string{id}
			event.ReturnedCount = 1
			return nil
		})
	return result, err
}

// Update 只允许系统管理员在工作空间锁内修改基本信息与启用状态。
func Update(
	ctx context.Context,
	id string,
	request workspaceModels.UpdateRequest,
) (workspaceModels.Info, error) {
	var result workspaceModels.Info
	err := accessUtils.System(ctx, accessConstants.WorkspaceUpdate, workspaceEvent(id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			workspace := principal.Workspace
			if !valid(request.CreateRequest) || request.IsEnabled == nil || request.Revision == 0 {
				return invalid()
			}
			if workspace.Revision != request.Revision {
				return &response.BusinessError{
					Status:  http.StatusConflict,
					Message: "工作空间已被修改，请刷新后重试",
				}
			}
			event.Before = snapshot(workspace)
			workspace.Name = strings.TrimSpace(request.Name)
			workspace.Description = strings.TrimSpace(request.Description)
			workspace.IsEnabled = *request.IsEnabled
			workspace.Revision++
			if err := tx.Model(&workspaceModels.Workspace{}).
				Where("id = ?", id).
				Updates(map[string]any{
					"name": workspace.Name, "description": workspace.Description,
					"is_enabled": workspace.IsEnabled, "revision": workspace.Revision,
				}).Error; err != nil {
				return workspaceWriteError(err)
			}
			if err := tx.Where("id = ?", id).Take(&workspace).Error; err != nil {
				return err
			}
			event.After = snapshot(workspace)
			result = workspaceUtils.Info(workspace)
			return nil
		})
	return result, err
}

func workspaceWriteError(err error) error {
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return &response.BusinessError{Status: http.StatusConflict, Message: "工作空间名称已存在"}
	}
	return err
}

// Delete 先校验空间授权及项目限制，再硬删除空间和依赖业务记录。
func Delete(ctx context.Context, id string) error {
	var avatarUploads []studentModels.AvatarUpload
	var notificationIDs []uint64
	err := accessUtils.System(ctx, accessConstants.WorkspaceDelete, workspaceEvent(id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			var count int64
			if err := tx.Model(&accessModels.Permission{}).
				Where("workspace_id = ?", id).
				Count(&count).
				Error; err != nil {
				return err
			}
			if count > 0 {
				return referenced()
			}
			if err := tx.Table("projects").
				Where("workspace_id = ?", id).
				Count(&count).
				Error; err != nil {
				return err
			}
			if count > 0 {
				return referenced()
			}
			event.Before = snapshot(principal.Workspace)
			var err error
			notificationIDs, err = notificationUtils.SnapshotStudentDeletion(tx, id, 0, event)
			if err != nil {
				return err
			}
			avatarUploads, err = studentUtils.SnapshotAvatarDeletion(tx, id, 0, event)
			if err != nil {
				return err
			}
			if err := tx.Where("id = ?", id).
				Delete(&workspaceModels.Workspace{}).
				Error; err != nil {
				var foreignKey *mysql.MySQLError
				if errors.As(err, &foreignKey) && foreignKey.Number == 1451 {
					return referenced()
				}
				return err
			}
			return nil
		})
	if err == nil {
		studentUtils.DeleteAvatarObjects(ctx, avatarUploads)
		notificationUtils.LogStudentDeleted(ctx, id, notificationIDs)
	}
	return err
}

// Enter 与管理功能分离，系统管理员也必须持有本空间的显式进入权限。
func Enter(ctx context.Context, id string) (accessModels.ContextData, error) {
	var result accessModels.ContextData
	err := accessUtils.Workspace(
		ctx,
		id,
		accessConstants.AccessWorkspace,
		workspaceEvent(id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			actions, err := accessUtils.Actions(tx, principal.User.ID, id)
			if err != nil {
				return err
			}
			assigned, err := projectUtils.HasReviewAssignment(tx, id, principal.User.ID)
			if err != nil {
				return err
			}
			result = accessModels.ContextData{
				Workspace: workspaceUtils.Info(principal.Workspace),
				Actions:   actions,
				CanReviewProjectApplications: assigned &&
					slices.Contains(actions, accessConstants.ProjectApplicationList),
			}
			event.ReturnedIDs = []string{id}
			event.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

func workspaceEvent(id string) auditModels.Event {
	return auditModels.Event{
		TenantID: &id, ObjectType: auditConstants.ObjectWorkspace,
		ObjectID:   id,
		Conditions: auditModels.Snapshot{"workspace_id": id},
	}
}

func snapshot(workspace workspaceModels.Workspace) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id": workspace.ID, "name": workspace.Name, "description": workspace.Description,
		"is_enabled": workspace.IsEnabled, "revision": workspace.Revision,
		"created_at": workspace.CreatedAt, "updated_at": workspace.UpdatedAt,
	}
}

func valid(request workspaceModels.CreateRequest) bool {
	return strings.TrimSpace(request.Name) != "" && utf8.RuneCountInString(request.Name) <= 100 &&
		utf8.RuneCountInString(request.Description) <= 500
}

func invalid() error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: "工作空间参数无效"}
}

func referenced() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "工作空间仍有授权或奖项项目，请先处理后再删除"}
}
