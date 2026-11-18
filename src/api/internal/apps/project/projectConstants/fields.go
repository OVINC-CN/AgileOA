package projectConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"

// 字段、表名、类别和边界由代码固定，外部输入不能选择数据库列。
const (
	MinYear            = 1900
	MaxYear            = 9999
	NameLength         = 100
	DescriptionLength  = 10000
	ProjectOrder       = "name ASC, year ASC, id DESC"
	ParticipationOrder = "p.name ASC, p.year ASC, r.rank ASC, r.id DESC"
	ExportBasic        = "basic"
	ExportFull         = "full"
)

// Fields 提供项目与评分明细的固定中文字段目录及示例。
var Fields = []projectModels.Field{
	{
		Resource: "project",
		Name:     "score_calculation",
		Label:    "学生申报总分计算配置，条件仅支持年级、学历、专业、班级的 in 及 AND、OR 组",
		Type:     "object",
		Example:  "{\"enabled\":false,\"ranking_dimensions\":[],\"rules\":[]}",
	},
	{
		Resource: "project",
		Name:     "self_apply_deadline",
		Label:    "自行申报截止时间，开放申报时必填；截止后 1 分钟自动提交草稿",
		Type:     "datetime",
		Example:  "2026-12-31T15:59:00Z",
	},
	{
		Resource: "project",
		Name:     "allow_self_apply",
		Label:    "允许学生新建申报，关闭不影响已有单据提交及流转",
		Type:     "boolean",
		Example:  "false",
	},
	{
		Resource: "project",
		Name:     "approval_steps",
		Label:    "按顺序执行的多级审批流",
		Type:     "array",
		Example:  "[{\"kind\":\"class_position\",\"sign_mode\":\"any\",\"class_positions\":[\"班长\"],\"student_ids\":[],\"user_ids\":[]}]",
	},
	{
		Resource: "application",
		Name:     "project_id",
		Label:    "申报项目编号（同一学生同一项目仅一单）",
		Type:     "integer",
		Required: true,
		Example:  "1",
	},
	{
		Resource: "application",
		Name:     "statement",
		Label:    "申报说明（最多 10000 字符）",
		Type:     "string",
		Example:  "申请年度优秀学生",
	},
	{
		Resource: "application",
		Name:     "rank",
		Label:    "排名，可空；启用总分计算后忽略传入值，已通过申报由后台分组排名",
		Type:     "integer",
		Example:  "1",
	},
	{
		Resource: "application",
		Name:     "score",
		Label:    "总分，可空；启用总分计算后忽略手填值并按公式计算",
		Type:     "double",
		Example:  "90.5",
	},
	{
		Resource: "application",
		Name:     "details",
		Label:    "项目配置的评分子项（单张申报内名称唯一）",
		Type:     "array",
		Example:  "[]",
	},
	{
		Resource: "application",
		Name:     "status",
		Label:    "审批状态（只读）",
		Type:     "string",
		Example:  "pending",
	},
	{
		Resource: "application",
		Name:     "current_step",
		Label:    "当前节点，零为申报人（只读）",
		Type:     "integer",
		Example:  "1",
	},
	{
		Resource: "application",
		Name:     "comment",
		Label:    "审批说明，通过选填、驳回必填，最多 10000 字符",
		Type:     "string",
		Example:  "请补充材料",
	},

	{
		Resource: "project",
		Name:     "name",
		Label:    "项目名称（最多 100 字符）",
		Type:     "string",
		Required: true,
		Example:  "优秀学生",
	},
	{
		Resource: "project",
		Name:     "year",
		Label:    "项目归属年份（1900 至 9999）",
		Type:     "integer",
		Required: true,
		Example:  "2026",
	},
	{
		Resource: "project",
		Name:     "description",
		Label:    "项目说明（最多 10000 字符）",
		Type:     "string",
		Example:  "年度评选",
	},
	{
		Resource: "project",
		Name:     "detail_names",
		Label:    "可选子项名称（每项最多 100 字符，评分时仅允许选择）",
		Type:     "array",
		Example:  "[\"志愿服务\",\"社会实践\"]",
	},
	{
		Resource: "project",
		Name:     "attachment_ids",
		Label:    "当前记录已完成的附件编号（只读）",
		Type:     "array",
		Example:  "[1,2]",
	},
	{
		Resource: "participation",
		Name:     "project_id",
		Label:    "项目编号",
		Type:     "integer",
		Required: true,
		Example:  "1",
	},
	{
		Resource: "participation",
		Name:     "student_id",
		Label:    "学生编号",
		Type:     "integer",
		Required: true,
		Example:  "1",
	},
	{
		Resource: "participation",
		Name:     "rank",
		Label:    "排名（独立录入，可空，1 至 2147483647 的正整数）",
		Type:     "integer",
		Example:  "1",
	},
	{
		Resource: "participation",
		Name:     "score",
		Label:    "总分（独立录入，可空，双精度数字）",
		Type:     "double",
		Example:  "90.50",
	},
	{
		Resource: "participation",
		Name:     "details",
		Label:    "评分子项列表（单条获奖记录内名称唯一）",
		Type:     "array",
		Example:  "[]",
	},
	{
		Resource: "detail",
		Name:     "name",
		Label:    "子项名称（仅可选择当前项目配置，最多 100 字符）",
		Type:     "string",
		Required: true,
		Example:  "志愿服务",
	},
	{
		Resource: "detail",
		Name:     "description",
		Label:    "子项说明（最多 10000 字符）",
		Type:     "string",
		Example:  "参加志愿活动",
	},
	{Resource: "detail", Name: "score", Label: "子项分数（可空，双精度数字）", Type: "double", Example: "5.50"},
}
