package projectRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ManagedApplicationVersions 查看填报历史版本。
// @Summary 查看填报历史版本
// @Description 沿用填报管理查看权限，从同空间申报的成功审计提取正文、附件名和修改人。仅返回上一版、截止前最后一版及截止时刻及之后首版，去重后最多三版；未设置截止时间时仅上一版，均早于当前版本，不提供恢复或历史附件下载。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationVersionsResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "查询参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/versions [get]
func ManagedApplicationVersions(c echo.Context) error {
	return readApplicationVersions(c, projectConstants.ApplicationManage)
}

// MyApplicationVersions 查看本人的申报历史版本。
// @Summary 查看我的申报历史版本
// @Description 仅限可信学生身份的本人申报，沿用详情范围校验，从成功审计提取正文、附件名和修改人。仅返回上一版、截止前最后一版及截止时刻及之后首版，去重后最多三版；未设置截止时间时仅上一版，不开放恢复或历史附件下载。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationVersionsResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "查询参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/versions [get]
func MyApplicationVersions(c echo.Context) error {
	return readApplicationVersions(c, projectConstants.ApplicationMine)
}

// StudentReviewVersions 查看学生可审核申报的历史版本。
// @Summary 查看学生审核历史版本
// @Description 实时复核可信学生身份、班级任职和审批节点，沿用审核详情范围，从成功审计提取正文、附件名和修改人。仅返回上一版、截止前最后一版及截止时刻及之后首版，去重后最多三版；未设置截止时间时仅上一版，不开放恢复或历史附件下载。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationVersionsResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "查询参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/versions [get]
func StudentReviewVersions(c echo.Context) error {
	return readApplicationVersions(c, projectConstants.ApplicationReviews)
}

func readApplicationVersions(c echo.Context, mode string) error {
	// 提取当前空间与申报编号，不接受额外范围或身份参数。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	if len(c.QueryParams()) != 0 {
		return echo.ErrBadRequest
	}

	// 原权限、租户及审计事务完成后返回固定正文模型。
	data, err := projectSvc.ApplicationVersions(c.Request().Context(), workspaceID, mode, id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
