package cpcpersonSvc

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// TimelineTransferOptions 先校验当前节点，再提供模板明确允许的转移目标。
func TimelineTransferOptions(
	ctx context.Context, workspaceID string, id uint64,
) (cpcpersonModels.TimelineTransferOptions, error) {
	result := cpcpersonModels.TimelineTransferOptions{Targets: []cpcpersonModels.Option{}}
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate,
		event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			_, source, err := transferSource(
				tx, access, workspaceID, id, accessConstants.CPCPersonUpdate,
			)
			if err != nil {
				return err
			}
			result.SourceTemplateID, result.SourceTemplateName = *source.TemplateID, source.TemplateName
			transfers, err := cpctimelineUtils.LoadTransfers(tx, workspaceID, *source.TemplateID)
			if err != nil {
				return err
			}
			for _, transfer := range transfers {
				target, err := timelineTemplate(tx, workspaceID, transfer.TargetTemplateID)
				if err != nil {
					return err
				}
				value := strconv.FormatUint(target.ID, 10)
				result.Targets = append(
					result.Targets,
					cpcpersonModels.Option{Value: value, Label: target.Name},
				)
				audit.ReturnedIDs = append(audit.ReturnedIDs, value)
			}
			audit.Conditions["operation"] = "transfer_options"
			audit.ReturnedCount = int64(len(result.Targets))
			return nil
		})
	return result, err
}

// PreviewTimelineTransfer 仅生成预览，读取审计持久化后才返回快照。
func PreviewTimelineTransfer(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.TimelineTransferRequest,
) (cpcpersonModels.TimelineTransferPreview, error) {
	var result cpcpersonModels.TimelineTransferPreview
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate,
		event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			plan, err := prepareTimelineTransfer(
				tx,
				access,
				workspaceID,
				id,
				request.TargetTemplateID,
				accessConstants.CPCPersonUpdate,
			)
			if err != nil {
				return err
			}
			result = plan.Preview
			audit.Conditions["operation"] = "preview_transfer"
			audit.Conditions["target_template_id"] = request.TargetTemplateID
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = int64(len(result.Timeline.Nodes))
			return nil
		})
	return result, err
}

// ReviewTimelineTransfer 仅按独立审核权限完成待审核转移，业务与审计原子提交。
func ReviewTimelineTransfer(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.TimelineTransferRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	var sourceID uint64
	var notifications []notificationModels.Notification
	err := within(ctx, workspaceID, accessConstants.CPCPersonTimelineTransferReview,
		event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 当前节点、配置、学生引用或附件在预览后变化时，必须重新预览。
			plan, err := prepareTimelineTransfer(
				tx,
				access,
				workspaceID,
				id,
				request.TargetTemplateID,
				accessConstants.CPCPersonTimelineTransferReview,
			)
			if err != nil {
				return err
			}
			if err := validateTransferPreview(plan, request); err != nil {
				return err
			}
			sourceID = *plan.Source.TemplateID
			audit.Before, err = timelineTransferSnapshot(tx, workspaceID, plan.Person)
			if err != nil {
				return err
			}
			audit.Conditions["operation"] = "transfer"
			audit.Conditions["transfer_result"] = cpcpersonConstants.TimelineTransferApproved
			audit.Conditions["source_template_id"] = sourceID
			audit.Conditions["target_template_id"] = request.TargetTemplateID
			audit.Conditions["node_mappings"] = plan.Mapping.NodeMappings

			// 仅删除数据库关联，不删除私有对象；失败时旧节点和附件全部回滚。
			review, err := removeTimelineTransferReview(tx, workspaceID, id, audit)
			if err != nil {
				return err
			}
			if err := rebuildTransferredTimeline(tx, workspaceID, plan); err != nil {
				return err
			}
			after, err := detail(
				tx, workspaceID, id, access, accessConstants.CPCPersonTimelineTransferReview,
			)
			if err != nil {
				return err
			}
			result, err = timelineData(tx, workspaceID, after)
			if err != nil {
				return err
			}
			audit.After, err = timelineTransferSnapshot(tx, workspaceID, after)
			if err != nil {
				return err
			}
			notifications, err = notifyTimelineTransferResult(
				tx, workspaceID, review, plan.Person, plan.Preview.Timeline.TemplateName,
				cpcpersonConstants.TimelineTransferApproved, "",
			)
			return err
		})
	if err == nil {
		notificationUtils.LogCreated(ctx, notifications)
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline transferred; workspace: %s; person: %d; before: %d; after: %d",
			workspaceID, id, sourceID, request.TargetTemplateID,
		)
	}
	return result, err
}

func validateTransferPreview(
	plan timelineTransferPlan, request cpcpersonModels.TimelineTransferRequest,
) error {
	if request.PreviewVersion == "" || request.PreviewVersion != plan.Preview.PreviewVersion {
		return &response.BusinessError{
			Status: http.StatusConflict, Message: "时间线或转移配置已发生变化，请重新预览后确认",
		}
	}
	return nil
}

func timelineTransferSnapshot(
	tx *gorm.DB, workspaceID string, person cpcpersonModels.Info,
) (auditModels.Snapshot, error) {
	// 使用固定快照目录保留全部原填写值和文件身份，自定义字段标识仅作为数组值。
	var records []cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id = ? AND timeline_template_id = ?",
		workspaceID, person.ID, *person.TimelineTemplateID).
		Order("node_id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	var files []attachmentModels.PersonTimelineAttachment
	if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, person.ID).
		Order("id ASC").Find(&files).Error; err != nil {
		return nil, err
	}
	byNode := make(map[string][]attachmentModels.PersonTimelineAttachment)
	for _, file := range files {
		byNode[file.NodeID] = append(byNode[file.NodeID], file)
	}
	nodes := make([]auditModels.Snapshot, 0, len(records))
	for _, record := range records {
		nodes = append(nodes, timelineResetSnapshot(record, byNode[record.NodeID]))
	}
	return auditModels.Snapshot{"person": snapshot(person), "nodes": nodes}, nil
}

func rebuildTransferredTimeline(tx *gorm.DB, workspaceID string, plan timelineTransferPlan) error {
	id := plan.Person.ID
	if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, id).
		Delete(&attachmentModels.PersonTimelineAttachment{}).Error; err != nil {
		return err
	}
	if err := tx.Where(
		"workspace_id = ? AND person_id = ? AND timeline_template_id = ?",
		workspaceID,
		id,
		*plan.Source.TemplateID,
	).Delete(&cpcpersonModels.TimelineNode{}).Error; err != nil {
		return err
	}
	written := tx.Model(&cpcpersonModels.Person{}).
		Where("workspace_id = ? AND id = ? AND revision = ? AND timeline_template_id = ?",
			workspaceID, id, plan.Person.Revision, *plan.Source.TemplateID).
		Updates(map[string]any{
			"timeline_template_id": *plan.Preview.Timeline.TemplateID,
			"timeline_status":      plan.Preview.Timeline.Status,
			"revision":             plan.Person.Revision + 1,
		})
	if written.Error != nil {
		return written.Error
	}
	if written.RowsAffected != 1 {
		return timelineStale()
	}
	for _, node := range plan.Preview.Timeline.Nodes {
		// 引用字段仅用于预览，重建时仍只保存手动字段。
		values := map[string]any{}
		for _, field := range node.Fields {
			if value, ok := node.Values[field.ID]; ok && field.StudentField == "" {
				values[field.ID] = value
			}
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return err
		}
		if err := tx.Create(&cpcpersonModels.TimelineNode{
			WorkspaceID:        workspaceID,
			PersonID:           id,
			TimelineTemplateID: *plan.Preview.Timeline.TemplateID,
			NodeID:             node.ID,
			FieldValues:        string(encoded),
			Status:             node.Status,
			Revision:           node.Revision,
		}).Error; err != nil {
			return err
		}
	}
	usedIDs := make(map[uint64]bool, len(plan.Files))
	for _, placement := range plan.Files {
		// 原附件首次使用保留编号，一对多的其余目标分配新编号，直接复用对象键。
		file := placement.Source
		if usedIDs[file.ID] {
			file.ID = 0
		}
		usedIDs[placement.Source.ID] = true
		file.NodeID = placement.NodeID
		file.Revision++
		if err := tx.Create(&file).Error; err != nil {
			return err
		}
	}
	return nil
}
