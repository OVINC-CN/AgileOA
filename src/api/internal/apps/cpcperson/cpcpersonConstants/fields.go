package cpcpersonConstants

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"

const (
	Object                       = "cpc_person"
	DefaultStatus                = "在编"
	StatusLength                 = 8
	PositionLength               = 64
	KeywordLength                = 200
	TimelineTransferApproved     = "approved"
	TimelineTransferRejected     = "rejected"
	TimelineTransferReasonLength = 1000
)

var TimelineStatuses = []cpcpersonModels.Option{
	{Value: "normal", Label: "正常"}, {Value: "abnormal", Label: "异常"},
	{Value: "transferring", Label: "转移中"},
}

var Stages = []cpcpersonModels.Option{
	{Value: "applicant", Label: "入党申请人"},
	{Value: "activist", Label: "入党积极分子"},
	{Value: "development_candidate", Label: "发展对象"},
	{Value: "probationary_member", Label: "预备党员"},
	{Value: "full_member", Label: "中共党员"},
}

// FilterColumns 只允许登记的基础维度及党建字段，不接受任意数据库列。
var FilterColumns = map[string]string{
	"timeline_status": "p.timeline_status",
	"stage":           "p.stage",
	"status":          "p.status",
	"party_position":  "p.party_position",
	"grade":           "s.grade",
	"student_status":  "s.status",
	"education_level": "s.education_level",
	"major":           "s.major",
	"class_name":      "s.class_name",
	"advisor":         "s.advisor",
}

var Fields = []cpcpersonModels.Field{
	{Key: "timeline_status", Label: "时间线状态（未绑定为空）", Type: "enum", ReadOnly: true,
		Example: "abnormal", Options: TimelineStatuses},
	{
		Key: "timeline_template_id", Label: "时间线模板（新增必填，存量未绑定人员可手动绑定）",
		Type: "integer", Required: true, Example: "1", Options: []cpcpersonModels.Option{},
	},
	{
		Key:      "student_id",
		Label:    "关联学生",
		Type:     "integer",
		Required: true,
		Example:  "1",
		Options:  []cpcpersonModels.Option{},
	},
	{
		Key:      "stage",
		Label:    "发展阶段",
		Type:     "enum",
		Required: true,
		Example:  "applicant",
		Options:  Stages,
	},
	{
		Key:     "status",
		Label:   "状态（最多 8 字符，可新增选项，默认在编）",
		Type:    "string",
		Example: DefaultStatus,
		Options: []cpcpersonModels.Option{},
	},
	{
		Key:      "party_branch_id",
		Label:    "所属党支部（规则自动确定）",
		Type:     "integer",
		Required: true,
		ReadOnly: true,
		Example:  "1",
		Options:  []cpcpersonModels.Option{},
	},
	{
		Key:     "party_position",
		Label:   "党内职务（最多 64 字符，可新增选项）",
		Type:    "string",
		Example: "组织委员",
		Options: []cpcpersonModels.Option{},
	},
}

var StudentFields = []cpcpersonModels.Field{
	{Key: "student_no", Label: "学号", Example: "S20260001"},
	{Key: "name", Label: "姓名", Example: "虚构学生甲"},
	{Key: "gender", Label: "性别", Example: "女"},
	{Key: "ethnicity", Label: "民族", Example: "汉族"},
	{Key: "grade", Label: "年级", Example: "2026级"},
	{Key: "student_status", Label: "学生状态", Example: "在籍"},
	{Key: "education_level", Label: "学历", Example: "硕士"},
	{Key: "class_name", Label: "班级", Example: "2026级一班"},
	{Key: "major", Label: "专业", Example: "虚构专业"},
	{Key: "advisor", Label: "导师", Example: "虚构导师"},
	{Key: "phone", Label: "联系电话", Example: "虚构电话"},
	{Key: "arrival_month", Label: "来校时间", Example: "2026-09"},
	{Key: "previous_school_or_employer", Label: "毕（肄）业学校及单位", Example: "虚构大学"},
	{Key: "previous_graduation_month", Label: "毕（肄）业时间", Example: "2026-06"},
}

// StageValue 只在导入边界将中文显示名称转换为英文代码。
func StageValue(value string) (string, bool) {
	for _, stage := range Stages {
		if value == stage.Value || value == stage.Label {
			return stage.Value, true
		}
	}
	return "", false
}
