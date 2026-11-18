package studentdashboardModels

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"

// AccommodationQuery 仅允许按住宿固定字段的具体值筛选，同一字段取并集。
type AccommodationQuery struct {
	AccommodationTypes []string
	Campuses           []string
	Buildings          []string
	Rooms              []string
}

// Option 分别提供稳定字段值与中文显示名称。
type Option struct {
	// Value 是住宿类型英文代码或住宿位置原值，位置空字符串表示未填写。
	Value string `json:"value" example:"on_campus"`
	// Label 是中文显示名称，位置空值显示半角短横线。
	Label string `json:"label" example:"校内"`
}

// AccommodationSummary 保存一组住宿信息及其在籍学生人数。
type AccommodationSummary struct {
	// AccommodationType 是住宿类型的英文代码及中文名称。
	AccommodationType Option `json:"accommodation_type"`
	// Campus 是住宿园区，非校内住宿统一为空。
	Campus string `json:"campus" example:"虚构园区"`
	// Building 是宿舍楼，非校内住宿统一为空。
	Building string `json:"building" example:"1号楼"`
	// Room 是宿舍号，非校内住宿统一为空。
	Room string `json:"room" example:"0101"`
	// Count 是当前住宿分组内的在籍学生人数。
	Count int64 `json:"count" example:"4"`
}

type AccommodationData struct {
	// Items 按住宿类型、园区、楼栋及宿舍号汇总，非校内类型各自只保留一行。
	Items []AccommodationSummary `json:"items"`
	// Total 是当前筛选下全部住宿分组的在籍学生人数之和。
	Total int64 `json:"total" example:"40"`
}

type AccommodationOptions struct {
	// AccommodationTypes 是当前空间在籍学生中实际存在的住宿类型。
	AccommodationTypes []Option `json:"accommodation_types"`
	// Campuses 是当前空间在籍校内住宿学生的完整园区选项。
	Campuses []Option `json:"campuses"`
	// Buildings 是当前空间在籍校内住宿学生的完整宿舍楼选项。
	Buildings []Option `json:"buildings"`
	// Rooms 是当前空间在籍校内住宿学生的完整宿舍号选项。
	Rooms []Option `json:"rooms"`
}

type AccommodationResponse struct {
	response.Envelope[AccommodationData]
}
type AccommodationOptionsResponse struct {
	response.Envelope[AccommodationOptions]
}
