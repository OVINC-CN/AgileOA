package cpctimelineConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"

const (
	StableIDPattern      = `^[a-z][a-z0-9_]{0,63}$`
	NameLength           = 100
	DescriptionLength    = 1000
	FieldText            = "text"
	FieldTextarea        = "textarea"
	FieldNumber          = "number"
	FieldDate            = "date"
	FieldBoolean         = "boolean"
	StudentBirthDate     = "birth_date"
	StudentAdulthoodDate = "adulthood_date"
	AdulthoodYears       = 18
)

// StudentFields 是允许引用的学生日期目录，不接受任意学生字段。
var StudentFields = []cpctimelineModels.Option{
	{Value: StudentBirthDate, Label: "出生日期"},
	{Value: StudentAdulthoodDate, Label: "成年日期（出生日期加 18 年）"},
}

// Fields 是固定协议目录，自定义字段只保存在定义中，不生成数据库列。
var Fields = []cpctimelineModels.CatalogField{
	{
		Name:     "name",
		Label:    "模板名称，最多 100 字符，空间内唯一",
		Type:     "string",
		Required: true,
		Example:  "发展工作时间线",
	},
	{Name: "description", Label: "模板说明，最多 1000 字符", Type: "string", Example: "按材料顺序维护节点"},
	{Name: "transfers", Label: "允许转移的多个目标及映射，省略或空数组表示不允许转移",
		Type: "array", Example: "[]"},
	{Name: "transfers[].target_template_id", Label: "同空间的目标模板编号，不能重复或指向自身",
		Type: "integer", Required: true, Example: "2"},
	{Name: "transfers[].node_mappings", Label: "节点映射，支持一对多、多对一及多对多",
		Type: "array", Required: true, Example: "[]"},
	{Name: "transfers[].node_mappings[].source_node_id", Label: "来源模板的稳定节点标识",
		Type: "string", Required: true, Example: "node_application"},
	{Name: "transfers[].node_mappings[].target_node_id", Label: "目标模板的稳定节点标识，人员附件随映射分发或合并",
		Type: "string", Required: true, Example: "node_start"},
	{Name: "transfers[].node_mappings[].fields", Label: "手动字段映射，不含引用字段；同一目标字段只能有一个来源",
		Type: "array", Required: true, Example: "[]"},
	{Name: "transfers[].node_mappings[].fields[].source_field_id", Label: "来源节点中的手动字段标识",
		Type: "string", Required: true, Example: "occurred_at"},
	{Name: "transfers[].node_mappings[].fields[].target_field_id", Label: "目标节点中的同类型手动字段标识",
		Type: "string", Required: true, Example: "submitted_at"},
	{Name: "definition.nodes", Label: "有序节点，至少一个", Type: "array", Required: true, Example: "[]"},
	{Name: "definition.nodes[].numeric_id", Label: "内部节点标识，系统分配，只读；展示及导出编号使用模板内顺序号",
		Type: "integer", Example: "1"},
	{
		Name:     "definition.nodes[].id",
		Label:    "模板内唯一英文节点标识",
		Type:     "string",
		Required: true,
		Example:  "node_application",
	},
	{
		Name:     "definition.nodes[].name",
		Label:    "节点名称，最多 100 字符",
		Type:     "string",
		Required: true,
		Example:  "申请材料",
	},
	{
		Name:    "definition.nodes[].description",
		Label:   "节点说明，最多 1000 字符",
		Type:    "string",
		Example: "核对材料日期",
	},
	{
		Name:    "definition.nodes[].group_name",
		Label:   "节点分组，最多 100 字符；连续同名节点合并展示，省略或空值表示不分组",
		Type:    "string",
		Example: "申请阶段",
	},
	{
		Name:    "definition.nodes[].is_baseline",
		Label:   "是否为基准节点，默认否，用于高亮展示及人员时间线筛选",
		Type:    "boolean",
		Example: "false",
	},
	{
		Name:     "definition.nodes[].time_field_id",
		Label:    "唯一发生时间字段，必填且为 date 类型",
		Type:     "string",
		Required: true,
		Example:  "occurred_at",
	},
	{
		Name:     "definition.nodes[].fields",
		Label:    "字段列表，含发生时间字段",
		Type:     "array",
		Required: true,
		Example:  "[]",
	},
	{
		Name:     "definition.nodes[].fields[].id",
		Label:    "节点内唯一英文标识",
		Type:     "string",
		Required: true,
		Example:  "occurred_at",
	},
	{
		Name:     "definition.nodes[].fields[].name",
		Label:    "字段显示名称，节点内唯一",
		Type:     "string",
		Required: true,
		Example:  "发生时间",
	},
	{
		Name:     "definition.nodes[].fields[].type",
		Label:    "text/textarea/number/date/boolean",
		Type:     "string",
		Required: true,
		Example:  "date",
	},
	{
		Name:     "definition.nodes[].fields[].required",
		Label:    "是否必填，发生时间必须为 true",
		Type:     "boolean",
		Required: true,
		Example:  "true",
	},
	{
		Name:  "definition.nodes[].fields[].student_field",
		Label: "只读学生日期引用，仅 date 字段可用；省略或为空时手动填写，引用缺失时为空",
		Type:  "string", Example: StudentBirthDate, Options: StudentFields,
	},
	{
		Name:     "definition.nodes[].rules",
		Label:    "多条提示规则，不阻止人员入库",
		Type:     "array",
		Required: true,
		Example:  "[]",
	},
	{
		Name:     "definition.nodes[].rules[].id",
		Label:    "节点内唯一英文规则标识",
		Type:     "string",
		Required: true,
		Example:  "rule_notice",
	},
	{
		Name:     "definition.nodes[].rules[].reference_node_id",
		Label:    "同模板的其他节点标识",
		Type:     "string",
		Required: true,
		Example:  "node_application",
	},
	{
		Name:    "definition.nodes[].rules[].min_offset",
		Label:   "相对时间含边界下限，null 为不限",
		Type:    "object",
		Example: `{"years":0,"months":0,"days":1,"workdays":0}`,
	},
	{
		Name:    "definition.nodes[].rules[].max_offset",
		Label:   "相对时间含边界上限，null 为不限",
		Type:    "object",
		Example: `{"years":0,"months":1,"days":0,"workdays":0}`,
	},
}
