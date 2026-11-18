package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonTask"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationTask"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectTask"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/crond"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

var server *asynq.Server
var taskMux *asynq.ServeMux

func init() {
	// 未启用 Redis 时保留空资源，由对应启动命令报告错误。
	if db.RDB() == nil {
		return
	}

	// 初始化工作进程，直接使用全局配置和已有连接池。
	server = asynq.NewServerFromRedisClient(db.RDB(), asynq.Config{
		Logger:            logger.Logger,
		Concurrency:       config.Config.Scheduler.Concurrency,
		Queues:            map[string]int{config.Config.Scheduler.Queue: crond.QueuePriority},
		TaskCheckInterval: config.Config.Scheduler.TaskCheckInterval,
		ShutdownTimeout:   config.Config.App.ShutdownTimeout,
	})

	// 注册党建人员归属重算任务，业务执行时继续实时读取规则并持有空间锁。
	taskMux = asynq.NewServeMux()
	taskMux.Use(traceTask, logTask)
	taskMux.HandleFunc(cpcpersonTask.RecalculateBranches, cpcpersonTask.Process)
	taskMux.HandleFunc(notificationTask.DeliverNotification, notificationTask.Process)
	taskMux.HandleFunc(projectConstants.AutoSubmitDrafts, projectTask.ProcessAutoSubmit)
	taskMux.HandleFunc(projectConstants.RankApplications, projectTask.ProcessRanking)
	taskMux.HandleFunc(projectConstants.NotifyRankingFailure, projectTask.ProcessRankingFailure)
}

// logTask 统一记录每次异步任务执行的开始与结束。
func logTask(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		// 结束日志覆盖正常返回、错误返回和异常退出。
		logger.Logger.WithContext(ctx).Infof("%s start", task.Type())
		defer logger.Logger.WithContext(ctx).Infof("%s end", task.Type())

		// 保留处理函数的返回结果，继续使用队列原有重试规则。
		return next.ProcessTask(ctx, task)
	})
}

// Serve 启动工作进程，并在取消后等待任务结束。
func Serve(ctx context.Context) error {
	// 使用 init 已准备的资源，不再创建客户端或透传配置。
	if server == nil {
		return errors.New("redis.addr is required for the worker")
	}
	if err := server.Start(taskMux); err != nil {
		return fmt.Errorf("worker startup failed: %w", err)
	}
	defer server.Shutdown()
	logger.Logger.WithContext(ctx).Infof("Worker started; queue: %s", config.Config.Scheduler.Queue)

	// 退出由命令层统一释放共享 Redis 连接。
	<-ctx.Done()
	return nil
}
