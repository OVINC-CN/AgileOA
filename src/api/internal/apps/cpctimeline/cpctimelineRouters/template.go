package cpctimelineRouters

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 查询时间线模板摘要。
// @Summary 查询时间线模板
// @Description 按编号倒序分页，关键词匹配名称和说明；仅返回当前空间摘要，先持久化读取审计。
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "名称或说明，最多 100 字符"
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认值及上限来自 YAML"
// @Success 200 {object} cpctimelineModels.ListResponse "模板摘要列表"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates [get]
func List(c echo.Context) error {
	// 提取空间及固定分页参数。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := cpctimelineSvc.List(c.Request().Context(), workspaceID,
		cpctimelineModels.Query{Query: page, Keyword: c.QueryParam("keyword")})
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Detail 读取完整模板。
// @Summary 时间线模板详情
// @Description 独立校验详情权限，返回完整有序节点、字段和提示规则；读取审计持久化后返回。
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "模板编号"
// @Success 200 {object} cpctimelineModels.InfoResponse "完整模板定义"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{id} [get]
func Detail(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := cpctimelineSvc.Detail(
		c.Request().Context(),
		workspaceID,
		id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Create 新增模板。
// @Summary 新增时间线模板
// @Description 名称在空间内唯一；每个节点指定必填日期字段；规则仅保存非阻断提示定义，不接收人员发生时间。
// @Tags 时间线模板
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body cpctimelineModels.Request true "完整模板，新增 revision 为零"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpctimelineModels.InfoResponse "新增后的模板"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates [post]
func Create(c echo.Context) error { return save(c, false) }

// Update 整体保存模板及顺序。
// @Summary 修改时间线模板
// @Description 按当前 revision 整体保存，拒绝悬空、自身引用；上下界含边界，按年、月、自然日、工作日顺序偏移，月末截断；工作日仅周一至周五，不含节假日调休；规则始终仅提示。
// @Tags 时间线模板
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "模板编号"
// @Param body body cpctimelineModels.Request true "完整模板及当前 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpctimelineModels.InfoResponse "保存后的模板"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{id} [put]
func Update(c echo.Context) error { return save(c, true) }

func save(c echo.Context, update bool) error {
	// 只提取并解码参数，业务校验交由服务层。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var id uint64
	if update {
		id, err = accessUtils.PathID(c, "id")
		if err != nil {
			return err
		}
	}
	var request cpctimelineModels.Request
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := cpctimelineSvc.Save(
		c.Request().Context(), workspaceID, id, request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 按版本硬删除模板。
// @Summary 删除时间线模板
// @Description 检查当前版本，已绑定人员时拒绝删除；否则级联删除模板、节点、节点附件记录及以此模板为来源或目标的转移配置，保留历史审计。
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "模板编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{id} [delete]
func Delete(c echo.Context) error {
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
	if err := cpctimelineSvc.Delete(
		c.Request().Context(), workspaceID, id, revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// Fields 返回固定协议字段目录。
// @Summary 时间线模板字段目录
// @Description 提供字段路径、类型、中文说明和示例，复用列表权限。
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpctimelineModels.FieldsResponse "固定字段目录"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpctimelineSvc.Fields(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ImportTemplate 下载 JSON 导入格式。
// @Summary 时间线模板导入格式
// @Description 仅含通用节点和必填发生时间字段，不包含预置党建业务模板；复用导入权限。
// @Tags 时间线模板
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpctimelineModels.FileResponse "JSON 导入文件"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/template [get]
func ImportTemplate(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := cpctimelineSvc.ImportTemplate(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Import 按 JSON 定义新增模板。
// @Summary 导入时间线模板
// @Description 只新增一个模板，不覆盖已有模板；结构、引用、唯一约束或审计失败时全部回滚；独立校验导入权限。
// @Tags 时间线模板
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body cpctimelineModels.Input true "JSON 模板定义，不含系统编号与 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpctimelineModels.InfoResponse "导入后的模板"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/import [post]
func Import(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	// JSON 导入保留原有格式，同时限制请求中的单个文件内容大小。
	c.Request().Body = http.MaxBytesReader(
		c.Response(), c.Request().Body, config.Config.COS.MaxUploadBytes,
	)
	var input cpctimelineModels.Input
	if accessUtils.Decode(c, &input) != nil {
		return echo.ErrBadRequest
	}
	data, err := cpctimelineSvc.Import(
		c.Request().Context(),
		workspaceID,
		input,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
