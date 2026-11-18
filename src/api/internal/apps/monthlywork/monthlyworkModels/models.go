package monthlyworkModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Record 保存工作日期、负责人和多个外部链接，日期不做时区转换。
type Record struct {
	ID          uint64 `gorm:"primaryKey"`
	WorkspaceID string
	// Category 区分学生工作和党建工作，业务接口不允许修改。
	Category    string `gorm:"default:student"`
	Date        string
	WorkItem    string
	OwnerID     uint64
	Description string
	Links       string
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定月度工作记录表。
func (Record) TableName() string { return "monthly_work_records" }

// Participant 保存记录自有的参与人明细，不参与授权推导。
type Participant struct {
	WorkspaceID   string    `gorm:"primaryKey"`
	MonthlyWorkID uint64    `gorm:"primaryKey;autoIncrement:false"`
	UserID        uint64    `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定月度工作参与人表。
func (Participant) TableName() string { return "monthly_work_participants" }

type Link struct {
	// Name 是可选的链接名称，留空时显示地址。
	Name string `json:"name" example:"工作材料"`
	// URL 是完整的 HTTP 或 HTTPS 外部链接，不接受内嵌账号口令。
	URL string `json:"url" example:"https://example.com/work"`
}

type Input struct {
	// Date 是工作日期，格式为 YYYY-MM-DD，按此日期归入月份。
	Date string `json:"date" example:"2026-11-03"`
	// WorkItem 是工作项目名称，最多 100 字符。
	WorkItem string `json:"work_item" example:"学生座谈会"`
	// OwnerID 是当前工作空间可用用户的负责人编号，关联用户表。
	OwnerID uint64 `json:"owner_id" example:"1"`
	// ParticipantIDs 是参与人用户编号，可多选；编辑省略或传 null 时保留，空数组清空。
	ParticipantIDs []uint64 `json:"participant_ids" example:"2,3"`
	// Description 是可选的详细说明，最多 10000 字符，支持换行。
	Description string `json:"description" example:"记录本月学生座谈会的安排及完成情况"`
	// Links 是按录入顺序展示的外部链接，可为空数组。
	Links []Link `json:"links"`
}

type Request struct {
	Input
	// Revision 是编辑时必填的当前版本，新增时为零。
	Revision uint64 `json:"revision" example:"1"`
}

type File struct {
	// ID 是当前工作记录中可下载的附件编号。
	ID uint64 `json:"id" example:"1"`
	// Filename 是原始文件名。
	Filename string `json:"filename" example:"工作材料.pdf"`
	// Size 是文件字节数。
	Size int64 `json:"size" example:"1024"`
}

type Info struct {
	Input
	// Participants 是按用户编号升序返回的参与人身份，不授予任何业务权限。
	Participants []ParticipantInfo `json:"participants"`
	// AttachmentIDs 是所属记录中已完成的附件编号。
	AttachmentIDs []uint64 `json:"attachment_ids" example:"1,2"`
	// Attachments 是可下载附件的元数据，仅在具备附件读取权限时返回，否则为空数组。
	Attachments []File `json:"attachments"`
	// OwnerName 是负责人的用户显示名称。
	OwnerName string `json:"owner_name" example:"虚构负责人"`
	// OwnerUsername 是负责人的登录账号，仅用于区分同名用户。
	OwnerUsername string `json:"owner_username" example:"fictional.user"`
	// ID 是工作记录编号。
	ID uint64 `json:"id" example:"1"`
	// Revision 是并发修改版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T00:00:00Z"`
	// UpdatedAt 是更新时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T00:00:00Z"`
}

type ParticipantInfo struct {
	// ID 是参与人的用户编号。
	ID uint64 `json:"id" example:"2"`
	// Name 是参与人的用户显示名称。
	Name string `json:"name" example:"虚构参与人"`
	// Username 是参与人的登录账号，用于区分同名用户。
	Username string `json:"username" example:"fictional.participant"`
}

type Query struct {
	Years   []int
	Months  []int
	Keyword string
	// Scopes 仅包含 mine 时查询当前用户负责或参与的工作，为空或含 all 时查询全部工作。
	Scopes []string
}

type ListData struct {
	// Items 是所选年份与月份范围内的全部记录，按日期和编号倒序排列，不分页。
	Items []Info `json:"items"`
	// Total 是符合筛选条件的全部记录数。
	Total int64 `json:"total" example:"1"`
}

// FilterOption 提供当前空间已有年份的固定筛选项。
type FilterOption struct {
	// Value 是四位年份代码。
	Value string `json:"value" example:"2026"`
	// Label 是年份显示名称。
	Label string `json:"label" example:"2026 年"`
}

type FilterOptionsData struct {
	// Years 是当前空间完整去重的已有年份，按年份倒序排列。
	Years []FilterOption `json:"years"`
}

type FilterOptionsResponse struct {
	response.Envelope[FilterOptionsData]
}

type Field struct {
	// Name 是固定字段代码，不能用于选择任意数据库列。
	Name string `json:"name" example:"work_item"`
	// Label 是字段中文说明。
	Label string `json:"label" example:"工作项目"`
	// Type 是字段类型。
	Type string `json:"type" example:"string"`
	// Required 表示是否必填。
	Required bool `json:"required" example:"true"`
	// Example 是字段示例文本。
	Example string `json:"example" example:"学生座谈会"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"monthly-work.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportData struct {
	// Created 是本次新增的记录数量，任一行失败整批回滚。
	Created int `json:"created" example:"1"`
}

type InfoResponse struct{ response.Envelope[Info] }
type ListResponse struct{ response.Envelope[ListData] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
type ImportResponse struct{ response.Envelope[ImportData] }

// OwnerOption 仅枚举当前空间可访问的启用账号，不泄露其他用户资料。
type OwnerOption struct {
	// Value 是用户表中的账号编号。
	Value uint64 `json:"value" example:"1"`
	// Label 使用显示名称(登录账号)格式。
	Label string `json:"label" example:"虚构负责人(fictional.user)"`
}

type OwnersResponse struct {
	response.Envelope[[]OwnerOption]
}
