package projectTask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/crond"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// ProcessRanking 执行重排，最终失败时将通知放入独立的可重试任务。
func ProcessRanking(ctx context.Context, task *asynq.Task) error {
	value, err := rankingPayload(task)
	if err != nil {
		return err
	}
	err = projectSvc.RankApplications(ctx, value.WorkspaceID, value.ProjectID)
	if err == nil {
		return nil
	}
	logger.Logger.WithContext(ctx).Errorf(
		"Application ranking failed; workspace: %s; project: %d; error: %v",
		value.WorkspaceID, value.ProjectID, err,
	)
	retried, _ := asynq.GetRetryCount(ctx)
	maximum, exists := asynq.GetMaxRetry(ctx)
	if !exists {
		maximum = config.Config.Project.RankingMaxRetries
	}
	if retried >= maximum {
		// 排名超时不妨碍通知入队；投递时再读取最新授权与奖项信息。
		notifyCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), config.Config.Notification.DeliveryTimeout,
		)
		defer cancel()
		if notifyErr := enqueueRankingFailure(
			notifyCtx,
			task.Payload(),
			value.SourceID,
		); notifyErr != nil {
			logger.Logger.WithContext(ctx).Errorf(
				"Application ranking failure notification enqueue failed; "+
					"workspace: %s; project: %d; error: %v",
				value.WorkspaceID, value.ProjectID, notifyErr,
			)
			return fmt.Errorf("%w: notification enqueue failed: %w", err, notifyErr)
		}
	}
	return err
}

func enqueueRankingFailure(ctx context.Context, content []byte, sourceID string) error {
	if crond.AsyncClient == nil {
		return errors.New("ranking notification queue is unavailable")
	}
	_, err := crond.AsyncClient.EnqueueContext(
		ctx,
		asynq.NewTask(projectConstants.NotifyRankingFailure, content),
		asynq.Queue(config.Config.Scheduler.Queue),
		asynq.TaskID("ranking-failure:"+sourceID),
		asynq.Timeout(config.Config.Notification.DeliveryTimeout),
		asynq.MaxRetry(config.Config.Notification.DeliveryMaxRetries),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// ProcessRankingFailure 独立重试失败通知，不重复执行排名或补发成功通知。
func ProcessRankingFailure(ctx context.Context, task *asynq.Task) error {
	value, err := rankingPayload(task)
	if err != nil {
		return err
	}
	if err := projectSvc.NotifyRankingFailure(ctx, value); err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"Application ranking failure notification failed; workspace: %s; project: %d; error: %v",
			value.WorkspaceID, value.ProjectID, err,
		)
		return err
	}
	return nil
}

func rankingPayload(task *asynq.Task) (projectModels.RankingPayload, error) {
	var value projectModels.RankingPayload
	if err := json.Unmarshal(task.Payload(), &value); err != nil ||
		!workspaceUtils.ValidID(value.WorkspaceID) || value.ProjectID == 0 ||
		!strings.HasPrefix(value.SourceID, "project-ranking:") || len(value.SourceID) > 100 {
		return value, fmt.Errorf("invalid application ranking payload: %w", asynq.SkipRetry)
	}
	return value, nil
}
