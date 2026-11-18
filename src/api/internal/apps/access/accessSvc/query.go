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
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
)

// MyWorkspaces 仅返回显式授予进入权限的空间，节点指派不参与空间授权。
func MyWorkspaces(
	ctx context.Context,
	query pagination.Query,
) (workspaceModels.ListData, error) {
	result := workspaceModels.ListData{
		Items: []workspaceModels.Info{}, Page: query.Page, PageSize: query.PageSize,
	}
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectWorkspace,
		Conditions: auditModels.Snapshot{"page": query.Page, "page_size": query.PageSize},
	}
	err := accessUtils.Session(ctx, accessConstants.AccessMyWorkspaces, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			permissions := tx.Model(&accessModels.Permission{}).Select("1").
				Where("access_permissions.workspace_id = workspaces.id AND user_id = ? AND action = ?",
					principal.User.ID, accessConstants.AccessWorkspace)
			scope := tx.Model(&workspaceModels.Workspace{}).
				Where("workspaces.is_enabled = ?", true).
				Where("EXISTS (?)", permissions)
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			var workspaces []workspaceModels.Workspace
			if err := scope.Select("workspaces.*").Order("workspaces.id DESC").
				Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
				Find(&workspaces).Error; err != nil {
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
