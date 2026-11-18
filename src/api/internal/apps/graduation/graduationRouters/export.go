package graduationRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Export 按当前筛选导出全部学生及毕业去向。
// @Summary 导出毕业去向
// @Description 独立导出权限，复用列表筛选、学生编号倒序和待就业语义，不受分页影响；审计提交后返回文本格式 XLSX。
// @Tags 毕业去向
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "学生学号或姓名关键词"
// @Param education_level query []string false "学历，支持多选" collectionFormat(multi)
// @Param major query []string false "专业，支持多选" collectionFormat(multi)
// @Param grade query []string false "年级，支持多选" collectionFormat(multi)
// @Param graduation_year query []string false "四位毕业年份；省略为全部，空值为未设置；支持多选" collectionFormat(multi)
// @Param type query []string false "类型代码，支持多选；显式空值表示待就业，省略表示全部" collectionFormat(multi)
// @Success 200 {object} graduationModels.FileResponse "审计提交后返回 XLSX 文件"
// @Router /v1/workspaces/{workspace_id}/graduation-destinations/export [get]
func Export(c echo.Context) error {
	// 校验空间编号并提取与列表一致的筛选，导出不读取分页参数。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}

	// 业务层复核独立导出权限并持久化审计。
	data, err := graduationSvc.Export(
		c.Request().Context(), workspaceID, filters(c),
	)
	if err != nil {
		return err
	}

	// XLSX 文件通过通用 JSON 协议返回。
	return response.Success(c, data)
}
