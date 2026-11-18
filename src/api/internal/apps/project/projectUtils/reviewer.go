package projectUtils

import (
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
)

// HasReviewAssignment 只提供组织审批人身份提示，不授予空间发现、进入或业务权限。
func HasReviewAssignment(tx *gorm.DB, workspaceID string, userID uint64) (bool, error) {
	for _, query := range []*gorm.DB{
		tx.Model(&projectModels.ApprovalStep{}), tx.Model(&projectModels.ApplicationNode{}),
	} {
		var count int64
		if err := query.Where("workspace_id = ? AND kind = ? AND user_id = ?", workspaceID,
			"user", userID).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
