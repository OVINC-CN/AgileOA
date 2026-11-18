package projectRouters

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// ManagedApplications 查询填报管理。
// @Summary 查询填报管理
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param student query string false "学号或姓名关键词"
// @Param project_name query []string false "项目名称，可多选" collectionFormat(multi)
// @Param year query []int false "归属年份，可多选" collectionFormat(multi)
// @Param status query []string false "审批状态，pending_mine 表示待我审批" Enums(draft,pending_mine,pending,rejected,approved) collectionFormat(multi)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} projectModels.ApplicationListResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications [get]
func ManagedApplications(c echo.Context) error {
	return listApplications(c, projectConstants.ApplicationManage)
}

// ManagedApplicationsDetail 查看填报管理详情。
// @Summary 查看填报管理详情
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id} [get]
func ManagedApplicationsDetail(c echo.Context) error {
	return readApplication(c, projectConstants.ApplicationManage)
}

// ManagedApplicationsDelete 按版本删除填报及其所属记录。
// @Summary 删除填报
// @Description 须同时具备查看填报管理与删除填报权限，不要求当前节点资格；允许删除待审批、已驳回或已通过的填报。硬删除申报、评分、审批、流转历史及全部状态的附件，保留审计、学生、项目及获奖记录。附件先提交删除状态和审计，再清理对象，失败可重试。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功，审计已提交"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "并发版本冲突或附件已变更"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id} [delete]
func ManagedApplicationsDelete(c echo.Context) error {
	// 提取租户、申报编号及必填的并发版本。
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

	// 业务层在空间事务内复核权限、版本并保存删除审计。
	if err := projectSvc.DeleteApplication(
		c.Request().Context(),
		workspaceID,
		id,
		revision,
	); err != nil {
		return err
	}

	// 删除成功返回空数据。
	return response.Success[*accessModels.EmptyData](c, nil)
}

// MyApplications 查询我的申报。
// @Summary 查询我的申报
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param student query string false "学号或姓名关键词"
// @Param project_name query []string false "项目名称，可多选" collectionFormat(multi)
// @Param year query []int false "归属年份，可多选" collectionFormat(multi)
// @Param status query []string false "审批状态，pending_mine 表示待我审批" Enums(draft,pending_mine,pending,rejected,approved) collectionFormat(multi)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} projectModels.ApplicationListResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications [get]
func MyApplications(c echo.Context) error {
	return listApplications(c, projectConstants.ApplicationMine)
}

// MyApplicationsDetail 查看我的申报详情。
// @Summary 查看我的申报详情
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id} [get]
func MyApplicationsDetail(c echo.Context) error {
	return readApplication(c, projectConstants.ApplicationMine)
}

// MyApplicationsUpdate 修改我的申报内容。
// @Summary 修改我的申报内容
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id} [put]
func MyApplicationsUpdate(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationMine,
		projectConstants.ApplicationUpdate,
	)
}

// MyApplicationsSubmit 重新提交申报。
// @Summary 重新提交申报
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/submit [post]
func MyApplicationsSubmit(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationMine,
		projectConstants.ApplicationSubmit,
	)
}

// StudentReviews 查询学生我的审核。
// @Summary 查询学生我的审核
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param student query string false "学号或姓名关键词"
// @Param project_name query []string false "项目名称，可多选" collectionFormat(multi)
// @Param year query []int false "归属年份，可多选" collectionFormat(multi)
// @Param status query []string false "审批状态，pending_mine 表示待我审批" Enums(draft,pending_mine,pending,rejected,approved) collectionFormat(multi)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} projectModels.ApplicationListResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews [get]
func StudentReviews(c echo.Context) error {
	return listApplications(c, projectConstants.ApplicationReviews)
}

// StudentReviewsDetail 查看学生我的审核详情。
// @Summary 查看学生我的审核详情
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id} [get]
func StudentReviewsDetail(c echo.Context) error {
	return readApplication(c, projectConstants.ApplicationReviews)
}

// StudentReviewsUpdate 修改学生我的审核内容。
// @Summary 修改学生我的审核内容
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id} [put]
func StudentReviewsUpdate(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationReviews,
		projectConstants.ApplicationUpdate,
	)
}

// StudentReviewsApprove 通过学生我的审核。
// @Summary 通过学生我的审核
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/approve [post]
func StudentReviewsApprove(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationReviews,
		projectConstants.ApplicationApprove,
	)
}

// StudentReviewsReject 驳回学生我的审核。
// @Summary 驳回学生我的审核
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/reject [post]
func StudentReviewsReject(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationReviews,
		projectConstants.ApplicationReject,
	)
}

// ManagedApplicationsUpdate 修改填报管理审批内容。
// @Summary 修改填报管理审批内容
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id} [put]
func ManagedApplicationsUpdate(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationManage,
		projectConstants.ApplicationUpdate,
	)
}

// ManagedApplicationsApprove 通过填报管理审批。
// @Summary 通过填报管理审批
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/approve [post]
func ManagedApplicationsApprove(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationManage,
		projectConstants.ApplicationApprove,
	)
}

// ManagedApplicationsReject 驳回填报管理审批。
// @Summary 驳回填报管理审批
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/reject [post]
func ManagedApplicationsReject(c echo.Context) error {
	return changeApplication(
		c,
		projectConstants.ApplicationManage,
		projectConstants.ApplicationReject,
	)
}

// CreateApplication 提交奖项申报。
// @Summary 提交奖项申报
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Accept json
// @Description 启用分数计算时在本次事务计算；无匹配或公式错误回滚。开启计算即忽略传入总分和排名；末级通过后后台重排同奖项全部已通过申报，同分并列且跳号。
// @Param body body projectModels.ApplicationRequest true "申报内容、审批意见与当前版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "审计提交后返回"
// @Failure 400 {object} accessModels.EmptyResponse "请求参数或流程状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限或不是当前处理人"
// @Failure 404 {object} accessModels.EmptyResponse "申报不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "重复申报、已有获奖记录或并发版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications [post]
func CreateApplication(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var payload projectModels.ApplicationPayload
	if accessUtils.Decode(c, &payload) != nil {
		return echo.ErrBadRequest
	}
	request := payload.ApplicationRequest
	request.RawRank, request.RawScore = payload.Rank, payload.Score
	data, err := projectSvc.CreateApplication(c.Request().Context(), workspaceID, request)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func listApplications(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := parseQuery(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ListApplications(c.Request().Context(), workspaceID, mode,
		projectModels.ApplicationQuery{
			Query:        query.Query,
			Student:      query.Student,
			ProjectNames: query.ProjectNames,
			Years:        query.Years,
			Statuses:     queryfilter.Values(c.QueryParams()["status"], false),
		})
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func readApplication(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := projectSvc.ApplicationDetail(c.Request().Context(), workspaceID, mode, id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func changeApplication(c echo.Context, mode, operation string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var payload projectModels.ApplicationPayload
	if accessUtils.Decode(c, &payload) != nil {
		return echo.ErrBadRequest
	}
	request := payload.ApplicationRequest
	request.RawRank, request.RawScore = payload.Rank, payload.Score
	data, err := projectSvc.ChangeApplication(
		c.Request().Context(),
		workspaceID,
		mode,
		operation,
		id,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ApprovalOptions 枚举审批人选项。
// @Summary 查询审批人选项
// @Description 按当前空间枚举班委、在籍学生或拥有 access_workspace 权限的启用组织账号，组织账号的搜索及保存校验均限定当前空间。
// @Tags 评奖评优
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param kind query string true "选项类型" Enums(class_position,student,user)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} projectModels.ApprovalOptionsResponse
// @Failure 400 {object} accessModels.EmptyResponse "选项类型或关键词无效"
// @Router /v1/workspaces/{workspace_id}/projects/approval-options [get]
func ApprovalOptions(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ApprovalOptions(c.Request().Context(), workspaceID,
		c.QueryParam("kind"), c.QueryParam("keyword"))
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// StudentProjects 查询本人可申报项目。
// @Summary 查询可自行申报的奖项
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 学生服务
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Param keyword query string false "项目名称关键词"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} projectModels.ProjectListResponse
// @Failure 400 {object} accessModels.EmptyResponse "筛选或分页参数无效"
// @Router /v1/workspaces/{workspace_id}/student-service/projects [get]
func StudentProjects(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.StudentProjects(c.Request().Context(), workspaceID,
		projectModels.Query{Query: page, Keyword: c.QueryParam("keyword")})
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ManagedApplicationProjects 查询申报项目筛选枚举。
// @Summary 查询申报项目筛选枚举
// @Description 须具备查看填报管理权限，节点指派不能替代；按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FilterOptionsResponse
// @Failure 400 {object} accessModels.EmptyResponse "请求参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "工作空间不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/filter-options [get]
func ManagedApplicationProjects(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ApplicationProjectOptions(
		c.Request().Context(),
		workspaceID,
		projectConstants.ApplicationManage,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// MyApplicationProjects 查询申报项目筛选枚举。
// @Summary 查询申报项目筛选枚举
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FilterOptionsResponse
// @Failure 400 {object} accessModels.EmptyResponse "请求参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "工作空间不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/filter-options [get]
func MyApplicationProjects(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ApplicationProjectOptions(
		c.Request().Context(),
		workspaceID,
		projectConstants.ApplicationMine,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// StudentReviewProjects 查询申报项目筛选枚举。
// @Summary 查询申报项目筛选枚举
// @Description 按可信身份、工作空间和节点限制读取及操作。任意一级驳回退回申报人，重提从第一级审批。当前节点按本级 allow_edit 决定能否修改内容及附件，全部通过后保留在原申报表；变更与审计同事务提交。
// @Tags 评奖评优申报
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} projectModels.FilterOptionsResponse
// @Failure 400 {object} accessModels.EmptyResponse "请求参数无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "工作空间不存在或不可访问"
// @Failure 409 {object} accessModels.EmptyResponse "记录冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/filter-options [get]
func StudentReviewProjects(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := projectSvc.ApplicationProjectOptions(
		c.Request().Context(),
		workspaceID,
		projectConstants.ApplicationReviews,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
