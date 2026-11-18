package projectModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Project 是按归属年份维护的项目。
type Project struct {
	ID                uint64 `gorm:"primaryKey"`
	WorkspaceID       string
	Name              string
	Year              int
	Description       string
	AllowSelfApply    bool
	SelfApplyDeadline *time.Time
	ScoreCalculation  *string
	Revision          uint64
	CreatedAt         time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt         time.Time `gorm:"->;autoUpdateTime:false"`
}

// Participation 保存学生参与记录，排名、总分和子项分别录入。
type Participation struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	ProjectID   uint64
	StudentID   uint64
	Rank        *int32
	Score       *float64
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定参与记录表。
func (Participation) TableName() string { return "project_participations" }

type ScoreItem struct {
	// Name 是评分子项名称，只能选择所属项目已配置的名称。
	Name string `json:"name" example:"志愿服务"`
	// Description 是子项的详细说明。
	Description string `json:"description" example:"参加校内志愿活动"`
	// Score 是可选的双精度分数，null 表示未填写。
	Score *float64 `json:"score" example:"5.5" extensions:"x-nullable"`
}

type ProjectInput struct {
	// Name 是项目名称，同名项目可按不同年份分别维护。
	Name string `json:"name" example:"优秀学生"`
	// Year 是项目归属年份，参与记录及人数统计均使用此年份。
	Year int `json:"year" example:"2026"`
	// Description 是项目说明。
	Description string `json:"description" example:"年度评选项目"`
	// DetailNames 是当前项目配置的可选子项名称，参与评分时只能从中选择。
	DetailNames []string `json:"detail_names" example:"志愿服务,社会实践" gorm:"-"`
	// AllowSelfApply 表示是否允许在籍学生新建申报，不影响已有申报的提交和审批流转。
	AllowSelfApply bool `json:"allow_self_apply" example:"false"`
	// SelfApplyDeadline 是自行申报截止时间，开放申报时必填；截止后 1 分钟自动提交草稿。
	SelfApplyDeadline *time.Time `json:"self_apply_deadline" example:"2026-12-31T15:59:00Z" extensions:"x-nullable"`
	// ApprovalSteps 是按顺序执行的审批节点，变更后未完成申报从新流程的第一级重新审批。
	ApprovalSteps []ApprovalStepInput `json:"approval_steps" gorm:"-"`
	// ScoreCalculation 是学生申报的条件和公式配置，编辑省略或 null 时保留原配置。
	ScoreCalculation *ScoreCalculation `json:"score_calculation" gorm:"-"`
}

type ProjectRequest struct {
	ProjectInput
	// Revision 是编辑时必填的当前版本，新增时为零。
	Revision uint64 `json:"revision" example:"1"`
}

type ProjectInfo struct {
	ProjectInput
	// AttachmentIDs 是所属记录中已完成的附件编号。
	AttachmentIDs []uint64 `json:"attachment_ids" example:"1,2"`
	// ID 是项目编号。
	ID uint64 `json:"id" example:"1"`
	// Revision 是并发修改版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-01-01T00:00:00Z"`
	// UpdatedAt 是更新时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-01-01T00:00:00Z"`
}

type ParticipationFields struct {
	// ProjectID 是当前工作空间中的项目编号。
	ProjectID uint64 `json:"project_id" example:"1"`
	// StudentID 是当前工作空间中的学生编号。
	StudentID uint64 `json:"student_id" example:"1"`
	// Rank 是独立填写的学生排名，正整数，null 表示未填写。
	Rank *int32 `json:"rank" example:"1" minimum:"1" maximum:"2147483647" extensions:"x-nullable"`
	// Score 是可选双精度总分，独立填写，不由子项求和。
	Score *float64 `json:"score" example:"90.5" extensions:"x-nullable"`
}

type ParticipationInput struct {
	ParticipationFields
	// Details 是评分子项列表，名称须在所属项目中配置且在本条记录内唯一，保留顺序，可为空数组。
	Details []ScoreItem `json:"details"`
}

type ParticipationRequest struct {
	ParticipationInput
	// Revision 是编辑时必填的当前版本，新增时为零。
	Revision uint64 `json:"revision" example:"1"`
}

type ParticipationSummary struct {
	ParticipationFields
	// DetailCount 是评分子项数量，列表不加载详情正文。
	DetailCount int64 `json:"detail_count" example:"2"`
	// ID 是参与记录编号。
	ID uint64 `json:"id" example:"1"`
	// ProjectName 是项目名称。
	ProjectName string `json:"project_name" example:"优秀学生"`
	// Year 是项目归属年份。
	Year int `json:"year" example:"2026"`
	// StudentNo 是学生学号。
	StudentNo string `json:"student_no" example:"20260001"`
	// StudentName 是学生姓名。
	StudentName string `json:"student_name" example:"虚构学生"`
	// EducationLevel 是学生当前学历。
	EducationLevel string `json:"education_level" example:"博士"`
	// Major 是学生当前专业。
	Major string `json:"major" example:"物流工程与管理"`
	// Grade 是学生当前年级。
	Grade string `json:"grade" example:"2021级"`
	// Revision 是并发修改版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-01-01T00:00:00Z"`
	// UpdatedAt 是更新时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-01-01T00:00:00Z"`
}

type ParticipationInfo struct {
	ParticipationSummary
	// Details 是按录入顺序读取的独立表子项，详情按需加载。
	Details []ScoreItem `json:"details"`
}

// Detail 是独立存储的评分子项，修改及移除由参与记录的并发版本保护。
type Detail struct {
	ID              uint64 `gorm:"primaryKey"`
	WorkspaceID     string
	ParticipationID uint64
	ProjectID       uint64
	SortOrder       int
	Name            string
	Description     string
	Score           *float64
	CreatedAt       time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt       time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定评分子项表。
func (Detail) TableName() string { return "project_participation_details" }

// DetailOption 是按项目维护的子项名称，Svc 显式校验已使用的名称不可移除。
type DetailOption struct {
	WorkspaceID string    `gorm:"primaryKey"`
	ProjectID   uint64    `gorm:"primaryKey;autoIncrement:false"`
	Name        string    `gorm:"primaryKey"`
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定项目子项配置表。
func (DetailOption) TableName() string { return "project_detail_options" }

type NameOption struct {
	// Value 是当前项目配置的子项名称，参与评分时不可新增名称。
	Value string `json:"value" example:"志愿服务"`
	// Label 是子项的中文显示名称。
	Label string `json:"label" example:"志愿服务"`
}

type NameOptionsResponse struct {
	response.Envelope[[]NameOption]
}

type Query struct {
	pagination.Query
	Keyword         string
	ProjectNames    []string
	DetailKeyword   string
	Student         string
	ProjectID       uint64
	Years           []int
	EducationLevels []string
	Majors          []string
	Grades          []string
}

type ProjectList struct {
	// Items 按项目名称、归属年份升序及编号倒序排列。
	Items []ProjectInfo `json:"items"`
	// Total 是符合条件的项目总数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type ParticipationList struct {
	// Items 按项目名称、归属年份、排名升序及编号倒序排列，空排名排在已填写排名前。
	Items []ParticipationSummary `json:"items"`
	// Total 是符合条件的参与记录总数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type Statistic struct {
	// Name 是跨年份保持一致的项目名称，即折线系列。
	Name string `json:"name" example:"优秀学生"`
	// Year 是项目归属年份。
	Year int `json:"year" example:"2026"`
	// Count 是该年份该项目的参加学生人数，未参加时为零。
	Count int64 `json:"count" example:"10"`
}

type Option struct {
	// Value 是当前空间内的学生或项目编号。
	Value uint64 `json:"value" example:"1"`
	// Label 使用学生姓名(学号)格式，或项目年份及名称。
	Label string `json:"label" example:"虚构学生(20260001)"`
}

type Field struct {
	// Resource 是固定资源代码。
	Resource string `json:"resource" example:"project"`
	// Name 是固定字段代码。
	Name string `json:"name" example:"year"`
	// Label 是字段中文说明。
	Label string `json:"label" example:"归属年份"`
	// Type 是字段类型。
	Type string `json:"type" example:"integer"`
	// Required 表示是否必须填写。
	Required bool `json:"required" example:"true"`
	// Example 是字段示例文本。
	Example string `json:"example" example:"2026"`
}

type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportData struct {
	// Created 是本次新增记录数，任一行失败整批回滚。
	Created int `json:"created" example:"1"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"projects.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ProjectResponse struct{ response.Envelope[ProjectInfo] }
type ProjectListResponse struct{ response.Envelope[ProjectList] }
type ParticipationResponse struct {
	response.Envelope[ParticipationInfo]
}
type ParticipationListResponse struct {
	response.Envelope[ParticipationList]
}
type StatisticsResponse struct{ response.Envelope[[]Statistic] }
type OptionsResponse struct{ response.Envelope[[]Option] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
type ImportResponse struct{ response.Envelope[ImportData] }

// FilterOption 是项目筛选枚举的实际值与显示名称。
type FilterOption struct {
	// Value 是项目名称、四位归属年份或学生基础字段值，空值由前端作为全部选项提供。
	Value string `json:"value" example:"2026"`
	// Label 是筛选项显示名称。
	Label string `json:"label" example:"2026 年"`
}

type FilterOptionsData struct {
	// Names 是当前工作空间完整的去重项目名称，按名称升序排列。
	Names []FilterOption `json:"names"`
	// Years 是当前工作空间完整的去重项目归属年份，按年份降序排列。
	Years []FilterOption `json:"years"`
	// EducationLevels 是当前工作空间学生的去重学历，仅获奖学生筛选返回选项。
	EducationLevels []FilterOption `json:"education_levels"`
	// Majors 是当前工作空间学生的去重专业，仅获奖学生筛选返回选项。
	Majors []FilterOption `json:"majors"`
	// Grades 是当前工作空间学生的去重年级，仅获奖学生筛选返回选项。
	Grades []FilterOption `json:"grades"`
}

type FilterOptionsResponse struct {
	response.Envelope[FilterOptionsData]
}
