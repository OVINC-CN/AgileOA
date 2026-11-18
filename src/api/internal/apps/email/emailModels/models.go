package emailModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// CallbackRequest 对应腾讯云邮件通知事件，不接收 IP 或关联历史发信记录。
type CallbackRequest struct {
	// Event 是腾讯云事件代码，异常事件范围由 YAML 配置。
	Event string `json:"event" validate:"required" example:"bounce"`
	// Email 是收件人地址，黑名单统一去除首尾空白并转换为小写。
	Email string `json:"email" validate:"required,max=254" example:"recipient@example.com"`
	// Link 是点击链接；保存时移除用户信息、查询参数及片段，避免保存登录码。
	Link string `json:"link,omitempty" validate:"max=4096" example:"https://example.com/login/email"`
	// BulkID 是 SendEmail 返回的 MessageId，仅作为回调字段保存。
	BulkID string `json:"bulkId" validate:"required,max=255" example:"qcloudses-example-message"`
	// Timestamp 是事件发生时的 Unix 秒时间戳。
	Timestamp int64 `json:"timestamp" validate:"required,gt=0" example:"1794508800"`
	// Reason 是投递失败原因。
	Reason string `json:"reason,omitempty" validate:"max=4096" example:"551 5.1.1 recipient is not exist"`
	// BounceType 是退信类型，取值为 soft_bounce 或 hard_bounce。
	BounceType string `json:"bounceType,omitempty" validate:"omitempty,oneof=soft_bounce hard_bounce" example:"hard_bounce"`
	// Username 是腾讯云账号对应的 AppID。
	Username string `json:"username,omitempty" validate:"max=64" example:"1234567890"`
	// From 是不含别名的发信地址。
	From string `json:"from,omitempty" validate:"max=254" example:"login@example.com"`
	// FromDomain 是发信域名。
	FromDomain string `json:"fromDomain,omitempty" validate:"max=253" example:"example.com"`
	// TemplateID 是腾讯云模板编号。
	TemplateID uint64 `json:"templateId,omitempty" example:"123456"`
	// UserAgent 是用户打开邮件或点击链接时的设备信息。
	UserAgent string `json:"useragent,omitempty" validate:"max=4096" example:"Mozilla/5.0"`
	// Subject 是邮件主题，非 ASCII 内容可能经过 MIME 编码。
	Subject string `json:"subject,omitempty" validate:"max=2048" example:"登录敏捷办公"`
	// MessageID 是 SMTP 协议头中的 Message-ID。
	MessageID string `json:"messageId,omitempty" validate:"max=255" example:"example-message@example.com"`
	// EventID 是事件编号，省略时根据事件代码补充。
	EventID int `json:"eventid,omitempty" validate:"min=0,max=8" example:"3"`
	// TaskID 是腾讯云批量发送任务编号。
	TaskID uint64 `json:"taskId,omitempty" example:"1"`
	// SentTimestamp 是邮件发送时的 Unix 秒时间戳。
	SentTimestamp int64 `json:"sentTimestamp,omitempty" validate:"gte=0" example:"1794508790"`
}

// EmailCallback 独立保存标准回调字段，重复通知由内容摘要去重。
type EmailCallback struct {
	ID              uint64 `gorm:"primaryKey"`
	EventKey        string
	CallbackRequest `gorm:"embedded"`
	CreatedAt       time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt       time.Time `gorm:"->;autoUpdateTime:false"`
}

// EmailBlacklist 保存永久禁发邮箱及首次触发原因，不依赖账号或业务记录。
type EmailBlacklist struct {
	Email     string `gorm:"primaryKey"`
	Event     string
	Reason    string
	CreatedAt time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"->;autoUpdateTime:false"`
}

// EmptyData 表示邮件回调不返回业务数据。
type EmptyData struct{}

// EmptyResponse 返回统一响应信封，成功和失败的 data 均为 null。
type EmptyResponse struct {
	response.Envelope[*EmptyData]
}
