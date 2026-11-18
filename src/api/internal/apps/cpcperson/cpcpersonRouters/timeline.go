package cpcpersonRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonSvc"
)

// CreateTimelineOptions 按人员新增权限查询模板选项。
// @Summary 人员新增的时间线模板选项
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词，最多 100 字符"
// @Success 200 {object} cpcpersonModels.OptionsResponse "同空间模板编号和名称"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-templates/create [get]
func CreateTimelineOptions(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineOptions(
			c.Request().Context(),
			workspaceID,
			accessConstants.CPCPersonCreate,
			c.QueryParam("keyword"),
		)
	})
}

// UpdateTimelineOptions 按人员编辑权限查询模板选项。
// @Summary 人员编辑的时间线模板选项
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词，最多 100 字符"
// @Success 200 {object} cpcpersonModels.OptionsResponse "同空间模板编号和名称"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-templates/update [get]
func UpdateTimelineOptions(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineOptions(
			c.Request().Context(),
			workspaceID,
			accessConstants.CPCPersonUpdate,
			c.QueryParam("keyword"),
		)
	})
}

// ImportTimelineOptions 按人员导入权限查询模板选项。
// @Summary 人员导入的时间线模板选项
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词，最多 100 字符"
// @Success 200 {object} cpcpersonModels.OptionsResponse "同空间模板编号和名称"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-templates/import [get]
func ImportTimelineOptions(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineOptions(
			c.Request().Context(),
			workspaceID,
			accessConstants.CPCPersonImport,
			c.QueryParam("keyword"),
		)
	})
}

// Timeline 按人员详情权限及支部范围读取时间线。
// @Summary 人员时间线详情
// @Description 返回当前模板定义、独立填写值和状态、模板附件及人员附件元数据，读取审计提交后返回。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Success 200 {object} cpcpersonModels.TimelineResponse "未绑定时 template_id 为 null 且 nodes 为空"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline [get]
func Timeline(c echo.Context) error { return readTimeline(c, false) }

// EditTimeline 按人员编辑权限及支部范围读取时间线。
// @Summary 人员时间线编辑
// @Description 返回当前模板定义、独立填写值和状态、模板附件及人员附件元数据，读取审计提交后返回。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Success 200 {object} cpcpersonModels.TimelineResponse "未绑定时 template_id 为 null 且 nodes 为空"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/edit [get]
func EditTimeline(c echo.Context) error { return readTimeline(c, true) }

func readTimeline(c echo.Context, editing bool) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Timeline(
			c.Request().Context(),
			workspaceID,
			id,
			editing,
		)
	})
}

// BindTimeline 为存量人员绑定模板。
// @Summary 绑定人员时间线模板
// @Description 要求人员编辑权限及当前人员编辑范围；仅允许绑定未绑定人员，原子建立全部待处理节点。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.BindTimelineRequest true "模板编号及人员当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse "绑定后的人员时间线"
// @Failure 409 {object} accessModels.EmptyResponse "人员版本已改变"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/bind [post]
func BindTimeline(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.BindTimelineRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.BindTimeline(
			c.Request().Context(),
			workspaceID,
			id,
			request,
		)
	})
}

// SaveTimelineNode 完整校验必填字段后将人员节点标记为已完成。
// @Summary 编辑人员时间线节点
// @Description 复用人员编辑权限及支部范围，检查节点和模板版本；仅提交手动填写字段，引用学生信息的字段禁止提交，缺失时允许为空。日期为 YYYY-MM-DD，文本最多 1000 字符，数字和是否分别使用 JSON number 和 boolean。时间规则只提示，不阻断保存。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param node_id path string true "模板内稳定节点标识"
// @Param body body cpcpersonModels.TimelineNodeRequest true "完整手动填写值及两个并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse "保存后的人员时间线"
// @Failure 400 {object} accessModels.EmptyResponse "手动必填值为空、格式无效或提交只读引用字段"
// @Failure 409 {object} accessModels.EmptyResponse "节点或模板版本已改变"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id} [put]
func SaveTimelineNode(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.TimelineNodeRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.SaveTimelineNode(
			c.Request().Context(),
			workspaceID,
			id,
			c.Param("node_id"),
			request,
		)
	})
}

// ResetTimelineNode 清空人员节点的填写值和独立附件，并恢复为待处理。
// @Summary 重置人员时间线节点
// @Description 复用人员编辑权限及支部范围，检查节点和模板版本；清空全部填写值及人员附件，恢复待处理并重算时间异常；模板定义及模板附件保持不变。附件清理失败时保留待删除记录，可重新重置。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param node_id path string true "模板内稳定节点标识"
// @Param body body cpcpersonModels.ResetTimelineNodeRequest true "节点和模板的当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse "重置后的人员时间线"
// @Failure 400 {object} accessModels.EmptyResponse "节点不存在或请求格式无效"
// @Failure 409 {object} accessModels.EmptyResponse "节点、模板或附件已改变"
// @Failure 503 {object} accessModels.EmptyResponse "附件服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id}/reset [post]
func ResetTimelineNode(c echo.Context) error {
	// 提取人员编号并校验请求格式。
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.ResetTimelineNodeRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}

	// 业务层在空间锁内复核权限和并发版本。
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.ResetTimelineNode(
			c.Request().Context(),
			workspaceID,
			id,
			c.Param("node_id"),
			request,
		)
	})
}

// SkipTimelineNode 跳过人员节点并保留已有填写值和附件。
// @Summary 跳过人员时间线节点
// @Description 复用人员编辑权限及支部范围，检查节点和模板版本；保留已保存的值和附件，不校验必填项，不提交表单草稿。已跳过节点不参与自身及被引用的时间校验，可通过保存并完成或重置恢复。重复跳过不修改版本。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param node_id path string true "模板内稳定节点标识"
// @Param body body cpcpersonModels.SkipTimelineNodeRequest true "当前节点和模板版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse "跳过后的人员时间线"
// @Failure 400 {object} accessModels.EmptyResponse "节点不存在或参数无效"
// @Failure 409 {object} accessModels.EmptyResponse "节点或模板版本已改变"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/nodes/{node_id}/skip [post]
func SkipTimelineNode(c echo.Context) error {
	// 提取固定节点标识及并发版本，人员范围由业务层在空间锁内复核。
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.SkipTimelineNodeRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.SkipTimelineNode(
			c.Request().Context(), workspaceID,
			id, c.Param("node_id"), request,
		)
	})
}
