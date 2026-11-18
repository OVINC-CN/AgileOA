package studentdashboardConstants

import "github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardModels"

// AccommodationFields 按住宿信息表及导出列顺序定义只读字段。
var AccommodationFields = []studentdashboardModels.Field{
	{
		Key: "accommodation_type", Label: "住宿类型", Type: "enum",
		Description: "在籍学生住宿类型；on_campus 为校内、off_campus 为校外、joint_abroad 为出国联培、unknown 为未知",
		Example:     "on_campus",
	},
	{
		Key: "campus", Label: "住宿园区", Type: "string",
		Description: "校内住宿的园区，去除两端空格；非校内住宿统一为空，不单独聚合",
		Example:     "虚构园区",
	},
	{
		Key: "building", Label: "宿舍楼", Type: "string",
		Description: "校内住宿的宿舍楼，去除两端空格；非校内住宿统一为空，不单独聚合",
		Example:     "1号楼",
	},
	{
		Key: "room", Label: "宿舍号", Type: "string",
		Description: "校内住宿的宿舍号，按文本保留前导零；非校内住宿统一为空，不单独聚合",
		Example:     "0101",
	},
	{
		Key: "count", Label: "人数", Type: "integer",
		Description: "当前住宿分组内的在籍学生人数", Example: "4",
	},
}
