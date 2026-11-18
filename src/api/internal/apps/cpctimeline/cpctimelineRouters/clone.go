package cpctimelineRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Clone 创建独立模板及其节点附件副本。
// @Summary 克隆时间线模板
// @Description 复用新建权限，无需额外权限。名称固定为原名称加空格及“(副本)”，重名拒绝且不自动编号；复制完整节点和转移配置，附件通过 COS PutObject-Copy 创建独立私有对象。未完成或待删除的附件须先处理；失败回滚模板及附件记录。
// @Tags 时间线模板
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "原模板编号"
// @Param body body cpctimelineModels.CloneRequest true "原模板当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpctimelineModels.InfoResponse "克隆后的模板"
// @Failure 400 {object} accessModels.EmptyResponse "名称超长或请求无效"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "原模板不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "名称重复、版本冲突或附件状态无效"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{id}/clone [post]
func Clone(c echo.Context) error {
	// 提取原模板及版本，名称和附件归属均由服务端生成。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpctimelineModels.CloneRequest
	if accessUtils.Decode(c, &request) != nil || request.Revision == 0 {
		return echo.ErrBadRequest
	}

	// 返回已提交的新模板定义。
	data, err := cpctimelineSvc.Clone(
		c.Request().Context(), workspaceID, id, request.Revision,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
