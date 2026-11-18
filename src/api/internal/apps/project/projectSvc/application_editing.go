package projectSvc

import (
	"slices"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func syncProjectApplicationEditing(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
	changes map[int]bool,
) ([]uint64, error) {
	// 草稿预览实时项目配置，已通过申报保留最终节点；只同步已提交的未完成申报。
	ids := []uint64{}
	var applications []projectModels.Application
	if err := tx.Where("workspace_id = ? AND project_id = ? AND status IN ?",
		workspaceID, projectID, []string{
			projectConstants.ApplicationPending, projectConstants.ApplicationRejected,
		}).Order("id ASC").Find(&applications).Error; err != nil {
		return nil, err
	}
	for _, application := range applications {
		before, err := applicationWorkflowSnapshot(tx, application)
		if err != nil {
			return nil, err
		}
		changed, err := updateApplicationEditing(tx, application, changes)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
		var updated projectModels.Application
		if err := tx.Where("workspace_id = ? AND project_id = ? AND id = ?",
			workspaceID, projectID, application.ID).Take(&updated).Error; err != nil {
			return nil, err
		}
		after, err := applicationWorkflowSnapshot(tx, updated)
		if err != nil {
			return nil, err
		}
		item := event(workspaceID, "project_application", application.ID)
		item.Action, item.Result = accessConstants.ProjectUpdate, auditConstants.Success
		item.Before, item.After = before, after
		if err := auditSvc.Write(tx, item); err != nil {
			return nil, err
		}
		ids = append(ids, application.ID)
	}
	return ids, nil
}

func updateApplicationEditing(
	tx *gorm.DB,
	application projectModels.Application,
	changes map[int]bool,
) (bool, error) {
	// 固定节点更新顺序，编辑开关不改变进度、有效签署或流转历史。
	positions := make([]int, 0, len(changes))
	for position := range changes {
		positions = append(positions, position)
	}
	slices.Sort(positions)
	changed := false
	for _, position := range positions {
		written := tx.Model(&projectModels.ApplicationNode{}).
			Where("workspace_id = ? AND application_id = ? AND position = ? AND allow_edit <> ?",
				application.WorkspaceID, application.ID, position, changes[position]).
			Update("allow_edit", changes[position])
		if written.Error != nil {
			return false, written.Error
		}
		changed = changed || written.RowsAffected > 0
	}
	if !changed {
		return false, nil
	}
	written := tx.Model(&projectModels.Application{}).
		Where("workspace_id = ? AND project_id = ? AND id = ? AND revision = ?",
			application.WorkspaceID, application.ProjectID, application.ID, application.Revision).
		Update("revision", application.Revision+1)
	if written.Error != nil {
		return false, written.Error
	}
	if written.RowsAffected != 1 {
		return false, stale()
	}
	return true, nil
}
