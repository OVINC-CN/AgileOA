package workspaceConstants

// 工作空间编号使用固定长度的英文小写字母与数字，不暴露顺序或数量。
const (
	IDLength   = 16
	IDLetters  = "abcdefghijklmnopqrstuvwxyz"
	IDDigits   = "0123456789"
	IDAlphabet = IDLetters + IDDigits
)
