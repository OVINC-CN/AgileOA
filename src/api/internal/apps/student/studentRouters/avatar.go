package studentRouters

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// UploadAvatar 使用学生修改权限替换已有学生的头像。
// @Summary 申请学生头像直传
// @Description 按学生版本登记 pending，仅允许 JPEG、PNG 或 WebP 的声明类型，原头像在完成确认前继续可用。
// @Tags 学生信息
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生记录编号"
// @Param body body studentModels.AvatarUploadRequest true "学生版本、文件名、准确大小及声明类型"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号及 COS 直传凭据"
// @Router /v1/workspaces/{workspace_id}/students/{id}/avatar [post]
func UploadAvatar(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}

	var request studentModels.AvatarUploadRequest
	if accessUtils.Decode(c, &request) != nil || request.Revision == 0 {
		return echo.ErrBadRequest
	}
	data, err := studentSvc.PrepareAvatar(
		c.Request().Context(), workspaceID, id, request.Revision, request.UploadRequest,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// CompleteAvatar 确认头像直传并替换学生当前头像。
// @Summary 确认学生头像直传
// @Description 复核学生版本，通过 HEAD 核对上传对象；成功后更新头像和学生版本，失败保留待完成记录。
// @Tags 学生信息
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生编号"
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和学生版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} studentModels.MutationResponse "更新后的学生版本"
// @Router /v1/workspaces/{workspace_id}/students/{id}/avatar/complete [post]
func CompleteAvatar(c echo.Context) error {
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
	data, err := studentSvc.CompleteAvatar(
		c.Request().Context(), workspaceID, id, request.UploadID, request.Revision,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DeleteAvatarUpload 删除失败的头像上传。
// @Summary 删除待完成学生头像
// @Description 使用学生修改权限显式清理 pending 或 deleting 头像记录，不删除当前已完成头像。
// @Tags 学生信息
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "学生编号"
// @Param upload_id path int true "待完成上传编号"
// @Param revision query int true "当前上传记录版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/students/{id}/avatar-uploads/{upload_id} [delete]
func DeleteAvatarUpload(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	uploadID, err := accessUtils.PathID(c, "upload_id")
	if err != nil {
		return err
	}
	revision, err := strconv.ParseUint(c.QueryParam("revision"), 10, 64)
	if err != nil || revision == 0 {
		return echo.ErrBadRequest
	}
	if err := studentSvc.DeleteAvatarUpload(
		c.Request().Context(),
		workspaceID,
		id,
		uploadID,
		revision,
	); err != nil {
		return err
	}
	return response.Success[any](c, nil)
}
