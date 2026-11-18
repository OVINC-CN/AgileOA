package projectSvc

import (
	"context"
	"net/http"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/storage"
)

// DeleteApplication 分阶段保存附件删除意图及审计，清理对象后硬删除填报及其所属记录。
func DeleteApplication(ctx context.Context, workspaceID string, id, revision uint64) error {
	var prepared map[uint64]uint64
	for {
		needsCleanup, prepareChanged := false, false
		var files []attachmentModels.ApplicationDetailAttachment
		var detailCount, historyCount int
		err := applicationAccess(ctx, workspaceID, accessConstants.ProjectApplicationDelete,
			event(workspaceID, "project_application", id),
			func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
				// 每阶段实时复核租户范围及版本，删除不依赖当前审批节点资格。
				before, err := applicationDetail(
					tx, workspaceID, projectConstants.ApplicationManage, actor, id,
				)
				if err != nil {
					return err
				}
				if revision == 0 || before.Revision != revision {
					return stale()
				}
				if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id).
					Order("id ASC").Find(&files).Error; err != nil {
					return err
				}
				if err := requireApplicationDeletionFiles(files); err != nil {
					return err
				}
				audit.Before, err = applicationDeletionSnapshot(tx, workspaceID, before)
				if err != nil {
					return err
				}

				// 上传同样持有空间锁，先提交删除意图可阻止未完成上传继续写入对象。
				if len(files) > 0 && prepared == nil {
					audit.Conditions["phase"] = "prepare_delete"
					for index := range files {
						file := &files[index]
						if file.State == attachmentConstants.Deleting {
							continue
						}
						file.State, file.Revision = attachmentConstants.Deleting, file.Revision+1
						if err := tx.Model(file).
							Where("workspace_id = ? AND application_id = ?", workspaceID, id).
							Updates(map[string]any{
								"state": file.State, "revision": file.Revision,
							}).Error; err != nil {
							return err
						}
						prepareChanged = true
					}
					after, err := applicationDetail(
						tx, workspaceID, projectConstants.ApplicationManage, actor, id,
					)
					if err != nil {
						return err
					}
					audit.After, err = applicationDeletionSnapshot(tx, workspaceID, after)
					needsCleanup = true
					return err
				}

				// 附件集合或版本变化时拒绝清理，避免两个事务之间误删新内容。
				if prepared != nil {
					if !applicationDeletionFilesMatch(files, prepared) {
						return stale()
					}
					for _, file := range files {
						if err := storage.Delete(tx.Statement.Context, file.ObjectKey); err != nil {
							return err
						}
					}
				}

				// 删除与完整快照审计同事务，外键级联清理评分、流程、签署、历史及附件。
				detailCount, historyCount = len(before.Details), len(before.History)
				result := tx.Where("workspace_id = ? AND id = ? AND revision = ?",
					workspaceID, id, revision).Delete(&projectModels.Application{})
				if result.Error != nil {
					return writeError(result.Error)
				}
				if result.RowsAffected != 1 {
					return stale()
				}
				// 已通过填报删除后，后台同步收紧其余填报的排名。
				if before.Status == projectConstants.ApplicationApproved {
					return enqueueApplicationRanking(
						tx, workspaceID, before.ProjectID, before.ScoreCalculationEnabled,
					)
				}
				return nil
			})
		if err != nil {
			logger.Logger.WithContext(ctx).Errorf(
				"Application deletion failed; action: %s; workspace: %s; application: %d",
				accessConstants.ProjectApplicationDelete, workspaceID, id,
			)
			return err
		}

		// 仅在实际变更与审计提交后记录成功日志，不输出私有对象地址。
		if needsCleanup {
			prepared = make(map[uint64]uint64, len(files))
			for _, file := range files {
				prepared[file.ID] = file.Revision
			}
			if prepareChanged {
				logger.Logger.WithContext(ctx).Infof(
					"Application deletion prepared; workspace: %s; application: %d; attachments: %d",
					workspaceID, id, len(files),
				)
			}
			continue
		}
		logger.Logger.WithContext(ctx).Infof(
			"Application deleted; action: %s; workspace: %s; application: %d; "+
				"details: %d; history: %d; attachments: %d",
			accessConstants.ProjectApplicationDelete, workspaceID, id,
			detailCount, historyCount, len(files),
		)
		return nil
	}
}

func requireApplicationDeletionFiles(files []attachmentModels.ApplicationDetailAttachment) error {
	// 未完成及待删除附件也要清理，无附件时不依赖对象存储。
	if len(files) > 0 && !storage.Enabled() {
		return &response.BusinessError{
			Status: http.StatusServiceUnavailable, Message: "附件服务暂时不可用",
		}
	}
	return nil
}

func applicationDeletionFilesMatch(
	files []attachmentModels.ApplicationDetailAttachment,
	prepared map[uint64]uint64,
) bool {
	// 同时匹配数量、编号、版本及状态，任何中途变更均要求重新确认。
	if len(files) != len(prepared) {
		return false
	}
	for _, file := range files {
		if file.State != attachmentConstants.Deleting || prepared[file.ID] != file.Revision {
			return false
		}
	}
	return true
}

func applicationDeletionSnapshot(
	tx *gorm.DB,
	workspaceID string,
	info projectModels.ApplicationInfo,
) (auditModels.Snapshot, error) {
	// 原申报快照包含评分、附件、历史及签署，补充申报人节点和全部原始流程引用。
	var nodes []projectModels.ApplicationNode
	if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, info.ID).
		Order("position ASC, target_order ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	snapshot := applicationSnapshot(info)
	flow := make([]auditModels.Snapshot, len(nodes))
	for index, node := range nodes {
		flow[index] = auditModels.Snapshot{
			"position": node.Position, "target_order": node.TargetOrder,
			"kind": node.Kind, "sign_mode": node.SignMode,
			"class_position": node.ClassPosition, "student_id": node.StudentID,
			"user_id": node.UserID, "reviewer_label": node.ReviewerLabel,
			"created_at": node.CreatedAt, "updated_at": node.UpdatedAt,
		}
	}
	snapshot["approval_nodes"] = flow
	return snapshot, nil
}
