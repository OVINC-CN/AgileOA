package graduationRouters

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 查询当前空间学生及其毕业去向。
// @Summary 查询学生毕业去向
// @Description 包含尚未绑定的学生，未绑定学生在统计中计入待就业。
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词"
// @Param education_level query []string false "学历，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param major query []string false "专业，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param grade query []string false "年级，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param graduation_year query []string false "四位毕业年份；省略为全部，空值为未设置；支持多选" collectionFormat(multi)
// @Param type query []string false "类型英文代码，精确匹配；省略时查询全部，传空字符串时仅查询待就业；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} graduationModels.ListResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations [get]
func List(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	query := filters(c)
	query.Query = page
	result, err := graduationSvc.List(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Save 仅提取固定字段和学生路径编号。
// @Summary 绑定或修改学生毕业去向
// @Description 每名学生唯一去向；类型从已保存数据枚举，允许通过名称新建；版本与学生信息共用。
// @Tags 毕业去向
// @Produce json
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "当前空间学生编号"
// @Param body body graduationModels.Request true "类型、毕业去向和学生版本"
// @Success 200 {object} graduationModels.MutationResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/{id} [put]
func Save(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request graduationModels.Request
	if err := accessUtils.Decode(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	result, err := graduationSvc.Save(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Clear 显式清除绑定，不删除学生。
// @Summary 清除学生毕业去向
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Description 清除后该学生继续计入待就业，历史审计保留。
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "当前空间学生编号"
// @Param revision query int true "最近读取的学生版本"
// @Success 200 {object} accessModels.EmptyResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/{id} [delete]
func Clear(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	revision, err := strconv.ParseUint(c.QueryParam("revision"), 10, 64)
	if err != nil || revision == 0 {
		return echo.ErrBadRequest
	}
	err = graduationSvc.Clear(
		c.Request().Context(),
		workspaceID,
		id,
		revision,
	)
	if err != nil {
		return err
	}
	return response.Success[any](c, nil)
}

// Statistics 读取固定统计维度和学生筛选。
// @Summary 按毕业年份统计毕业去向人数和就业率
// @Description 未绑定学生计入待就业；就业率为非待就业人数除以筛选后的总人数，空结果为数组。
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param dimension query string false "维度，默认 major 专业，type 去向类型" Enums(major,type)
// @Param keyword query string false "学号或姓名关键词"
// @Param education_level query []string false "学历，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param major query []string false "专业，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param grade query []string false "年级，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param graduation_year query []string false "四位毕业年份；省略为全部，空值为未设置；支持多选" collectionFormat(multi)
// @Param type query []string false "类型英文代码，精确匹配；省略时查询全部，传空字符串时仅查询待就业；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Success 200 {object} graduationModels.StatisticsResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/statistics [get]
func Statistics(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	dimension := c.QueryParam("dimension")
	if dimension == "" {
		dimension = graduationConstants.DimensionMajor
	}
	result, err := graduationSvc.Statistics(
		c.Request().Context(), workspaceID, filters(c), dimension,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Options 返回当前空间完整筛选与已保存类型的去重枚举。
// @Summary 获取毕业去向筛选枚举
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} graduationModels.FilterOptionsResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/filter-options [get]
func Options(c echo.Context) error { return options(c, false) }

// StatisticsOptions 复用统计权限获取完整筛选枚举，包含已保存的类型。
// @Summary 获取毕业去向统计筛选枚举
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} graduationModels.FilterOptionsResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/statistics/filter-options [get]
func StatisticsOptions(c echo.Context) error { return options(c, true) }

func options(c echo.Context, statistics bool) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	result, err := graduationSvc.Options(
		c.Request().Context(),
		workspaceID,
		statistics,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Fields 返回固定字段说明。
// @Summary 获取毕业去向字段目录
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} graduationModels.FieldsResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	result, err := graduationSvc.Fields(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Template 返回不含业务数据的固定 XLSX 表头。
// @Summary 下载毕业去向导入模板
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} graduationModels.FileResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/template [get]
func Template(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	result, err := graduationSvc.Template(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

// Import 读取固定模板，在服务层整批新增绑定。
// @Summary 导入学生毕业去向
// @Description 按当前空间学号匹配，仅绑定尚未填写的学生；任一行或审计失败整批回滚。
// @Tags 毕业去向
// @Produce json
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param workspace_id path string true "工作空间编号"
// @Param body body graduationModels.ImportRequest true "Base64 XLSX 内容"
// @Success 200 {object} graduationModels.ImportResponse
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/import [post]
func Import(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request graduationModels.ImportRequest
	if err := accessUtils.Decode(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	result, err := graduationSvc.Import(
		c.Request().Context(),
		workspaceID,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}

func filters(c echo.Context) graduationModels.Query {
	// 多选参数使用重复键，保留未设置年份和待就业类型的显式空值。
	parameters := c.QueryParams()
	return graduationModels.Query{
		Keyword:         strings.TrimSpace(c.QueryParam("keyword")),
		EducationLevels: queryfilter.Values(parameters["education_level"], false),
		Majors:          queryfilter.Values(parameters["major"], false),
		Grades:          queryfilter.Values(parameters["grade"], false),
		GraduationYears: queryfilter.Values(parameters["graduation_year"], true),
		Types:           queryfilter.Values(parameters["type"], true),
	}
}
