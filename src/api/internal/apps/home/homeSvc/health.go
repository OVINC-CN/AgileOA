package homeSvc

import (
	"context"

	"go.opentelemetry.io/otel/codes"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// Health 处理进程存活检查，不公开内部状态。
func Health(ctx context.Context) error {
	// 创建服务阶段追踪，不生成业务数据。
	_, span := tracing.Tracer.Start(ctx, "homeSvc.Health")
	defer span.End()
	return nil
}

// Ready 检查当前进程是否能够提供服务。
func Ready(ctx context.Context) error {
	// 依赖检查和异常详情仅保留在服务端。
	ctx, span := tracing.Tracer.Start(ctx, "homeSvc.Ready")
	defer span.End()
	// 就绪检查仅供当前服务使用，直接在 Svc 内设置独立超时。
	checkContext, cancel := context.WithTimeout(ctx, config.Config.HTTP.HealthCheckTimeout)
	defer cancel()
	if err := db.Ping(checkContext); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "service not ready")
		return err
	}
	return nil
}
