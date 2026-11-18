package attachmentRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentUtils"
)

// UploadPersonTimeline 登记待完成附件并返回 COS 直传凭据。
// @Summary 申请附件直传
// @Description 先登记 pending 及审计并占用名额，客户端按 STS 约束直传正式路径；失败记录可显式删除。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param body body attachmentModels.UploadRequest true "文件名、准确字节数及声明媒体类型"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号及仅本对象可用的临时凭据"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/attachments [post]
func UploadPersonTimeline(c echo.Context) error {
	return upload(c, attachmentConstants.CategoryCPCPersonTimelineNode, "person_id")
}

// DeletePersonTimeline 显式删除人员时间线附件。
// @Summary 删除人员时间线附件
// @Description 复用人员编辑权限及支部范围；校验版本，两阶段删除保留失败恢复与审计。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Param revision query int true "当前附件版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-person-timeline-nodes/{id} [delete]
func DeletePersonTimeline(c echo.Context) error {
	return remove(c, attachmentConstants.CategoryCPCPersonTimelineNode)
}

// DownloadPersonTimeline 按人员详情权限下载当前节点的人员附件。
// @Summary 下载时间线人员附件（人员详情范围）
// @Description 复核当前人员权限、支部及节点归属，仅 ready 可下载；审计持久化后以 JSON 返回短时 CDN URL。
// @Produce json
// @Tags 党建人员时间线
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/attachments/{file_id}/download [get]
func DownloadPersonTimeline(c echo.Context) error { return personTimelineDownload(c, false, false) }

// DownloadPersonTemplate 按人员详情权限下载当前节点的模板附件。
// @Summary 下载时间线模板附件（人员详情范围）
// @Description 复核当前人员权限、支部及节点归属，仅 ready 可下载；审计持久化后以 JSON 返回短时 CDN URL。
// @Produce json
// @Tags 党建人员时间线
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/template-attachments/{file_id}/download [get]
func DownloadPersonTemplate(c echo.Context) error { return personTimelineDownload(c, true, false) }

// EditDownloadPersonTimeline 按人员编辑权限下载当前节点的人员附件。
// @Summary 下载时间线人员附件（人员编辑范围）
// @Description 复核当前人员权限、支部及节点归属，仅 ready 可下载；审计持久化后以 JSON 返回短时 CDN URL。
// @Produce json
// @Tags 党建人员时间线
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/attachments/{file_id}/edit/download [get]
func EditDownloadPersonTimeline(
	c echo.Context,
) error {
	return personTimelineDownload(c, false, true)
}

// EditDownloadPersonTemplate 按人员编辑权限下载当前节点的模板附件。
// @Summary 下载时间线模板附件（人员编辑范围）
// @Description 复核当前人员权限、支部及节点归属，仅 ready 可下载；审计持久化后以 JSON 返回短时 CDN URL。
// @Produce json
// @Tags 党建人员时间线
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param file_id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/template-attachments/{file_id}/edit/download [get]
func EditDownloadPersonTemplate(
	c echo.Context,
) error {
	return personTimelineDownload(c, true, true)
}

func personTimelineDownload(c echo.Context, templateFile, editing bool) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	personID, err := accessUtils.PathID(c, "person_id")
	if err != nil {
		return err
	}
	fileID, err := accessUtils.PathID(c, "file_id")
	if err != nil {
		return err
	}
	data, err := attachmentSvc.PersonTimelineDownload(
		c.Request().Context(),
		workspaceID,
		personID,
		c.Param("node_id"),
		fileID,
		templateFile,
		editing,
	)
	if err != nil {
		return err
	}
	return attachmentUtils.RespondDownload(c, data)
}

// CompletePersonTimeline 核对对象元数据后完成已登记的附件。
// @Summary 确认附件直传完成
// @Description 实时复核权限、业务归属及并发版本，仅通过 HEAD 检查大小和声明类型；已完成记录可幂等重试。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param person_id path int true "人员编号"
// @Param node_id path string true "稳定节点标识"
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.InfoResponse "已完成附件元数据"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{person_id}/timeline/nodes/{node_id}/attachments/complete [post]
func CompletePersonTimeline(c echo.Context) error {
	return completeUpload(c, attachmentConstants.CategoryCPCPersonTimelineNode, "person_id")
}
