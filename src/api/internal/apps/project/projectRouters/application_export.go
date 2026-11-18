package projectRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ManagedApplicationExport 导出当前填报管理筛选下的全部申报及评分子项。
// @Summary 导出填报明细
// @Description 须具备查看填报管理权限，节点指派不能替代；独立校验导出权限，复用填报管理可见范围和筛选，不受分页影响。每张申报一行，子项按项目名称和子项名称展开分数、说明列，同名子项编号；不导出附件和敏感档案，审计提交后返回。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param student query string false "学号或姓名关键词"
// @Param project_name query []string false "项目名称，可多选" collectionFormat(multi)
// @Param year query []int false "归属年份，可多选" collectionFormat(multi)
// @Param status query []string false "审批状态，pending_mine 表示待我审批" Enums(draft,pending_mine,pending,rejected,approved) collectionFormat(multi)
// @Success 200 {object} projectModels.FileResponse "Base64 编码的填报明细 XLSX"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效或导出超过限制"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有导出权限"
// @Failure 404 {object} accessModels.EmptyResponse "工作空间不存在或不可访问"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/export [get]
func ManagedApplicationExport(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ExportApplications(c.Request().Context(), workspaceID,
		projectModels.ApplicationQuery{
			Query: query.Query, Student: query.Student, ProjectNames: query.ProjectNames,
			Years: query.Years, Statuses: queryfilter.Values(c.QueryParams()["status"], false),
		})
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
