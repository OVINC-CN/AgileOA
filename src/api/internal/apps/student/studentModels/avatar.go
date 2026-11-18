package studentModels

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"

// AvatarUpload 保留头像直传的状态，只有确认完成后才替换学生的当前头像。
type AvatarUpload struct {
	attachmentModels.Record
	StudentID       uint64
	StudentRevision uint64
}

func (AvatarUpload) TableName() string { return "student_avatar_uploads" }

// AvatarUploadRequest 复用学生并发版本和固定文件元数据。
type AvatarUploadRequest struct {
	attachmentModels.UploadRequest
	// Revision 是当前学生并发版本。
	Revision uint64 `json:"revision" example:"1"`
}
