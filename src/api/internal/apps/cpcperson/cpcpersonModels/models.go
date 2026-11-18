package cpcpersonModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Person 仅保存当前空间的党建专有信息，基础信息通过学生关联读取。
type Person struct {
	ID                 uint64 `gorm:"primaryKey"`
	WorkspaceID        string
	StudentID          uint64
	Stage              string
	Status             string
	PartyBranchID      *uint64
	PartyPosition      string
	TimelineTemplateID *uint64
	TimelineStatus     *string
	Revision           uint64
	CreatedAt          time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt          time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定党建人员表名。
func (Person) TableName() string { return "cpc_people" }

// Input 是允许维护的全部党建人员字段。
type Input struct {
	// StudentID 是当前空间学生编号，新增必填，编辑时不得更换。
	StudentID uint64 `json:"student_id" example:"1"`
	// Stage 是发展阶段的固定英文代码。
	Stage string `json:"stage" example:"applicant"`
	// Status 是自定义的党建人员状态，最多 8 字符，新增省略或留空时默认为在编。
	Status string `json:"status" maxLength:"8" example:"在编"`
	// PartyPosition 是可自定义的党内职务，最多 64 字符，空字符串表示未填写。
	PartyPosition string `json:"party_position" maxLength:"64" example:"组织委员"`
	// TimelineTemplateID 是绑定的时间线模板编号，新增必填，存量未绑定人员可为空。
	TimelineTemplateID *uint64 `json:"timeline_template_id" extensions:"x-nullable" example:"1"`
}

// UpdateRequest 只接受人员资料，时间线模板通过独立绑定入口维护。
type UpdateRequest struct {
	// StudentID 是关联学生编号，编辑时不能更换。
	StudentID uint64 `json:"student_id" example:"1"`
	// Stage 是发展阶段英文代码。
	Stage string `json:"stage" example:"applicant"`
	// Status 是自定义的党建人员状态，最多 8 字符，省略或留空时保留原值。
	Status string `json:"status" maxLength:"8" example:"在编"`
	// PartyPosition 是自定义党内职务，最多 64 字符。
	PartyPosition string `json:"party_position" maxLength:"64" example:"组织委员"`
	// Revision 是当前人员版本。
	Revision uint64 `json:"revision" example:"1"`
}

type Request struct {
	Input
	// Revision 是当前并发版本，新增时为零，编辑时必填。
	Revision uint64 `json:"revision" example:"1"`
}

// StudentInfo 是党建读取所需的固定学生基础字段，不包含入学政治面貌及敏感字段。
type StudentInfo struct {
	// StudentNo 是由学生信息维护的学号。
	StudentNo string `json:"student_no" example:"S20260001"`
	// Name 是由学生信息维护的姓名。
	Name string `json:"name" example:"虚构学生甲"`
	// Gender 是性别。
	Gender string `json:"gender" example:"女"`
	// Ethnicity 是民族。
	Ethnicity string `json:"ethnicity" example:"汉族"`
	// Grade 是年级。
	Grade string `json:"grade" example:"2026级"`
	// StudentStatus 是学生状态，与党建发展阶段独立。
	StudentStatus string `json:"student_status" example:"在籍"`
	// EducationLevel 是学历。
	EducationLevel string `json:"education_level" example:"硕士"`
	// ClassName 是班级。
	ClassName string `json:"class_name" example:"2026级一班"`
	// Major 是专业。
	Major string `json:"major" example:"虚构专业"`
	// Advisor 是导师姓名。
	Advisor string `json:"advisor" example:"虚构导师甲"`
	// Phone 是联系电话。
	Phone string `json:"phone" example:"虚构电话"`
	// ArrivalMonth 是来校年月，未填写时为空。
	ArrivalMonth string `json:"arrival_month" example:"2026-09"`
	// PreviousSchoolOrEmployer 是毕（肄）业学校及单位。
	PreviousSchoolOrEmployer string `json:"previous_school_or_employer" example:"虚构大学"`
	// PreviousGraduationMonth 是既往毕（肄）业年月，未填写时为空。
	PreviousGraduationMonth string `json:"previous_graduation_month" example:"2026-06"`
}

type Info struct {
	Input
	StudentInfo
	// PartyBranchID 是规则自动确定的必填党支部编号，不接受客户端写入。
	PartyBranchID uint64 `json:"party_branch_id" example:"1"`
	// ID 是党建人员记录编号。
	ID uint64 `json:"id" example:"1"`
	// PartyBranchName 是自动确定的所属党支部全称。
	PartyBranchName string `json:"party_branch_name" example:"中共示例支部委员会"`
	// PartyBranchShortName 是自动确定的所属党支部简称。
	PartyBranchShortName string `json:"party_branch_short_name" example:"示例支部"`
	// Revision 是修改及删除使用的并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CanUpdate 表示当前账号同时具备编辑权限及此人员的编辑范围。
	CanUpdate bool `json:"can_update" example:"true"`
	// CanDelete 表示当前账号同时具备删除权限及此人员的删除范围。
	CanDelete bool `json:"can_delete" example:"false"`
	// TimelineTemplateName 是已绑定模板名称，未绑定时为空。
	TimelineTemplateName string `json:"timeline_template_name" example:"发展工作时间线"`
	// TimelinePending 是待处理节点数量。
	TimelinePending int64 `json:"timeline_pending" example:"3"`
	// TimelineCompleted 是已完成节点数量。
	TimelineCompleted int64 `json:"timeline_completed" example:"1"`
	// TimelineSkipped 是已跳过节点数量，和已完成节点一起计入已处理进度。
	TimelineSkipped int64 `json:"timeline_skipped" example:"1"`
	// TimelineStatus 是 normal（正常）、abnormal（异常）或 transferring（转移中）。
	TimelineStatus *string `json:"timeline_status" extensions:"x-nullable" example:"normal"`
	// TimelineTransferTargetID 是待审核转移的目标模板编号，无待审核转移时为 null。
	TimelineTransferTargetID *uint64 `json:"timeline_transfer_target_id" extensions:"x-nullable" example:"2"`
	// CanReviewTimelineTransfer 表示具有此人员的转移审核权限及支部范围。
	CanReviewTimelineTransfer bool `json:"can_review_timeline_transfer" example:"false"`
	// CreatedAt 是记录创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-06T00:00:00Z"`
	// UpdatedAt 是记录修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-06T00:00:00Z"`
}

type Query struct {
	pagination.Query
	Keyword         string
	Filters         map[string][]string
	TimelineFilters []TimelineFilter
	Format          string
	TemplateID      uint64
	NodeID          uint64
	NodeStatuses    []string
}

type ListData struct {
	// CanCreate 表示当前账号具有新增权限及新增范围。
	CanCreate bool `json:"can_create" example:"true"`
	// CanImport 表示当前账号具有导入权限及新增范围。
	CanImport bool `json:"can_import" example:"true"`
	// CanExport 表示当前账号具有导出权限及读取范围。
	CanExport bool `json:"can_export" example:"false"`
	// Items 是已按人员权限及书记范围过滤的名单。
	Items []Info `json:"items"`
	// Total 是范围内符合筛选条件的数量。
	Total int64 `json:"total" example:"10"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是当前每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type Option struct {
	// Value 是固定英文代码、编号或用户自定义文本。
	Value string `json:"value" example:"applicant"`
	// Label 是中文名称或用户自定义文本。
	Label string `json:"label" example:"入党申请人"`
}

type StageCount struct {
	Option
	// Count 是当前读取范围与筛选条件下该阶段的人员数。
	Count int64 `json:"count" example:"10"`
}

type StatisticsData struct {
	// Total 是当前范围及筛选条件下的人员总数。
	Total int64 `json:"total" example:"50"`
	// Stages 固定返回五种发展阶段，无人员的阶段计数为零。
	Stages []StageCount `json:"stages"`
}

type Candidate struct {
	StudentInfo
	// Value 是可新增到当前可操作支部名单中的学生编号。
	Value uint64 `json:"value" example:"1"`
	// Label 使用姓名(学号)格式。
	Label string `json:"label" example:"虚构学生甲(S20260001)"`
	// PartyBranchID 是全部规则按优先级倒序、同值按编号倒序匹配的党支部，不接受手工修改。
	PartyBranchID uint64 `json:"party_branch_id" example:"1"`
	// PartyBranchName 是自动确定的党支部名称。
	PartyBranchName string `json:"party_branch_name" example:"示例支部"`
}

type Field struct {
	// Key 是固定字段代码。
	Key string `json:"key" example:"stage"`
	// Label 是中文字段名称。
	Label string `json:"label" example:"发展阶段"`
	// Type 是字段类型：string、integer 或 enum。
	Type string `json:"type" example:"enum"`
	// Required 表示新增是否必填。
	Required bool `json:"required" example:"true"`
	// ReadOnly 表示基础信息只允许在学生信息维护。
	ReadOnly bool `json:"read_only" example:"false"`
	// Example 是填写示例。
	Example string `json:"example" example:"applicant"`
	// Options 是固定英文代码及中文名称，动态字段为空数组。
	Options []Option `json:"options"`
}

type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportData struct {
	// Created 是事务提交后新增的人员数。
	Created int `json:"created" example:"1"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"cpc-people-full.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type InfoResponse struct{ response.Envelope[Info] }
type ListResponse struct{ response.Envelope[ListData] }
type OptionsResponse struct{ response.Envelope[[]Option] }
type CandidatesResponse struct{ response.Envelope[[]Candidate] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type ImportResponse struct{ response.Envelope[ImportData] }
type FileResponse struct{ response.Envelope[FileData] }
type StatisticsResponse struct {
	response.Envelope[StatisticsData]
}
