package cpcpersonRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonSvc"
)

// ListTimelineOptions 按人员列表权限查询读取范围内实际绑定的模板。
// @Summary 人员时间线视图的模板选项
// @Description 仅返回当前读取范围内人员实际绑定的模板，不要求模板管理权限，无可读支部时返回空数组。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "模板名称关键词，最多 100 字符"
// @Success 200 {object} cpcpersonModels.OptionsResponse "按编号倒序排列的模板编号和名称"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-templates/list [get]
func ListTimelineOptions(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineOptions(
			c.Request().Context(),
			workspaceID,
			accessConstants.CPCPersonList,
			c.QueryParam("keyword"),
		)
	})
}

// TimelineView 提供人员时间线视图所需的有序节点和字段目录。
// @Summary 人员时间线视图的节点目录
// @Description 复用人员列表权限及支部读取范围，仅提供范围内人员已绑定的模板定义，审计提交后返回。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "人员已绑定的时间线模板编号"
// @Success 200 {object} cpcpersonModels.TimelineViewResponse "按模板顺序排列的节点及全部字段"
// @Failure 404 {object} accessModels.EmptyResponse "模板不存在或不可访问"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id} [get]
func TimelineView(c echo.Context) error {
	templateID, err := accessUtils.PathID(c, "template_id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineView(
			c.Request().Context(), workspaceID, templateID,
		)
	})
}

// TimelineViewList 按人员列表权限分页读取一个节点的人员填写值。
// @Summary 人员时间线节点名单
// @Description 与人员视图共用全部筛选和支部读取范围，仅返回所选模板及节点；学生引用实时解析，审计提交后返回。
// @Description 分页前按人员编号倒序排列。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "时间线模板编号"
// @Param node_id path int true "模板节点的内部数字编号，取自节点目录 numeric_id"
// @Param keyword query string false "学号或姓名关键词，最多 200 字符"
// @Param timeline_filter query []string false "时间线字段条件 JSON，与人员名单相同，重复传参取交集，字段取自 timeline-fields" collectionFormat(multi)
// @Param node_status query []string false "所选节点状态：pending 待处理、skipped 已跳过、completed 已完成，多选取并集，省略为全部" collectionFormat(multi)
// @Param stage query []string false "发展阶段英文代码" collectionFormat(multi)
// @Param status query []string false "党建人员状态，最多 8 字符" collectionFormat(multi)
// @Param party_branch_id query []string false "党支部编号" collectionFormat(multi)
// @Param party_position query []string false "党内职务" collectionFormat(multi)
// @Param grade query []string false "年级" collectionFormat(multi)
// @Param student_status query []string false "学生状态" collectionFormat(multi)
// @Param education_level query []string false "学历" collectionFormat(multi)
// @Param major query []string false "专业" collectionFormat(multi)
// @Param class_name query []string false "班级" collectionFormat(multi)
// @Param advisor query []string false "导师姓名" collectionFormat(multi)
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认值及上限来自 YAML"
// @Success 200 {object} cpcpersonModels.TimelineViewListResponse "人员及该节点的全部字段、处理状态"
// @Failure 404 {object} accessModels.EmptyResponse "模板或节点不存在或不可访问"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id}/nodes/{node_id} [get]
func TimelineViewList(c echo.Context) error { return timelineViewQuery(c, false) }

// TimelineViewStatistics 按独立人员统计权限汇总所选节点。
// @Summary 人员时间线节点统计
// @Description 与节点名单共用全部筛选和支部读取范围，分页不影响总人数、已完成、待处理及已跳过人数，审计提交后返回。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "时间线模板编号"
// @Param node_id path int true "模板节点的内部数字编号，取自节点目录 numeric_id"
// @Param keyword query string false "学号或姓名关键词，最多 200 字符"
// @Param timeline_filter query []string false "时间线字段条件 JSON，与人员名单相同，重复传参取交集，字段取自 timeline-fields" collectionFormat(multi)
// @Param node_status query []string false "所选节点状态：pending 待处理、skipped 已跳过、completed 已完成，多选取并集，省略为全部" collectionFormat(multi)
// @Param stage query []string false "发展阶段英文代码" collectionFormat(multi)
// @Param status query []string false "党建人员状态，最多 8 字符" collectionFormat(multi)
// @Param party_branch_id query []string false "党支部编号" collectionFormat(multi)
// @Param party_position query []string false "党内职务" collectionFormat(multi)
// @Param grade query []string false "年级" collectionFormat(multi)
// @Param student_status query []string false "学生状态" collectionFormat(multi)
// @Param education_level query []string false "学历" collectionFormat(multi)
// @Param major query []string false "专业" collectionFormat(multi)
// @Param class_name query []string false "班级" collectionFormat(multi)
// @Param advisor query []string false "导师姓名" collectionFormat(multi)
// @Success 200 {object} cpcpersonModels.TimelineViewStatisticsResponse "当前筛选全部人员的节点状态人数"
// @Failure 404 {object} accessModels.EmptyResponse "模板或节点不存在或不可访问"
// @Router /v1/workspaces/{workspace_id}/cpc-people/timeline-view/{template_id}/nodes/{node_id}/statistics [get]
func TimelineViewStatistics(c echo.Context) error { return timelineViewQuery(c, true) }

func timelineViewQuery(c echo.Context, statistics bool) error {
	// 路由只提取固定目录编号及共用筛选，业务授权在服务的空间锁内复核。
	templateID, err := accessUtils.PathID(c, "template_id")
	if err != nil {
		return err
	}
	nodeID, err := accessUtils.PathID(c, "node_id")
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	if query.TemplateID != 0 && query.TemplateID != templateID {
		return echo.NewHTTPError(400, "时间线模板筛选与所选模板不一致")
	}
	if query.NodeID != 0 && query.NodeID != nodeID {
		return echo.NewHTTPError(400, "时间线节点筛选与所选节点不一致")
	}
	return handle(c, func(workspaceID string) (any, error) {
		if statistics {
			return cpcpersonSvc.TimelineViewStatistics(
				c.Request().Context(), workspaceID, templateID, nodeID, query,
			)
		}
		return cpcpersonSvc.TimelineViewList(
			c.Request().Context(), workspaceID, templateID, nodeID, query,
		)
	})
}
