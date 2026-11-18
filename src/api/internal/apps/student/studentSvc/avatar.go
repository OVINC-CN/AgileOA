package studentSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// PrepareAvatar 保留当前头像，在学生版本保护下登记独立待完成上传。
func PrepareAvatar(
	ctx context.Context, workspaceID string, id, revision uint64,
	request attachmentModels.UploadRequest,
) (attachmentModels.UploadData, error) {
	var result attachmentModels.UploadData
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentUpdate,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			// 复核学生版本，头像只接受固定的三种声明类型。
			before, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			if !storage.Enabled() {
				return &response.BusinessError{
					Status: http.StatusServiceUnavailable, Message: "头像上传暂时不可用",
				}
			}
			if err := attachmentUtils.ValidateUploadRequest(request); err != nil {
				return err
			}
			if request.ContentType != "image/jpeg" && request.ContentType != "image/png" &&
				request.ContentType != "image/webp" {
				return invalid("头像仅支持 JPEG、PNG 或 WebP 图片")
			}
			if !validAvatarKey(workspaceID, avatarPrefix(workspaceID)+request.Filename) {
				return invalid("头像文件名须使用 JPEG、PNG 或 WebP 扩展名")
			}
			key, err := attachmentUtils.RandomObjectName(
				avatarPrefix(workspaceID)+strconv.FormatUint(id, 10), request.Filename,
			)
			if err != nil {
				return err
			}
			if len(key) > studentConstants.AvatarMaxKeyBytes {
				return invalid("头像文件名过长")
			}
			digest := sha256.Sum256([]byte(key))
			file := studentModels.AvatarUpload{
				StudentID: id, StudentRevision: revision,
				Record: attachmentModels.Record{
					WorkspaceID: workspaceID,
					ObjectKey:   key,
					ObjectHash:  hex.EncodeToString(digest[:]),
					Filename:    request.Filename,
					Size:        request.Size,
					ContentType: request.ContentType,
					UploadActor: attachmentUtils.UploadActorKey(principal.User.ID, 0),
					State:       attachmentConstants.Pending,
					Revision:    1,
				},
			}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, file.ID).
				Take(&file).
				Error; err != nil {
				return err
			}
			event.After = studentUtils.AvatarUploadSnapshot(file)
			result.UploadID, result.Revision = file.ID, revision
			result.Upload, err = tencentcos.Credentials(ctx, key, request.ContentType, request.Size)
			return err
		})
	if err == nil {
		logAvatarUpload(ctx, workspaceID, id, result.UploadID, "prepare_upload")
	}
	return result, err
}

// CompleteAvatar 只核对 COS 元数据，同事务替换头像、更新学生版本并保存审计。
func CompleteAvatar(
	ctx context.Context, workspaceID string, id, uploadID, revision uint64,
) (studentModels.MutationData, error) {
	var result studentModels.MutationData
	changed := false
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentUpdate,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			before, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			var file studentModels.AvatarUpload
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Take(&file).
				Error; err != nil {
				return writeError(err)
			}
			if file.UploadActor != attachmentUtils.UploadActorKey(principal.User.ID, 0) {
				return invalid("上传记录不存在或不可访问")
			}
			if file.State == attachmentConstants.Ready && before.Avatar == file.ObjectKey {
				result = studentModels.MutationData{ID: id, Revision: before.Revision}
				return nil
			}
			if file.State != attachmentConstants.Pending || before.Revision != revision ||
				file.StudentRevision != revision {
				return stale()
			}
			if err := attachmentUtils.VerifyUploadedObject(ctx, file.Record); err != nil {
				return err
			}

			// 上传失败或事务失败时，学生仍使用旧头像，待完成记录可显式删除。
			if err := tx.Model(&studentModels.Student{}).
				Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, revision).
				Updates(map[string]any{"avatar": file.ObjectKey, "revision": revision + 1}).
				Error; err != nil {
				return err
			}
			if err := tx.Model(&file).
				Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Updates(map[string]any{"state": attachmentConstants.Ready, "revision": file.Revision + 1}).
				Error; err != nil {
				return err
			}
			after, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			event.Before = infoSnapshot(studentInfo(before), before.Avatar)
			event.Before["avatar_upload"] = studentUtils.AvatarUploadSnapshot(file)
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Take(&file).
				Error; err != nil {
				return err
			}
			event.After = infoSnapshot(studentInfo(after), file.ObjectKey)
			event.After["avatar_upload"] = studentUtils.AvatarUploadSnapshot(file)
			result = studentModels.MutationData{ID: id, Revision: after.Revision}
			changed = true
			return nil
		})
	if err == nil && changed {
		logAvatarUpload(ctx, workspaceID, id, uploadID, "complete_upload")
	}
	return result, err
}

// DeleteAvatarUpload 显式删除失败的头像上传，删除失败保留 deleting 状态供重试。
func DeleteAvatarUpload(
	ctx context.Context, workspaceID string, id, uploadID, revision uint64,
) error {
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentUpdate,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			if _, err := basicDetail(tx, workspaceID, id); err != nil {
				return err
			}
			var file studentModels.AvatarUpload
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Take(&file).
				Error; err != nil {
				return writeError(err)
			}
			if file.Revision != revision || file.State == attachmentConstants.Ready {
				return stale()
			}
			event.Before = studentUtils.AvatarUploadSnapshot(file)
			file.State, file.Revision = attachmentConstants.Deleting, file.Revision+1
			if err := tx.Model(&file).
				Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Updates(map[string]any{"state": file.State, "revision": file.Revision}).
				Error; err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Take(&file).
				Error; err != nil {
				return err
			}
			event.After = studentUtils.AvatarUploadSnapshot(file)
			return nil
		})
	if err != nil {
		return err
	}
	logAvatarUpload(ctx, workspaceID, id, uploadID, "prepare_delete")
	err = accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentUpdate,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			var file studentModels.AvatarUpload
			if err := tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Take(&file).
				Error; err != nil {
				return writeError(err)
			}
			if file.State != attachmentConstants.Deleting {
				return stale()
			}
			if err := storage.Delete(ctx, file.ObjectKey); err != nil {
				return err
			}
			event.Before = studentUtils.AvatarUploadSnapshot(file)
			return tx.Where("workspace_id = ? AND student_id = ? AND id = ?", workspaceID, id, uploadID).
				Delete(&file).
				Error
		})
	if err == nil {
		logAvatarUpload(ctx, workspaceID, id, uploadID, "delete")
	}
	return err
}

func logAvatarUpload(
	ctx context.Context,
	workspaceID string,
	id, uploadID uint64,
	operation string,
) {
	logger.Logger.WithContext(ctx).Infof(
		"Student avatar changed; operation: %s; workspace: %s; student: %d; upload: %d",
		operation, workspaceID, id, uploadID,
	)
}

func avatarPrefix(workspaceID string) string {
	return "workspaces/" + workspaceID + "/students/avatars/"
}

func validAvatarKey(workspaceID, key string) bool {
	relative, ok := strings.CutPrefix(key, avatarPrefix(workspaceID))
	if !ok || len(key) > studentConstants.AvatarMaxKeyBytes ||
		!validText(key, studentConstants.AvatarMaxKeyBytes, false) ||
		strings.ContainsAny(relative, `\%?#`) {
		return false
	}
	// 拒绝可能被浏览器或 CDN 解释为目录跳转的路径段。
	for segment := range strings.SplitSeq(relative, "/") {
		switch strings.TrimSpace(segment) {
		case "", ".", "..":
			return false
		}
	}
	switch strings.ToLower(path.Ext(relative)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}
