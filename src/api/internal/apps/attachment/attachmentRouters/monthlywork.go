package attachmentRouters

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
)

// UploadMonthlyWork 登记待完成附件并返回 COS 直传凭据。
// @Summary 申请附件直传
// @Description 先登记 pending 及审计并占用名额，客户端按 STS 约束直传正式路径；失败记录可显式删除。
// @Tags 月度工作附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param work_id path int true "文件所属工作记录编号"
// @Param body body attachmentModels.UploadRequest true "文件名、准确字节数及声明媒体类型"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.UploadResponse "待完成编号及仅本对象可用的临时凭据"
// @Router /v1/workspaces/{workspace_id}/monthly-work/{work_id}/attachments [post]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/{work_id}/attachments [post]
func UploadMonthlyWork(c echo.Context) error {
	return upload(c, workAttachmentCategory(c), "work_id")
}

// ListMonthlyWork 查询固定月度工作类别的附件。
// @Summary 查询月度工作附件
// @Description 只返回当前空间指定类别的元数据及存储流程状态，不返回对象名、存储地址或签名。
// @Tags 月度工作附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param monthly_work_id query int false "仅查询指定工作记录附件，可省略"
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，上限来自 YAML"
// @Success 200 {object} attachmentModels.ListResponse "附件目录"
// @Router /v1/workspaces/{workspace_id}/attachments/monthly-work [get]
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-monthly-work [get]
func ListMonthlyWork(c echo.Context) error {
	return list(c, workAttachmentCategory(c), "monthly_work_id")
}

// DownloadMonthlyWork 按月度工作附件读取权限审计并返回下载地址。
// @Summary 下载月度工作附件
// @Description 实时复核当前空间及所属类别的附件读取权限，审计提交后 返回短期 CDN URL 的 JSON；学生工作、党建工作及项目附件权限互不通用。
// @Produce json
// @Tags 月度工作附件
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Success 200 {object} attachmentModels.URLResponse "文件名、下载地址及过期时间"
// @Failure 400 {object} accessModels.EmptyResponse "参数无效"
// @Router /v1/workspaces/{workspace_id}/attachments/monthly-work/{id}/download [get]
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-monthly-work/{id}/download [get]
func DownloadMonthlyWork(c echo.Context) error {
	return download(c, workAttachmentCategory(c))
}

// DeleteMonthlyWork 显式删除工作附件。
// @Summary 删除月度工作附件
// @Description 检查当前版本及独立删除权限；先提交删除意图及审计，再删除对象及元数据，失败可显式重试。
// @Tags 月度工作附件
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "附件编号"
// @Param revision query int true "当前并发版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "删除成功"
// @Router /v1/workspaces/{workspace_id}/attachments/monthly-work/{id} [delete]
// @Router /v1/workspaces/{workspace_id}/attachments/cpc-monthly-work/{id} [delete]
func DeleteMonthlyWork(c echo.Context) error {
	return remove(c, workAttachmentCategory(c))
}

func workAttachmentCategory(c echo.Context) string {
	// 使用固定路由模板区分业务，客户端不能通过附件编号切换类别。
	if strings.Contains(c.Path(), "/cpc-monthly-work") {
		return attachmentConstants.CategoryCPCMonthlyWork
	}
	return attachmentConstants.CategoryMonthlyWork
}

// CompleteMonthlyWork 核对对象元数据后完成已登记的附件。
// @Summary 确认附件直传完成
// @Description 实时复核权限、业务归属及并发版本，仅通过 HEAD 检查大小和声明类型；已完成记录可幂等重试。
// @Tags 月度工作附件
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param work_id path int true "文件所属工作记录编号"
// @Param body body attachmentModels.CompleteUploadRequest true "申请返回的上传编号和版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} attachmentModels.InfoResponse "已完成附件元数据"
// @Router /v1/workspaces/{workspace_id}/monthly-work/{work_id}/attachments/complete [post]
// @Router /v1/workspaces/{workspace_id}/cpc-monthly-work/{work_id}/attachments/complete [post]
func CompleteMonthlyWork(c echo.Context) error {
	return completeUpload(c, workAttachmentCategory(c), "work_id")
}
