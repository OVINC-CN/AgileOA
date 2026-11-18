package workspaceUtils

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceConstants"

// ValidID 只接受同时包含英文小写字母与数字的 16 位工作空间编号。
func ValidID(value string) bool {
	if len(value) != workspaceConstants.IDLength {
		return false
	}
	var letter, digit bool
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			letter = true
		case character >= '0' && character <= '9':
			digit = true
		default:
			return false
		}
	}
	return letter && digit
}
