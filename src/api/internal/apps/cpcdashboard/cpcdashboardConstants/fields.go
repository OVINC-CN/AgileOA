package cpcdashboardConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcdashboard/cpcdashboardModels"

const (
	StageFullMember         = "full_member"
	StageProbationaryMember = "probationary_member"
)

// Fields 按仪表及下载表格的列顺序定义只读字段。
var Fields = []cpcdashboardModels.Field{
	{
		Key: "party_branch_name", Label: "支部名称", Type: "string",
		Description: "党支部简称，保留当前空间全部支部", Example: "示例学生党支部",
	},
	{
		Key: "full_members", Label: "正式党员", Type: "integer",
		Description: "在编且发展阶段为 full_member 的人数", Example: "34",
	},
	{
		Key: "probationary_members", Label: "预备党员", Type: "integer",
		Description: "在编且发展阶段为 probationary_member 的人数", Example: "6",
	},
	{
		Key: "total", Label: "合计", Type: "integer",
		Description: "正式党员与预备党员人数之和", Example: "40",
	},
	{
		Key: "secretaries", Label: "支部书记", Type: "string",
		Description: "在编人员中职务为支部书记、党支部书记或书记的姓名，去重后以英文逗号拼接",
		Example:     "虚构学生甲",
	},
	{
		Key: "organization_members", Label: "组织委员", Type: "string",
		Description: "在编组织委员姓名，去重后以英文逗号拼接，未填写为空", Example: "虚构学生乙",
	},
	{
		Key: "publicity_members", Label: "宣传委员", Type: "string",
		Description: "在编宣传委员姓名，去重后以英文逗号拼接，未填写为空", Example: "虚构学生丙",
	},
	{
		Key: "discipline_members", Label: "纪律委员", Type: "string",
		Description: "在编人员中纪检委员或纪律委员姓名，去重后以英文逗号拼接，未填写为空",
		Example:     "虚构学生丁",
	},
	{
		Key: "advisors", Label: "导师姓名", Type: "string",
		Description: "支部内全部在编人员的导师姓名，去除空值并去重，以英文逗号拼接",
		Example:     "虚构导师甲,虚构导师乙",
	},
}
