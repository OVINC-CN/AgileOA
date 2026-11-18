package attachmentUtils

import (
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
)

// HasOtherTimelineReferences 在空间锁内判断私有对象是否仍被本租户其他人员节点引用。
func HasOtherTimelineReferences(
	tx *gorm.DB, workspaceID, objectHash string, excluding []uint64,
) (bool, error) {
	var count int64
	scope := tx.Model(&attachmentModels.PersonTimelineAttachment{}).
		Where("workspace_id = ? AND object_hash = ?", workspaceID, objectHash)
	if len(excluding) > 0 {
		scope = scope.Where("id NOT IN ?", excluding)
	}
	err := scope.Count(&count).Error
	return count > 0, err
}
