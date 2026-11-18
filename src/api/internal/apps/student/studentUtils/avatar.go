package studentUtils

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// AvatarUploadSnapshot 只返回安全元数据，不公开对象路径或临时凭据。
func AvatarUploadSnapshot(file studentModels.AvatarUpload) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id": file.ID, "student_id": file.StudentID, "filename": file.Filename,
		"size": file.Size, "content_type": file.ContentType, "state": file.State,
		"revision": file.Revision, "student_revision": file.StudentRevision,
		"object_hash": file.ObjectHash, "created_at": file.CreatedAt, "updated_at": file.UpdatedAt,
	}
}

// SnapshotAvatarDeletion 在学生或空间级联删除前保存全部上传记录及加密对象清理信息。
func SnapshotAvatarDeletion(
	tx *gorm.DB, workspaceID string, studentID uint64, event *auditModels.Event,
) ([]studentModels.AvatarUpload, error) {
	query := tx.Where("workspace_id = ?", workspaceID)
	if studentID != 0 {
		query = query.Where("student_id = ?", studentID)
	}
	var files []studentModels.AvatarUpload
	if err := query.Order("id").Find(&files).Error; err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return files, nil
	}

	// 路径只进入独立加密快照，供提交后对象清理失败时人工恢复。
	snapshots := make([]auditModels.Snapshot, 0, len(files))
	objects := make([]auditModels.Snapshot, 0, len(files))
	for _, file := range files {
		snapshots = append(snapshots, AvatarUploadSnapshot(file))
		objects = append(objects, auditModels.Snapshot{
			"id": file.ID, "student_id": file.StudentID, "object_key": file.ObjectKey,
		})
	}
	if event.Before == nil {
		event.Before = auditModels.Snapshot{}
	}
	if event.SensitiveBefore == nil {
		event.SensitiveBefore = auditModels.Snapshot{}
	}
	event.Before["avatar_uploads"] = snapshots
	event.SensitiveBefore["avatar_upload_objects"] = objects
	return files, nil
}

// DeleteAvatarObjects 在已审计的主记录删除提交后清理对象，失败仅记录安全标识。
func DeleteAvatarObjects(ctx context.Context, files []studentModels.AvatarUpload) {
	for _, file := range files {
		if err := storage.Delete(ctx, file.ObjectKey); err != nil {
			logger.Logger.WithContext(ctx).Errorf(
				"Student avatar cleanup failed; workspace: %s; student: %d; upload: %d",
				file.WorkspaceID, file.StudentID, file.ID,
			)
			continue
		}
		logger.Logger.WithContext(ctx).Infof(
			"Student avatar object deleted; workspace: %s; student: %d; upload: %d",
			file.WorkspaceID, file.StudentID, file.ID,
		)
	}
}
