package partybranchModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Branch 保存工作空间内的党支部，书记关联账号作为人员范围依据，不自动授予操作权限。
type Branch struct {
	ID            uint64 `gorm:"primaryKey"`
	WorkspaceID   string
	FullName      string
	ShortName     string
	Priority      int32
	SecretaryID   *uint64
	AdmissionRule *string
	Revision      uint64
	CreatedAt     time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt     time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定党支部表名。
func (Branch) TableName() string { return "cpc_party_branches" }

type Input struct {
	// FullName 是党支部全称，最多 200 字符，同一空间内唯一。
	FullName string `json:"full_name" example:"中共示例学院计算机专业学生第一支部委员会"`
	// ShortName 是党支部简称，最多 100 字符，同一空间内唯一。
	ShortName string `json:"short_name" example:"计算机学生第一党支部"`
	// SecretaryID 是当前空间可用用户的编号，null 表示未配置或清空书记。
	SecretaryID *uint64 `json:"secretary_id" example:"1" extensions:"x-nullable"`
}

type Request struct {
	Input
	// Priority 是有符号 32 位整数，数字越大越优先；新增省略时为 0，编辑省略时保留原值。
	Priority *int32 `json:"priority,omitempty" example:"100" minimum:"-2147483648" maximum:"2147483647"`
	// AdmissionRule 是可选规则更新，省略时保留原规则，传入 rule:null 时清空。
	AdmissionRule *AdmissionRuleChange `json:"admission_rule,omitempty"`
	// Revision 是当前并发版本，新增时为零，编辑时必填。
	Revision uint64 `json:"revision" example:"1"`
}

type Info struct {
	Input
	// Priority 是党支部匹配和列表排序使用的优先级，数字越大越优先。
	Priority int32 `json:"priority" example:"100"`
	// ID 是数据库自增的党支部编号。
	ID uint64 `json:"id" example:"1"`
	// SecretaryName 是书记的用户显示名称，未配置时为空。
	SecretaryName string `json:"secretary_name" example:"张三"`
	// SecretaryUsername 是书记的登录账号，未配置时为空。
	SecretaryUsername string `json:"secretary_username" example:"zhangsan"`
	// Revision 是修改及删除时使用的并发版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-05T00:00:00Z"`
	// UpdatedAt 是最后修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-05T00:00:00Z"`
}

type Query struct {
	pagination.Query
	Keyword string
}

type ListData struct {
	// Items 是当前空间内按优先级倒序、同优先级按编号倒序排列的党支部。
	Items []Info `json:"items"`
	// Total 是符合搜索条件的总条数。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"10"`
}

type SecretaryOption struct {
	// Value 是用户表中的账号编号。
	Value uint64 `json:"value" example:"1"`
	// Label 使用显示名称(登录账号)格式。
	Label string `json:"label" example:"张三(zhangsan)"`
}

type Field struct {
	// Name 是固定字段代码。
	Name string `json:"name" example:"full_name"`
	// Label 是字段的中文名称及限制说明。
	Label string `json:"label" example:"党支部全称（最多 200 字符）"`
	// Type 是字段类型。
	Type string `json:"type" example:"string"`
	// Required 表示是否必须填写。
	Required bool `json:"required" example:"true"`
	// ReadOnly 表示字段由系统生成，不接受写入。
	ReadOnly bool `json:"read_only" example:"false"`
	// Example 是字段示例。
	Example string `json:"example" example:"中共示例学院计算机专业学生第一支部委员会"`
}

type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type ImportData struct {
	// Created 是本次新增党支部数，任一行失败整批回滚。
	Created int `json:"created" example:"1"`
}

type ReorderData struct {
	// Queued 表示后台重排任务已提交，当前空间没有在编人员时为 false。
	Queued bool `json:"queued" example:"true"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"party-branches.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type InfoResponse struct{ response.Envelope[Info] }
type ListResponse struct{ response.Envelope[ListData] }
type SecretariesResponse struct {
	response.Envelope[[]SecretaryOption]
}
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
type ImportResponse struct{ response.Envelope[ImportData] }
type ReorderResponse struct{ response.Envelope[ReorderData] }
