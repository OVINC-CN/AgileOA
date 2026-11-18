package studentdashboardRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Summary 查询当前空间各班级的学生专业人数与班长。
// @Summary 查询学生数据仪表
// @Description 独立要求 student.dashboard.list，仅汇总当前空间状态为在籍的学生，班长同样限定在籍。
// @Description 班级和专业去除首尾空白后汇总，均按名称升序排列，历史空值保留。
// @Description 第一列为班级，中间按 majors 展开各专业人数，末两列为总数和班长，提供表尾合计。
// @Description 班长来自学生班内任职，不包含副班长；姓名去重排序后以英文逗号拼接，审计保存后返回。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} studentdashboardModels.DataResponse "学生专业表"
// @Router /v1/workspaces/{workspace_id}/student-dashboard [get]
func Summary(c echo.Context) error {
	// 校验空间路径，服务层负责独立授权及读取审计。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.Summary(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}

	// 返回已完成审计的专业汇总。
	return response.Success(c, data)
}

// Fields 提供学生数据仪表只读字段说明。
// @Summary 学生数据仪表字段目录
// @Description 复用 student.dashboard.list，提供学生专业表与住宿信息表的固定字段、类型、统计口径与示例。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} studentdashboardModels.FieldsResponse "固定字段目录"
// @Router /v1/workspaces/{workspace_id}/student-dashboard/fields [get]
func Fields(c echo.Context) error {
	// 校验路径并读取固定字段目录。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.Fields(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Export 下载与仪表口径一致的完整学生专业表。
// @Summary 导出学生数据仪表
// @Description 独立要求 student.dashboard.export，仅导出当前空间在籍学生的班级、专业人数、班长及合计。
// @Description 第一列为班级，末两列为总数和班长，专业列按名称升序排列，不受页面显示字段影响。
// @Description 复用 student.max_export_rows 限制班级行数、student.max_file_bytes 限制文件大小。
// @Description 全部单元格按文本保存，空名称保留空单元格；审计完成后返回 Base64 XLSX。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} studentdashboardModels.FileResponse "XLSX 学生专业表"
// @Router /v1/workspaces/{workspace_id}/student-dashboard/export [get]
func Export(c echo.Context) error {
	// 校验路径，服务层独立复核导出权限并保存审计。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.Export(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
