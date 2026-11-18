package projectSvc

import (
	"crypto/rand"
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/crond"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func enqueueApplicationRanking(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
	enabled bool,
) error {
	// 无已通过申报时无需入队，排名始终由后台在同一空间锁内读取最新配置。
	if !enabled {
		return nil
	}
	var count int64
	if err := tx.Model(&projectModels.Application{}).
		Where("workspace_id = ? AND project_id = ? AND status = ?",
			workspaceID, projectID, projectConstants.ApplicationApproved).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	unavailable := &response.BusinessError{
		Status: http.StatusServiceUnavailable, Message: "后台排名任务暂不可用，请稍后重试",
	}
	if crond.AsyncClient == nil {
		return unavailable
	}
	ctx := tx.Statement.Context
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	content, err := json.Marshal(projectModels.RankingPayload{
		WorkspaceID: workspaceID, ProjectID: projectID,
		SourceID: "project-ranking:" + rand.Text(), TraceContext: carrier,
	})
	if err != nil {
		return err
	}

	// 入队失败阻断原事务及审计，避免审批完成或规则保存后漏排。
	_, err = crond.AsyncClient.EnqueueContext(
		ctx,
		asynq.NewTask(projectConstants.RankApplications, content),
		asynq.Queue(config.Config.Scheduler.Queue),
		asynq.Timeout(config.Config.Project.RankingTimeout),
		asynq.MaxRetry(config.Config.Project.RankingMaxRetries),
	)
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"Application ranking enqueue failed; workspace: %s; project: %d; error: %v",
			workspaceID, projectID, err,
		)
		return unavailable
	}
	return nil
}
