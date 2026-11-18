package cpctimelineModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// TemplateTransfer 保存同空间模板之间的有序转移配置。
type TemplateTransfer struct {
	WorkspaceID      string `gorm:"primaryKey"`
	SourceTemplateID uint64 `gorm:"primaryKey;autoIncrement:false"`
	TargetTemplateID uint64 `gorm:"primaryKey;autoIncrement:false"`
	NodeMappings     string
	Position         int
	CreatedAt        time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt        time.Time `gorm:"->;autoUpdateTime:false"`
}

func (TemplateTransfer) TableName() string { return "cpc_timeline_template_transfers" }

type TransferFieldMapping struct {
	// SourceFieldID 是来源节点内的手动填写字段标识，不允许引用字段。
	SourceFieldID string `json:"source_field_id" example:"occurred_at"`
	// TargetFieldID 是目标节点内同类型的手动填写字段标识，不允许引用字段。
	TargetFieldID string `json:"target_field_id" example:"submitted_at"`
}

type TransferNodeMapping struct {
	// SourceNodeID 是来源模板节点标识，可映射到多个不同的目标节点。
	SourceNodeID string `json:"source_node_id" example:"node_application"`
	// TargetNodeID 是目标模板节点标识，可合并多个来源节点的字段及附件。
	TargetNodeID string `json:"target_node_id" example:"node_start"`
	// Fields 是节点间的手动字段映射，同一目标字段只能配置一次，可为空数组。
	Fields []TransferFieldMapping `json:"fields"`
}

type Transfer struct {
	// TargetTemplateID 是同空间内允许转移到的模板编号，不允许指向来源模板自身。
	TargetTemplateID uint64 `json:"target_template_id" example:"2"`
	// NodeMappings 支持一对多、多对一及多对多，同一节点对只能出现一次。
	NodeMappings []TransferNodeMapping `json:"node_mappings"`
}

type TransferTarget struct {
	// ID 是同空间可用目标模板编号。
	ID uint64 `json:"id" example:"2"`
	// Name 是目标模板名称。
	Name string `json:"name" example:"后续发展时间线"`
	// Definition 是用于选择目标节点和手动字段的固定定义。
	Definition Definition `json:"definition"`
}

type TransferTargetsResponse struct {
	response.Envelope[[]TransferTarget]
}
