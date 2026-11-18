package notificationUtils

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// SnapshotStudentDeletion 在级联删除前保存通知及接收人身份；学生编号为零时保存整个空间。
func SnapshotStudentDeletion(
	tx *gorm.DB,
	workspaceID string,
	studentID uint64,
	event *auditModels.Event,
) ([]uint64, error) {
	// 查询始终限定租户，接收人姓名和学号在主记录删除前一并固定。
	var rows []struct {
		notificationModels.StudentNotification
		StudentNo   string
		StudentName string
	}
	scope := tx.Table("student_notifications AS n").
		Select("n.*, s.student_no, s.name AS student_name").
		Joins("JOIN students AS s ON s.workspace_id = n.workspace_id AND s.id = n.student_id").
		Where("n.workspace_id = ?", workspaceID)
	if studentID != 0 {
		scope = scope.Where("n.student_id = ?", studentID)
	}
	if err := scope.Order("n.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	snapshots := make([]auditModels.Snapshot, 0, len(rows))
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		snapshots = append(snapshots, auditModels.Snapshot{
			"id": row.ID, "workspace_id": row.WorkspaceID, "student_id": row.StudentID,
			"student_no": row.StudentNo, "student_name": row.StudentName,
			"source_id": row.SourceID, "title": row.Title, "content": row.Content,
			"url": row.URL, "read_at": row.ReadAt,
			"created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
		})
	}
	if len(snapshots) > 0 {
		event.Before["student_notifications"] = snapshots
	}
	return ids, nil
}

// LogStudentDeleted 仅在级联删除和审计提交后记录实际删除的通知编号。
func LogStudentDeleted(ctx context.Context, workspaceID string, ids []uint64) {
	if len(ids) > 0 {
		logger.Logger.WithContext(ctx).Infof(
			"Student notifications deleted; workspace: %s; ids: %v", workspaceID, ids,
		)
	}
}
