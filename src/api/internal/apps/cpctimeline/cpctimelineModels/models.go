package cpctimelineModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Template 保存模板基础资料，节点定义及顺序统一保存在节点子表。
type Template struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	Name        string
	Description string
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定模板表名。
func (Template) TableName() string { return "cpc_timeline_templates" }

// TemplateNode 保存完整节点定义和展示顺序，稳定标识供规则及附件引用。
type TemplateNode struct {
	NumericID          uint64 `gorm:"column:id;autoIncrement"`
	WorkspaceID        string `gorm:"primaryKey"`
	TimelineTemplateID uint64 `gorm:"primaryKey;autoIncrement:false"`
	NodeID             string `gorm:"primaryKey"`
	Name               string
	Description        string
	GroupName          string
	IsBaseline         bool
	TimeFieldID        string
	FieldDefinitions   string
	ValidationRules    string
	Position           int
	CreatedAt          time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt          time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定模板节点子表。
func (TemplateNode) TableName() string { return "cpc_timeline_template_nodes" }

type Field struct {
	// ID 是节点内稳定的英文标识，重命名及排序时保留。
	ID string `json:"id" example:"occurred_at"`
	// Name 是自定义显示名称，最多 100 字符。
	Name string `json:"name" example:"发生时间"`
	// Type 是 text、textarea、number、date 或 boolean。
	Type string `json:"type" example:"date"`
	// Required 表示手动录入时是否必填，发生时间字段始终为 true；学生引用缺失时允许为空。
	Required bool `json:"required" example:"true"`
	// StudentField 是只读学生日期引用，省略或为空时手动填写；引用缺失时返回空值。
	StudentField string `json:"student_field,omitempty" example:"birth_date"`
}

type Offset struct {
	// Years 是相对年份偏移，负数表示提前。
	Years int `json:"years" example:"1"`
	// Months 是年份偏移后的月份偏移，月末按目标月最后一天截断。
	Months int `json:"months" example:"0"`
	// Days 是年月偏移后的自然日偏移。
	Days int `json:"days" example:"-7"`
	// Workdays 是最后计算的周一至周五偏移，不包含节假日和调休修正。
	Workdays int `json:"workdays" example:"0"`
}

type Rule struct {
	// ID 是节点内稳定的英文规则标识。
	ID string `json:"id" example:"rule_notice"`
	// ReferenceNodeID 是同模板内其他节点的稳定标识，可引用前后任意节点。
	ReferenceNodeID string `json:"reference_node_id" example:"node_application"`
	// MinOffset 是相对参考节点发生时间的含边界下限，null 表示不限。
	MinOffset *Offset `json:"min_offset" extensions:"x-nullable"`
	// MaxOffset 是含边界上限，null 表示不限；上下限至少配置一项。
	MaxOffset *Offset `json:"max_offset" extensions:"x-nullable"`
}

type Node struct {
	// NumericID 是内部节点标识，写入时忽略；展示和导出编号使用模板内顺序号。
	NumericID uint64 `json:"numeric_id,omitempty" example:"1"`
	// ID 是模板内稳定的英文节点标识，拖拽排序和重命名时保留。
	ID string `json:"id" example:"node_application"`
	// Name 是节点名称，最多 100 字符。
	Name string `json:"name" example:"申请材料"`
	// Description 是节点说明，最多 1000 字符。
	Description string `json:"description" example:"核对材料落款日期"`
	// GroupName 是可选分组名称，最多 100 字符；连续同名节点合并展示，空值表示不分组。
	GroupName string `json:"group_name" example:"申请阶段"`
	// IsBaseline 表示是否为基准节点，默认 false，用于高亮展示和人员节点筛选。
	IsBaseline bool `json:"is_baseline" example:"false"`
	// TimeFieldID 指定唯一发生时间字段，该字段必须为必填 date 类型。
	TimeFieldID string `json:"time_field_id" example:"occurred_at"`
	// Fields 是有序的自定义字段定义，字段标识和名称在节点内分别唯一。
	Fields []Field `json:"fields"`
	// Rules 是同时生效的提示规则，可为空，不接受阻断入库的强制模式。
	Rules []Rule `json:"rules"`
}

type Definition struct {
	// Nodes 是按展示顺序排列的时间线节点，至少一个。
	Nodes []Node `json:"nodes"`
}

type Input struct {
	// Name 是当前工作空间内唯一的模板名称，最多 100 字符。
	Name string `json:"name" example:"发展工作时间线"`
	// Description 是模板说明，最多 1000 字符。
	Description string `json:"description" example:"按材料顺序维护节点和时间提示"`
	// Definition 是节点、字段和提示规则的完整定义。
	Definition Definition `json:"definition"`
	// Transfers 是允许转移到的多个模板及各自映射，省略或空数组表示不允许转移。
	Transfers []Transfer `json:"transfers"`
}

type Request struct {
	Input
	// Revision 是修改所需的并发版本，新增为零。
	Revision uint64 `json:"revision" example:"1"`
	// ConfirmNodeDeletion 显式确认同步删除所有绑定人员的相应节点及填写内容。
	ConfirmNodeDeletion bool `json:"confirm_node_deletion" example:"false"`
}

type CloneRequest struct {
	// Revision 是原模板当前的并发版本。
	Revision uint64 `json:"revision" example:"1"`
}

type Summary struct {
	// ID 是模板自增编号。
	ID uint64 `json:"id" example:"1"`
	// Name 是模板名称。
	Name string `json:"name" example:"发展工作时间线"`
	// Description 是模板说明。
	Description string `json:"description" example:"按材料顺序维护节点"`
	// NodeCount 是模板节点数量。
	NodeCount int `json:"node_count" example:"10"`
	// Revision 是当前并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-06T00:00:00Z"`
	// UpdatedAt 是最后修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-06T00:00:00Z"`
}

type Info struct {
	Summary
	// Definition 是完整的有序节点、字段和提示规则。
	Definition Definition `json:"definition"`
	// Transfers 是允许转移到的多个模板及节点、字段映射。
	Transfers []Transfer `json:"transfers"`
}

type Query struct {
	pagination.Query
	Keyword string
}

type ListData struct {
	// Items 是当前页模板，按编号倒序排列。
	Items []Summary `json:"items"`
	// Total 是符合条件的模板总数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type CatalogField struct {
	// Name 是固定协议字段路径，不作为数据库列名使用。
	Name string `json:"name" example:"definition.nodes[].time_field_id"`
	// Label 是中文说明。
	Label string `json:"label" example:"节点发生时间字段标识"`
	// Type 是协议类型。
	Type string `json:"type" example:"string"`
	// Required 表示字段必须提供。
	Required bool `json:"required" example:"true"`
	// Example 是字段示例。
	Example string `json:"example" example:"occurred_at"`
	// Options 是固定英文代码及中文名称，仅枚举字段提供。
	Options []Option `json:"options,omitempty"`
}

type Option struct {
	// Value 是固定英文代码。
	Value string `json:"value" example:"birth_date"`
	// Label 是中文显示名称。
	Label string `json:"label" example:"出生日期"`
}

type FileData struct {
	// Filename 是 JSON 空模板文件名。
	Filename string `json:"filename" example:"timeline-template.json"`
	// ContentType 是文件类型。
	ContentType string `json:"content_type" example:"application/json;charset=utf-8"`
	// Content 是包含通用节点的 JSON 导入格式，不包含预置业务数据。
	Content string `json:"content"`
}

type InfoResponse struct{ response.Envelope[Info] }
type ListResponse struct{ response.Envelope[ListData] }
type FieldsResponse struct {
	response.Envelope[[]CatalogField]
}
type FileResponse struct{ response.Envelope[FileData] }
