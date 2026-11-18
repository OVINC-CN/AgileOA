package projectModels

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"

// AttachmentUploadRequest 为当前申报子项申请独立的直传对象。
type AttachmentUploadRequest struct {
	attachmentModels.UploadRequest
	// DetailKey 是已保存的子项稳定标识。
	DetailKey string `json:"detail_key" example:"detail_first"`
	// ApplicationRevision 是当前申报并发版本。
	ApplicationRevision uint64 `json:"application_revision" example:"1"`
}
