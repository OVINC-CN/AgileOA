package cpcpersonRouters

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// List 分页查询书记范围内的人员。
// @Summary 党建人员名单
// @Description 始终要求人员列表权限；查看党支部权限仅扩大范围，否则限本人任书记的支部。基础信息不含入学政治面貌及敏感字段。
// @Description 已有人员列表权限但没有可读支部时，正常返回空名单，不因未担任书记返回权限错误。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词，最多 200 字符"
// @Param timeline_template_id query int false "时间线模板编号，省略为全部"
// @Param timeline_node_id query int false "模板节点内部数字编号，须同时指定 timeline_template_id；省略时不限制节点"
// @Param node_status query []string false "节点状态：pending 待处理、skipped 已跳过、completed 已完成，多选取并集，须指定模板及节点" collectionFormat(multi)
// @Param timeline_status query []string false "时间线状态：normal 正常、abnormal 异常、transferring 转移中，省略为全部" collectionFormat(multi)
// @Param timeline_filter query []string false "时间线字段条件 JSON，重复传参取交集；例如 {\"node_id\":7,\"field_id\":\"occurred_at\",\"operator\":\"gte\",\"value\":\"2026-11-06\"}。字段取自 timeline-fields；支持 eq、contains、gt、gte、lt、lte、empty、not_empty，按字段类型校验；空值条件仅匹配已绑定相应模板的人员" collectionFormat(multi)
// @Param stage query []string false "五种发展阶段英文代码，支持多选" collectionFormat(multi)
// @Param status query []string false "自定义的党建人员状态，最多 8 字符，支持多选" collectionFormat(multi)
// @Param party_branch_id query []string false "党支部编号，支持多选" collectionFormat(multi)
// @Param party_position query []string false "自定义党内职务，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，支持多选" collectionFormat(multi)
// @Param student_status query []string false "学生状态，支持多选" collectionFormat(multi)
// @Param education_level query []string false "学历，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，支持多选" collectionFormat(multi)
// @Param class_name query []string false "班级，支持多选" collectionFormat(multi)
// @Param advisor query []string false "导师姓名，支持多选" collectionFormat(multi)
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认值及上限来自 YAML"
// @Success 200 {object} cpcpersonModels.ListResponse "审计持久化后返回名单"
// @Router /v1/workspaces/{workspace_id}/cpc-people [get]
func List(c echo.Context) error {
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.List(
			c.Request().Context(),
			workspaceID,
			query,
		)
	})
}

// Detail 读取范围内的人员详情。
// @Summary 党建人员详情
// @Description 要求人员详情权限及读取范围，范围外对象统一返回不存在或不可操作。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党建人员记录编号"
// @Success 200 {object} cpcpersonModels.InfoResponse "审计持久化后的详情"
// @Failure 404 {object} accessModels.EmptyResponse "人员不存在或不可操作"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id} [get]
func Detail(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Detail(
			c.Request().Context(),
			workspaceID,
			id,
		)
	})
}

// Create 新增党建人员关联。
// @Summary 新增党建人员
// @Description 要求人员新增权限及新增范围；学生、支部必须同空间，并检查准入规则，不写入学生基础字段，必须绑定同空间的时间线模板。
// @Tags 党建人员
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body cpcpersonModels.Request true "党建专有字段，revision 为零"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.InfoResponse "创建后的记录"
// @Failure 409 {object} accessModels.EmptyResponse "该学生已入册或不可操作"
// @Router /v1/workspaces/{workspace_id}/cpc-people [post]
func Create(c echo.Context) error { return save(c, false) }

// Update 编辑当前人员的党建字段。
// @Summary 编辑党建人员
// @Description 要求人员编辑权限及当前支部编辑范围，复核 revision；学生及党支部均不允许更换，归属仅由后台规则重算维护。
// @Tags 党建人员
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党建人员记录编号"
// @Param body body cpcpersonModels.UpdateRequest true "人员字段及当前 revision，不接受时间线模板"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.InfoResponse "修改后的记录"
// @Failure 409 {object} accessModels.EmptyResponse "记录版本已改变"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id} [put]
func Update(c echo.Context) error { return save(c, true) }

func save(c echo.Context, update bool) error {
	// 请求只接受固定模型，基础信息字段不能通过此入口写入。
	var id uint64
	var err error
	if update {
		id, err = accessUtils.PathID(c, "id")
		if err != nil {
			return err
		}
	}
	var request cpcpersonModels.Request
	if update {
		var input cpcpersonModels.UpdateRequest
		if accessUtils.Decode(c, &input) != nil {
			return echo.ErrBadRequest
		}
		request = cpcpersonModels.Request{
			Input: cpcpersonModels.Input{
				StudentID:     input.StudentID,
				Stage:         input.Stage,
				Status:        input.Status,
				PartyPosition: input.PartyPosition,
			},
			Revision: input.Revision,
		}
	} else if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Save(
			c.Request().Context(),
			workspaceID,
			id,
			request,
		)
	})
}

// Delete 显式硬删除党建记录。
// @Summary 删除党建人员
// @Description 要求人员删除权限及删除范围，删除党支部权限只扩大删除范围，不删除学生基础档案。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党建人员记录编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功，data 为 null"
// @Failure 409 {object} accessModels.EmptyResponse "记录版本已改变或仍有关联"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id} [delete]
func Delete(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	revision, err := strconv.ParseUint(c.QueryParam("revision"), 10, 64)
	if err != nil || revision == 0 {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return nil, cpcpersonSvc.Delete(
			c.Request().Context(),
			workspaceID,
			id,
			revision,
		)
	})
}

// Fields 返回固定字段目录。
// @Summary 党建人员字段目录
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpcpersonModels.FieldsResponse "固定字段及阶段枚举"
// @Router /v1/workspaces/{workspace_id}/cpc-people/fields [get]
func Fields(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Fields(c.Request().Context(), workspaceID)
	})
}

// Options 查询范围内已有维度选项。
// @Summary 党建人员筛选及动态状态、职务选项
// @Description 复用人员列表权限与读取范围，不提供其他支部的状态、职务、专业或学生信息。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param field query string true "固定筛选维度：stage、status、timeline_status、party_position、grade、student_status、education_level、major、class_name、advisor"
// @Success 200 {object} cpcpersonModels.OptionsResponse "范围内去重选项"
// @Router /v1/workspaces/{workspace_id}/cpc-people/options [get]
func Options(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Options(
			c.Request().Context(),
			workspaceID,
			c.QueryParam("field"),
		)
	})
}

// Branches 查询表单或筛选可选支部。
// @Summary 党建人员可选党支部
// @Description 要求人员列表权限，仅提供读取范围内的支部供筛选使用，人员归属不能手工修改。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param operation query string false "选项用途固定为 list，可省略"
// @Success 200 {object} cpcpersonModels.OptionsResponse "读取范围内支部选项"
// @Router /v1/workspaces/{workspace_id}/cpc-people/branches [get]
func Branches(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Branches(
			c.Request().Context(),
			workspaceID,
			c.QueryParam("operation"),
		)
	})
}

// Candidates 查询符合新增范围的在籍学生选项。
// @Summary 党建人员学生候选项
// @Description 要求人员新增权限；仅提供状态为“在籍”且尚未入册的学生，非管理范围仅返回符合本人任书记支部准入规则的学生。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名，选项过多时必须搜索"
// @Success 200 {object} cpcpersonModels.CandidatesResponse "学生基础信息及可准入支部编号"
// @Router /v1/workspaces/{workspace_id}/cpc-people/student-options [get]
func Candidates(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Candidates(
			c.Request().Context(),
			workspaceID,
			strings.TrimSpace(c.QueryParam("keyword")),
		)
	})
}

// Template 生成党建人员导入模板。
// @Summary 党建人员导入模板
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} cpcpersonModels.FileResponse "固定 XLSX 模板"
// @Router /v1/workspaces/{workspace_id}/cpc-people/template [get]
func Template(c echo.Context) error {
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Template(c.Request().Context(), workspaceID)
	})
}

// Import 整批新增党建记录。
// @Summary 导入党建人员
// @Description 要求人员导入权限及新增范围，按学号关联当前空间学生；任一行范围、准入、唯一约束或审计失败均整批回滚。
// @Tags 党建人员
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body cpcpersonModels.ImportRequest true "Base64 XLSX 内容"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.ImportResponse "新增数量"
// @Router /v1/workspaces/{workspace_id}/cpc-people/import [post]
func Import(c echo.Context) error {
	var request cpcpersonModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Import(
			c.Request().Context(),
			workspaceID,
			request,
		)
	})
}

// Export 导出读取范围内符合筛选的人员。
// @Summary 导出党建人员
// @Description 要求人员导出权限，使用与列表相同的筛选，基础数据与完整数据均包含所属党支部全称及简称，导出审计持久化后返回，基础信息不含入学政治面貌及敏感字段。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词"
// @Param format query string false "basic 基础数据（默认）、full 完整数据，完整数据表头为模板内节点顺序号-字段名称，顺序号从 1 开始" Enums(basic,full)
// @Param timeline_filter query []string false "时间线字段条件 JSON，与名单相同，重复传参取交集，字段来自 timeline-fields" collectionFormat(multi)
// @Param timeline_template_id query int false "时间线模板编号，省略为全部"
// @Param timeline_node_id query int false "模板节点内部数字编号，须同时指定 timeline_template_id；省略时不限制节点"
// @Param node_status query []string false "节点状态：pending 待处理、skipped 已跳过、completed 已完成，多选取并集，须指定模板及节点" collectionFormat(multi)
// @Param timeline_status query []string false "时间线状态：normal 正常、abnormal 异常、transferring 转移中，省略为全部" collectionFormat(multi)
// @Param stage query []string false "发展阶段，支持多选" collectionFormat(multi)
// @Param status query []string false "自定义的党建人员状态，最多 8 字符，支持多选" collectionFormat(multi)
// @Param party_branch_id query []string false "党支部编号，支持多选" collectionFormat(multi)
// @Param party_position query []string false "党内职务，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，支持多选" collectionFormat(multi)
// @Param student_status query []string false "学生状态，支持多选" collectionFormat(multi)
// @Param education_level query []string false "学历，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，支持多选" collectionFormat(multi)
// @Param class_name query []string false "班级，支持多选" collectionFormat(multi)
// @Param advisor query []string false "导师姓名，支持多选" collectionFormat(multi)
// @Success 200 {object} cpcpersonModels.FileResponse "范围内人员 XLSX"
// @Router /v1/workspaces/{workspace_id}/cpc-people/export [get]
func Export(c echo.Context) error {
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	query.Format = c.QueryParam("format")
	delete(query.Filters, "format")
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Export(
			c.Request().Context(),
			workspaceID,
			query,
		)
	})
}

// Statistics 按阶段统计当前读取范围的人员。
// @Summary 党建人员发展阶段统计
// @Description 要求独立人员统计权限；与名单共用读取范围及筛选，固定返回五阶段人数，分页不影响结果。
// @Tags 党建人员
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学号或姓名关键词"
// @Param timeline_filter query []string false "时间线字段条件 JSON，与名单相同，重复传参取交集，字段来自 timeline-fields" collectionFormat(multi)
// @Param timeline_template_id query int false "时间线模板编号，省略为全部"
// @Param timeline_node_id query int false "模板节点内部数字编号，须同时指定 timeline_template_id；省略时不限制节点"
// @Param node_status query []string false "节点状态：pending 待处理、skipped 已跳过、completed 已完成，多选取并集，须指定模板及节点" collectionFormat(multi)
// @Param timeline_status query []string false "时间线状态：normal 正常、abnormal 异常、transferring 转移中，省略为全部" collectionFormat(multi)
// @Param stage query []string false "发展阶段，支持多选" collectionFormat(multi)
// @Param status query []string false "自定义的党建人员状态，最多 8 字符，支持多选" collectionFormat(multi)
// @Param party_branch_id query []string false "党支部编号，支持多选" collectionFormat(multi)
// @Param party_position query []string false "党内职务，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，支持多选" collectionFormat(multi)
// @Param student_status query []string false "学生状态，支持多选" collectionFormat(multi)
// @Param education_level query []string false "学历，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，支持多选" collectionFormat(multi)
// @Param class_name query []string false "班级，支持多选" collectionFormat(multi)
// @Param advisor query []string false "导师姓名，支持多选" collectionFormat(multi)
// @Success 200 {object} cpcpersonModels.StatisticsResponse "五种阶段及总人数，审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/cpc-people/statistics [get]
func Statistics(c echo.Context) error {
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.Statistics(
			c.Request().Context(),
			workspaceID,
			query,
		)
	})
}

func parseQuery(c echo.Context) (cpcpersonModels.Query, error) {
	page, err := pagination.Parse(c)
	query := cpcpersonModels.Query{
		Query:   page,
		Keyword: c.QueryParam("keyword"),
		Filters: map[string][]string{},
	}
	if err != nil {
		return query, err
	}
	if values, exists := c.QueryParams()["timeline_template_id"]; exists {
		if len(values) != 1 {
			return query, echo.NewHTTPError(400, "时间线模板编号无效")
		}
		query.TemplateID, err = strconv.ParseUint(values[0], 10, 64)
		if err != nil || query.TemplateID == 0 {
			return query, echo.NewHTTPError(400, "时间线模板编号无效")
		}
	}
	if values, exists := c.QueryParams()["timeline_node_id"]; exists {
		if len(values) != 1 {
			return query, echo.NewHTTPError(400, "时间线节点编号无效")
		}
		query.NodeID, err = strconv.ParseUint(values[0], 10, 64)
		if err != nil || query.NodeID == 0 {
			return query, echo.NewHTTPError(400, "时间线节点编号无效")
		}
	}
	query.NodeStatuses = c.QueryParams()["node_status"]
	// 时间线条件只接收目录标识和固定运算符，多个条件取交集。
	conditions := c.QueryParams()["timeline_filter"]
	if len(conditions) > config.Config.CPCPerson.MaxOptions {
		return query, echo.NewHTTPError(400, "时间线筛选条件超过限制")
	}
	for _, value := range conditions {
		var condition cpcpersonModels.TimelineFilter
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&condition); err != nil {
			return query, echo.NewHTTPError(400, "时间线筛选条件格式无效")
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return query, echo.NewHTTPError(400, "时间线筛选条件格式无效")
		}
		query.TimelineFilters = append(query.TimelineFilters, condition)
	}
	for key, values := range c.QueryParams() {
		if key != "page" && key != "page_size" && key != "keyword" &&
			key != "timeline_filter" && key != "timeline_template_id" &&
			key != "timeline_node_id" && key != "node_status" {
			query.Filters[key] = values
		}
	}
	return query, nil
}

func handle(c echo.Context, operation func(string) (any, error)) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := operation(workspaceID)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
