package graduationConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationModels"

// 固定代码、长度与统计维度不接受外部数据库列名。
const (
	Object            = "graduation"
	PendingLabel      = "待就业"
	CustomTypePrefix  = "custom_"
	TypeCodeLength    = 71
	TypeLabelLength   = 100
	DestinationLength = 255
	StudentNoLength   = 32
	KeywordLength     = 100
	EducationLength   = 32
	MajorLength       = 100
	GradeLength       = 16
	YearLength        = 4
	MinYear           = 1900
	MaxYear           = 9999
	DimensionMajor    = "major"
	DimensionType     = "type"
	ListOrder         = "id DESC"
)

// Fields 是接口、表单与导入模板共用的固定字段目录。
var Fields = []graduationModels.Field{
	{
		Name:      "student_no",
		Label:     "学号",
		MaxLength: StudentNoLength,
		Required:  true,
		Example:   "S20260001",
	},
	{Name: "graduation_year", Label: "毕业年份", MaxLength: YearLength, Example: "2026"},
	{Name: "type", Label: "类型代码", MaxLength: TypeCodeLength, Example: ""},
	{
		Name:      "type_label",
		Label:     "类型",
		MaxLength: TypeLabelLength,
		Required:  false,
		Example:   "",
	},
	{Name: "destination", Label: "毕业去向", MaxLength: DestinationLength, Example: "虚构单位"},
}
