package cpcpersonModels

import "github.com/OVINC-CN/AgileOA/src/api/internal/response"

type TimelineFilterField struct {
	// NodeID 是筛选定位使用的内部节点标识，显示名称中的编号为模板内顺序号。
	NodeID uint64 `json:"node_id" example:"7"`
	// FieldID 是当前节点定义中的稳定字段标识。
	FieldID string `json:"field_id" example:"occurred_at"`
	// Label 包含模板、节点及字段名称，用于区分同名字段。
	Label string `json:"label" example:"发展工作时间线 / 1-提交申请 / 发生时间"`
	// Type 是 text、textarea、number、date 或 boolean。
	Type string `json:"type" example:"date"`
}

type TimelineFilter struct {
	// NodeID 是字段目录提供的模板节点数字编号。
	NodeID uint64 `json:"node_id" example:"7"`
	// FieldID 是字段目录提供的稳定字段标识。
	FieldID string `json:"field_id" example:"occurred_at"`
	// Operator 是 eq、contains、gt、gte、lt、lte、empty 或 not_empty，按字段类型限制。
	Operator string `json:"operator" example:"gte"`
	// Value 是文本、YYYY-MM-DD 日期、有限数字或 true/false；空值判断时省略。
	Value string `json:"value,omitempty" example:"2026-11-06"`
}

type TimelineFilterFieldsResponse struct {
	response.Envelope[[]TimelineFilterField]
}
