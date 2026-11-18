package partybranchConstants

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"

const (
	FullNameLength  = 200
	ShortNameLength = 100
	KeywordLength   = 200
	ListOrder       = "b.priority DESC, b.id DESC"
)

// Fields 维护固定字段目录，不接受外部表名或列名。
var Fields = []partybranchModels.Field{
	{Name: "id", Label: "党支部编号（系统自增）", Type: "integer", ReadOnly: true, Example: "1"},
	{
		Name: "priority", Label: "优先级（整数，数字越大越优先，默认 0）",
		Type: "integer", Example: "100",
	},
	{
		Name: "full_name", Label: "党支部全称（最多 200 字符，同一空间内唯一）",
		Type: "string", Required: true, Example: "中共示例学院计算机专业学生第一支部委员会",
	},
	{
		Name: "short_name", Label: "党支部简称（最多 100 字符，同一空间内唯一）",
		Type: "string", Required: true, Example: "计算机学生第一党支部",
	},
	{
		Name: "secretary_id", Label: "党支部书记（当前空间可用用户编号，可空）",
		Type: "integer", Example: "1",
	},
	{
		Name: "admission_rule", Label: "人员准入规则树（编辑时可一并维护，rule:null 清空）",
		Type:    "object",
		Example: `{"rule":{"type":"condition","field":"major","operator":"in","values":["会计"]}}`,
	},
}
