package partybranchRouters

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Export 导出当前工作空间内符合搜索条件的党支部。
// @Summary 导出党支部
// @Description 独立要求 cpc.party_branch.export，复用列表关键词及优先级倒序、编号倒序，不受分页和显示字段影响。
// @Description 导出优先级、党支部简称、党支部全称、管理员姓名及账号；全部单元格按文本保存，审计提交后返回 Base64 XLSX。
// @Description 行数及文件大小分别使用 party_branch.max_export_rows、party_branch.max_file_bytes，超限整体拒绝。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param keyword query string false "匹配全称、简称、管理员姓名或账号，最多 200 字符"
// @Success 200 {object} partybranchModels.FileResponse "党支部 XLSX 文件"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/export [get]
func Export(c echo.Context) error {
	// 提取当前空间及固定搜索参数，分页不参与导出。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.Export(
		c.Request().Context(), workspaceID,
		partybranchModels.Query{Keyword: strings.TrimSpace(c.QueryParam("keyword"))},
	)
	if err != nil {
		return err
	}

	// 返回已完成审计的文件。
	return response.Success(c, data)
}
