package attachmentSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// PrepareUpload 登记独立的待完成附件及审计，再返回客户端直传凭据。
func PrepareUpload(
	ctx context.Context,
	workspaceID, category string,
	ownerID uint64,
	nodeID string,
	request attachmentModels.UploadRequest,
) (attachmentModels.UploadData, error) {
	var result attachmentModels.UploadData
	filename, size := request.Filename, request.Size
	business, err := policyFor(category)
	if err != nil {
		return result, err
	}
	var record attachmentModels.Record
	err = accessUtils.Workspace(ctx, workspaceID, business.uploadAction,
		attachmentEvent(workspaceID, category, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 文件名与业务归属必须有效；不向 COS 发起存在性检查。
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				ownerID,
				nodeID,
				business.uploadAction,
			); err != nil {
				return err
			}
			if business.workCategory == "" && !storage.Enabled() {
				return unavailable()
			}
			if !validFilename(filename) || size < 1 || size > config.Config.COS.MaxUploadBytes {
				return failure(http.StatusBadRequest, "文件名、文件大小无效或超过上传限制")
			}
			if err := attachmentUtils.ValidateUploadRequest(request); err != nil {
				return err
			}
			object, err := objectName(tx, principal, category, ownerID, nodeID, filename)
			if err != nil {
				return err
			}
			if !storage.Enabled() {
				return unavailable()
			}
			if len(object) > attachmentConstants.ObjectKeyMaxBytes {
				return failure(http.StatusBadRequest, "文件名过长")
			}
			digest := sha256.Sum256([]byte(object))
			objectHash := hex.EncodeToString(digest[:])

			// 每次上传创建独立附件，待完成和待删除的记录也占用名额。
			var count int64
			scope := business.scope(tx, principal, business.uploadAction).
				Where(business.ownerColumn+" = ?", ownerID)
			if category == attachmentConstants.CategoryCPCTimelineNode ||
				category == attachmentConstants.CategoryCPCPersonTimelineNode {
				scope = scope.Where("node_id = ?", nodeID)
			}
			if err := scope.Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(business.maxAttachments) {
				return failure(http.StatusConflict, "附件数量已达上限，请先删除已有附件")
			}
			record = attachmentModels.Record{
				WorkspaceID: workspaceID, ObjectKey: object,
				ObjectHash: objectHash, Filename: filename, Size: size,
				ContentType: request.ContentType,
				UploadActor: attachmentUtils.UploadActorKey(principal.User.ID, 0),
				State:       attachmentConstants.Pending, Revision: 1,
			}
			record.ID, err = create(tx, category, ownerID, nodeID, record)
			if err != nil {
				return err
			}
			audit.ObjectID = strconv.FormatUint(record.ID, 10)
			audit.After = snapshot(category, ownerID, nodeID, record)
			result.UploadID, result.Revision = record.ID, record.Revision
			result.Upload, err = tencentcos.Credentials(
				ctx,
				record.ObjectKey,
				record.ContentType,
				size,
			)
			return err
		})
	if err != nil {
		return result, err
	}
	logChange(ctx, workspaceID, category, ownerID, record.ID, "prepare_upload")
	return result, err
}

// CompleteUpload 复核当前权限和对象元数据，幂等完成已登记的附件。
func CompleteUpload(
	ctx context.Context,
	workspaceID, category string,
	ownerID uint64,
	nodeID string,
	id, revision uint64,
) (attachmentModels.Info, error) {
	var result attachmentModels.Info
	business, err := policyFor(category)
	if err != nil {
		return result, err
	}
	var resultOwnerID uint64
	changed := false
	err = accessUtils.Workspace(ctx, workspaceID, business.uploadAction,
		attachmentEvent(workspaceID, category, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			pending, err := load(business.scope(tx, principal, business.uploadAction), business, id)
			if err != nil {
				return err
			}
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				pending.OwnerID,
				pending.NodeID,
				business.uploadAction,
			); err != nil {
				return err
			}
			if pending.OwnerID != ownerID || pending.NodeID != nodeID ||
				pending.UploadActor != attachmentUtils.UploadActorKey(principal.User.ID, 0) {
				return failure(http.StatusNotFound, "附件不存在或不可访问")
			}
			if pending.State == attachmentConstants.Ready {
				result = info(category, pending)
				return nil
			}
			if pending.State != attachmentConstants.Pending || pending.Revision != revision {
				return failure(http.StatusConflict, "附件已更新，请刷新后重试")
			}
			// 只核对 COS 元数据；上传正文由客户端直接发送到对象存储。
			if err := attachmentUtils.VerifyUploadedObject(ctx, pending.Record); err != nil {
				return err
			}
			resultOwnerID = pending.OwnerID
			audit.Before = snapshot(category, pending.OwnerID, pending.NodeID, pending.Record)
			pending.State = attachmentConstants.Ready
			pending.Revision++
			if err := business.scope(tx, principal, business.uploadAction).
				Where("id = ?", pending.ID).
				Updates(map[string]any{"state": pending.State, "revision": pending.Revision}).
				Error; err != nil {
				return err
			}
			pending, err = load(
				business.scope(tx, principal, business.uploadAction),
				business,
				pending.ID,
			)
			if err != nil {
				return err
			}
			audit.After = snapshot(category, pending.OwnerID, pending.NodeID, pending.Record)
			result = info(category, pending)
			changed = true
			return nil
		})
	if err == nil && changed {
		logChange(ctx, workspaceID, category, resultOwnerID, id, "complete_upload")
	}
	return result, err
}

// List 按固定业务类别及归属查询附件，不签发链接或泄露对象路径。
func List(
	ctx context.Context,
	workspaceID, category string,
	ownerID uint64,
	nodeID string,
	page pagination.Query,
) (attachmentModels.ListData, error) {
	result := attachmentModels.ListData{
		Items:    []attachmentModels.Info{},
		Page:     page.Page,
		PageSize: page.PageSize,
	}
	business, err := policyFor(category)
	if err != nil {
		return result, err
	}
	err = accessUtils.Workspace(ctx, workspaceID, business.readAction,
		attachmentEvent(workspaceID, category, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				ownerID,
				nodeID,
				business.readAction,
			); err != nil {
				return err
			}
			scope := business.scope(tx, principal, business.readAction)
			if category == attachmentConstants.CategoryCPCTimelineNode {
				if err := requireTimelineNode(tx, workspaceID, ownerID, nodeID); err != nil {
					return err
				}
				scope = scope.Where("node_id = ?", nodeID)
			}
			if category == attachmentConstants.CategoryCPCPersonTimelineNode {
				scope = scope.Where("node_id = ?", nodeID)
			}
			if ownerID != 0 {
				scope = scope.Where(business.ownerColumn+" = ?", ownerID)
			}
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			var records []attachmentModels.OwnedRecord
			if err := scope.Select(business.ownerSelection()).Order("id DESC").
				Offset((page.Page - 1) * page.PageSize).
				Limit(page.PageSize).
				Find(&records).
				Error; err != nil {
				return err
			}
			for _, record := range records {
				result.Items = append(result.Items, info(category, record))
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(record.ID, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// Download 按固定类别授权，仅对已完成文件在审计提交后提供临时下载地址。
func Download(
	ctx context.Context,
	workspaceID, category string,
	id uint64,
) (attachmentModels.URLData, error) {
	var result attachmentModels.URLData
	business, err := policyFor(category)
	if err != nil {
		return result, err
	}
	err = accessUtils.Workspace(ctx, workspaceID, business.readAction,
		attachmentEvent(workspaceID, category, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			if business.workCategory == "" && !storage.Enabled() {
				return unavailable()
			}
			record, err := load(business.scope(tx, principal, business.readAction), business, id)
			if err != nil {
				return err
			}
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				record.OwnerID,
				record.NodeID,
				business.readAction,
			); err != nil {
				return err
			}
			if !storage.Enabled() {
				return unavailable()
			}
			if record.State != attachmentConstants.Ready {
				return failure(http.StatusConflict, "附件暂时无法下载，请稍后重试")
			}
			result.Filename = record.Filename
			result.URL = storage.SignedCDNURL(tx.Statement.Context, record.ObjectKey)
			audit.ReturnedIDs, audit.ReturnedCount = []string{strconv.FormatUint(id, 10)}, 1
			return nil
		})
	return result, err
}

// Delete 先提交文件的删除意图及审计，再删除对象并硬删除记录；失败可显式重试。
func Delete(ctx context.Context, workspaceID, category string, id, revision uint64) error {
	business, err := policyFor(category)
	if err != nil {
		return err
	}
	var ownerID uint64
	err = accessUtils.Workspace(ctx, workspaceID, business.deleteAction,
		attachmentEvent(workspaceID, category, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			if business.workCategory == "" && !storage.Enabled() {
				return unavailable()
			}
			record, err := load(business.scope(tx, principal, business.deleteAction), business, id)
			if err != nil {
				return err
			}
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				record.OwnerID,
				record.NodeID,
				business.deleteAction,
			); err != nil {
				return err
			}
			if !storage.Enabled() {
				return unavailable()
			}
			if revision != record.Revision {
				return failure(http.StatusConflict, "文件记录已改变，请刷新后重试")
			}
			ownerID = record.OwnerID
			audit.Before = snapshot(category, record.OwnerID, record.NodeID, record.Record)
			record.State = attachmentConstants.Deleting
			record.Revision++
			if err := business.scope(tx, principal, business.deleteAction).Where("id = ?", id).
				Updates(map[string]any{
					"state":    record.State,
					"revision": record.Revision,
				}).
				Error; err != nil {
				return err
			}
			audit.After = snapshot(category, record.OwnerID, record.NodeID, record.Record)
			return nil
		})
	if err != nil {
		return err
	}
	logChange(ctx, workspaceID, category, ownerID, id, "prepare_delete")
	err = accessUtils.Workspace(ctx, workspaceID, business.deleteAction,
		attachmentEvent(workspaceID, category, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			record, err := load(business.scope(tx, principal, business.deleteAction), business, id)
			if err != nil {
				return err
			}
			if err := requirePersonTimeline(
				tx,
				principal,
				category,
				record.OwnerID,
				record.NodeID,
				business.deleteAction,
			); err != nil {
				return err
			}
			if record.State != attachmentConstants.Deleting {
				return failure(http.StatusConflict, "附件已更新，请刷新后重试")
			}
			// 转移产生的多个节点引用共用私有对象，仅最后一个引用负责删除对象。
			shared := false
			if category == attachmentConstants.CategoryCPCPersonTimelineNode {
				shared, err = attachmentUtils.HasOtherTimelineReferences(
					tx, workspaceID, record.ObjectHash, []uint64{id},
				)
				if err != nil {
					return err
				}
			}
			if !shared {
				if err := storage.Delete(tx.Statement.Context, record.ObjectKey); err != nil {
					return err
				}
			}
			audit.Before = snapshot(category, record.OwnerID, record.NodeID, record.Record)
			return business.scope(tx, principal, business.deleteAction).Where("id = ?", id).
				Delete(&attachmentModels.Record{}).
				Error
		})
	if err == nil {
		logChange(ctx, workspaceID, category, ownerID, id, "delete")
	}
	return err
}

func logChange(
	ctx context.Context,
	workspaceID, category string,
	ownerID, id uint64,
	operation string,
) {
	// 仅在对应事务提交后记录变更，不包含文件内容、对象键或签名。
	logger.Logger.WithContext(ctx).Infof(
		"Attachment changed; operation: %s; workspace: %s; category: %s; owner: %d; attachment: %d",
		operation, workspaceID, category, ownerID, id,
	)
}

func load(
	scope *gorm.DB,
	business attachmentPolicy,
	id uint64,
) (attachmentModels.OwnedRecord, error) {
	var record attachmentModels.OwnedRecord
	err := scope.Select(business.ownerSelection()).
		Where("id = ?", id).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = failure(http.StatusNotFound, "附件不存在或不可访问")
	}
	return record, err
}

func info(category string, record attachmentModels.OwnedRecord) attachmentModels.Info {
	result := attachmentModels.Info{
		ID:        record.ID,
		Category:  category,
		Filename:  record.Filename,
		Size:      record.Size,
		State:     record.State,
		Revision:  record.Revision,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
	switch category {
	case attachmentConstants.CategoryProject:
		result.ProjectID = record.OwnerID
	case attachmentConstants.CategoryCPCPersonTimelineNode:
		result.PersonID, result.NodeID = record.OwnerID, record.NodeID
	case attachmentConstants.CategoryCPCTimelineNode:
		result.TimelineTemplateID = record.OwnerID
		result.NodeID = record.NodeID
	default:
		result.MonthlyWorkID = record.OwnerID
	}
	return result
}

func snapshot(
	category string,
	ownerID uint64,
	nodeID string,
	record attachmentModels.Record,
) auditModels.Snapshot {
	result := auditModels.Snapshot{
		"id":              record.ID,
		"project_id":      nil,
		"monthly_work_id": nil,
		"filename":        record.Filename,
		"category":        category,
		"size":            record.Size,
		"content_type":    record.ContentType,
		"state":           record.State,
		"revision":        record.Revision,
	}
	switch category {
	case attachmentConstants.CategoryProject:
		result["project_id"] = ownerID
	case attachmentConstants.CategoryCPCPersonTimelineNode:
		result["person_id"], result["node_id"] = ownerID, nodeID
	case attachmentConstants.CategoryCPCTimelineNode:
		result["timeline_template_id"] = ownerID
		result["node_id"] = nodeID
	default:
		result["monthly_work_id"] = ownerID
	}
	return result
}
func attachmentEvent(workspaceID, category string, id uint64) auditModels.Event {
	result := auditModels.Event{TenantID: &workspaceID, ObjectType: "attachment",
		Conditions: auditModels.Snapshot{"category": category}}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}
func validFilename(value string) bool {
	return attachmentUtils.ValidFilename(value)
}

func failure(status int, message string) error {
	return &response.BusinessError{Status: status, Message: message}
}
func unavailable() error {
	return failure(http.StatusServiceUnavailable, "附件服务暂时不可用")
}
