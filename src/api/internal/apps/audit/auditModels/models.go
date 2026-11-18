package auditModels

import (
	"encoding/json"
	"time"
)

// Actor 是审计保留的操作者身份快照，不关联可能被删除的业务记录。
type Actor struct {
	// StudentID 是学生操作者编号，与全局账号编号分开保存。
	StudentID uint64 `json:"student_id,omitempty" example:"1"`
	// IdentityType 在学生操作中固定为 student，历史账号快照可省略。
	IdentityType string `json:"identity_type,omitempty" example:"student"`
	// WorkspaceID 是学生身份绑定的工作空间编号。
	WorkspaceID string `json:"workspace_id,omitempty" example:"w7m2k9p4s6n8c3x5"`
	// ID 是已确认身份的操作者账号编号。
	ID uint64 `json:"id" example:"1"`
	// Username 是操作者账号快照。
	Username string `json:"username" example:"zhangsan"`
	// DisplayName 是操作者显示名称快照。
	DisplayName string `json:"display_name" example:"张三"`
	// IsAdmin 是操作发生时的系统管理员标识。
	IsAdmin bool `json:"is_admin" example:"false"`
}

// Request 是请求入口生成的审计上下文，不保存请求体或认证请求头。
type Request struct {
	ID        string
	StartedAt time.Time
	Method    string
	Path      string
	Source    string
	ClientIP  string
	UserAgent string
}

// Snapshot 由业务按固定字段目录构造，口令、会话、密钥和签名字段禁止写入。
type Snapshot map[string]any

// Change 保存单个非敏感字段的前后值。
type Change struct {
	// Before 是变更前公开字段值。
	Before any `json:"before"`
	// After 是变更后公开字段值。
	After any `json:"after"`
}

// Event 是所有 app 共用的审计写入参数；批量变更逐条写入并复用请求上下文。
type Event struct {
	TenantID        *string
	Actor           Actor
	Action          string
	ObjectType      string
	ObjectID        string
	Conditions      Snapshot
	ReturnedIDs     []string
	ReturnedCount   int64
	Before          Snapshot
	After           Snapshot
	SensitiveBefore Snapshot
	SensitiveAfter  Snapshot
	Result          string
	ErrorCode       string
	ErrorMessage    string
}

// Record 是标准审计事件，按开始时刻的 UTC 日期分区，不建立物理外键。
type Record struct {
	ID                   uint64 `gorm:"primaryKey;autoIncrement"`
	EventDate            string `gorm:"primaryKey"`
	EventID              string
	WorkspaceID          string
	EventContent         *string
	ActionID             string
	RequestID            string
	UserID               uint64
	UserIdentifyType     uint8
	UserIdentifyTenantID string
	StartTime            int64
	EndTime              int64
	AccessType           uint8
	AccessSourceIP       string
	AccessUserAgent      string
	ResourceTypeID       string
	InstanceID           string
	InstanceName         *string
	InstanceData         json.RawMessage
	InstanceOriginData   json.RawMessage
	ResultCode           int
	ResultContent        string
	SnapshotUserInfo     json.RawMessage
	ExtendData           json.RawMessage
	SensitiveSnapshot    []byte
	SensitiveKeyID       string
	SensitiveContext     json.RawMessage
	IsLoginEvent         bool `gorm:"->"`
	HasSensitiveSnapshot bool `gorm:"->"`
}

// TableName 固定审计表名，避免由业务 app 或外部请求选择表。
func (Record) TableName() string { return "audit_events" }
