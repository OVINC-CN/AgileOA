package projectRouters

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ManagedApplicationAttachmentUpload 申请申报子项附件直传。
// @Summary 申请申报子项附件直传
// @Description 申请时登记 pending，客户端直传正式路径。须具备查看填报管理权限，节点指派不能替代；附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body projectModels.AttachmentUploadRequest true "文件元数据、子项标识及当前申报版本"
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号、更新后的申报版本及 COS 直传凭据"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/attachments [post]
func ManagedApplicationAttachmentUpload(c echo.Context) error {
	return applicationAttachmentUpload(c, projectConstants.ApplicationManage)
}

// ManagedApplicationAttachmentDelete 删除申报子项附件。
// @Summary 删除申报子项附件
// @Description 须具备查看填报管理权限，节点指派不能替代；附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Param revision query int true "当前附件版本"
// @Param application_revision query int true "当前申报版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "变更后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/attachments/{file_id} [delete]
func ManagedApplicationAttachmentDelete(c echo.Context) error {
	return applicationAttachmentDelete(c, projectConstants.ApplicationManage)
}

// ManagedApplicationAttachmentDownload 下载申报子项附件。
// @Summary 下载申报子项附件
// @Description 须具备查看填报管理权限，节点指派不能替代；附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/attachments/{file_id}/download [get]
func ManagedApplicationAttachmentDownload(c echo.Context) error {
	return applicationAttachmentDownload(c, projectConstants.ApplicationManage)
}

// MyApplicationAttachmentUpload 申请申报子项附件直传。
// @Summary 申请申报子项附件直传
// @Description 申请时登记 pending，客户端直传正式路径。附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body projectModels.AttachmentUploadRequest true "文件元数据、子项标识及当前申报版本"
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号、更新后的申报版本及 COS 直传凭据"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/attachments [post]
func MyApplicationAttachmentUpload(c echo.Context) error {
	return applicationAttachmentUpload(c, projectConstants.ApplicationMine)
}

// MyApplicationAttachmentDelete 删除申报子项附件。
// @Summary 删除申报子项附件
// @Description 附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Param revision query int true "当前附件版本"
// @Param application_revision query int true "当前申报版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "变更后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/attachments/{file_id} [delete]
func MyApplicationAttachmentDelete(c echo.Context) error {
	return applicationAttachmentDelete(c, projectConstants.ApplicationMine)
}

// MyApplicationAttachmentDownload 下载申报子项附件。
// @Summary 下载申报子项附件
// @Description 附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/attachments/{file_id}/download [get]
func MyApplicationAttachmentDownload(c echo.Context) error {
	return applicationAttachmentDownload(c, projectConstants.ApplicationMine)
}

// StudentReviewAttachmentUpload 申请申报子项附件直传。
// @Summary 申请申报子项附件直传
// @Description 申请时登记 pending，客户端直传正式路径。附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body projectModels.AttachmentUploadRequest true "文件元数据、子项标识及当前申报版本"
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号、更新后的申报版本及 COS 直传凭据"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/attachments [post]
func StudentReviewAttachmentUpload(c echo.Context) error {
	return applicationAttachmentUpload(c, projectConstants.ApplicationReviews)
}

// StudentReviewAttachmentDelete 删除申报子项附件。
// @Summary 删除申报子项附件
// @Description 附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Param revision query int true "当前附件版本"
// @Param application_revision query int true "当前申报版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} projectModels.ApplicationResponse "变更后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/attachments/{file_id} [delete]
func StudentReviewAttachmentDelete(c echo.Context) error {
	return applicationAttachmentDelete(c, projectConstants.ApplicationReviews)
}

// StudentReviewAttachmentDownload 下载申报子项附件。
// @Summary 下载申报子项附件
// @Description 附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/attachments/{file_id}/download [get]
func StudentReviewAttachmentDownload(c echo.Context) error {
	return applicationAttachmentDownload(c, projectConstants.ApplicationReviews)
}

func applicationAttachmentUpload(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request projectModels.AttachmentUploadRequest
	if accessUtils.Decode(c, &request) != nil || request.ApplicationRevision == 0 {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.PrepareApplicationAttachment(
		c.Request().Context(), workspaceID, mode, id, request.ApplicationRevision,
		request.DetailKey, request.UploadRequest,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func applicationAttachmentDelete(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	fileID, err := accessUtils.PathID(c, "file_id")
	if err != nil {
		return err
	}
	revision, err := strconv.ParseUint(c.QueryParam("revision"), 10, 64)
	if err != nil || revision == 0 {
		return echo.ErrBadRequest
	}
	appRevision, err := strconv.ParseUint(c.QueryParam("application_revision"), 10, 64)
	if err != nil || appRevision == 0 {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.DeleteApplicationAttachment(
		c.Request().Context(),
		workspaceID,
		mode,
		id,
		fileID,
		revision,
		appRevision,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func applicationAttachmentDownload(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	fileID, err := accessUtils.PathID(c, "file_id")
	if err != nil {
		return err
	}
	data, err := projectSvc.DownloadApplicationAttachment(
		c.Request().Context(),
		workspaceID,
		mode,
		id,
		fileID,
	)
	if err != nil {
		return err
	}
	return attachmentUtils.RespondDownload(c, data)
}

func applicationAttachmentComplete(c echo.Context, mode string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request attachmentModels.CompleteUploadRequest
	if accessUtils.Decode(c, &request) != nil || request.UploadID == 0 || request.Revision == 0 {
		return echo.ErrBadRequest
	}
	data, err := projectSvc.CompleteApplicationAttachment(
		c.Request().Context(), workspaceID, mode, id, request.UploadID, request.Revision,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// ManagedApplicationAttachmentComplete 确认申报子项附件直传。
// @Summary 确认申报子项附件直传
// @Description 通过 HEAD 核对大小及声明类型，完成已登记的附件。须具备查看填报管理权限，节点指派不能替代；附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和更新后的申报版本"
// @Success 200 {object} projectModels.ApplicationResponse "完成后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/project-applications/{id}/attachments/complete [post]
func ManagedApplicationAttachmentComplete(c echo.Context) error {
	return applicationAttachmentComplete(c, projectConstants.ApplicationManage)
}

// MyApplicationAttachmentComplete 确认申报子项附件直传。
// @Summary 确认申报子项附件直传
// @Description 通过 HEAD 核对大小及声明类型，完成已登记的附件。附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和更新后的申报版本"
// @Success 200 {object} projectModels.ApplicationResponse "完成后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-applications/{id}/attachments/complete [post]
func MyApplicationAttachmentComplete(c echo.Context) error {
	return applicationAttachmentComplete(c, projectConstants.ApplicationMine)
}

// StudentReviewAttachmentComplete 确认申报子项附件直传。
// @Summary 确认申报子项附件直传
// @Description 通过 HEAD 核对大小及声明类型，完成已登记的附件。附件只关联申报子项，按工作空间、申报编号和子项标识保存到私有 COS，审批完成后继续保留在原申报中。读取检查单据范围，写入检查当前节点，变更使当前级签署失效。
// @Tags 评奖评优申报附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "申报编号"
// @Accept json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和更新后的申报版本"
// @Success 200 {object} projectModels.ApplicationResponse "完成后的申报详情"
// @Failure 400 {object} accessModels.EmptyResponse "参数或附件状态无效"
// @Failure 401 {object} accessModels.EmptyResponse "请先登录"
// @Failure 403 {object} accessModels.EmptyResponse "没有操作权限"
// @Failure 404 {object} accessModels.EmptyResponse "申报或附件不存在"
// @Failure 409 {object} accessModels.EmptyResponse "版本冲突"
// @Failure 503 {object} accessModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/project-reviews/{id}/attachments/complete [post]
func StudentReviewAttachmentComplete(c echo.Context) error {
	return applicationAttachmentComplete(c, projectConstants.ApplicationReviews)
}
