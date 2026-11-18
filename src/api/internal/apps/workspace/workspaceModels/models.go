package workspaceModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Workspace 是租户边界，不携带角色或隐含业务权限。
type Workspace struct {
	ID          string `gorm:"primaryKey"`
	Name        string
	Description string
	IsEnabled   bool
	Revision    uint64
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// Info 是工作空间固定的公开字段目录。
type Info struct {
	// ID 是同时包含英文小写字母与数字的 16 位随机编号，也是租户编号。
	ID string `json:"id" example:"w7m2k9p4s6n8c3x5"`
	// Name 是唯一的工作空间显示名称。
	Name string `json:"name" example:"教学事务"`
	// Description 是工作空间用途说明。
	Description string `json:"description" example:"处理日常教学事务"`
	// IsEnabled 表示工作空间是否启用，停用后所有用户无法进入。
	IsEnabled bool `json:"is_enabled" example:"true"`
	// Revision 是并发修改校验版本，修改后递增。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是创建时间，使用 UTC。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T00:00:00Z"`
	// UpdatedAt 是最近修改时间，使用 UTC。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T00:00:00Z"`
}

// CreateRequest 只接收工作空间基本信息，创建时不自动生成授权。
type CreateRequest struct {
	// Name 是唯一的工作空间显示名称，最多 100 个字符。
	Name string `json:"name" validate:"required,max=100" example:"教学事务"`
	// Description 是用途说明，最多 500 个字符。
	Description string `json:"description" validate:"max=500" example:"处理日常教学事务"`
}

// UpdateRequest 使用明确的启用状态与版本校验更新基本信息。
type UpdateRequest struct {
	CreateRequest
	// IsEnabled 是更新后的工作空间状态，必须明确提交。
	IsEnabled *bool `json:"is_enabled" validate:"required" example:"true"`
	// Revision 必须与最近读取的工作空间版本一致。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
}

// ListQuery 仅允许分页、名称关键词及固定状态筛选。
type ListQuery struct {
	pagination.Query
	Keyword  string
	Statuses []string
}

// ListData 是工作空间分页结果。
type ListData struct {
	// Items 是本页工作空间，空结果为数组。
	Items []Info `json:"items"`
	// Total 是符合条件的工作空间数量。
	Total int64 `json:"total" example:"1"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是当前每页数量。
	PageSize int `json:"page_size" example:"20"`
}

// InfoResponse 复用统一响应字段。
type InfoResponse struct{ response.Envelope[Info] }

// ListResponse 复用统一响应字段。
type ListResponse struct{ response.Envelope[ListData] }

// EnumOption 是工作空间选择器使用的最小固定字段目录。
type EnumOption struct {
	// Value 是工作空间的 16 位随机编号。
	Value string `json:"value" example:"w7m2k9p4s6n8c3x5"`
	// Label 是工作空间显示名称。
	Label string `json:"label" example:"教学事务"`
	// IsEnabled 表示工作空间是否启用，停用空间仍可由管理员配置权限。
	IsEnabled bool `json:"is_enabled" example:"true"`
}

// EnumResponse 一次返回完整枚举，不接收分页或搜索参数。
type EnumResponse struct {
	response.Envelope[[]EnumOption]
}
