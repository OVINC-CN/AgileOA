package attachmentRouters

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Upload 登记待完成附件并返回 COS 直传凭据。
// @Summary 申请附件直传
// @Description 先登记 pending 及审计并占用名额，客户端按 STS 约束直传正式路径；失败记录可显式删除。
// @Tags 评奖评优附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param project_id path int true "文件所属项目编号"
// @Param body body attachmentModels.UploadRequest true "文件名、准确字节数及声明媒体类型"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号及仅本对象可用的临时凭据"
// @Router /v1/workspaces/{workspace_id}/projects/{project_id}/attachments [post]
func Upload(c echo.Context) error {
	return upload(c, attachmentConstants.CategoryProject, "project_id")
}

func upload(c echo.Context, category, ownerParam string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	projectID, err := accessUtils.PathID(c, ownerParam)
	if err != nil {
		return err
	}
	var request attachmentModels.UploadRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := attachmentSvc.PrepareUpload(
		c.Request().Context(), workspaceID, category, projectID, c.Param("node_id"), request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// List 查询项目附件元数据。
// @Summary 查询项目附件
// @Description 仅返回当前空间的固定项目类别，包含所属项目和存储流程状态，不生成 URL。
// @Tags 评奖评优附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param project_id query int false "仅查询指定项目附件，可省略"
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，上限来自 YAML"
// @Success 200 {object} attachmentModels.ListResponse "附件目录"
// @Router /v1/workspaces/{workspace_id}/attachments/project [get]
func List(c echo.Context) error {
	return list(c, attachmentConstants.CategoryProject, "project_id")
}

func list(c echo.Context, category, ownerQuery string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var projectID uint64
	if value := c.QueryParam(ownerQuery); value != "" {
		projectID, err = strconv.ParseUint(value, 10, 64)
		if err != nil || projectID == 0 {
			return echo.ErrBadRequest
		}
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := attachmentSvc.List(
		c.Request().Context(),
		workspaceID,
		category,
		projectID,
		"",
		page,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Download 通过已审计的 API 获取下载地址。
// @Summary 下载项目附件
// @Description 实时检查启用账号、工作空间及项目附件读取权限，审计提交后返回短期 CDN URL 的 JSON；不提供 COS URL。
// @Produce json
// @Tags 评奖评优附件
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/attachments/{id}/download [get]
func Download(c echo.Context) error {
	return download(c, attachmentConstants.CategoryProject)
}

func download(c echo.Context, category string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	data, err := attachmentSvc.Download(
		c.Request().Context(),
		workspaceID,
		category,
		id,
	)
	if err != nil {
		return err
	}
	return attachmentUtils.RespondDownload(c, data)
}

// Delete 显式删除项目附件。
// @Summary 删除项目附件
// @Description 删除意图先提交审计，再幂等删除存储对象并硬删除记录；失败时刷新版本后可显式重试。
// @Tags 评奖评优附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/attachments/{id} [delete]
func Delete(c echo.Context) error {
	return remove(c, attachmentConstants.CategoryProject)
}

func remove(c echo.Context, category string) error {
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
	if err := attachmentSvc.Delete(
		c.Request().Context(),
		workspaceID,
		category,
		id,
		revision,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

func completeUpload(c echo.Context, category, ownerParam string) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	ownerID, err := accessUtils.PathID(c, ownerParam)
	if err != nil {
		return err
	}
	var request attachmentModels.CompleteUploadRequest
	if accessUtils.Decode(c, &request) != nil || request.UploadID == 0 || request.Revision == 0 {
		return echo.ErrBadRequest
	}
	data, err := attachmentSvc.CompleteUpload(
		c.Request().Context(), workspaceID, category, ownerID, c.Param("node_id"),
		request.UploadID, request.Revision,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// CompleteUpload 核对对象元数据后完成已登记的附件。
// @Summary 确认附件直传完成
// @Description 实时复核权限、业务归属及并发版本，仅通过 HEAD 检查大小和声明类型；已完成记录可幂等重试。
// @Tags 评奖评优附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param project_id path int true "文件所属项目编号"
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.InfoResponse "已完成附件元数据"
// @Router /v1/workspaces/{workspace_id}/projects/{project_id}/attachments/complete [post]
func CompleteUpload(c echo.Context) error {
	return completeUpload(c, attachmentConstants.CategoryProject, "project_id")
}
