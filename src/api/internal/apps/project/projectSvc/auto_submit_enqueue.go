package projectSvc

import (
	"encoding/json"
	"net/http"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/propagation"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/crond"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func enqueueDraftAutoSubmit(tx *gorm.DB, project projectModels.Project) error {
	// 没有截止时间时无需调度，已关闭新建的项目仍处理其绑定草稿。
	if project.SelfApplyDeadline == nil {
		return nil
	}
	unavailable := &response.BusinessError{
		Status:  http.StatusServiceUnavailable,
		Message: "自动提交任务暂不可用，请稍后重试",
	}
	if crond.AsyncClient == nil {
		return unavailable
	}
	ctx := tx.Statement.Context
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	content, err := json.Marshal(projectModels.AutoSubmitPayload{
		WorkspaceID:  project.WorkspaceID,
		ProjectID:    project.ID,
		Deadline:     *project.SelfApplyDeadline,
		TraceContext: carrier,
	})
	if err != nil {
		return err
	}

	// 每次保存独立下发；后台以最新截止时间、空间锁和草稿状态处理重复任务。
	_, err = crond.AsyncClient.EnqueueContext(
		ctx,
		asynq.NewTask(projectConstants.AutoSubmitDrafts, content),
		asynq.ProcessAt(project.SelfApplyDeadline.Add(projectConstants.AutoSubmitDelay)),
		asynq.Queue(config.Config.Scheduler.Queue),
		asynq.Timeout(config.Config.Project.AutoSubmitTimeout),
		asynq.MaxRetry(config.Config.Project.AutoSubmitMaxRetries),
	)
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"Application auto-submit enqueue failed; workspace: %s; project: %d; error: %v",
			project.WorkspaceID, project.ID, err,
		)
		return unavailable
	}
	return nil
}
