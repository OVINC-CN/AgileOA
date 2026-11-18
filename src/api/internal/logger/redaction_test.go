package logger

import (
	"strings"
	"testing"
)

func TestDiagnosticMessagesRedactCredentials(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		message string
		secret  string
	}{
		{
			name:    "password",
			message: `connection failed: password="private password value"`,
			secret:  "private password value",
		},
		{ // #nosec G101 -- 仅用于脱敏验证的虚构认证头。
			name:    "authorization",
			message: "request failed\nAuthorization: Bearer private-auth-value\nconnection reset",
			secret:  "private-auth-value",
		},
		{
			name:    "cookie",
			message: "request failed\nCookie: custom_name=private-cookie-value; extra=value",
			secret:  "private-cookie-value",
		},
		{
			name:    "signature",
			message: "request failed: https://cdn.example/object?X-Amz-Signature=private-signature&x=1",
			secret:  "private-signature",
		},
		{ // #nosec G101 -- 仅用于脱敏验证的虚构连接凭据。
			name:    "dsn",
			message: "connection failed: example:private-dsn-value@tcp(db.example:3306)/app",
			secret:  "private-dsn-value",
		},
		{
			name:    "session digest",
			message: "Duplicate entry '" + strings.Repeat("a", 64) + "' for key 'token_hash'",
			secret:  strings.Repeat("a", 64),
		},
		{
			name:    "password hash",
			message: "invalid hash: $2a$10$" + strings.Repeat("a", 53),
			secret:  "$2a$10$" + strings.Repeat("a", 53),
		},
		{ // #nosec G101 -- 仅用于脱敏验证的虚构签名令牌。
			name:    "signed token",
			message: "verification failed: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJleGFtcGxlIn0.example-signature",
			secret:  "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJleGFtcGxlIn0.example-signature",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			message := redactCredentials(scenario.message)
			if strings.Contains(message, scenario.secret) ||
				!strings.Contains(message, "[redacted]") {
				t.Fatalf("诊断信息没有脱敏：%s", message)
			}
		})
	}
}

func TestDiagnosticMessagesKeepFailureDetails(t *testing.T) {
	const message = "Span exception\noperation: mysql.ping\n" +
		"error: dial tcp db.example:3306: connection refused"
	if redactCredentials(message) != message {
		t.Fatal("普通异常详情被脱敏规则修改")
	}
}
