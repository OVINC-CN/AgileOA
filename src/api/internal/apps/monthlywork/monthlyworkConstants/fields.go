package monthlyworkConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"

const (
	MinYear           = 1900
	MaxYear           = 9999
	MinDate           = "1900-01-01"
	MaxDate           = "9999-12-31"
	NameLength        = 100
	DescriptionLength = 10000
	URLLength         = 2048
)

// Fields 提供固定字段、中文说明及示例。
var Fields = []monthlyworkModels.Field{
	{Name: "date", Label: "日期（YYYY-MM-DD）", Type: "date", Required: true, Example: "2026-11-03"},
	{Name: "work_item", Label: "工作项目（最多 100 字符）", Type: "string", Required: true, Example: "学生座谈会"},
	{Name: "owner_id", Label: "负责人用户编号（当前空间启用用户）", Type: "integer", Required: true, Example: "1"},
	{
		Name:    "participant_ids",
		Label:   "参与人用户编号（当前空间启用用户，可多选）",
		Type:    "array",
		Example: "[2,3]",
	},
	{Name: "description", Label: "详细说明（最多 10000 字符）", Type: "string", Example: "本月工作安排"},
	{Name: "attachment_ids", Label: "当前记录已完成的附件编号（只读）", Type: "array", Example: "[1,2]"},
	{
		Name:    "links",
		Label:   "外部链接（名称和 HTTP/HTTPS 地址）",
		Type:    "array",
		Example: `[{"name":"工作材料","url":"https://example.com/work"}]`,
	},
}
