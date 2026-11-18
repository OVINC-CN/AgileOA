package studentdashboardRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Accommodation 查询在籍学生的住宿信息汇总。
// @Summary 查询学生住宿信息表
// @Description 要求 student.dashboard.list，仅统计当前空间在籍学生，按住宿类型、园区、宿舍楼和宿舍号汇总人数。
// @Description 非校内住宿先将园区、楼栋和宿舍号归为空值，再筛选和聚合，每种非校内类型最多一行。
// @Description 同一字段多选取并集，不同字段取交集；省略条件表示不限，位置显式空值表示匹配归并后的空位置。
// @Description 返回全部分组及人数合计，读取审计提交后返回。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param accommodation_type query []string false "住宿类型英文代码，支持多选" Enums(on_campus,off_campus,joint_abroad,unknown) collectionFormat(multi)
// @Param campus query []string false "住宿园区，支持多选；显式空值匹配空位置" collectionFormat(multi)
// @Param building query []string false "宿舍楼，支持多选；显式空值匹配空位置" collectionFormat(multi)
// @Param room query []string false "宿舍号，支持多选，按文本保留前导零；显式空值匹配空位置" collectionFormat(multi)
// @Success 200 {object} studentdashboardModels.AccommodationResponse "住宿分组及人数合计"
// @Router /v1/workspaces/{workspace_id}/student-dashboard/accommodation [get]
func Accommodation(c echo.Context) error {
	// 提取空间及固定筛选字段，参数校验和读取审计由服务层完成。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.Accommodation(
		c.Request().Context(),
		workspaceID,
		accommodationFilters(c),
	)
	if err != nil {
		return err
	}

	// 返回完成审计的住宿统计。
	return response.Success(c, data)
}

// AccommodationOptions 查询当前空间在籍学生的住宿筛选选项。
// @Summary 学生住宿信息表筛选选项
// @Description 要求 student.dashboard.list，住宿类型返回英文 value 和中文 label，位置空值显示为半角短横线。
// @Description 园区、楼栋和宿舍号仅从在籍校内住宿学生读取，选项上限复用 student.max_options。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} studentdashboardModels.AccommodationOptionsResponse "住宿筛选选项"
// @Router /v1/workspaces/{workspace_id}/student-dashboard/accommodation/filter-options [get]
func AccommodationOptions(c echo.Context) error {
	// 校验空间路径，选项保持租户及在籍范围隔离。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.AccommodationOptions(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ExportAccommodation 按当前筛选导出在籍学生住宿信息汇总。
// @Summary 导出学生住宿信息表
// @Description 要求独立 student.dashboard.export，复用住宿表的在籍范围、具体值筛选和非校内位置归并。
// @Description 导出完整固定字段及人数合计，所有单元格按文本保存；审计提交后返回 Base64 XLSX。
// @Description 行数及文件大小复用 student.max_export_rows、student.max_file_bytes，超限整体拒绝。
// @Tags 学生数据仪表
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param accommodation_type query []string false "住宿类型英文代码，支持多选" Enums(on_campus,off_campus,joint_abroad,unknown) collectionFormat(multi)
// @Param campus query []string false "住宿园区，支持多选；显式空值匹配空位置" collectionFormat(multi)
// @Param building query []string false "宿舍楼，支持多选；显式空值匹配空位置" collectionFormat(multi)
// @Param room query []string false "宿舍号，支持多选，按文本保留前导零；显式空值匹配空位置" collectionFormat(multi)
// @Success 200 {object} studentdashboardModels.FileResponse "住宿信息表 XLSX"
// @Router /v1/workspaces/{workspace_id}/student-dashboard/accommodation/export [get]
func ExportAccommodation(c echo.Context) error {
	// 只接收固定字段筛选，独立复核导出权限并持久化审计。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := studentdashboardSvc.ExportAccommodation(
		c.Request().Context(),
		workspaceID,
		accommodationFilters(c),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func accommodationFilters(c echo.Context) studentdashboardModels.AccommodationQuery {
	return studentdashboardModels.AccommodationQuery{
		AccommodationTypes: queryfilter.Values(c.QueryParams()["accommodation_type"], false),
		Campuses:           queryfilter.Values(c.QueryParams()["campus"], true),
		Buildings:          queryfilter.Values(c.QueryParams()["building"], true),
		Rooms:              queryfilter.Values(c.QueryParams()["room"], true),
	}
}
