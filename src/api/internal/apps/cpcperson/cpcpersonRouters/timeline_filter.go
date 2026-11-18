package cpcpersonRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonSvc"
)

// TimelineFilterFields 查询可读人员绑定模板的字段目录。
// @Summary 党建人员时间线筛选字段
// @Description 复用人员列表权限和读取范围，仅返回范围内人员绑定模板的字段定义，不返回填写值。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板、节点或字段名称关键词，最多 200 字符"
// @Success 200 {object} cpcpersonModels.TimelineFilterFieldsResponse "可筛选的字段目录"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-fields [get]
func TimelineFilterFields(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineFilterFields(
			c.Request().Context(),
			workspaceID,
			c.QueryParam("keyword"),
		)
	})
}
