package projectRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// PreviewNewProjectScore 试算新项目尚未保存的分数配置。
// @Summary 试算新项目总分
// @Description 要求项目新增权限；使用当前提交的条件、公式、维度和子项分数，与申报共用计算器，仅保存操作审计。
// @Tags 评奖评优
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param body body projectModels.ScorePreviewRequest true "当前未保存的配置和试算数据"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ScorePreviewResponse
// @Failure 400 {object} accessModels.EmptyResponse "未匹配规则或公式错误"
// @Failure 401 {object} accessModels.EmptyResponse
// @Failure 403 {object} accessModels.EmptyResponse
// @Failure 404 {object} accessModels.EmptyResponse
// @Failure 409 {object} accessModels.EmptyResponse
// @Failure 503 {object} accessModels.EmptyResponse
// @Router /v1/workspaces/{workspace_id}/projects/score-preview [post]
func PreviewNewProjectScore(c echo.Context) error {
	return previewProjectScore(c, 0)
}

// PreviewProjectScore 复用项目编辑权限试算当前草稿配置。
// @Summary 试算项目总分
// @Description 要求项目编辑权限并检查当前空间内的项目；试算当前提交的草稿配置，不写入项目、申报或获奖记录。
// @Tags 评奖评优
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "项目编号"
// @Param body body projectModels.ScorePreviewRequest true "当前未保存的配置和试算数据"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ScorePreviewResponse
// @Failure 400 {object} accessModels.EmptyResponse "未匹配规则或公式错误"
// @Failure 401 {object} accessModels.EmptyResponse
// @Failure 403 {object} accessModels.EmptyResponse
// @Failure 404 {object} accessModels.EmptyResponse
// @Failure 409 {object} accessModels.EmptyResponse
// @Failure 503 {object} accessModels.EmptyResponse
// @Router /v1/workspaces/{workspace_id}/projects/{id}/score-preview [post]
func PreviewProjectScore(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return previewProjectScore(c, id)
}

func previewProjectScore(c echo.Context, id uint64) error {
	// 提取固定请求，身份、租户和权限仍由公共入口及 Svc 事务确认。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request projectModels.ScorePreviewRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	result, err := projectSvc.PreviewScoreCalculation(
		c.Request().Context(),
		workspaceID,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, result)
}
