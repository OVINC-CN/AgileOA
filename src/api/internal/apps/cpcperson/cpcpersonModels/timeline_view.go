package cpcpersonModels

import (
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

type TimelineViewData struct {
	// TemplateID 是当前读取范围内人员已绑定的模板编号。
	TemplateID uint64 `json:"template_id" example:"1"`
	// TemplateName 是时间线模板名称。
	TemplateName string `json:"template_name" example:"发展工作时间线"`
	// Nodes 是按模板顺序排列的节点及完整字段目录。
	Nodes []cpctimelineModels.Node `json:"nodes"`
}

type TimelineViewPerson struct {
	// Person 是符合筛选条件及读取范围的人员基础信息。
	Person Info `json:"person"`
	// Values 仅包含所选节点的当前字段，学生引用实时解析，缺失字段不返回。
	Values map[string]any `json:"values"`
	// Status 是 pending、completed 或 skipped。
	Status string `json:"status" example:"pending"`
	// StatusLabel 是节点处理状态的中文名称。
	StatusLabel string `json:"status_label" example:"待处理"`
}

type TimelineViewListData struct {
	// CanCreate 表示当前账号具有人员新增权限及新增范围。
	CanCreate bool `json:"can_create" example:"true"`
	// CanImport 表示当前账号具有人员导入权限及新增范围。
	CanImport bool `json:"can_import" example:"true"`
	// CanExport 表示当前账号具有人员导出权限及读取范围。
	CanExport bool `json:"can_export" example:"true"`
	// Items 按人员编号倒序排列。
	Items []TimelineViewPerson `json:"items"`
	// Total 是当前模板、筛选及读取范围内的人员总数。
	Total int64 `json:"total" example:"10"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是当前每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type TimelineViewStatisticsData struct {
	// Total 是当前模板、筛选及读取范围内的人员总数，不受分页影响。
	Total int64 `json:"total" example:"10"`
	// Completed 是所选节点已完成的人数。
	Completed int64 `json:"completed" example:"4"`
	// Pending 是所选节点待处理的人数。
	Pending int64 `json:"pending" example:"5"`
	// Skipped 是所选节点已跳过的人数。
	Skipped int64 `json:"skipped" example:"1"`
}

type TimelineViewResponse struct {
	response.Envelope[TimelineViewData]
}
type TimelineViewListResponse struct {
	response.Envelope[TimelineViewListData]
}
type TimelineViewStatisticsResponse struct {
	response.Envelope[TimelineViewStatisticsData]
}
