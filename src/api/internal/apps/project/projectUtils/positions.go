package projectUtils

import (
	"strings"
	"unicode"
)

// Positions 拆分档案中的独立班委职务，避免将副班长误匹配为班长。
func Positions(value string) []string {
	return strings.FieldsFunc(value, func(char rune) bool {
		return unicode.IsSpace(char) || strings.ContainsRune(",，、;；/兼", char)
	})
}
