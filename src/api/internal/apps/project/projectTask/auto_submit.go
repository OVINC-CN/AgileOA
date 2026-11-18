package projectTask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

// ProcessAutoSubmit 执行草稿自动提交，失败由队列重试。
func ProcessAutoSubmit(ctx context.Context, task *asynq.Task) error {
	// 内部任务必须完整携带租户、项目及调度时的截止时间。
	var value projectModels.AutoSubmitPayload
	if err := json.Unmarshal(task.Payload(), &value); err != nil ||
		!workspaceUtils.ValidID(value.WorkspaceID) || value.ProjectID == 0 ||
		value.Deadline.IsZero() {
		return fmt.Errorf("invalid application auto-submit payload: %w", asynq.SkipRetry)
	}
	err := projectSvc.AutoSubmitDraftApplications(ctx, value)
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"Application auto-submit task failed; workspace: %s; project: %d; error: %v",
			value.WorkspaceID, value.ProjectID, err,
		)
	}
	return err
}
