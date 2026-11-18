package projectRouters

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ListProjects 查询评优项目。
// @Summary 查询评优项目
// @Description 项目按归属年份维护；学生申报可配置条件和纯公式计算总分，获奖记录仍独立录入；操作在工作空间锁内重新授权，审计提交后返回。
// @Description 按项目名称正序、归属年份正序、ID 逆序排列。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Param project_name query []string false "枚举选择的完整项目名称，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param year query []int false "项目归属年份；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，上限来自 YAML"
// @Success 200 {object} projectModels.ProjectListResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/projects [get]
func ListProjects(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ListProjects(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// CreateProject 创建评优项目。
// @Summary 创建评优项目
// @Description 项目按归属年份维护，同一空间的项目名称和年份组合唯一。detail_names 配置评分可选子项名称。
// @Description 允许自行申报时必须填写 self_apply_deadline；保存时调度截止时间后 1 分钟的草稿自动提交任务。
// @Description 启用总分计算后仅用公式计算；ranking_dimensions 可组合 grade、major、class_name，空数组表示全奖项统一排名。保存后后台重排全部已通过申报，同分并列且跳号。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body projectModels.ProjectRequest true "固定字段请求，修改时必须提供 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ProjectResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "项目字段或子项配置无效"
// @Failure 409 {object} accessModels.EmptyResponse "同年同名项目已存在"
// @Failure 503 {object} accessModels.EmptyResponse "自动提交或后台排名任务暂不可用"
// @Router /v1/workspaces/{workspace_id}/projects [post]
func CreateProject(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id := uint64(0)
	var request projectModels.ProjectRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.SaveProject(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// UpdateProject 修改评优项目。
// @Summary 修改评优项目
// @Description 开启总分计算即忽略手填总分和排名；ranking_dimensions 支持年级、专业、班级的任意组合，空数组为全奖项统一排名；保存后后台重排全部已通过申报，入队失败回滚。
// @Description 在工作空间锁内复核修改权限及版本，同时维护 detail_names；已被评分引用的子项名称不可移除。
// @Description 允许自行申报时必须填写 self_apply_deadline；每次保存重新调度，截止时间变更后旧任务失效。
// @Description 审批级别、处理人或签署方式变更后，未完成申报清空有效签署并从新流程第一级重新审批；草稿仍为草稿，已通过申报保留最终流程。仅修改每级 allow_edit 时同步未完成申报并提升版本，保留进度和签署；省略保留原值。项目内容及申报开关实时读取最新配置。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "记录编号"
// @Accept json
// @Param body body projectModels.ProjectRequest true "固定字段请求，修改时必须提供 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ProjectResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "项目字段或子项配置无效"
// @Failure 409 {object} accessModels.EmptyResponse "记录版本冲突或子项配置仍被评分引用"
// @Failure 503 {object} accessModels.EmptyResponse "自动提交或后台排名任务暂不可用"
// @Router /v1/workspaces/{workspace_id}/projects/{id} [put]
func UpdateProject(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request projectModels.ProjectRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.SaveProject(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DeleteProject 删除评优项目。
// @Summary 删除评优项目
// @Description 检查当前版本，硬删除项目、子项及审批流程配置；仍有关联申报、获奖记录或附件时拒绝删除，历史审计保留。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "记录编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/projects/{id} [delete]
func DeleteProject(c echo.Context) error {
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
	if err := projectSvc.DeleteProject(
		c.Request().Context(),
		workspaceID,
		id,
		revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// ProjectTemplate 项目导入模板。
// @Summary 项目导入模板
// @Description 仅含项目名称、归属年份、说明、子项名称(JSON)四列基础资料；子项名称填写字符串数组，可留空。审批设置及分数计算配置在项目编辑中维护。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FileResponse "空白模板"
// @Router /v1/workspaces/{workspace_id}/projects/template [get]
func ProjectTemplate(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Template(
		c.Request().Context(),
		workspaceID,
		false,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ProjectImport 项目整批导入。
// @Summary 项目整批导入
// @Description 仅接收项目名称、归属年份、说明、子项名称(JSON)四列基础资料；子项名称填写字符串数组，任一失败全部回滚。导入项目默认关闭自行申报及分数计算，不接收审批设置或计算配置，不调度草稿自动提交。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body projectModels.ImportRequest true "Base64 XLSX 文本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ImportResponse "导入结果"
// @Failure 400 {object} accessModels.EmptyResponse "项目基础字段或模板表头无效"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/projects/import [post]
func ProjectImport(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request projectModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.Import(
		c.Request().Context(),
		workspaceID,
		request,
		false,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ListParticipations 查询学生项目参与记录。
// @Summary 查询学生项目参与记录
// @Description 项目按归属年份维护；排名、总分和各评分子项独立录入，排名默认留空；操作在工作空间锁内重新授权，审计提交后返回。
// @Description 按项目名称正序、归属年份正序、排名正序、ID 逆序排列；空排名排在已填写排名前。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Param project_name query []string false "枚举选择的完整项目名称，精确匹配；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param year query []int false "项目归属年份；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，上限来自 YAML"
// @Param detail_keyword query string false "子项名称或说明的包含关键词，最多 100 字符；必须同时指定完整项目名称或编号及归属年份"
// @Param student query string false "学生学号或姓名关键词"
// @Param education_level query []string false "学历，精确匹配，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，精确匹配，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，精确匹配，支持多选" collectionFormat(multi)
// @Param project_id query int false "精确项目编号"
// @Success 200 {object} projectModels.ParticipationListResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/project-participations [get]
func ListParticipations(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ListParticipations(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// CreateParticipation 创建学生项目参与记录。
// @Summary 创建学生项目参与记录
// @Description 排名为可选正整数，默认留空；排名、总分和评分子项独立录入，操作在工作空间锁内重新授权，审计提交后返回。
// @Description 子项名称只能选择所属项目已配置的名称，不支持在参与评分中新增名称。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body projectModels.ParticipationRequest true "固定字段请求，修改时必须提供 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ParticipationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "评分字段无效或子项名称未在所属项目中配置"
// @Failure 409 {object} accessModels.EmptyResponse "学生已参与当前项目"
// @Router /v1/workspaces/{workspace_id}/project-participations [post]
func CreateParticipation(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id := uint64(0)
	var request projectModels.ParticipationRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.SaveParticipation(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// UpdateParticipation 修改学生项目参与记录。
// @Summary 修改学生项目参与记录
// @Description 排名为可选正整数，传入 null 可清空；排名、总分和评分子项独立录入，操作在工作空间锁内重新授权，审计提交后返回。
// @Description 子项名称只能选择所属项目已配置的名称；变更项目时按新项目的配置校验全部子项。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "记录编号"
// @Accept json
// @Param body body projectModels.ParticipationRequest true "固定字段请求，修改时必须提供 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ParticipationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "评分字段无效或子项名称未在所属项目中配置"
// @Failure 409 {object} accessModels.EmptyResponse "记录版本冲突或学生已参与当前项目"
// @Router /v1/workspaces/{workspace_id}/project-participations/{id} [put]
func UpdateParticipation(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request projectModels.ParticipationRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.SaveParticipation(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DeleteParticipation 删除学生项目参与记录。
// @Summary 删除学生项目参与记录
// @Description 检查当前版本，硬删除参与记录及自有评分子项；同事务保留总分、子项及学生身份快照，不删除项目或学生。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "记录编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/project-participations/{id} [delete]
func DeleteParticipation(c echo.Context) error {
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
	if err := projectSvc.DeleteParticipation(
		c.Request().Context(),
		workspaceID,
		id,
		revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// ParticipationTemplate 参与记录导入模板。
// @Summary 参与记录导入模板
// @Description 使用固定中文 XLSX 表头；评分子项使用 JSON 数组，name 只能填写所属项目已配置的名称。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FileResponse "空白模板"
// @Router /v1/workspaces/{workspace_id}/project-participations/template [get]
func ParticipationTemplate(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Template(
		c.Request().Context(),
		workspaceID,
		true,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ParticipationImport 参与记录整批导入。
// @Summary 参与记录整批导入
// @Description 使用固定中文 XLSX 表头；子项名称必须在所属项目中配置，不自动新增名称，任一失败全部回滚。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Param body body projectModels.ImportRequest true "Base64 XLSX 文本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ImportResponse "导入结果"
// @Router /v1/workspaces/{workspace_id}/project-participations/import [post]
func ParticipationImport(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request projectModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.Import(
		c.Request().Context(),
		workspaceID,
		request,
		true,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Statistics 项目年度参加人数。
// @Summary 项目年度参加人数
// @Description 复用已提交的项目及学生筛选，按项目归属年份统计去重人数；无匹配学生的项目保留零人数。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Param project_name query []string false "完整项目名称，精确匹配，支持多选" collectionFormat(multi)
// @Param year query []int false "项目归属年份，支持多选" collectionFormat(multi)
// @Param project_id query int false "精确项目编号"
// @Param student query string false "学生学号或姓名关键词"
// @Param education_level query []string false "学历，精确匹配，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，精确匹配，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，精确匹配，支持多选" collectionFormat(multi)
// @Param detail_keyword query string false "子项名称或说明关键词；须指定完整项目名称或编号及归属年份"
// @Success 200 {object} projectModels.StatisticsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/projects/statistics [get]
func Statistics(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseFilters(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Statistics(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Fields 项目固定字段目录。
// @Summary 项目固定字段目录
// @Description 年份来自项目归属年份；学生选择项仅返回学号、姓名和编号；统计按项目名称形成年度系列。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FieldsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/projects/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Fields(c.Request().Context(), workspaceID)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ProjectOptions 项目选择项。
// @Summary 项目选择项
// @Description 年份来自项目归属年份；学生选择项仅返回学号、姓名和编号；统计按项目名称形成年度系列。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Success 200 {object} projectModels.OptionsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/projects/options [get]
func ProjectOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Options(
		c.Request().Context(),
		workspaceID,
		c.QueryParam("keyword"),
		false,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// StudentOptions 参与学生选择项。
// @Summary 参与学生选择项
// @Description 年份来自项目归属年份；学生选择项仅返回学号、姓名和编号；统计按项目名称形成年度系列。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学生学号或姓名关键词"
// @Success 200 {object} projectModels.OptionsResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/project-participations/students [get]
func StudentOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.Options(
		c.Request().Context(),
		workspaceID,
		c.QueryParam("keyword"),
		true,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func parseQuery(c echo.Context) (projectModels.Query, error) {
	page, err := pagination.Parse(c)
	if err != nil {
		return projectModels.Query{}, err
	}
	query, err := parseFilters(c)
	query.Query = page
	return query, err
}

func parseFilters(c echo.Context) (projectModels.Query, error) {
	// 列表和导出共用筛选解析，分页只由列表单独读取。
	query := projectModels.Query{
		ProjectNames:    queryfilter.Values(c.QueryParams()["project_name"], false),
		EducationLevels: queryfilter.Values(c.QueryParams()["education_level"], false),
		Majors:          queryfilter.Values(c.QueryParams()["major"], false),
		Grades:          queryfilter.Values(c.QueryParams()["grade"], false),
		DetailKeyword:   strings.TrimSpace(c.QueryParam("detail_keyword")),
		Keyword:         strings.TrimSpace(c.QueryParam("keyword")),
		Student:         strings.TrimSpace(c.QueryParam("student")),
	}
	var err error
	if value := c.QueryParam("project_id"); value != "" {
		query.ProjectID, err = strconv.ParseUint(value, 10, 64)
		if err != nil || query.ProjectID == 0 {
			return query, echo.ErrBadRequest
		}
	}
	query.Years, err = queryfilter.Integers(c.QueryParams()["year"])
	if err != nil {
		return query, echo.ErrBadRequest
	}
	return query, nil
}

// ParticipationDetail 按需读取参与记录的完整评分子项。
// @Summary 参与记录详情
// @Description 只查询当前空间的独立评分子项表，按录入顺序返回；列表只返回子项数量。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "参与记录编号"
// @Success 200 {object} projectModels.ParticipationResponse "审计提交后返回"
// @Router /v1/workspaces/{workspace_id}/project-participations/{id} [get]
func ParticipationDetail(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := projectSvc.ParticipationDetail(
		c.Request().Context(),
		workspaceID,
		id,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DetailNames 枚举当前项目配置的子项名称，参与评分时不可新增名称。
// @Summary 评分子项名称枚举
// @Description 仅查询当前工作空间指定项目的子项配置；新增、编辑及导入评分只能使用已配置名称。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param project_id query int true "当前工作空间的项目编号，包含归属年份范围"
// @Param keyword query string false "子项名称关键词，最多 100 个字符"
// @Success 200 {object} projectModels.NameOptionsResponse "子项名称选项"
// @Failure 400 {object} accessModels.EmptyResponse "项目编号或搜索关键词无效"
// @Failure 404 {object} accessModels.EmptyResponse "项目不存在或不属于当前工作空间"
// @Router /v1/workspaces/{workspace_id}/project-participations/detail-names [get]
func DetailNames(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	projectID, err := strconv.ParseUint(c.QueryParam("project_id"), 10, 64)
	if err != nil || projectID == 0 || len(c.QueryParams()["project_id"]) != 1 {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.DetailNames(
		c.Request().Context(),
		workspaceID,
		projectID,
		c.QueryParam("keyword"),
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ProjectFilterOptions 查询项目名称及归属年份枚举。
// @Summary 奖项管理筛选枚举
// @Description 复用当前列表权限；只返回当前工作空间完整名称及年份，供自定义下拉本地搜索。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FilterOptionsResponse "审计持久化后返回完整筛选选项"
// @Router /v1/workspaces/{workspace_id}/projects/filter-options [get]
func ProjectFilterOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.FilterOptions(
		c.Request().Context(),
		workspaceID,
		false,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ParticipationFilterOptions 查询项目、归属年份及学生基础字段枚举。
// @Summary 获奖学生筛选枚举
// @Description 复用当前列表权限；返回当前工作空间完整项目名称、归属年份及学生学历、专业、年级，供自定义下拉本地搜索。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FilterOptionsResponse "审计持久化后返回完整筛选选项"
// @Router /v1/workspaces/{workspace_id}/project-participations/filter-options [get]
func ParticipationFilterOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.FilterOptions(
		c.Request().Context(),
		workspaceID,
		true,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
