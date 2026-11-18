package storage

import (
	"context"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentcos"
)

// Enabled 表示私有 COS 已通过 YAML 配置。
func Enabled() bool { return tencentcos.Enabled() }

// Copy 供模板克隆在同一私有桶内复制对象。
func Copy(ctx context.Context, source, destination string) error {
	return tencentcos.Copy(ctx, source, destination)
}

// Delete 幂等删除已审计的私有对象。
func Delete(ctx context.Context, object string) error {
	return tencentcos.Delete(ctx, object)
}

// Close 在所有请求退出后释放 SDK 的空闲连接。
func Close() { tencentcos.Close() }
