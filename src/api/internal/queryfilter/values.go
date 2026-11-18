package queryfilter

import (
	"slices"
	"strconv"
	"strings"
)

// ContainsOnly 校验全部筛选值均来自固定目录。
func ContainsOnly(values []string, allowed ...string) bool {
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
	}
	return true
}

// Values 提取重复筛选参数，去除首尾空白和重复值，按业务需要保留空值。
func Values(raw []string, keepEmpty bool) []string {
	values := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if (!keepEmpty && value == "") || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}

// Integers 提取可重复的整数筛选参数，拒绝无效整数。
func Integers(raw []string) ([]int, error) {
	values := Values(raw, false)
	result := make([]int, 0, len(values))
	for _, value := range values {
		number, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		result = append(result, number)
	}
	return result, nil
}
