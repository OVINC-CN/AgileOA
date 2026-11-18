package attachmentUtils

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
)

// ValidFilename 保留中文原名，拒绝路径、控制字符及超长文件名。
func ValidFilename(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || value == "." ||
		value == ".." ||
		utf8.RuneCountInString(value) > attachmentConstants.FilenameLength ||
		strings.ContainsAny(value, "/\\*") {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
