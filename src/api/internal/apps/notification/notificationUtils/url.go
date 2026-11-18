package notificationUtils

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationConstants"
)

// ValidURL 仅允许空链接或站内绝对路径，拒绝外站、协议及控制字符。
func ValidURL(value string) bool {
	if value == "" {
		return true
	}
	if utf8.RuneCountInString(value) > notificationConstants.URLMaxLength ||
		strings.Contains(value, "\\") || strings.IndexFunc(value, invalidURLRune) >= 0 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" &&
		strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//") &&
		!strings.Contains(parsed.Path, "\\") && strings.IndexFunc(parsed.Path, invalidURLRune) < 0
}

func invalidURLRune(value rune) bool {
	return unicode.IsControl(value) || unicode.IsSpace(value)
}

// WorkspaceURL 将工作空间和业务定位参数编码到固定页面的查询参数中。
func WorkspaceURL(path, workspaceID string, parameters map[string]string) string {
	query := url.Values{"workspace": {workspaceID}}
	for name, value := range parameters {
		query.Set(name, value)
	}
	return path + "?" + query.Encode()
}
