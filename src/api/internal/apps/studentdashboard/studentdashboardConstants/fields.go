package studentdashboardConstants

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardModels"

const MonitorPosition = "班长"

// Fields 定义只读字段目录，专业人数按响应中的专业列展开展示和导出。
var Fields = []studentdashboardModels.Field{
	{
		Key: "class_name", Label: "班级", Type: "string",
		Description: "在籍学生的班级名称，去除首尾空白后汇总；历史空班级仍保留", Example: "2026级一班",
	},
	{
		Key: "major_counts", Label: "专业人数", Type: "object",
		Description: "以专业名称为键的在籍人数，按 majors 升序展开；空专业保留，缺少的键按零计数",
		Example:     `{"会计学":25,"审计学":15}`,
	},
	{
		Key: "total", Label: "总数", Type: "integer",
		Description: "当前班级全部专业的在籍学生人数之和", Example: "40",
	},
	{
		Key: "monitors", Label: "班长", Type: "string",
		Description: "在籍学生中任职包含独立班长职务的姓名，不包含副班长；去重排序后以英文逗号拼接",
		Example:     "虚构学生甲,虚构学生乙",
	},
}
