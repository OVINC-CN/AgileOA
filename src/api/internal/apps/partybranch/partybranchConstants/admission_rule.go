package partybranchConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"

const (
	RuleGroup                = "group"
	RuleCondition            = "condition"
	RuleAnd                  = "and"
	RuleOr                   = "or"
	RuleIn                   = "in"
	RuleAdvisor              = "advisor"
	RuleMajor                = "major"
	RuleEducationLevel       = "education_level"
	RuleStudentNo            = "student_no"
	RuleAdvisorLength        = 50
	RuleMajorLength          = 100
	RuleEducationLevelLength = 32
	RuleStudentNoLength      = 32
)

// RuleFieldLengths 同时限定规则保存与人员匹配的字段及长度。
var RuleFieldLengths = map[string]int{
	RuleAdvisor:        RuleAdvisorLength,
	RuleMajor:          RuleMajorLength,
	RuleEducationLevel: RuleEducationLevelLength,
	RuleStudentNo:      RuleStudentNoLength,
}

// RuleFields 对应人员资料的固定字段，不允许客户端配置任意列。
var RuleFields = []partybranchModels.RuleField{
	{
		RuleOption: partybranchModels.RuleOption{Value: RuleAdvisor, Label: "导师姓名"},
		MaxLength:  RuleAdvisorLength,
	},
	{
		RuleOption: partybranchModels.RuleOption{Value: RuleMajor, Label: "专业"},
		MaxLength:  RuleMajorLength,
	},
	{
		RuleOption: partybranchModels.RuleOption{Value: RuleEducationLevel, Label: "学历"},
		MaxLength:  RuleEducationLevelLength,
	},
	{
		RuleOption: partybranchModels.RuleOption{Value: RuleStudentNo, Label: "学号"},
		MaxLength:  RuleStudentNoLength,
	},
}

var RuleLogics = []partybranchModels.RuleOption{
	{Value: RuleAnd, Label: "全部满足（AND）"},
	{Value: RuleOr, Label: "任一满足（OR）"},
}

var RuleOperators = []partybranchModels.RuleOption{
	{Value: RuleIn, Label: "包含于（in）"},
}
