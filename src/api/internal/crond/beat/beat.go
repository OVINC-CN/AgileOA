package beat

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

var scheduler *asynq.Scheduler

func init() {
	// 未启用 Redis 时无需创建调度器。
	if db.RDB() == nil {
		return
	}

	// 初始化调度器，时区和心跳周期均由 YAML 配置。
	scheduler = asynq.NewSchedulerFromRedisClient(db.RDB(), &asynq.SchedulerOpts{
		Logger:            logger.Logger,
		Location:          config.Config.Scheduler.Location,
		HeartbeatInterval: config.Config.Scheduler.HeartbeatInterval,
	})
}

// Serve 启动周期调度，同一队列只运行一个实例。
func Serve(ctx context.Context) error {
	// 直接使用初始化好的调度资源。
	if scheduler == nil {
		return errors.New("redis.addr is required for the scheduler")
	}
	if err := scheduler.Start(); err != nil {
		return fmt.Errorf("scheduler startup failed: %w", err)
	}
	defer scheduler.Shutdown()
	logger.Logger.WithContext(ctx).
		Infof("Scheduler started; queue: %s", config.Config.Scheduler.Queue)

	// 收到取消信号后停止入队，再由命令层清理连接。
	<-ctx.Done()
	return nil
}
