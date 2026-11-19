package cpcpersonModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// TimelineNode 保存每名人员独立的节点填写值与处理状态。
type TimelineNode struct {
	WorkspaceID        string `gorm:"primaryKey"`
	PersonID           uint64 `gorm:"primaryKey;autoIncrement:false"`
	TimelineTemplateID uint64
	NodeID             string `gorm:"primaryKey"`
	FieldValues        string
	Status             string
	Revision           uint64
	CreatedAt          time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt          time.Time `gorm:"->;autoUpdateTime:false"`
}

func (TimelineNode) TableName() string { return "cpc_person_timeline_nodes" }

type TimelineNodeInfo struct {
	cpctimelineModels.Node
	// Values 是当前字段值，学生引用实时解析且不写入节点，缺失时不包含对应字段。
	Values map[string]any `json:"values"`
	// Status 是 pending（待处理）、completed（已完成）或 skipped（已跳过）。
	Status string `json:"status" example:"pending"`
	// StatusLabel 是状态中文名称。
	StatusLabel string `json:"status_label" example:"待处理"`
	// Revision 是人员节点并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// TemplateAttachments 是此节点模板的可下载附件。
	TemplateAttachments []attachmentModels.Info `json:"template_attachments"`
	// Attachments 是此人员独立上传的附件，包含待完成及待删除记录。
	Attachments []attachmentModels.Info `json:"attachments"`
	// Issues 是当前节点不符合时间范围规则的明细，空数组表示未发现异常。
	Issues []TimelineIssue `json:"issues"`
	// Checks 仅包含参考节点已有有效日期的规则，当前节点未填写时仍展示。
	Checks []TimelineRuleCheck `json:"checks"`
}

type TimelineRuleCheck struct {
	TimelineIssue
	// ReferenceDate 是已填写的参考节点日期。
	ReferenceDate string `json:"reference_date" example:"2026-11-06"`
	// Result 是 pending（待校验）、normal（符合范围）或 abnormal（时间异常）。
	Result string `json:"result" example:"pending"`
	// ResultLabel 是规则校验结果的中文名称。
	ResultLabel string `json:"result_label" example:"待校验"`
}

type TimelineIssue struct {
	// RuleID 是产生异常的规则标识。
	RuleID string `json:"rule_id" example:"rule_notice"`
	// ReferenceNodeName 是参考节点名称。
	ReferenceNodeName string `json:"reference_node_name" example:"提交申请"`
	// ActualDate 是当前节点的实际发生日期，待填写时为空。
	ActualDate string `json:"actual_date" example:"2026-11-06"`
	// MinDate 是包含边界的预期最早日期，空字符串表示不限。
	MinDate string `json:"min_date" example:"2026-11-07"`
	// MaxDate 是包含边界的预期最晚日期，空字符串表示不限。
	MaxDate string `json:"max_date" example:"2026-11-14"`
	// Message 是系统提供的通用校验提示，不接受模板自定义文案。
	Message string `json:"message" example:"请按预期范围核对发生时间"`
}

type TimelineData struct {
	// PersonID 是所属人员编号。
	PersonID uint64 `json:"person_id" example:"1"`
	// PersonName 是所属人员的姓名。
	PersonName string `json:"person_name" example:"虚构学生甲"`
	// PersonRevision 是绑定模板时校验的人员版本。
	PersonRevision uint64 `json:"person_revision" example:"1"`
	// TemplateID 是已绑定模板编号，未绑定时为 null。
	TemplateID *uint64 `json:"template_id" extensions:"x-nullable" example:"1"`
	// TemplateName 是模板名称。
	TemplateName string `json:"template_name" example:"发展工作时间线"`
	// TemplateRevision 是编辑节点时校验的最新模板版本。
	TemplateRevision uint64 `json:"template_revision" example:"1"`
	// CanUpdate 表示具有此人员的编辑权限及支部范围。
	CanUpdate bool `json:"can_update" example:"true"`
	// CanCancelTransfer 表示转移中可按人员编辑权限取消转移。
	CanCancelTransfer bool `json:"can_cancel_transfer" example:"false"`
	// CanReviewTransfer 表示具有独立的时间线转移审核权限及支部范围。
	CanReviewTransfer bool `json:"can_review_transfer" example:"false"`
	// TransferTargetID 是待审核转移的目标模板编号，无待审核转移时为 null。
	TransferTargetID *uint64 `json:"transfer_target_id" extensions:"x-nullable" example:"2"`
	// TransferTargetName 是待审核转移的目标模板名称。
	TransferTargetName string `json:"transfer_target_name" example:"党员发展时间线"`
	// HasTransferTargets 表示当前绑定模板是否配置了可转移的目标模板。
	HasTransferTargets bool `json:"has_transfer_targets" example:"false"`
	// Nodes 按模板顺序返回人员独立节点，未绑定时为空数组。
	Nodes []TimelineNodeInfo `json:"nodes"`
	// Status 是 normal、abnormal 或 transferring，未绑定时为空字符串。
	Status string `json:"status" example:"abnormal"`
}

type BindTimelineRequest struct {
	// TemplateID 是同空间模板编号，绑定后仅允许通过独立转移流程更换。
	TemplateID uint64 `json:"template_id" example:"1"`
	// Revision 是当前人员版本。
	Revision uint64 `json:"revision" example:"1"`
}

type TimelineNodeRequest struct {
	// Values 是完整手动填写值，不接受学生引用字段；所有手动必填字段必须填写。
	Values map[string]any `json:"values"`
	// Revision 是当前人员节点版本。
	Revision uint64 `json:"revision" example:"1"`
	// TemplateRevision 是读取节点时的模板版本，防止使用过期字段定义提交。
	TemplateRevision uint64 `json:"template_revision" example:"1"`
}

type ResetTimelineNodeRequest struct {
	// Revision 是当前人员节点版本。
	Revision uint64 `json:"revision" example:"1"`
	// TemplateRevision 是读取节点时的模板版本，防止重置已变更的节点。
	TemplateRevision uint64 `json:"template_revision" example:"1"`
}

type SkipTimelineNodeRequest struct {
	// Revision 是当前人员节点版本。
	Revision uint64 `json:"revision" example:"1"`
	// TemplateRevision 是读取节点时的模板版本，防止跳过已变更的节点。
	TemplateRevision uint64 `json:"template_revision" example:"1"`
}

type TimelineResponse struct {
	response.Envelope[TimelineData]
}
