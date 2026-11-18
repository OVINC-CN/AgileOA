package projectRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ProjectExport 按当前筛选导出完整项目表格。
// @Summary 导出评奖评优项目
// @Description 使用独立项目导出权限，复用列表筛选及排序，导出全部匹配项目的名称、年份、说明和附件数量；审计提交后返回文本格式 XLSX。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Param project_name query []string false "完整项目名称，精确匹配，支持多选" collectionFormat(multi)
// @Param year query []int false "项目归属年份，支持多选" collectionFormat(multi)
// @Success 200 {object} projectModels.FileResponse "审计提交后返回 XLSX 文件"
// @Router /v1/workspaces/{workspace_id}/projects/export [get]
func ProjectExport(c echo.Context) error {
	// 校验并提取租户及筛选参数，导出不使用分页。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseFilters(c)
	if err != nil {
		return err
	}

	// 业务层独立复核导出权限并保存审计。
	data, err := projectSvc.Export(
		c.Request().Context(),
		workspaceID,
		query,
		false,
		"",
	)
	if err != nil {
		return err
	}

	// 文件通过通用 JSON 协议返回。
	return response.Success(c, data)
}

// ParticipationExport 按当前筛选导出获奖学生及完整评分子项。
// @Summary 导出获奖学生明细
// @Description 使用独立参与记录导出权限，复用列表筛选及排序，每条获奖记录一行。
// @Description 基础列含学生学历、专业、年级、排名及总分；完整明细将子项分数写入“项目名称-子项名称”列，说明另列，审计提交后返回 XLSX。
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
// @Param format query string false "basic 为获奖学生，full 为含子项列的获奖明细" Enums(basic,full) default(basic)
// @Param detail_keyword query string false "子项名称或说明关键词；须指定完整项目名称或编号及归属年份，匹配后导出该参与记录的全部子项"
// @Success 200 {object} projectModels.FileResponse "审计提交后返回 XLSX 明细文件"
// @Router /v1/workspaces/{workspace_id}/project-participations/export [get]
func ParticipationExport(c echo.Context) error {
	// 校验并提取租户及筛选参数，导出不使用分页。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseFilters(c)
	if err != nil {
		return err
	}

	// 业务层完整展开评分子项并保存审计。
	data, err := projectSvc.Export(
		c.Request().Context(),
		workspaceID,
		query,
		true,
		c.QueryParam("format"),
	)
	if err != nil {
		return err
	}

	// 文件通过通用 JSON 协议返回。
	return response.Success(c, data)
}
