package accessSvc

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
)

// MyWorkspaces 仅返回显式授予进入权限的空间，节点指派不参与空间授权。
func MyWorkspaces(ctx context.Context) (accessModels.MyWorkspacesData, error) {
	result := accessModels.MyWorkspacesData{Items: []accessModels.MyWorkspace{}}
	event := auditModels.Event{ObjectType: auditConstants.ObjectWorkspace}
	err := accessUtils.Session(ctx, accessConstants.AccessMyWorkspaces, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			// 仅查询当前账号显式获准进入的启用空间。
			permissions := tx.Model(&accessModels.Permission{}).Select("1").
				Where("access_permissions.workspace_id = workspaces.id AND user_id = ? AND action = ?",
					principal.User.ID, accessConstants.AccessWorkspace)
			if err := tx.Model(&workspaceModels.Workspace{}).
				Select("workspaces.id, workspaces.name, workspaces.description").
				Where("workspaces.is_enabled = ?", true).
				Where("EXISTS (?)", permissions).
				Order("workspaces.id DESC").
				Find(&result.Items).Error; err != nil {
				return err
			}

			// 记录本次返回的空间编号及数量。
			for _, workspace := range result.Items {
				event.ReturnedIDs = append(event.ReturnedIDs, workspace.ID)
			}
			event.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}
