package accessSvc

import (
	"context"
	"slices"
	"strconv"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
)

// Catalog 由系统管理员读取完整操作目录，不接受外部新增或修改。
func Catalog(ctx context.Context) (accessModels.CatalogData, error) {
	result := accessModels.CatalogData{Items: accessConstants.Catalog()}
	event := auditModels.Event{ObjectType: auditConstants.ObjectPermission}
	err := accessUtils.System(ctx, accessConstants.AccessCatalog, event,
		func(_ *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			for _, item := range result.Items {
				event.ReturnedIDs = append(event.ReturnedIDs, item.Value)
			}
			event.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// UserActions 查询指定空间用户的显式权限，不从其他用户或空间推导。
func UserActions(
	ctx context.Context,
	workspaceID string,
	userID uint64,
) (accessModels.ActionsData, error) {
	var result accessModels.ActionsData
	err := accessUtils.System(
		ctx,
		accessConstants.AccessActionsRead,
		permissionEvent(workspaceID, userID),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			if err := targetUser(tx, userID); err != nil {
				return err
			}
			actions, err := accessUtils.Actions(tx, userID, workspaceID)
			if err != nil {
				return err
			}
			result = accessModels.ActionsData{
				Actions:  actions,
				Revision: principal.Workspace.Revision,
			}
			event.ReturnedIDs = actions
			event.ReturnedCount = int64(len(actions))
			return nil
		},
	)
	return result, err
}

// UpdateActions 完整替换单个用户的操作权限，在空间锁内复核并逐条审计增删。
func UpdateActions(
	ctx context.Context,
	workspaceID string,
	userID uint64,
	request accessModels.UpdateActionsRequest,
) (accessModels.ActionsData, error) {
	var result accessModels.ActionsData
	err := accessUtils.System(
		ctx,
		accessConstants.AccessActionsUpdate,
		permissionEvent(workspaceID, userID),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			if err := targetUser(tx, userID); err != nil {
				return err
			}
			if request.Actions == nil || request.Revision == 0 {
				return invalid()
			}
			if principal.Workspace.Revision != request.Revision {
				return stale()
			}
			wanted := make(map[string]bool, len(request.Actions))
			for _, action := range request.Actions {
				if !accessConstants.Grantable(action) || wanted[action] {
					return invalid()
				}
				wanted[action] = true
			}

			// 读出本空间内全部旧授权；代码中已移除的旧标识也只能显式撤销。
			var grants []accessModels.Permission
			if err := tx.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
				Order("action ASC").Find(&grants).Error; err != nil {
				return err
			}
			existing := make(map[string]bool, len(grants))
			for _, grant := range grants {
				existing[grant.Action] = true
				if wanted[grant.Action] {
					continue
				}
				if err := tx.Where("workspace_id = ? AND user_id = ? AND action = ?", workspaceID, userID, grant.Action).
					Delete(&accessModels.Permission{}).
					Error; err != nil {
					return err
				}
				if err := auditPermission(tx, grant, false); err != nil {
					return err
				}
			}
			actions := append([]string{}, request.Actions...)
			slices.Sort(actions)
			for _, action := range actions {
				if existing[action] {
					continue
				}
				grant := accessModels.Permission{
					WorkspaceID: workspaceID, UserID: userID, Action: action,
				}
				if err := tx.Create(&grant).Error; err != nil {
					return err
				}
				if err := tx.Where(
					"workspace_id = ? AND user_id = ? AND action = ?", workspaceID, userID, action,
				).Take(&grant).Error; err != nil {
					return err
				}
				if err := auditPermission(tx, grant, true); err != nil {
					return err
				}
			}
			event.Before = auditModels.Snapshot{"revision": principal.Workspace.Revision}
			principal.Workspace.Revision++
			if err := tx.Model(&workspaceModels.Workspace{}).Where("id = ?", workspaceID).
				Updates(map[string]any{"revision": principal.Workspace.Revision}).
				Error; err != nil {
				return err
			}
			event.After = auditModels.Snapshot{"revision": principal.Workspace.Revision}
			result = accessModels.ActionsData{
				Actions:  actions,
				Revision: principal.Workspace.Revision,
			}
			return nil
		},
	)
	return result, err
}

func auditPermission(tx *gorm.DB, grant accessModels.Permission, created bool) error {
	snapshot := auditModels.Snapshot{
		"workspace_id": grant.WorkspaceID, "user_id": grant.UserID,
		"action": grant.Action, "created_at": grant.CreatedAt,
	}
	event := auditModels.Event{
		TenantID: &grant.WorkspaceID, Actor: auditUtils.Actor(tx.Statement.Context),
		Action: accessConstants.AccessActionsUpdate, ObjectType: auditConstants.ObjectPermission,
		ObjectID: strconv.FormatUint(grant.UserID, 10) + "/" + grant.Action,
		Result:   auditConstants.Success,
	}
	if created {
		event.After = snapshot
	} else {
		event.Before = snapshot
	}
	return auditSvc.Write(tx, event)
}
