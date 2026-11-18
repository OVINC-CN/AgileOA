package projectSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/storage"
)

func applicationFileAction(student bool, mode, operation string) string {
	if !student {
		if operation == "upload" {
			return accessConstants.ProjectApplicationAttachmentUpload
		}
		return accessConstants.ProjectApplicationAttachmentDelete
	}
	if mode == projectConstants.ApplicationMine {
		if operation == "upload" {
			return accessConstants.StudentApplicationAttachmentUpload
		}
		return accessConstants.StudentApplicationAttachmentDelete
	}
	if operation == "upload" {
		return accessConstants.StudentReviewAttachmentUpload
	}
	return accessConstants.StudentReviewAttachmentDelete
}

func applicationAttachmentInfo(
	file attachmentModels.ApplicationDetailAttachment,
) attachmentModels.Info {
	return attachmentModels.Info{
		ID:            file.ID,
		ApplicationID: file.ApplicationID,
		DetailID:      file.DetailID,
		Category:      attachmentConstants.CategoryProjectApplicationDetail,
		Filename:      file.Filename,
		Size:          file.Size,
		State:         file.State,
		Revision:      file.Revision,
		CreatedAt:     file.CreatedAt,
		UpdatedAt:     file.UpdatedAt,
	}
}

func requireApplicationFileEdit(
	tx *gorm.DB,
	workspaceID, mode string,
	actor applicationActor,
	id uint64,
) (projectModels.ApplicationInfo, error) {
	info, err := applicationDetail(tx, workspaceID, mode, actor, id)
	if err != nil {
		return info, err
	}
	if !info.CanEdit {
		return info, accessUtils.Denied()
	}
	if !storage.Enabled() {
		return info, &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "附件服务暂时不可用",
		}
	}
	return info, nil
}

func bumpApplicationForAttachment(
	tx *gorm.DB,
	workspaceID string,
	info projectModels.ApplicationInfo,
) error {
	// 附件属于申报内容，变更后当前级已有签署必须重新确认。
	if err := clearNodeApprovals(tx, workspaceID, info.ID, info.CurrentStep, false); err != nil {
		return err
	}
	return tx.Model(&projectModels.Application{}).
		Where("workspace_id = ? AND id = ?", workspaceID, info.ID).
		Update("revision", info.Revision+1).
		Error
}

func logApplicationFile(ctx context.Context, operation, workspaceID string, id, fileID uint64) {
	logger.Logger.WithContext(ctx).
		Infof("Application attachment changed; operation: %s; workspace: %s; application: %d; attachment: %d", operation, workspaceID, id, fileID)
}

// PrepareApplicationAttachment 登记待完成的子项附件并返回 COS 直传凭据。
func PrepareApplicationAttachment(
	ctx context.Context,
	workspaceID, mode string,
	id, revision uint64,
	detailKey string,
	request attachmentModels.UploadRequest,
) (attachmentModels.UploadData, error) {
	var result attachmentModels.UploadData
	filename, size := request.Filename, request.Size
	var prepared attachmentModels.ApplicationDetailAttachment
	var notifications applicationNotifications
	action := applicationFileAction(isStudentActor(ctx), mode, "upload")
	err := applicationAccess(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_application_attachment", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			before, err := requireApplicationFileEdit(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			if !applicationDetailKeyPattern.MatchString(detailKey) ||
				!attachmentUtils.ValidFilename(filename) ||
				size < 1 ||
				size > config.Config.COS.MaxUploadBytes {
				return invalid("子项、文件名或文件大小无效")
			}
			if err := attachmentUtils.ValidateUploadRequest(request); err != nil {
				return err
			}
			if !attachmentUtils.ContentTypeAllowed(
				request.ContentType,
				[]string{"image/*", "application/pdf"},
			) {
				return invalid("子项附件仅支持图片或 PDF")
			}
			var detail projectModels.ApplicationDetail
			if err := tx.Where("workspace_id = ? AND application_id = ? AND detail_key = ?", workspaceID, id, detailKey).
				Take(&detail).
				Error; err != nil {
				return lookupError(err)
			}
			var count int64
			if err := tx.Model(&attachmentModels.ApplicationDetailAttachment{}).
				Where("workspace_id = ? AND application_id = ? AND detail_id = ?", workspaceID, id, detail.ID).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(config.Config.Project.MaxAttachments) {
				return invalid("子项附件数量已达上限，请先删除已有附件")
			}
			prefix := fmt.Sprintf(
				"workspaces/%s/project-applications/%d/%s",
				workspaceID,
				id,
				detailKey,
			)
			object, err := attachmentUtils.RandomObjectName(prefix, filename)
			if err != nil {
				return err
			}
			if len(object) > attachmentConstants.ObjectKeyMaxBytes {
				return invalid("文件名过长")
			}
			digest := sha256.Sum256([]byte(object))
			prepared = attachmentModels.ApplicationDetailAttachment{
				ApplicationID: id,
				DetailID:      detail.ID,
				Record: attachmentModels.Record{
					WorkspaceID: workspaceID,
					ObjectKey:   object,
					ObjectHash: hex.EncodeToString(
						digest[:],
					),
					Filename:    filename,
					Size:        size,
					ContentType: request.ContentType,
					UploadActor: attachmentUtils.UploadActorKey(actor.UserID, actor.StudentID),
					State:       attachmentConstants.Pending,
					Revision:    1,
				},
			}
			if err := tx.Create(&prepared).Error; err != nil {
				return err
			}
			if err := bumpApplicationForAttachment(tx, workspaceID, before); err != nil {
				return err
			}
			after, err := applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.Before, audit.After = applicationSnapshot(before), applicationSnapshot(after)
			audit.Conditions["attachment_id"] = prepared.ID
			notifications, err = notifyApplication(tx, workspaceID, before, after)
			if err != nil {
				return err
			}
			result.UploadID, result.Revision = prepared.ID, after.Revision
			result.Upload, err = tencentcos.Credentials(ctx, object, request.ContentType, size)
			return err
		},
	)
	if err != nil {
		return result, err
	}
	logApplicationFile(ctx, "prepare_upload", workspaceID, id, prepared.ID)
	notifications.logCreated(ctx)
	return result, nil
}

// CompleteApplicationAttachment 复核当前节点和上传元数据，再幂等发布附件。
func CompleteApplicationAttachment(
	ctx context.Context, workspaceID, mode string, id, fileID, revision uint64,
) (projectModels.ApplicationInfo, error) {
	var result projectModels.ApplicationInfo
	var notifications applicationNotifications
	changed := false
	action := applicationFileAction(isStudentActor(ctx), mode, "upload")
	err := applicationAccess(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_application_attachment", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			before, err := requireApplicationFileEdit(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			var file attachmentModels.ApplicationDetailAttachment
			if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Take(&file).
				Error; err != nil {
				return lookupError(err)
			}
			if file.UploadActor != attachmentUtils.UploadActorKey(actor.UserID, actor.StudentID) {
				return invalid("上传记录不存在或不可访问")
			}
			if file.State == attachmentConstants.Ready {
				result, err = applicationDetail(tx, workspaceID, mode, actor, id)
				return err
			}
			if file.State != attachmentConstants.Pending || before.Revision != revision {
				return stale()
			}
			if err := attachmentUtils.VerifyUploadedObject(ctx, file.Record); err != nil {
				return err
			}
			if err := tx.Model(&attachmentModels.ApplicationDetailAttachment{}).
				Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, file.ID).
				Updates(map[string]any{"state": attachmentConstants.Ready, "revision": file.Revision + 1}).
				Error; err != nil {
				return err
			}
			if err := bumpApplicationForAttachment(tx, workspaceID, before); err != nil {
				return err
			}
			result, err = applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.Before, audit.After = applicationSnapshot(before), applicationSnapshot(result)
			audit.Conditions["attachment_id"] = file.ID
			changed = true
			notifications, err = notifyApplication(tx, workspaceID, before, result)
			return err
		},
	)
	if err == nil && changed {
		logApplicationFile(ctx, "complete_upload", workspaceID, id, fileID)
		notifications.logCreated(ctx)
	}
	return result, err
}

// DeleteApplicationAttachment 先保存删除意图及审计，再清理对象和元数据，失败可重试。
func DeleteApplicationAttachment(
	ctx context.Context,
	workspaceID, mode string,
	id, fileID, revision, applicationRevision uint64,
) (projectModels.ApplicationInfo, error) {
	var result projectModels.ApplicationInfo
	var notifications applicationNotifications
	action := applicationFileAction(isStudentActor(ctx), mode, "delete")
	err := applicationAccess(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_application_attachment", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			before, err := requireApplicationFileEdit(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			if before.Revision != applicationRevision {
				return stale()
			}
			var file attachmentModels.ApplicationDetailAttachment
			if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Take(&file).
				Error; err != nil {
				return lookupError(err)
			}
			if file.Revision != revision {
				return stale()
			}
			if err := tx.Model(&file).
				Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Updates(map[string]any{"state": attachmentConstants.Deleting, "revision": file.Revision + 1}).
				Error; err != nil {
				return err
			}
			if err := bumpApplicationForAttachment(tx, workspaceID, before); err != nil {
				return err
			}
			after, err := applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.Before, audit.After = applicationSnapshot(before), applicationSnapshot(after)
			audit.Conditions["attachment_id"] = fileID
			notifications, err = notifyApplication(tx, workspaceID, before, after)
			return err
		},
	)
	if err != nil {
		return result, err
	}
	logApplicationFile(ctx, "prepare_delete", workspaceID, id, fileID)
	notifications.logCreated(ctx)
	err = applicationAccess(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_application_attachment", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			before, err := requireApplicationFileEdit(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			var file attachmentModels.ApplicationDetailAttachment
			if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Take(&file).
				Error; err != nil {
				return lookupError(err)
			}
			if file.State != attachmentConstants.Deleting {
				return stale()
			}
			if err := storage.Delete(tx.Statement.Context, file.ObjectKey); err != nil {
				return err
			}
			if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Delete(&file).
				Error; err != nil {
				return err
			}
			if err := bumpApplicationForAttachment(tx, workspaceID, before); err != nil {
				return err
			}
			result, err = applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.Before, audit.After = applicationSnapshot(before), applicationSnapshot(result)
			audit.Conditions["attachment_id"] = fileID
			notifications, err = notifyApplication(tx, workspaceID, before, result)
			return err
		},
	)
	if err == nil {
		logApplicationFile(ctx, "delete", workspaceID, id, fileID)
		notifications.logCreated(ctx)
	}
	return result, err
}

// DownloadApplicationAttachment 在审计提交后返回短时私有 CDN 链接。
func DownloadApplicationAttachment(
	ctx context.Context,
	workspaceID, mode string,
	id, fileID uint64,
) (attachmentModels.URLData, error) {
	var result attachmentModels.URLData
	err := applicationAccess(
		ctx,
		workspaceID,
		applicationAction(isStudentActor(ctx), mode, ""),
		event(workspaceID, "project_application_attachment", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			if _, err := applicationDetail(tx, workspaceID, mode, actor, id); err != nil {
				return err
			}
			if !storage.Enabled() {
				return &response.BusinessError{
					Status:  http.StatusServiceUnavailable,
					Message: "附件服务暂时不可用",
				}
			}
			var file attachmentModels.ApplicationDetailAttachment
			if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, fileID).
				Take(&file).
				Error; err != nil {
				return lookupError(err)
			}
			if file.State != attachmentConstants.Ready {
				return invalid("附件尚不可下载")
			}
			result.Filename = file.Filename
			result.URL = storage.SignedCDNURL(tx.Statement.Context, file.ObjectKey)
			audit.ReturnedIDs, audit.ReturnedCount = []string{strconv.FormatUint(fileID, 10)}, 1
			return nil
		},
	)
	return result, err
}
