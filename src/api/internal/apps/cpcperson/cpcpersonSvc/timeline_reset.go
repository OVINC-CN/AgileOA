package cpcpersonSvc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/storage"
)

// ResetTimelineNode 清空人员节点的填写值和独立附件，并将处理状态恢复为待处理。
func ResetTimelineNode(
	ctx context.Context, workspaceID string, id uint64, nodeID string,
	request cpcpersonModels.ResetTimelineNodeRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	var prepared map[uint64]uint64
	for {
		// 每个阶段都在空间锁内复核权限及版本，并独立持久化审计。
		changed, prepareChanged, needsCleanup := false, false, false
		var files []attachmentModels.PersonTimelineAttachment
		auditEvent := event(workspaceID, id)
		auditEvent.ObjectType = "cpc_person_timeline_node"
		auditEvent.Conditions["node_id"] = nodeID
		auditEvent.Conditions["operation"] = "reset"
		err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate, auditEvent,
			func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
				before, data, err := loadTimelineNodeForUpdate(
					tx, access, workspaceID, id, nodeID, request.Revision, request.TemplateRevision,
				)
				if err != nil {
					return err
				}
				if err := tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
					workspaceID, id, nodeID).Order("id ASC").Find(&files).Error; err != nil {
					return err
				}
				audit.Before = timelineResetSnapshot(before, files)
				if len(files) > 0 && !storage.Enabled() {
					return &response.BusinessError{
						Status: http.StatusServiceUnavailable, Message: "附件服务暂时不可用",
					}
				}

				// 先提交删除意图；对象或后续事务失败时保留待删除记录供重试。
				if len(files) > 0 && prepared == nil {
					audit.Conditions["phase"] = "prepare_delete"
					for index := range files {
						file := &files[index]
						if file.State == attachmentConstants.Deleting {
							continue
						}
						file.State = attachmentConstants.Deleting
						file.Revision++
						if err := tx.Model(file).
							Where("workspace_id = ? AND person_id = ? AND node_id = ?",
								workspaceID, id, nodeID).
							Updates(map[string]any{
								"state": file.State, "revision": file.Revision,
							}).Error; err != nil {
							return err
						}
						prepareChanged = true
					}
					audit.After = timelineResetSnapshot(before, files)
					needsCleanup = true
					return nil
				}

				// 防止两个事务之间新增、删除或重新上传附件后误清理新的内容。
				if prepared != nil {
					if len(files) != len(prepared) {
						return timelineStale()
					}
					for _, file := range files {
						if file.State != attachmentConstants.Deleting ||
							prepared[file.ID] != file.Revision {
							return timelineStale()
						}
					}
					if err := cleanupTimelineObjects(tx, workspaceID, files); err != nil {
						return err
					}
					if err := tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
						workspaceID, id, nodeID).
						Delete(&attachmentModels.PersonTimelineAttachment{}).Error; err != nil {
						return err
					}
				}

				// 填写值、节点状态、时间线异常状态及成功审计一起提交。
				result, changed, err = clearTimelineNode(
					tx,
					access,
					before,
					data,
					len(files) > 0,
					audit,
				)
				return err
			})
		if err != nil {
			return result, err
		}

		// 仅在事务提交且确有变更时记录日志，附件标识不包含对象键或签名。
		if needsCleanup {
			prepared = make(map[uint64]uint64, len(files))
			for _, file := range files {
				prepared[file.ID] = file.Revision
			}
			if prepareChanged {
				logger.Logger.WithContext(ctx).Infof(
					"CPC person timeline reset prepared; workspace: %s; person: %d; "+
						"node: %s; attachment revisions: %v",
					workspaceID, id, nodeID, prepared,
				)
			}
			continue
		}
		if changed {
			logger.Logger.WithContext(ctx).Infof(
				"CPC person timeline node reset; workspace: %s; person: %d; "+
					"node: %s; attachment revisions: %v",
				workspaceID, id, nodeID, prepared,
			)
		}
		return result, nil
	}
}

func cleanupTimelineObjects(
	tx *gorm.DB, workspaceID string, files []attachmentModels.PersonTimelineAttachment,
) error {
	// 排除本次清理的全部引用，避免重置一个节点时删除其他节点仍使用的对象。
	ids := make([]uint64, 0, len(files))
	for _, file := range files {
		ids = append(ids, file.ID)
	}
	seen := map[string]bool{}
	for _, file := range files {
		if seen[file.ObjectHash] {
			continue
		}
		seen[file.ObjectHash] = true
		shared, err := attachmentUtils.HasOtherTimelineReferences(
			tx,
			workspaceID,
			file.ObjectHash,
			ids,
		)
		if err != nil {
			return err
		}
		if !shared {
			if err := storage.Delete(tx.Statement.Context, file.ObjectKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadTimelineNodeForUpdate(
	tx *gorm.DB, access personAccess, workspaceID string, id uint64, nodeID string,
	revision, templateRevision uint64,
) (cpcpersonModels.TimelineNode, cpcpersonModels.TimelineData, error) {
	// 保持与编辑节点一致的权限、人员范围和并发版本校验。
	var before cpcpersonModels.TimelineNode
	var data cpcpersonModels.TimelineData
	if err := access.require(accessConstants.CPCPersonUpdate); err != nil {
		return before, data, err
	}
	person, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
	if err != nil {
		return before, data, err
	}
	if err := cpcpersonUtils.RequireTimelineWritable(person.TimelineStatus); err != nil {
		return before, data, err
	}
	data, err = timelineData(tx, workspaceID, person)
	if err != nil {
		return before, data, err
	}
	if data.TemplateID == nil || data.TemplateRevision != templateRevision {
		return before, data, timelineStale()
	}
	err = tx.Where(
		"workspace_id = ? AND person_id = ? AND timeline_template_id = ? AND node_id = ?",
		workspaceID, id, *data.TemplateID, nodeID,
	).Take(&before).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return before, data, invalid("人员时间线节点不存在，请刷新后重试")
	}
	if err != nil {
		return before, data, err
	}
	if before.Revision != revision {
		return before, data, timelineStale()
	}
	return before, data, nil
}

func clearTimelineNode(
	tx *gorm.DB, access personAccess, before cpcpersonModels.TimelineNode,
	data cpcpersonModels.TimelineData, hadAttachments bool, audit *auditModels.Event,
) (cpcpersonModels.TimelineData, bool, error) {
	// 已清空的待处理节点可重复重置，仍有旧字段或附件时必须完整清理。
	values := map[string]any{}
	if err := json.Unmarshal([]byte(before.FieldValues), &values); err != nil {
		return data, false, err
	}
	if before.Status == cpcpersonUtils.TimelinePending && len(values) == 0 && !hadAttachments {
		audit.After = audit.Before
		return data, false, nil
	}
	after := before
	after.FieldValues, after.Status = "{}", cpcpersonUtils.TimelinePending
	after.Revision++
	written := tx.Model(&cpcpersonModels.TimelineNode{}).
		Where("workspace_id = ? AND person_id = ? AND node_id = ? AND revision = ?",
			before.WorkspaceID, before.PersonID, before.NodeID, before.Revision).
		Updates(map[string]any{
			"field_values": after.FieldValues, "status": after.Status,
			"revision": after.Revision,
		})
	if written.Error != nil {
		return data, false, written.Error
	}
	if written.RowsAffected != 1 {
		return data, false, timelineStale()
	}
	audit.After = timelineResetSnapshot(after, nil)

	// 清空参考日期后重新计算全部依赖节点，更新名单筛选使用的持久化状态。
	definitions := make([]cpctimelineModels.Node, 0, len(data.Nodes))
	for _, node := range data.Nodes {
		definitions = append(definitions, node.Node)
	}
	if err := cpcpersonUtils.RefreshTimelineStatus(
		tx, before.WorkspaceID, accessConstants.CPCPersonUpdate,
		before.PersonID, before.TimelineTemplateID, definitions,
	); err != nil {
		return data, false, err
	}
	person, err := detail(
		tx,
		before.WorkspaceID,
		before.PersonID,
		access,
		accessConstants.CPCPersonUpdate,
	)
	if err != nil {
		return data, false, err
	}
	result, err := timelineData(tx, before.WorkspaceID, person)
	return result, true, err
}

func timelineResetSnapshot(
	node cpcpersonModels.TimelineNode, files []attachmentModels.PersonTimelineAttachment,
) auditModels.Snapshot {
	// 固定附件快照目录，硬删除后仍保留文件身份，不记录存储地址。
	result := cpcpersonUtils.TimelineSnapshot(node)
	attachments := make([]auditModels.Snapshot, 0, len(files))
	for _, file := range files {
		attachments = append(attachments, auditModels.Snapshot{
			"id": file.ID, "filename": file.Filename, "size": file.Size,
			"state": file.State, "revision": file.Revision,
		})
	}
	result["attachments"] = attachments
	return result
}
