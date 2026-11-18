package userModels

import "time"

// StudentSession 独立保存学生会话，不关联全局账号及其权限。
type StudentSession struct {
	ID          uint64 `gorm:"primaryKey"`
	TokenHash   string `                               json:"-"`
	WorkspaceID string
	StudentID   uint64
	StudentNo   string
	Email       string
	ExpiresAt   time.Time
	CreatedAt   time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt   time.Time `gorm:"->;autoUpdateTime:false"`
}

// TableName 固定学生会话表名。
func (StudentSession) TableName() string { return "student_sessions" }

// StudentIdentity 只读取当前学生及工作空间的登录所需字段，不加载敏感档案。
type StudentIdentity struct {
	StudentID        uint64
	StudentNo        string
	Name             string
	Email            string
	Status           string
	WorkspaceID      string
	WorkspaceName    string
	WorkspaceEnabled bool
	Session          StudentSession `gorm:"embedded;embeddedPrefix:session_"`
}

// SendStudentLoginEmailRequest 按空间、学号和登记邮箱申请学生登录。
type SendStudentLoginEmailRequest struct {
	// WorkspaceName 是唯一的工作空间名称，必须逐字匹配。
	WorkspaceName string `json:"workspace_name" validate:"required,max=100" example:"教学事务"`
	// StudentNo 是该工作空间中的学号，保留前导零。
	StudentNo string `json:"student_no" validate:"required,max=32" example:"20260001"`
	// Email 必须与在籍学生档案中的邮箱逐字匹配。
	Email string `json:"email" validate:"required,max=254" example:"student@example.com"`
	// CaptchaTicket 是安全验证通过后返回的单次票据。
	CaptchaTicket string `json:"captcha_ticket,omitempty" validate:"omitempty,max=4096"`
	// CaptchaRandstr 是与验证码票据对应的随机串。
	CaptchaRandstr string `json:"captcha_randstr,omitempty" validate:"omitempty,max=256"`
}
