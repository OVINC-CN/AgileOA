package userModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// User 仅保存全局账号，不携带租户业务权限。
type User struct {
	ID          uint64 `gorm:"primaryKey"`
	Username    string
	DisplayName string
	// Email 是可选的联系邮箱，未填写时为空字符串。
	Email        string
	PasswordHash string `json:"-"`
	// PasswordLoginEnabled 表示账号是否允许使用密码登录。
	PasswordLoginEnabled bool
	// EmailLoginVerified 表示当前登记邮箱已在邮件验证码登录时通过用户名核验。
	EmailLoginVerified bool `json:"-"`
	// PasswordChangeRequired 表示账号使用初始密码或管理员重置的临时密码。
	PasswordChangeRequired bool
	SessionVersion         uint64 `gorm:"default:1"               json:"-"`
	IsAdmin                bool
	IsEnabled              bool
	CreatedAt              time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt              time.Time `gorm:"->;autoUpdateTime:false"`
}

// Session 仅持久化随机会话的 SHA-256 摘要，不保存原始 Cookie。
type Session struct {
	ID             uint64 `gorm:"primaryKey"`
	TokenHash      string `                               json:"-"`
	SessionVersion uint64 `gorm:"default:1"               json:"-"`
	UserID         uint64
	ExpiresAt      time.Time
	CreatedAt      time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt      time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定会话表名。
func (Session) TableName() string { return "user_sessions" }

// Bootstrap 通过手工 SQL 预置的单行锁控制并发初始化。
type Bootstrap struct {
	ID          uint64 `gorm:"primaryKey"`
	Initialized bool
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定初始化状态表名。
func (Bootstrap) TableName() string { return "user_bootstrap" }

// LoginRequest 是用户名密码登录参数，不提供注册入口。
type LoginRequest struct {
	// Username 是长度为 3 至 64 的小写英文、数字、点、下划线或连字符账号。
	Username string `json:"username" validate:"required,min=3,max=64" example:"zhangsan"`
	// PasswordSHA256 是 sha256(username + ":" + password) 的 64 位小写十六进制摘要。
	PasswordSHA256 string `json:"password_sha256" validate:"required,len=64,hexadecimal" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
	// CaptchaTicket 是验证码通过后返回的单次校验票据，启用验证码时必填。
	CaptchaTicket string `json:"captcha_ticket,omitempty" validate:"omitempty,max=4096" example:"tr03ExampleCaptchaTicket"`
	// CaptchaRandstr 是与票据对应的随机串，启用验证码时必填。
	CaptchaRandstr string `json:"captcha_randstr,omitempty" validate:"omitempty,max=256" example:"@Vki"`
}

// LoginCaptcha 是一次登录尝试使用的验证码初始化参数。
type LoginCaptcha struct {
	// AppID 是腾讯云 Web 验证码应用 ID，以字符串返回。
	AppID string `json:"app_id" example:"199999164"`
	// AidEncrypted 是后端新签发的短时一次性应用鉴权密文。
	AidEncrypted string `json:"aid_encrypted" example:"ZXhhbXBsZQ=="`
	// AidEncryptedType 是腾讯云验证码应用鉴权使用的加密模式。
	AidEncryptedType string `json:"aid_encrypted_type" example:"gcm" enums:"gcm"`
}

// LoginCaptchaResponse 返回验证码初始化参数，未启用时 data 为 null。
type LoginCaptchaResponse struct {
	response.Envelope[*LoginCaptcha]
}

// ChangePasswordRequest 是待改密账号更换临时口令的参数。
type ChangePasswordRequest struct {
	// CurrentPasswordSHA256 是当前口令按 sha256(username + ":" + password) 计算的摘要。
	CurrentPasswordSHA256 string `json:"current_password_sha256" validate:"required,len=64,hexadecimal" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
	// NewPasswordSHA256 是新口令使用同一账号计算的 64 位小写十六进制摘要。
	NewPasswordSHA256 string `json:"new_password_sha256" validate:"required,len=64,hexadecimal" example:"be2fccb9f088c506ce25d4d92c6cd8159dbe1b59214e07d0189b02f5c48f40a"`
}

// CreateRequest 是管理员添加普通账号的参数。
type CreateRequest struct {
	// Username 是唯一账号，仅接受小写英文、数字、点、下划线和连字符。
	Username string `json:"username" validate:"required,min=3,max=64" example:"zhangsan"`
	// DisplayName 是界面展示名称，最多 100 个字符。
	DisplayName string `json:"display_name" validate:"required,max=100" example:"张三"`
	// Email 是可选的联系邮箱，最多 254 个字符。
	Email string `json:"email" validate:"max=254" example:"zhangsan@example.com"`
	// PasswordLoginEnabled 表示是否允许密码登录，省略时默认允许。
	PasswordLoginEnabled *bool `json:"password_login_enabled,omitempty" example:"true"`
	// PasswordSHA256 是初始口令摘要，允许密码登录时必填，禁止密码登录时须省略。
	PasswordSHA256 string `json:"password_sha256,omitempty" validate:"omitempty,len=64,hexadecimal" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
}

// Info 是可公开给当前登录用户的账号字段目录。
type Info struct {
	// IdentityType 区分管理账号和绑定工作空间的学生身份。
	IdentityType string `json:"identity_type" example:"user" enums:"user,student"`
	// WorkspaceID 是学生会话绑定的工作空间编号，管理账号为空。
	WorkspaceID string `json:"workspace_id" example:"w7m2k9p4s6n8c3x5"`
	// WorkspaceName 是学生会话绑定的工作空间名称，管理账号为空。
	WorkspaceName string `json:"workspace_name" example:"教学事务"`
	// ID 是当前身份的编号，学生身份时为绑定空间内的学生编号。
	ID uint64 `json:"id" example:"1"`
	// Username 是登录账号，学生身份时为学号。
	Username string `json:"username" example:"zhangsan"`
	// DisplayName 是界面展示名称。
	DisplayName string `json:"display_name" example:"张三"`
	// Email 是联系邮箱，未填写时为空字符串。
	Email string `json:"email" example:"zhangsan@example.com"`
	// IsAdmin 仅表示可管理全局账号，不代表拥有租户 action。
	IsAdmin bool `json:"is_admin" example:"false"`
	// PasswordLoginEnabled 表示账号是否允许密码登录，学生身份固定为否。
	PasswordLoginEnabled bool `json:"password_login_enabled" example:"true"`
	// PasswordChangeRequired 表示网页登录会话完成密码修改后才可访问业务接口。
	PasswordChangeRequired bool `json:"password_change_required" example:"true"`
}

// InfoResponse 复用统一响应协议。
type InfoResponse struct {
	response.Envelope[Info]
}

// EmptyData 表示操作成功后没有业务数据。
type EmptyData struct{}

// EmptyResponse 是退出登录的空响应。
type EmptyResponse struct {
	response.Envelope[*EmptyData]
}
