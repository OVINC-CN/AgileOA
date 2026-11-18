package auditConstants

// 审计类型与对象标识均使用固定英文值。
const (
	FilterExcludeAudit   = "exclude_audit"
	Success              = "success"
	Failure              = "failure"
	SourceWeb            = "web"
	SourceMCP            = "mcp"
	ObjectUser           = "user"
	ObjectSession        = "user_session"
	ObjectStudentSession = "student_session"
	ObjectAPIToken       = "user_api_token"
	ObjectEmailLogin     = "user_email_login"
	ObjectAudit          = "audit"
	ObjectWorkspace      = "workspace"
	ObjectPermission     = "access_permission"
	ObjectNotification   = "notification"
	RequestIDHeader      = "X-Request-ID"
	RandomBytes          = 16
	UserAgentMaxLength   = 512
	SystemWorkspace      = "system"
	IdentityUser         = 1
	IdentityStudent      = 2
	AccessWeb            = 1
	AccessMCP            = 2
)
