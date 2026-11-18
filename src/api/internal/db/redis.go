package db

import (
	"context"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.opentelemetry.io/otel/attribute"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

var rdb *redis.Client

const redisRetriesDisabled = -1

func init() {
	// 未配置地址时不启用 Redis。
	if config.Config.Redis.Addr == "" {
		return
	}

	// 初始化全局连接池，零次重试需要转换为客户端的禁用标识。
	maxRetries := config.Config.Redis.MaxRetries
	if maxRetries == 0 {
		maxRetries = redisRetriesDisabled
	}
	rdb = redis.NewClient(&redis.Options{
		ContextTimeoutEnabled: true,
		Addr:                  config.Config.Redis.Addr,
		Password:              config.Config.Redis.Password,
		DB:                    config.Config.Redis.DB,
		PoolSize:              config.Config.Redis.PoolSize,
		DialTimeout:           config.Config.Redis.DialTimeout,
		ReadTimeout:           config.Config.Redis.ReadTimeout,
		WriteTimeout:          config.Config.Redis.WriteTimeout,
		PoolTimeout:           config.Config.Redis.PoolTimeout,
		MaxRetries:            maxRetries,
		DisableIdentity:       !config.Config.Redis.IdentityEnabled,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: config.Config.Redis.MaintNotificationsMode,
		},
	})

	// 在首次访问前接入官方追踪，记录命令、流水线和连接，省略命令参数。
	if err := redisotel.InstrumentTracing(
		rdb,
		redisotel.WithTracerProvider(tracing.Provider),
		redisotel.WithDBStatement(false),
		redisotel.WithAttributes(attribute.Int("db.redis.database_index", config.Config.Redis.DB)),
	); err != nil {
		logger.Logger.Fatalf("Redis trace instrumentation failed: %v", err)
	}

	// 启动时检查连接，不让无效资源进入业务处理。
	ctx, cancel := context.WithTimeout(context.Background(), config.Config.Redis.PingTimeout)
	err := rdb.Ping(ctx).Err()
	cancel()
	if err != nil {
		if closeErr := rdb.Close(); closeErr != nil {
			logger.Logger.Errorf("Redis shutdown failed: %v", closeErr)
		}
		logger.Logger.Fatalf("Redis connection failed: %v", err)
	}
	logger.Logger.Info("Redis initialized")
}

// RDB 返回 init 阶段创建的 Redis 客户端，未启用时返回 nil。
func RDB() *redis.Client {
	// 直接使用已经初始化的共享资源。
	return rdb
}
