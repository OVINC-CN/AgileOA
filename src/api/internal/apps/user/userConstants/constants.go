package userConstants

// 固定协议字段与账号校验规则供路由和业务层共用。
const (
	UsernamePattern  = `^[a-z0-9][a-z0-9._-]{2,63}$`
	DigestPattern    = `^[0-9a-f]{64}$`
	EmailCodePattern = `^[0-9]{6}$`
	EmailPattern     = `^([A-Za-z0-9_'+\-]+\.)*[A-Za-z0-9_'+\-]*[A-Za-z0-9_+-]@` +
		`([A-Za-z0-9][A-Za-z0-9\-]*\.)+[A-Za-z]{2,}$`
	MinPasswordBytes           = 12
	MaxPasswordBytes           = 72
	MaxEmailLength             = 254
	SessionBytes               = 32
	EmailCodeLimit             = 1000000
	APITokenPrefix             = "awt_"
	StudentSessionPrefix       = "aws_"
	IdentityUser               = "user"
	IdentityStudent            = "student"
	CSRFHeader                 = "X-CSRF-Protection"
	CSRFValue                  = "1"
	EmailLoginChallengePrefix  = "user:email_login:challenge:"
	EmailLoginAttemptsPrefix   = "user:email_login:attempts:"
	EmailLoginUserPrefix       = "user:email_login:user:"
	EmailLoginCooldownPrefix   = "user:email_login:cooldown:"
	StudentEmailLoginPrefix    = "student:email_login:student:"
	StudentEmailCooldownPrefix = "student:email_login:cooldown:"
	StudentEmailPendingPrefix  = "student:email_login:pending:"
)
