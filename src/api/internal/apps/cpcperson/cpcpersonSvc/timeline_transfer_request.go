package cpcpersonSvc

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

// TransferTimeline 发起转移并锁定人员时间线，审核前保留全部原节点和附件。
func TransferTimeline(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.TimelineTransferRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	var notifications []notificationModels.Notification
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate,
		event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 发起时复核来源时间线、映射及预览版本，目标时间异常仅提示。
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
			if err := validateTransferPreview(plan, request); err != nil {
				return err
			}
			audit.Before = snapshot(plan.Person)
			audit.Conditions["operation"] = "request_transfer"
			audit.Conditions["target_template_id"] = request.TargetTemplateID
			// 审核申请独立保存，同空间每人仅能存在一条待审核记录。
			review := cpcpersonModels.TimelineTransferReview{
				WorkspaceID:      workspaceID,
				PersonID:         id,
				RequestedByID:    auditUtils.Actor(tx.Statement.Context).ID,
				SourceTemplateID: *plan.Source.TemplateID,
				TargetTemplateID: request.TargetTemplateID,
			}
			if err := tx.Create(&review).Error; err != nil {
				return err
			}
			audit.Conditions["transfer_review_id"] = review.ID
			written := tx.Model(&cpcpersonModels.Person{}).
				Where("workspace_id = ? AND id = ? AND revision = ?",
					workspaceID, id, plan.Person.Revision).
				Updates(map[string]any{
					"timeline_status": cpcpersonUtils.TimelineTransferring,
					"revision":        plan.Person.Revision + 1,
				})
			if written.Error != nil {
				return written.Error
			}
			if written.RowsAffected != 1 {
				return timelineStale()
			}
			after, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			audit.After = snapshot(after)
			result, err = timelineData(tx, workspaceID, after)
			if err != nil {
				return err
			}

			// 审批通知与申请及成功审计共用事务，任一步失败全部回滚。
			notifications, err = notifyTimelineTransferReview(tx, workspaceID, review, plan.Person,
				plan.Source.TemplateName, plan.Preview.Timeline.TemplateName)
			return err
		})
	if err == nil {
		notificationUtils.LogCreated(ctx, notifications)
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline transfer requested; workspace: %s; person: %d; target template: %d",
			workspaceID, id, request.TargetTemplateID,
		)
	}
	return result, err
}

// PreviewTimelineTransferReview 按待审核的固定目标生成最新预览。
func PreviewTimelineTransferReview(
	ctx context.Context, workspaceID string, id uint64,
) (cpcpersonModels.TimelineTransferPreview, error) {
	var result cpcpersonModels.TimelineTransferPreview
	action := accessConstants.CPCPersonTimelineTransferReview
	err := within(ctx, workspaceID, action, event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 审核只能使用已发起的目标，不能借此直接发起新的转移。
			person, err := detail(tx, workspaceID, id, access, action)
			if err != nil {
				return err
			}
			if person.TimelineTransferTargetID == nil {
				return invalid("当前人员没有待审核的时间线转移")
			}
			plan, err := prepareTimelineTransfer(
				tx, access, workspaceID, id, *person.TimelineTransferTargetID, action,
			)
			if err != nil {
				return err
			}
			result = plan.Preview
			audit.Conditions["operation"] = "preview_transfer_review"
			audit.Conditions["target_template_id"] = *person.TimelineTransferTargetID
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = int64(len(result.Timeline.Nodes))
			return nil
		})
	return result, err
}

// CancelTimelineTransfer 按人员编辑权限取消待审核转移，恢复实时计算的时间线状态。
func CancelTimelineTransfer(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.TimelineTransferRevisionRequest,
) (cpcpersonModels.TimelineData, error) {
	return closeTimelineTransferRequest(
		ctx, workspaceID, id, request.Revision, accessConstants.CPCPersonUpdate, "",
	)
}

// RejectTimelineTransfer 按独立审核权限驳回申请，保留原时间线并通知提交人。
func RejectTimelineTransfer(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.TimelineTransferRejectRequest,
) (cpcpersonModels.TimelineData, error) {
	return closeTimelineTransferRequest(
		ctx, workspaceID, id, request.Revision,
		accessConstants.CPCPersonTimelineTransferReview, request.Reason,
	)
}

func closeTimelineTransferRequest(
	ctx context.Context, workspaceID string, id uint64,
	revision uint64, action, reason string,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	var notifications []notificationModels.Notification
	operation := "cancel_transfer"
	if action == accessConstants.CPCPersonTimelineTransferReview {
		operation = "reject_transfer"
	}
	err := within(ctx, workspaceID, action,
		event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(action); err != nil {
				return err
			}
			// 审批意见仅用于驳回结果通知，不增加人员或审核申请字段。
			if action == accessConstants.CPCPersonTimelineTransferReview {
				reason = strings.TrimSpace(reason)
				if reason == "" {
					return invalid("请填写审批意见")
				}
				if utf8.RuneCountInString(
					reason,
				) > cpcpersonConstants.TimelineTransferReasonLength {
					return invalid("审批意见不能超过 1000 个字符")
				}
			}
			before, err := detail(tx, workspaceID, id, access, action)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			if before.TimelineTransferTargetID == nil {
				return invalid("当前人员没有待处理的时间线转移")
			}
			data, err := timelineData(tx, workspaceID, before)
			if err != nil {
				return err
			}

			// 不依赖转移映射，配置或学生引用变化后仍可取消并解除锁定。
			audit.Before = snapshot(before)
			audit.Conditions["operation"] = operation
			review, err := removeTimelineTransferReview(tx, workspaceID, id, audit)
			if err != nil {
				return err
			}
			written := tx.Model(&cpcpersonModels.Person{}).
				Where("workspace_id = ? AND id = ? AND revision = ?",
					workspaceID, id, revision).
				Updates(map[string]any{
					"timeline_status": timelineActualStatus(data),
					"revision":        before.Revision + 1,
				})
			if written.Error != nil {
				return written.Error
			}
			if written.RowsAffected != 1 {
				return timelineStale()
			}
			after, err := detail(tx, workspaceID, id, access, action)
			if err != nil {
				return err
			}
			audit.After = snapshot(after)
			result, err = timelineData(tx, workspaceID, after)
			if err != nil {
				return err
			}
			if action == accessConstants.CPCPersonTimelineTransferReview {
				audit.Conditions["transfer_result"] = cpcpersonConstants.TimelineTransferRejected
				notifications, err = notifyTimelineTransferResult(
					tx, workspaceID, review, before, data.TransferTargetName,
					cpcpersonConstants.TimelineTransferRejected, reason,
				)
			}
			return err
		})
	if err == nil {
		notificationUtils.LogCreated(ctx, notifications)
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline transfer request closed; operation: %s; workspace: %s; person: %d",
			operation, workspaceID, id,
		)
	}
	return result, err
}

func removeTimelineTransferReview(
	tx *gorm.DB, workspaceID string, personID uint64, audit *auditModels.Event,
) (cpcpersonModels.TimelineTransferReview, error) {
	// 在更新人员模板前显式移除审核申请，保护型外键不允许自动解除绑定。
	var review cpcpersonModels.TimelineTransferReview
	if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, personID).
		Take(&review).Error; err != nil {
		return review, err
	}
	audit.Conditions["transfer_review_id"] = review.ID
	audit.Conditions["source_template_id"] = review.SourceTemplateID
	audit.Conditions["target_template_id"] = review.TargetTemplateID
	audit.Conditions["requested_by_id"] = review.RequestedByID
	err := tx.Where("workspace_id = ? AND id = ?", workspaceID, review.ID).
		Delete(&cpcpersonModels.TimelineTransferReview{}).Error
	return review, err
}
