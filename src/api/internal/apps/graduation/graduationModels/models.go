package graduationModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Input 维护毕业去向，空类型代表待就业，自定义类型使用英文代码。
type Input struct {
	// GraduationYear 是毕业年份，使用四位年份文本，留空时统计到未填写毕业年份。
	GraduationYear string `json:"graduation_year" example:"2026"`
	// Type 是类型代码；新建自定义类型时留空，由服务端根据名称生成稳定代码。
	Type string `json:"type" example:""`
	// TypeLabel 是类型名称，最多 100 字符；空字符串表示待就业。
	TypeLabel string `json:"type_label" example:""`
	// Destination 是毕业去向，最多 255 字符，待就业时可留空。
	Destination string `json:"destination" example:"虚构单位"`
}

type Request struct {
	Input
	// Revision 是最近读取的学生版本，毕业去向和学生信息共用版本。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
}

// Info 仅选择毕业去向所需的基础字段，不加载敏感信息。
type Info struct {
	Input
	// StudentID 是当前工作空间的学生编号。
	StudentID uint64 `json:"student_id" example:"1"`
	// StudentNo 是学生学号。
	StudentNo string `json:"student_no" example:"S20260001"`
	// StudentName 是学生姓名。
	StudentName string `json:"student_name" example:"虚构学生"`
	// EducationLevel 是学生学历。
	EducationLevel string `json:"education_level" example:"本科"`
	// Major 是学生专业。
	Major string `json:"major" example:"虚构专业"`
	// Grade 是学生年级。
	Grade string `json:"grade" example:"2026级"`
	// Revision 是学生信息共用的并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是学生记录的创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T00:00:00Z"`
	// UpdatedAt 是最近修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T00:00:00Z"`
}

type MutationData struct {
	// StudentID 是本次维护的学生编号。
	StudentID uint64 `json:"student_id" example:"1"`
	// Revision 是维护后的学生版本。
	Revision uint64 `json:"revision" example:"2"`
}

type Query struct {
	pagination.Query
	Keyword         string
	EducationLevels []string
	Majors          []string
	Grades          []string
	// GraduationYears 为空时查询全部年份，空字符串选项对应未设置。
	GraduationYears []string
	// Types 为空时查询全部类型，空字符串选项对应待就业。
	Types []string
}

type ListData struct {
	// Items 是学生及其毕业去向，包含尚未填写的学生，空结果为数组。
	Items []Info `json:"items"`
	// Total 是筛选后的学生总数。
	Total int64 `json:"total" example:"100"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type Option struct {
	// Value 是实际字段值；毕业去向类型使用英文代码。
	Value string `json:"value" example:""`
	// Label 是显示名称。
	Label string `json:"label" example:"待就业"`
}

type FilterOptions struct {
	// EducationLevels 是当前空间学生学历的完整去重枚举。
	EducationLevels []Option `json:"education_levels"`
	// Majors 是当前空间学生专业的完整去重枚举。
	Majors []Option `json:"majors"`
	// Grades 是当前空间学生年级的完整去重枚举。
	Grades []Option `json:"grades"`
	// GraduationYears 是当前空间已填写毕业年份的完整去重枚举。
	GraduationYears []Option `json:"graduation_years"`
	// Types 是当前空间已保存的毕业去向类型，不建立枚举配置表。
	Types []Option `json:"types"`
}

type Statistic struct {
	// GraduationYear 是毕业年份，空字符串表示未填写毕业年份，独立归组。
	GraduationYear string `json:"graduation_year" example:"2026"`
	// Value 是维度值，未填写时为空字符串。
	Value string `json:"value" example:"虚构专业"`
	// Label 是维度的显示名称。
	Label string `json:"label" example:"虚构专业"`
	// Total 是该维度下筛选范围内的学生总数，包含未填写的学生。
	Total int64 `json:"total" example:"100"`
	// Employed 是类型非空的学生数量。
	Employed int64 `json:"employed" example:"80"`
	// Pending 是待就业学生数量，包含尚未绑定毕业去向的学生。
	Pending int64 `json:"pending" example:"15"`
	// EmploymentRate 是已就业人数除以总人数的比例，取值为 0 至 1。
	EmploymentRate float64 `json:"employment_rate" example:"0.8"`
}

type Field struct {
	// Name 是固定字段代码。
	Name string `json:"name" example:"destination"`
	// Label 是字段的中文名称。
	Label string `json:"label" example:"毕业去向"`
	// MaxLength 是字段最多允许的字符数。
	MaxLength int `json:"max_length" example:"255"`
	// Required 表示绑定去向时是否必填。
	Required bool `json:"required" example:"false"`
	// Example 是虚构示例。
	Example string `json:"example" example:"虚构单位"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"graduation-destinations.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" validate:"required" example:"UEsDBAoAAAAAAA=="`
}

type ImportData struct {
	// Created 是本次新增绑定数量，任一行失败全部回滚。
	Created int `json:"created" example:"1"`
}

type ListResponse struct{ response.Envelope[ListData] }
type MutationResponse struct {
	response.Envelope[MutationData]
}
type FilterOptionsResponse struct {
	response.Envelope[FilterOptions]
}
type StatisticsResponse struct{ response.Envelope[[]Statistic] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
type ImportResponse struct{ response.Envelope[ImportData] }
