package notificationTask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel/propagation"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/crond"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

const DeliverNotification = "notification:deliver"

type payload struct {
	notificationModels.Delivery
	TraceContext propagation.MapCarrier `json:"trace_context"`
}

// Enqueue 将通知投递与原业务任务分开重试，重复提交同一来源不会创建重复任务。
func Enqueue(ctx context.Context, value notificationModels.Delivery) error {
	// 只接受内部业务构造的有效消息，不在请求中直接创建通知。
	if !notificationUtils.ValidDelivery(value) {
		return errors.New("invalid notification delivery")
	}
	if crond.AsyncClient == nil {
		return errors.New("notification queue is unavailable")
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	content, err := json.Marshal(payload{Delivery: value, TraceContext: carrier})
	if err != nil {
		return err
	}
	_, err = crond.AsyncClient.EnqueueContext(ctx, asynq.NewTask(DeliverNotification, content),
		asynq.Queue(config.Config.Scheduler.Queue),
		asynq.TaskID(fmt.Sprintf("notification:%d:%s", value.UserID, value.SourceID)),
		asynq.Timeout(config.Config.Notification.DeliveryTimeout),
		asynq.MaxRetry(config.Config.Notification.DeliveryMaxRetries),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// Process 持久化通知，失败由队列独立重试，不携带用户审计身份。
func Process(ctx context.Context, task *asynq.Task) error {
	// 无效载荷不重试，避免将内部错误或敏感信息拼入正文。
	var value payload
	if err := json.Unmarshal(task.Payload(), &value); err != nil ||
		!notificationUtils.ValidDelivery(value.Delivery) {
		return fmt.Errorf("invalid notification payload: %w", asynq.SkipRetry)
	}
	err := notificationSvc.Deliver(ctx, value.Delivery)
	if err != nil {
		logger.Logger.WithContext(ctx).Errorf(
			"Notification delivery failed; user: %d; source: %s; error: %v",
			value.UserID, value.SourceID, err,
		)
	}
	return err
}
