package attachmentRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// UploadTimelineNode 登记待完成附件并返回 COS 直传凭据。
// @Summary 申请附件直传
// @Description 先登记 pending 及审计并占用名额，客户端按 STS 约束直传正式路径；失败记录可显式删除。
// @Tags 时间线节点附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "所属时间线模板编号"
// @Param node_id path string true "所属模板内的稳定节点标识，须先保存节点"
// @Param body body attachmentModels.UploadRequest true "文件名、准确字节数及声明媒体类型"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号及仅本对象可用的临时凭据"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{template_id}/nodes/{node_id}/attachments [post]
func UploadTimelineNode(c echo.Context) error {
	return upload(c, attachmentConstants.CategoryCPCTimelineNode, "template_id")
}

// ListTimelineNode 只查询当前空间、指定模板和节点的附件。
// @Summary 查询时间线节点附件
// @Description 独立校验节点附件读取权限及节点归属，只返回指定节点附件元数据，不返回对象键或签名；读取审计提交后返回。
// @Tags 时间线节点附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "所属时间线模板编号"
// @Param node_id path string true "所属模板内的稳定节点标识"
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，上限来自 YAML"
// @Success 200 {object} attachmentModels.ListResponse "附件目录"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{template_id}/nodes/{node_id}/attachments [get]
func ListTimelineNode(c echo.Context) error {
	// 提取固定路径和分页参数，节点归属在空间锁内复核。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	id, err := accessUtils.PathID(c, "template_id")
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := attachmentSvc.List(
		c.Request().Context(), workspaceID,
		attachmentConstants.CategoryCPCTimelineNode, id, c.Param("node_id"), page,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// DownloadTimelineNode 审计并返回可用附件下载地址。
// @Summary 下载时间线节点附件
// @Description 实时复核当前空间节点附件读取权限，仅 ready 状态可下载；审计持久化后 返回短时 CDN URL 的 JSON，不提供公开 COS 地址。
// @Produce json
// @Tags 时间线节点附件
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-timeline-nodes/{id}/download [get]
func DownloadTimelineNode(c echo.Context) error {
	return download(c, attachmentConstants.CategoryCPCTimelineNode)
}

// DeleteTimelineNode 显式硬删除模板附件。
// @Summary 删除时间线节点附件
// @Description 校验独立删除权限及 revision，先提交删除意图和审计，再删除私有对象及元数据；失败可刷新版本后重试，pending 状态可显式清理。删除节点时附件元数据按复合外键级联删除。
// @Tags 时间线节点附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-timeline-nodes/{id} [delete]
func DeleteTimelineNode(c echo.Context) error {
	return remove(c, attachmentConstants.CategoryCPCTimelineNode)
}

// CompleteTimelineNode 核对对象元数据后完成已登记的附件。
// @Summary 确认附件直传完成
// @Description 实时复核权限、业务归属及并发版本，仅通过 HEAD 检查大小和声明类型；已完成记录可幂等重试。
// @Tags 时间线节点附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param template_id path int true "所属时间线模板编号"
// @Param node_id path string true "所属模板内的稳定节点标识，须先保存节点"
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.InfoResponse "已完成附件元数据"
// @Router /v1/workspaces/{workspace_id}/cpc-timeline-templates/{template_id}/nodes/{node_id}/attachments/complete [post]
func CompleteTimelineNode(c echo.Context) error {
	return completeUpload(c, attachmentConstants.CategoryCPCTimelineNode, "template_id")
}
