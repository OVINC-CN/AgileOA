package cpcdashboardRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcdashboard/cpcdashboardSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Summary 查询各党支部在编党员数量和姓名汇总。
// @Summary 查询党建数据仪表
// @Description 独立要求 cpc.dashboard.list，汇总当前空间全部党支部的在编人员。
// @Description 正式和预备党员分别计数，书记及委员从人员党内职务读取，导师从关联学生读取。
// @Description 姓名去重后以英文逗号拼接，保留空支部并提供人数合计，审计保存后返回。
// @Tags 党建数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpcdashboardModels.DataResponse "支部党员表"
// @Router /v1/workspaces/{workspace_id}/cpc-dashboard [get]
func Summary(c echo.Context) error {
	// 校验空间路径，业务鉴权及审计由服务层处理。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpcdashboardSvc.Summary(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}

	// 返回已完成审计的汇总数据。
	return response.Success(c, data)
}

// Fields 查询数据仪表固定字段目录。
// @Summary 党建数据仪表字段目录
// @Description 复用 cpc.dashboard.list，按页面及 XLSX 列顺序提供中文名称、类型、统计口径及示例，全部字段只读。
// @Tags 党建数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpcdashboardModels.FieldsResponse "固定字段目录"
// @Router /v1/workspaces/{workspace_id}/cpc-dashboard/fields [get]
func Fields(c echo.Context) error {
	// 校验路径并读取固定字段目录。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpcdashboardSvc.Fields(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Export 下载与仪表口径一致的全部支部汇总表。
// @Summary 下载党建数据仪表
// @Description 独立要求 cpc.dashboard.export，下载当前空间全部支部的在编人员汇总及人数合计。
// @Description 最后一列为英文逗号拼接的去重导师姓名，全部单元格以文本保存。
// @Description 审计完成后返回 Base64 XLSX。
// @Tags 党建数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpcdashboardModels.FileResponse "XLSX 汇总表"
// @Router /v1/workspaces/{workspace_id}/cpc-dashboard/export [get]
func Export(c echo.Context) error {
	// 校验路径，下载在服务层独立复核权限并保存审计。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpcdashboardSvc.Export(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
