package cpcpersonTask

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationTask"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/crond"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

const RecalculateBranches = "cpc_person:recalculate_branches"

type payload struct {
	WorkspaceID        string                 `json:"workspace_id"`
	WorkspaceName      string                 `json:"workspace_name"`
	RecipientID        uint64                 `json:"recipient_id"`
	NotificationSource string                 `json:"notification_source"`
	TraceContext       propagation.MapCarrier `json:"trace_context"`
}

// Enqueue 仅供手动重排调用，任务取得同一空间锁后读取已提交规则及人员状态。
func Enqueue(
	tx *gorm.DB,
	workspaceID string,
	recipientID uint64,
	workspaceName string,
) (bool, error) {
	// 没有在编人员时不创建任务，范围与后台执行保持一致。
	var count int64
	scope := tx.Table("cpc_people").
		Where("workspace_id = ?", workspaceID).
		Where("status = ?", cpcpersonConstants.DefaultStatus)
	if err := scope.Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	if crond.AsyncClient == nil {
		return false, &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "后台重算任务暂不可用，请稍后重试",
		}
	}
	ctx := tx.Statement.Context
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	content, err := json.Marshal(
		payload{
			WorkspaceID: workspaceID, WorkspaceName: workspaceName,
			RecipientID: recipientID, NotificationSource: "cpc-person-reorder:" + rand.Text(),
			TraceContext: carrier,
		},
	)
	if err != nil {
		return false, err
	}
	_, err = crond.AsyncClient.EnqueueContext(ctx, asynq.NewTask(RecalculateBranches, content),
		asynq.Queue(config.Config.Scheduler.Queue),
		asynq.Timeout(config.Config.CPCPerson.RecalculationTimeout),
		asynq.MaxRetry(config.Config.CPCPerson.RecalculationMaxRetries),
	)
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"CPC branch recalculation enqueue failed; workspace: %s; error: %v",
			workspaceID,
			err,
		)
		return false, &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "后台重算任务暂不可用，请稍后重试",
		}
	}
	return true, nil
}

// Process 执行后台重算，结果通知接收人不作为用户审计身份。
func Process(ctx context.Context, task *asynq.Task) error {
	var value payload
	if err := json.Unmarshal(
		task.Payload(),
		&value,
	); err != nil || !workspaceUtils.ValidID(value.WorkspaceID) {
		return fmt.Errorf("invalid CPC branch recalculation payload: %w", asynq.SkipRetry)
	}
	err := cpcpersonSvc.Recalculate(
		ctx,
		value.WorkspaceID,
		value.RecipientID,
		value.NotificationSource,
	)
	if err == nil {
		return notifyResult(ctx, value, nil)
	}

	logger.Logger.WithContext(ctx).Errorf(
		"CPC branch recalculation failed; workspace: %s; error: %v",
		value.WorkspaceID,
		err,
	)
	retried, _ := asynq.GetRetryCount(ctx)
	maximum, hasMaximum := asynq.GetMaxRetry(ctx)
	if !hasMaximum {
		maximum = config.Config.CPCPerson.RecalculationMaxRetries
	}
	if finalFailure(err, retried, maximum) {
		if notifyErr := notifyResult(ctx, value, err); notifyErr != nil {
			// 通知尚未入队时不跳过重排重试，避免永久性业务错误直接丢失通知。
			return fmt.Errorf("%w: notification enqueue failed: %w", err, notifyErr)
		}
	}
	if errors.Is(err, cpcpersonSvc.ErrUnmatched) {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}

func finalFailure(err error, retried, maximum int) bool {
	return errors.Is(err, cpcpersonSvc.ErrUnmatched) || retried >= maximum
}

func notifyResult(ctx context.Context, value payload, cause error) error {
	// 兼容升级前未携带接收人的历史任务，不据此推导接收人或审计身份。
	if value.RecipientID == 0 || value.NotificationSource == "" {
		return nil
	}
	// 任务超时后仍使用有界上下文提交通知；投递与重排分别重试。
	notificationCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), config.Config.Notification.DeliveryTimeout,
	)
	defer cancel()
	err := notificationTask.Enqueue(notificationCtx, resultNotification(value, cause))
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"CPC reorder result notification enqueue failed; workspace: %s; user: %d; error: %v",
			value.WorkspaceID, value.RecipientID, err,
		)
	}
	return err
}

func resultNotification(value payload, cause error) notificationModels.Delivery {
	// 仅使用公开的中文原因，不将原始错误、人员编号或数据库细节写入通知。
	link := notificationUtils.WorkspaceURL(
		cpcpersonConstants.PeoplePagePath,
		value.WorkspaceID,
		nil,
	)
	if cause == nil {
		return notificationModels.Delivery{
			UserID: value.RecipientID, SourceID: value.NotificationSource,
			Title: "重排人员成功",
			URL:   link,
			Content: fmt.Sprintf(
				"工作空间“%s”的重排人员任务已完成，可刷新人员名单查看最新归属。",
				value.WorkspaceName,
			),
		}
	}
	reason := "后台处理未能完成，请稍后重新提交重排任务。"
	if errors.Is(cause, cpcpersonSvc.ErrUnmatched) {
		reason = "存在未匹配党支部的在编人员，已保留全部原归属，请修正准入规则后重新提交。"
	}
	return notificationModels.Delivery{
		UserID: value.RecipientID, SourceID: value.NotificationSource,
		Title:   "重排人员失败",
		Content: fmt.Sprintf("工作空间“%s”的重排人员任务失败。\n%s", value.WorkspaceName, reason),
		URL:     link,
	}
}
