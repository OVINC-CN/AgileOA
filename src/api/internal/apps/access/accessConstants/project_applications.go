package accessConstants

const (
	ProjectApplicationAttachmentUpload = "project.application.attachment.upload"
	ProjectApplicationAttachmentDelete = "project.application.attachment.delete"
	StudentApplicationAttachmentUpload = "student.application.attachment.upload"
	StudentApplicationAttachmentDelete = "student.application.attachment.delete"
	StudentReviewAttachmentUpload      = "student.review.attachment.upload"
	StudentReviewAttachmentDelete      = "student.review.attachment.delete"

	ProjectApplicationList   = "project.application.list"
	ProjectApplicationExport = "project.application.export"
	ProjectApplicationDelete = "project.application.delete"
	ProjectReviewUpdate      = "project.review.update"
	ProjectReviewApprove     = "project.review.approve"
	ProjectReviewReject      = "project.review.reject"
	StudentProjectList       = "student.project.list"
	StudentApplicationList   = "student.application.list"
	StudentApplicationCreate = "student.application.create"
	StudentApplicationUpdate = "student.application.update"
	StudentApplicationSubmit = "student.application.submit"
	StudentReviewList        = "student.review.list"
	StudentReviewUpdate      = "student.review.update"
	StudentReviewApprove     = "student.review.approve"
	StudentReviewReject      = "student.review.reject"
)

// StudentServiceAction 只允许明确登记的学生自助操作，不按角色推导管理权限。
func StudentServiceAction(action string) bool {
	switch action {
	case StudentApplicationAttachmentUpload,
		StudentNotificationList,
		StudentNotificationUnread,
		StudentNotificationRead,
		StudentApplicationAttachmentDelete,
		StudentReviewAttachmentUpload,
		StudentReviewAttachmentDelete,
		StudentServiceRead,
		StudentProjectList,
		StudentApplicationList,
		StudentApplicationCreate,
		StudentApplicationUpdate,
		StudentApplicationSubmit,
		StudentReviewList,
		StudentReviewUpdate,
		StudentReviewApprove,
		StudentReviewReject:
		return true
	default:
		return false
	}
}
