package monthlyworkUtils

import (
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
)

// Records 限定工作记录所属的空间与类别。
func Records(tx *gorm.DB, workspaceID, category string) *gorm.DB {
	return tx.Model(&monthlyworkModels.Record{}).
		Where("workspace_id = ? AND category = ?", workspaceID, category)
}

// VisibleRecords 在独立操作权限通过后，限定为本人负责或参与的工作。
func VisibleRecords(tx *gorm.DB, workspaceID, category string, userID uint64) *gorm.DB {
	participating := tx.Model(&monthlyworkModels.Participant{}).
		Select("monthly_work_id").
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID)
	return Records(tx, workspaceID, category).
		Where("(owner_id = ? OR id IN (?))", userID, participating)
}
