package accessUtils

import (
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
)

// WorkspaceUsers 提供当前空间拥有进入权限的启用账号，供候选项及关联校验复用。
func WorkspaceUsers(tx *gorm.DB, workspaceID string) *gorm.DB {
	// 全局账号必须在当前空间显式获得进入权限，其他业务授权不能替代。
	permissions := tx.Table("access_permissions").Select("user_id").
		Where("workspace_id = ? AND action = ?", workspaceID, accessConstants.AccessWorkspace)
	return tx.Table("users").
		Where("users.is_enabled = ? AND users.id IN (?)", true, permissions)
}
