package userModels

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// EmailLogin 是 Redis 中短时保存的登录信息，只保存绑定校验标识的验证码摘要。
type EmailLogin struct {
	CodeHash       string    `json:"code_hash"`
	StudentID      uint64    `json:"student_id,omitempty"`
	WorkspaceID    string    `json:"workspace_id,omitempty"`
	WorkspaceName  string    `json:"workspace_name,omitempty"`
	StudentNo      string    `json:"student_no,omitempty"`
	UserID         uint64    `json:"user_id"`
	Email          string    `json:"email"`
	SessionVersion uint64    `json:"session_version"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// SendLoginEmailRequest 使用用户名和已登记邮箱申请一次性数字验证码。
type SendLoginEmailRequest struct {
	// Username 是已创建的小写登录账号。
	Username string `json:"username" validate:"required,min=3,max=64" example:"zhangsan"`
	// Email 必须与该账号已登记的邮箱完全一致。
	Email string `json:"email" validate:"required,max=254" example:"zhangsan@example.com"`
	// CaptchaTicket 是安全验证通过后返回的单次票据。
	CaptchaTicket string `json:"captcha_ticket,omitempty" validate:"omitempty,max=4096" example:"tr03ExampleCaptchaTicket"`
	// CaptchaRandstr 是与验证码票据对应的随机串。
	CaptchaRandstr string `json:"captcha_randstr,omitempty" validate:"omitempty,max=256" example:"@Vki"`
}

// EmailLoginChallengeData 返回本次验证码的校验标识及有效期，不包含验证码。
type EmailLoginChallengeData struct {
	// ChallengeID 是本次发送的随机校验标识，仅用于请求体，不进入 URL 或浏览器存储。
	ChallengeID string `json:"challenge_id" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
	// ExpiresIn 是验证码剩余有效秒数。
	ExpiresIn int64 `json:"expires_in" example:"600"`
	// RetryAfter 是允许再次发送前需要等待的秒数。
	RetryAfter int64 `json:"retry_after" example:"60"`
}

// EmailLoginChallengeResponse 返回验证码发送后的校验信息。
type EmailLoginChallengeResponse struct {
	response.Envelope[EmailLoginChallengeData]
}

// EmailLoginRequest 通过随机校验标识及邮件中的一次性验证码建立会话。
type EmailLoginRequest struct {
	// ChallengeID 是发送接口返回的本次校验标识，不得写入日志或审计。
	ChallengeID string `json:"challenge_id" validate:"required,len=64,hexadecimal" example:"a187fc74508468a29c069c56d61017c5a9f1c121692566074babe6a27b60da74"`
	// Code 是邮件中的 6 位数字验证码，作为字符串保留前导零，不得写入日志或审计。
	Code string `json:"code" validate:"required,len=6,numeric" example:"012345"`
	// Username 在首次邮件登录或登记邮箱变更后必填，必须与账号逐字一致。
	Username string `json:"username,omitempty" validate:"omitempty,min=3,max=64" example:"zhangsan"`
}

// EmailLoginData 区分待核验用户名和已完成登录，不在核验前透露账号信息。
type EmailLoginData struct {
	// UsernameRequired 为 true 时须补交用户名，登录码尚未消费且未建立会话。
	UsernameRequired bool `json:"username_required" example:"true"`
	// User 仅在登录成功时提供，待核验用户名时为 null。
	User *Info `json:"user"`
}

// EmailLoginResponse 返回邮件验证码登录的核验状态或登录结果。
type EmailLoginResponse struct {
	response.Envelope[EmailLoginData]
}
