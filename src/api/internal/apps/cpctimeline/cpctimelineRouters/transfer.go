package cpctimelineRouters

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// CreateTransferTargets 按模板新增权限读取转移目标节点目录。
// @Summary 新增模板的转移目标目录
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词"
// @Param id query int false "精确查询已选目标模板编号"
// @Success 200 {object} cpctimelineModels.TransferTargetsResponse
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/transfer-targets/create [get]
func CreateTransferTargets(c echo.Context) error {
	return transferTargets(c, accessConstants.CPCTimelineCreate)
}

// UpdateTransferTargets 按模板修改权限读取转移目标节点目录。
// @Summary 修改模板的转移目标目录
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词"
// @Param id query int false "精确查询已选目标模板编号"
// @Success 200 {object} cpctimelineModels.TransferTargetsResponse
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/transfer-targets/update [get]
func UpdateTransferTargets(c echo.Context) error {
	return transferTargets(c, accessConstants.CPCTimelineUpdate)
}

func transferTargets(c echo.Context, action string) error {
	var id uint64
	if raw := c.QueryParam("id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			return echo.ErrBadRequest
		}
		id = parsed
	}
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpctimelineSvc.TransferTargets(
		c.Request().Context(), workspaceID, action,
		c.QueryParam("keyword"), id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
