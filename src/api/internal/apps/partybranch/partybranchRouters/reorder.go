package partybranchRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ReorderPeople 由用户二次确认后手动提交当前空间在编人员的后台归属重算。
// @Summary 重排人员
// @Description 复用党支部编辑权限，仅异步重算当前空间党建人员状态为“在编”的已入册人员。
// @Description 规则或学生资料变更不自动触发；非在编人员保留原归属，任一在编人员未匹配时整批回滚。
// @Description 重排成功、无匹配支部或任务重试最终失败时，向提交任务的账号发送站内通知。
// @Tags 党支部
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} partybranchModels.ReorderResponse "任务提交结果，无在编人员时不创建任务"
// @Router /v1/workspaces/{workspace_id}/cpc-party-branches/reorder-people [post]
func ReorderPeople(c echo.Context) error {
	// 校验空间范围，授权及任务提交由业务事务处理。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := partybranchSvc.ReorderPeople(
		c.Request().Context(),
		workspaceID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
