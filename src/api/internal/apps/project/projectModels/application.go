package projectModels

import (
	"encoding/json"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ApprovalStepInput 定义一级审批的多选处理人及签署方式。
type ApprovalStepInput struct {
	// Kind 是班委、申报人、学生用户或组织用户对应的英文代码。
	Kind string `json:"kind" example:"class_position" enums:"class_position,applicant,student,user"`
	// SignMode 是 any（或签）或 all（并签），同一处理人重复匹配时仅需签署一次。
	SignMode string `json:"sign_mode" example:"any" enums:"any,all"`
	// AllowEdit 控制本级处理人能否修改申报内容及附件；新建时省略默认开启，编辑时省略保留。
	AllowEdit *bool `json:"allow_edit" example:"true" default:"true"`
	// ClassPositions 是多选班委任职，仅 class_position 节点填写。
	ClassPositions []string `json:"class_positions" example:"班长,团支书"`
	// StudentIDs 是同空间学生用户的编号，仅 student 节点填写。
	StudentIDs []uint64 `json:"student_ids" example:"1,2"`
	// UserIDs 是当前空间拥有 access_workspace 权限的启用组织用户编号，仅 user 节点填写。
	UserIDs []uint64 `json:"user_ids" example:"1,2"`
	// ReviewerLabel 是服务端生成的处理人显示名称。
	ReviewerLabel string `json:"reviewer_label" example:"班长、团支书"`
	// ReviewerOptions 是已选对象的显示名称，只读且不参与授权。
	ReviewerOptions []NameOption `json:"reviewer_options"`
}

// ApprovalStep 保存流程配置或申报快照中的处理人引用。
type ApprovalStep struct {
	WorkspaceID   string `gorm:"primaryKey"`
	ProjectID     uint64 `gorm:"primaryKey;autoIncrement:false"`
	Position      int    `gorm:"primaryKey;autoIncrement:false"`
	TargetOrder   int    `gorm:"primaryKey;autoIncrement:false"`
	SignMode      string
	AllowEdit     bool
	Kind          string
	ClassPosition string
	StudentID     *uint64
	UserID        *uint64
	ReviewerLabel string
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定项目流程配置表。
func (ApprovalStep) TableName() string { return "project_approval_steps" }

// Application 保存一名学生在一个项目中的唯一申报单。
type Application struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	ProjectID   uint64
	StudentID   uint64
	Statement   string
	Rank        *int32
	Score       *float64
	Status      string
	CurrentStep int
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定申报表。
func (Application) TableName() string { return "project_applications" }

// ApplicationNode 保存申报流程，项目流程变更时同步重建未完成申报的节点。
type ApplicationNode struct {
	WorkspaceID   string `gorm:"primaryKey"`
	ApplicationID uint64 `gorm:"primaryKey;autoIncrement:false"`
	Position      int    `gorm:"primaryKey;autoIncrement:false"`
	TargetOrder   int    `gorm:"primaryKey;autoIncrement:false"`
	SignMode      string
	AllowEdit     bool
	Kind          string
	ClassPosition string
	StudentID     *uint64
	UserID        *uint64
	ReviewerLabel string
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定申报流程表。
func (ApplicationNode) TableName() string { return "project_application_nodes" }

// ApplicationDetail 独立保存申报评分，复用项目子项目录。
type ApplicationDetail struct {
	ID            uint64 `gorm:"primaryKey"`
	WorkspaceID   string
	ApplicationID uint64
	ProjectID     uint64
	SortOrder     int
	DetailKey     string
	Name          string
	Description   string
	Score         *float64
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定申报子项表。
func (ApplicationDetail) TableName() string { return "project_application_details" }

// ApplicationHistory 保存不可修改的流转记录及当时的处理人身份快照。
type ApplicationHistory struct {
	// ID 是只追加的流转记录编号。
	ID            uint64 `gorm:"primaryKey" json:"id" example:"1"`
	WorkspaceID   string `                  json:"-"`
	ApplicationID uint64 `                  json:"-"`
	// Step 是操作时的节点，零表示申报人填报。
	Step int `json:"step" example:"1"`
	// Operation 是 draft、submit、update、approve、reject 或项目流程变更产生的 reset。
	Operation string `json:"operation" example:"approve" enums:"draft,submit,update,approve,reject,reset"`
	// ActorType 是 student、user 或自动提交使用的 system。
	ActorType string `json:"actor_type" example:"student" enums:"student,user,system"`
	// ActorID 是对应身份类型下的编号，仅作为历史快照；system 固定为零。
	ActorID uint64 `json:"actor_id" example:"1"`
	// ActorName 是操作时的处理人名称。
	ActorName string `json:"actor_name" example:"虚构学生"`
	// Comment 是审批说明，通过时选填，驳回时必填。
	Comment string `json:"comment" example:"请补充志愿服务说明"`
	// CreatedAt 是数据库记录的操作时间。
	CreatedAt time.Time `gorm:"->;autoCreateTime:false" json:"created_at"`
}

// TableName 固定只追加的流转记录表。
func (ApplicationHistory) TableName() string { return "project_application_history" }

type ApplicationValues struct {
	// Statement 是可选的申报说明，最多 10000 字符。
	Statement string `json:"statement" example:"申请年度优秀学生"`
	// Rank 是申报排名，留空为 null；启用总分计算时忽略写请求的排名，由后台重排已通过申报。
	Rank *int32 `json:"rank" example:"1" extensions:"x-nullable"`
	// Score 是总分；启用计算后忽略手填值，公式结果四舍五入到两位小数。
	Score *float64 `json:"score" example:"90.5" extensions:"x-nullable"`
}

type ApplicationDetailInput struct {
	ScoreItem
	// Key 是子项在申报中的稳定标识，32 位小写十六进制，附件按该标识归档。
	Key string `json:"key" example:"0123456789abcdef0123456789abcdef"`
}

type ApplicationContent struct {
	ApplicationValues
	// Details 是已配置的评分子项，名称和稳定标识在本张申报单内均不能重复。
	Details []ApplicationDetailInput `json:"details"`
}

type ApplicationDetailInfo struct {
	ApplicationDetailInput
	// ID 是数据库子项编号，编辑时使用稳定 key 关联。
	ID uint64 `json:"id" example:"1"`
	// Attachments 是该申报子项的私有附件，审批完成后继续保留在原申报中。
	Attachments []attachmentModels.Info `json:"attachments"`
}

type ApplicationRequest struct {
	ApplicationContent
	// RawRank、RawScore 保存入口提取的值，供 Svc 在读取项目配置后选择解析或忽略。
	RawRank  json.RawMessage `json:"-" swaggerignore:"true"`
	RawScore json.RawMessage `json:"-" swaggerignore:"true"`
	// SaveAsDraft 表示先保存草稿以取得附件归档所需的申报编号。
	SaveAsDraft bool `json:"save_as_draft" example:"false"`
	// ProjectID 仅新建申报时填写，申报人由可信学生身份确定。
	ProjectID uint64 `json:"project_id" example:"1"`
	// Revision 是当前并发版本，新建时为零。
	Revision uint64 `json:"revision" example:"1"`
	// Comment 是审批说明，通过时选填，驳回时必须填写，最多 10000 字符。
	Comment string `json:"comment" example:"材料已核对"`
}

// ApplicationPayload 延迟总分、排名的类型校验，其他字段沿用固定请求模型。
type ApplicationPayload struct {
	ApplicationRequest
	Rank  json.RawMessage `json:"rank"`
	Score json.RawMessage `json:"score"`
}

type ApplicationStepInfo struct {
	ApprovalStepInput
	// Position 是节点顺序，从一级开始。
	Position int `json:"position" example:"1"`
	// Reviewers 是去重后的实际处理人及本轮签署状态。
	Reviewers []ApplicationReviewer `json:"reviewers"`
	// Ready 表示班委并签要求的全部任职当前均有可处理人员。
	Ready bool `json:"ready" example:"true"`
}

type ApplicationSummary struct {
	// ID 是申报编号。
	ID uint64 `json:"id" example:"1"`
	// ProjectID 是项目编号。
	ProjectID uint64 `json:"project_id" example:"1"`
	// ProjectName 是项目名称。
	ProjectName string `json:"project_name" example:"优秀学生"`
	// AllowSelfApply 仅决定是否可新建申报，关闭不影响已有草稿提交及审批流转。
	AllowSelfApply bool `json:"allow_self_apply" example:"true"`
	// Year 是项目归属年份。
	Year int `json:"year" example:"2026"`
	// StudentID 是申报学生编号。
	StudentID uint64 `json:"student_id" example:"1"`
	// StudentNo 是申报人学号。
	StudentNo string `json:"student_no" example:"20260001"`
	// StudentName 是申报人姓名。
	StudentName string `json:"student_name" example:"虚构学生"`
	// ClassName 是申报人当前班级。
	ClassName string `json:"class_name" example:"2026级一班"`
	// Status 是 draft、pending、rejected 或 approved。
	Status string `json:"status" example:"pending" enums:"draft,pending,rejected,approved"`
	// CurrentStep 是当前待处理节点，零表示退回申报人。
	CurrentStep int `json:"current_step" example:"1"`
	// CurrentReviewer 是当前待处理节点的名称，完成时为空。
	CurrentReviewer string `json:"current_reviewer" example:"班长"`
	// CanEdit 表示当前身份可修改申报内容及附件，审批级别关闭编辑时为 false。
	CanEdit bool `json:"can_edit" example:"true"`
	// CanApprove 表示当前身份可提交或通过当前节点。
	CanApprove bool `json:"can_approve" example:"true"`
	// CanReject 表示当前身份可将单据直接退回申报人。
	CanReject bool `json:"can_reject" example:"true"`
	// Revision 是并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是数据库维护的创建时间。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 是数据库维护的更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

type ApplicationInfo struct {
	// ProjectDescription 是项目当前说明，项目修改后实时应用于已有申报。
	ProjectDescription string `json:"project_description" example:"年度评选项目"`
	// ScoreCalculationEnabled 开启后申报和审核编辑不提供总分、排名输入框。
	ScoreCalculationEnabled bool `json:"score_calculation_enabled" example:"false"`
	// SelfApplyDeadline 是项目当前的自行申报截止时间，草稿在截止后 1 分钟自动提交。
	SelfApplyDeadline *time.Time `json:"self_apply_deadline" example:"2026-12-31T15:59:00Z" extensions:"x-nullable"`
	// ApprovalRecords 是审计使用的当前签署行，不直接输出到接口。
	ApprovalRecords []ApplicationApproval `json:"-"`
	ApplicationSummary
	ApplicationValues
	// Details 是包含独立附件的评分子项，审批完成后仍保留在原申报中。
	Details []ApplicationDetailInfo `json:"details"`
	// Steps 是当前审批流，未完成申报随项目流程变更重置，已通过申报保留最终流程。
	Steps []ApplicationStepInfo `json:"steps"`
	// DetailNames 是当前项目可选的子项名称。
	DetailNames []string `json:"detail_names"`
	// History 按操作顺序保存提交、修改、通过、驳回和流程重置记录。
	History []ApplicationHistory `json:"history"`
}

type ApplicationQuery struct {
	pagination.Query
	Student      string
	ProjectNames []string
	Years        []int
	Statuses     []string
}

type ApplicationList struct {
	// Items 按申报编号倒序排列。
	Items []ApplicationSummary `json:"items"`
	// Total 是符合筛选的总数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"20"`
}

type ApprovalOptions struct {
	// Items 是当前种类的数据库枚举，班委为任职名称，其他为账号编号字符串。
	Items []NameOption `json:"items"`
}

type ApplicationResponse struct {
	response.Envelope[ApplicationInfo]
}
type ApplicationListResponse struct {
	response.Envelope[ApplicationList]
}
type ApprovalOptionsResponse struct {
	response.Envelope[ApprovalOptions]
}

// ApplicationApproval 仅保存当前轮次有效的通过记录，内容修改或退回后明确清空相关轮次。
type ApplicationApproval struct {
	WorkspaceID   string `gorm:"primaryKey"`
	ApplicationID uint64 `gorm:"primaryKey;autoIncrement:false"`
	Position      int    `gorm:"primaryKey;autoIncrement:false"`
	ActorType     string `gorm:"primaryKey"`
	ActorID       uint64 `gorm:"primaryKey;autoIncrement:false"`
	StudentID     *uint64
	UserID        *uint64
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

func (ApplicationApproval) TableName() string { return "project_application_approvals" }

type ApplicationReviewer struct {
	// IdentityType 是 student 或 user，避免两类编号互相冒用。
	IdentityType string `json:"identity_type" example:"student"`
	// ID 是对应身份类型下的编号。
	ID uint64 `json:"id" example:"1"`
	// Label 使用处理人姓名(学号)或显示名称(登录账号)格式。
	Label string `json:"label" example:"虚构学生(20260001)"`
	// Approved 表示本轮已通过，内容变更或退回会使之前的签署失效。
	Approved bool `json:"approved" example:"false"`
}
