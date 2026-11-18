package crond

import (
	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

// AsyncClient 在 init 中初始化，复用全局 Redis 连接。
var AsyncClient *asynq.Client

func init() {
	// API 未配置 Redis 时无需初始化后台任务资源。
	if db.RDB() == nil {
		return
	}

	// 共享连接池由 db.Close 统一释放，不单独关闭客户端包装。
	AsyncClient = asynq.NewClientFromRedisClient(db.RDB())
}
