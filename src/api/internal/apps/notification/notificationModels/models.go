package notificationModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Notification 保存个人通知，来源标识仅用于后台幂等投递。
type Notification struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64
	SourceID  string
	Title     string
	Content   string
	URL       string
	ReadAt    *time.Time
	CreatedAt time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"->;autoUpdateTime:false"`
}

// Delivery 由后台业务提供，接收人不作为执行任务的用户审计身份。
type Delivery struct {
	UserID   uint64 `json:"user_id"`
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	URL      string `json:"url"`
}

// Info 是通知固定的对外字段目录，不包含接收人及内部来源标识。
type Info struct {
	// ID 是通知编号。
	ID uint64 `json:"id" example:"1"`
	// Title 是通知标题，最多 200 个字符。
	Title string `json:"title" example:"重排人员失败"`
	// Content 是通知正文，以纯文本展示。
	Content string `json:"content" example:"存在未匹配党支部的人员，已保留全部原归属，请修正规则后重新提交。"`
	// URL 是可选的站内跳转地址，最多 2048 个字符，空字符串表示没有链接。
	URL string `json:"url" example:"/cpc/roster?workspace=w7m2k9p4s6n8c3x5"`
	// ReadAt 是已读时间，未读时为 null。
	ReadAt *time.Time `json:"read_at" example:"2026-11-08T08:00:00Z"`
	// CreatedAt 是通知创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-08T07:00:00Z"`
}

// UnreadData 返回当前接收人的全部未读通知数量。
type UnreadData struct {
	// UnreadCount 是当前接收人的未读通知总数。
	UnreadCount int64 `json:"unread_count" example:"2"`
}

// ListQuery 只允许未读或全部通知，默认仅查询未读。
type ListQuery struct {
	pagination.Query
	// Status 是通知范围，固定为 unread（未读通知）或 all（全部通知）。
	Status string
}

// ListData 按未读优先、组内编号倒序分页返回个人通知。
type ListData struct {
	UnreadData
	// Items 是当前页的通知，空列表返回空数组。
	Items []Info `json:"items"`
	// Total 是当前接收人在所选范围内的通知总数。
	Total int64 `json:"total" example:"12"`
	// Page 是从 1 开始的页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"5"`
}

// ReadRequest 只允许将明确指定的个人通知标为已读，数量不超过 YAML 单页上限。
type ReadRequest struct {
	// IDs 是不重复的通知编号，必须全部属于当前接收人。
	IDs []uint64 `json:"ids" validate:"required,min=1,dive,gt=0" example:"1,2"`
}

// ReadData 返回实际变更数与最新未读总数，重复标记不会修改已读时间。
type ReadData struct {
	UnreadData
	// Changed 是本次从未读改为已读的通知数量。
	Changed int64 `json:"changed" example:"2"`
}

// ListResponse 返回通知列表。
type ListResponse struct {
	response.Envelope[ListData]
}

// UnreadResponse 返回未读总数。
type UnreadResponse struct {
	response.Envelope[UnreadData]
}

// ReadResponse 返回标记已读结果。
type ReadResponse struct {
	response.Envelope[ReadData]
}

// EmptyData 表示失败响应没有业务数据。
type EmptyData struct{}

// EmptyResponse 用于通知参数错误及不可访问的响应。
type EmptyResponse struct {
	response.Envelope[*EmptyData]
}
