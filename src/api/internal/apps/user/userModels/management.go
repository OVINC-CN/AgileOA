package userModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ListQuery 是固定用户目录的搜索条件。
type ListQuery struct {
	pagination.Query
	Keyword  string
	Statuses []string
}

// UpdateRequest 只允许更新显示名称、邮箱、启用状态、密码登录设置和可选的新口令摘要。
type UpdateRequest struct {
	// DisplayName 是更新后的显示名称。
	DisplayName string `json:"display_name" validate:"required,max=100" example:"张三"`
	// Email 是可选的联系邮箱，最多 254 个字符；省略时保留原值，空字符串清空。
	Email *string `json:"email,omitempty" validate:"omitempty,max=254" example:"zhangsan@example.com"`
	// IsEnabled 表示更新后是否允许登录。
	IsEnabled *bool `json:"is_enabled" validate:"required" example:"true"`
	// PasswordLoginEnabled 表示是否允许密码登录，省略时保留原值。
	PasswordLoginEnabled *bool `json:"password_login_enabled,omitempty" example:"true"`
	// PasswordSHA256 是临时口令摘要，首次启用密码登录时必填，禁止密码登录时须省略。
	PasswordSHA256 string `json:"password_sha256,omitempty" validate:"omitempty,len=64,hexadecimal" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
}

// Row 是用户管理表格的固定字段目录，不包含认证凭据。
type Row struct {
	Info
	// IsEnabled 表示账号是否启用。
	IsEnabled bool `json:"is_enabled" example:"true"`
	// CreatedAt 是账号创建时间，使用 UTC 时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T01:00:00Z"`
	// UpdatedAt 是账号最近修改时间，使用 UTC 时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T01:00:00Z"`
	// LastLoginAt 是最近成功登录时间，从审计读取；未登录时为空。
	LastLoginAt *time.Time `json:"last_login_at" extensions:"x-nullable" example:"2026-11-03T02:00:00Z"`
	// SessionCount 是保护账号删除的会话引用数量，包含已失效会话。
	SessionCount int64 `json:"session_count" example:"1"`
	// HasAPIToken 表示存在 API Token，删除账号前必须显式撤销。
	HasAPIToken bool `json:"has_api_token" example:"false"`
}

// ListData 是服务端分页的用户目录。
type ListData struct {
	// Items 是当前页用户。
	Items []Row `json:"items"`
	// Total 是符合查询条件的账号总数。
	Total int64 `json:"total" example:"36"`
	// Page 是当前页码，从 1 开始。
	Page int `json:"page" example:"1"`
	// PageSize 是每页数量。
	PageSize int `json:"page_size" example:"10"`
}

// ListResponse 复用统一响应字段。
type ListResponse struct{ response.Envelope[ListData] }

// Day 是真实每日统计，不使用预置演示数据。
type Day struct {
	// Date 是上海日期。
	Date string `json:"date" example:"2026-11-03"`
	// Created 是当天新增账号数量。
	Created int64 `json:"created" example:"3"`
	// Logins 是当天成功登录次数，旋转会话只计一次。
	Logins int64 `json:"logins" example:"12"`
}

// StatisticsData 是用户管理顶部的真实统计。
type StatisticsData struct {
	// Total 是全部账号数量。
	Total int64 `json:"total" example:"36"`
	// Enabled 是启用账号数量。
	Enabled int64 `json:"enabled" example:"32"`
	// Disabled 是停用账号数量。
	Disabled int64 `json:"disabled" example:"4"`
	// Admins 是系统管理员数量。
	Admins int64 `json:"admins" example:"1"`
	// Logins 是统计窗口内成功登录次数。
	Logins int64 `json:"logins" example:"83"`
	// Days 是连续的每日新增账号和登录统计。
	Days []Day `json:"days" gorm:"-"`
}

// StatisticsResponse 复用统一响应字段。
type StatisticsResponse struct {
	response.Envelope[StatisticsData]
}
