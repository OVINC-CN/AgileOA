package cpcpersonModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// TimelineTransferReview 保存当前待审核申请，完成或取消后硬删除，历史由审计保留。
type TimelineTransferReview struct {
	ID               uint64 `gorm:"primaryKey"`
	WorkspaceID      string
	PersonID         uint64
	RequestedByID    uint64
	SourceTemplateID uint64
	TargetTemplateID uint64
	CreatedAt        time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt        time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定人员时间线转移审核表名。
func (TimelineTransferReview) TableName() string { return "cpc_person_timeline_transfer_reviews" }

type TimelineTransferOptions struct {
	// SourceTemplateID 是当前绑定的来源模板编号。
	SourceTemplateID uint64 `json:"source_template_id" example:"1"`
	// SourceTemplateName 是来源模板名称。
	SourceTemplateName string `json:"source_template_name" example:"发展工作时间线"`
	// Targets 只包含来源模板明确配置的同空间目标模板。
	Targets []Option `json:"targets"`
}

type TimelineTransferRequest struct {
	// TargetTemplateID 是来源模板允许转移到的目标模板编号。
	TargetTemplateID uint64 `json:"target_template_id" example:"2"`
	// PreviewVersion 是预览返回的快照版本，发起和审核时必填，预览时省略。
	PreviewVersion string `json:"preview_version,omitempty" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
}

type TimelineTransferRevisionRequest struct {
	// Revision 是取消转移时的当前人员版本，防止处理后来重新发起的申请。
	Revision uint64 `json:"revision" example:"2"`
}

type TimelineTransferRejectRequest struct {
	// Revision 是驳回转移时的当前人员版本，防止处理后来重新发起的申请。
	Revision uint64 `json:"revision" validate:"required" example:"2"`
	// Reason 是必填的审批意见，去除首尾空白后最多 1000 字符，仅写入结果通知正文。
	Reason string `json:"reason" validate:"required" minLength:"1" maxLength:"1000" example:"请补充申请材料后重新发起转移"`
}

type TimelineDiscardedField struct {
	// NodeName 是未映射字段所属的来源节点名称。
	NodeName string `json:"node_name" example:"申请材料"`
	// FieldName 是未映射的已填写手动字段名称。
	FieldName string `json:"field_name" example:"备注"`
	// Value 是未映射字段当前的填写值，零和否仍视为有效填写。
	Value any `json:"value"`
}

type TimelineTransferPreview struct {
	// SourceTemplateID 是当前绑定的来源模板编号。
	SourceTemplateID uint64 `json:"source_template_id" example:"1"`
	// SourceTemplateName 是来源模板名称。
	SourceTemplateName string `json:"source_template_name" example:"发展工作时间线"`
	// PreviewVersion 关联来源节点、附件、学生引用和两个模板的当前快照。
	PreviewVersion string `json:"preview_version" example:"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"`
	// Timeline 是转移后的完整时间线预览，尚未写入业务数据。
	Timeline TimelineData `json:"timeline"`
	// DiscardedFields 是转移后不再保留的已填写手动字段，历史审计仍保存来源快照。
	DiscardedFields []TimelineDiscardedField `json:"discarded_fields"`
}

type TimelineTransferOptionsResponse struct {
	response.Envelope[TimelineTransferOptions]
}
type TimelineTransferPreviewResponse struct {
	response.Envelope[TimelineTransferPreview]
}
