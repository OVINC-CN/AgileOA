package workspaceUtils

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"

// Info 只映射固定的公开工作空间字段，供工作空间及权限 app 复用。
func Info(workspace workspaceModels.Workspace) workspaceModels.Info {
	return workspaceModels.Info(workspace)
}
