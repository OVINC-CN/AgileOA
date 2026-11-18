package partybranchModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// AdmissionRuleNode 是固定字段目录内的递归规则节点，不接受表达式或数据库标识。
type AdmissionRuleNode struct {
	// Type 是节点类型，group 为条件组，condition 为字段条件。
	Type string `json:"type" example:"group" enums:"group,condition"`
	// Logic 仅用于条件组，and 表示全部满足，or 表示任一满足。
	Logic string `json:"logic,omitempty" example:"and" enums:"and,or"`
	// Children 是条件组的非空子节点，可包含条件或嵌套条件组。
	Children []AdmissionRuleNode `json:"children,omitempty"`
	// Field 仅用于条件，advisor 为导师姓名，major 为专业，education_level 为学历，student_no 为学号。
	Field string `json:"field,omitempty" example:"advisor" enums:"advisor,major,education_level,student_no"`
	// Operator 仅用于条件，固定为 in，按完整文本精确匹配任一候选值。
	Operator string `json:"operator,omitempty" example:"in" enums:"in"`
	// Values 是非空候选值集合，导师姓名最多 50 字符，专业最多 100 字符，学历和学号最多 32 字符；学号保留前导零。
	Values []string `json:"values,omitempty" example:"张三,王五"`
}

type RuleOption struct {
	// Value 是固定的英文枚举代码。
	Value string `json:"value" example:"and"`
	// Label 是枚举的中文显示名称。
	Label string `json:"label" example:"全部满足（AND）"`
}

type RuleField struct {
	RuleOption
	// MaxLength 是此字段单个候选值的字符数上限。
	MaxLength int `json:"max_length" example:"50"`
}

type AdmissionRuleChange struct {
	// Rule 是完整准入规则树，null 表示清空；条件组不能为空，叶子条件仅支持 in。
	Rule *AdmissionRuleNode `json:"rule" extensions:"x-nullable"`
}

type AdmissionRuleRequest struct {
	AdmissionRuleChange
	// Revision 是当前党支部版本，与基础信息及删除共用并发保护。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
}

type AdmissionRuleInfo struct {
	// ID 是所属党支部编号。
	ID uint64 `json:"id" example:"1"`
	// Rule 是完整规则树，null 表示尚未配置，不表示允许所有人员准入。
	Rule *AdmissionRuleNode `json:"rule" extensions:"x-nullable"`
	// Revision 是当前党支部并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// UpdatedAt 是最后更新时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-05T00:00:00Z"`
	// Fields 是规则条件允许选择的固定字段目录及长度限制。
	Fields []RuleField `json:"fields"`
	// Logics 是条件组允许选择的逻辑代码及显示名称。
	Logics []RuleOption `json:"logics"`
	// Operators 是字段条件允许选择的运算符，当前仅提供 in。
	Operators []RuleOption `json:"operators"`
}

type AdmissionRuleResponse struct {
	response.Envelope[AdmissionRuleInfo]
}
