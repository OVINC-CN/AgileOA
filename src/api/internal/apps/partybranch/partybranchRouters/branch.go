package partybranchRouters

import (
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// List 分页查询当前工作空间党支部。
// @Summary 查询党支部
// @Description 按优先级倒序分页查询，同优先级按编号倒序；关键词匹配全称、简称和书记名称或账号；审计提交后返回。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "搜索关键词，最多 200 字符"
// @Param page query int false "页码，默认 1"
// @Param page_size query int false "每页条数，默认值及上限来自 YAML"
// @Success 200 {object} partybranchModels.ListResponse "党支部列表"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches [get]
func List(c echo.Context) error {
	// 校验路径及分页，限定固定搜索参数。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.List(c.Request().Context(), workspaceID,
		partybranchModels.Query{
			Query: page, Keyword: strings.TrimSpace(c.QueryParam("keyword")),
		})
	if err != nil {
		return err
	}

	// 返回已完成审计的列表。
	return response.Success(c, data)
}

// Create 新增党支部并可选设置书记。
// @Summary 新增党支部
// @Description 全称、简称必填，分别在空间内唯一；书记来自当前空间可用用户，可为 null；编号自动生成；可选 admission_rule.rule 在同一事务保存准入规则树。
// @Tags 党支部
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body partybranchModels.Request true "党支部字段，新增时 revision 为零"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} partybranchModels.InfoResponse "创建后的党支部"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches [post]
func Create(c echo.Context) error {
	return save(c, false)
}

// Update 按版本修改党支部字段及书记。
// @Summary 修改党支部
// @Description 提交完整基础字段及当前版本；secretary_id 为 null 时清空书记，原书记停用或失去进入权限后允许保留原关联。admission_rule 省略时保留规则，传 {"rule":null} 清空，传规则树则与基础信息同事务保存并审计。
// @Tags 党支部
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党支部编号"
// @Param body body partybranchModels.Request true "完整党支部字段及当前 revision"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} partybranchModels.InfoResponse "修改后的党支部"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/{id} [put]
func Update(c echo.Context) error {
	return save(c, true)
}

func save(c echo.Context, update bool) error {
	// 校验路径和固定请求体，拒绝外部编号及未知字段。
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
	var request partybranchModels.Request
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}

	// 将业务校验、事务和审计交由服务层处理。
	data, err := partybranchSvc.Save(
		c.Request().Context(), workspaceID, id, request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 按版本硬删除党支部。
// @Summary 删除党支部
// @Description 删除当前空间的党支部及依赖的准入规则、党建人员、时间线和附件记录，保留历史审计及身份快照；学生档案和书记账号保留。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "党支部编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/{id} [delete]
func Delete(c echo.Context) error {
	// 校验删除目标和并发版本。
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

	// 显式删除当前租户内记录，成功后返回空数据。
	if err := partybranchSvc.Delete(
		c.Request().Context(), workspaceID, id, revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// Secretaries 枚举书记候选用户。
// @Summary 党支部书记用户选项
// @Description 复用党支部列表权限；仅返回当前空间有进入权限的启用用户编号、显示名称及账号，不授予任何权限。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} partybranchModels.SecretariesResponse "书记用户选项"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/secretaries [get]
func Secretaries(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.Secretaries(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Fields 返回党支部固定字段目录。
// @Summary 党支部字段目录
// @Description 复用列表权限，提供固定字段、类型、中文说明、只读标记及示例。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} partybranchModels.FieldsResponse "固定字段目录"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/fields [get]
func Fields(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.Fields(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Template 下载党支部 XLSX 导入模板。
// @Summary 党支部导入模板
// @Description 提供固定中文表头：党支部全称、党支部简称、党支部书记账号；书记可留空，编号由系统生成。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} partybranchModels.FileResponse "导入模板"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/template [get]
func Template(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.Template(
		c.Request().Context(), workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Import 按固定模板整批新增党支部。
// @Summary 导入党支部
// @Description 只新增，书记按当前空间可用用户账号匹配；全称或简称冲突、任一行无效或审计失败均整批回滚。
// @Tags 党支部
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body partybranchModels.ImportRequest true "Base64 XLSX 内容"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} partybranchModels.ImportResponse "导入结果"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/import [post]
func Import(c echo.Context) error {
	// 校验路径和导入请求，模板解析与整批事务位于服务层。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request partybranchModels.ImportRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := partybranchSvc.Import(
		c.Request().Context(), workspaceID, request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
