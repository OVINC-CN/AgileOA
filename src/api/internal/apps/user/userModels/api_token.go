package userModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// APIToken 保存每个账号唯一的访问凭据，密文仅供授权后的再次查看。
type APIToken struct {
	UserID     uint64 `gorm:"primaryKey"`
	TokenHash  string `                  json:"-"`
	Ciphertext []byte `                  json:"-"`
	// SessionVersion 仅记录生成时的会话版本，不参与 API Token 认证。
	SessionVersion uint64    `json:"-"`
	CreatedAt      time.Time `         gorm:"->;autoCreateTime:false"`
	UpdatedAt      time.Time `         gorm:"->;autoUpdateTime:false"`
}

// TableName 固定 API Token 表名。
func (APIToken) TableName() string { return "user_api_tokens" }

// Identity 是认证中间件确认的请求身份，不保存原始凭据。
type Identity struct {
	User       User             `gorm:"embedded"`
	Session    Session          `gorm:"embedded;embeddedPrefix:session_"`
	IsAPIToken bool             `gorm:"-"`
	Student    *StudentIdentity `gorm:"-"`
}

// APITokenData 是管理员查看或生成凭据后的响应，未配置时 Token 为空。
type APITokenData struct {
	// Token 是完整的 API Token，仅在本次抽屉中展示，不写入浏览器存储。
	Token string `json:"token" example:"awt_example"`
	// CreatedAt 是首次生成时间，未配置时为空。
	CreatedAt *time.Time `json:"created_at" extensions:"x-nullable" example:"2026-11-11T01:00:00Z"`
	// UpdatedAt 是最近重置时间，未配置时为空。
	UpdatedAt *time.Time `json:"updated_at" extensions:"x-nullable" example:"2026-11-11T02:00:00Z"`
}

// APITokenResponse 复用统一响应协议。
type APITokenResponse struct {
	response.Envelope[APITokenData]
}
