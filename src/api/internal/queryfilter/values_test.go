package queryfilter

import (
	"slices"
	"testing"
)

func TestValuesPreserveSelectedEmptyOptionAndLiteralCommas(t *testing.T) {
	raw := []string{"", " 专业甲,方向一 ", "专业乙", "专业乙"}
	if got := Values(raw, false); !slices.Equal(got, []string{"专业甲,方向一", "专业乙"}) {
		t.Fatalf("多选值被截断或未去重：%q", got)
	}
	if got := Values(raw, true); !slices.Equal(got, []string{"", "专业甲,方向一", "专业乙"}) {
		t.Fatalf("显式空选项未被保留：%q", got)
	}
	if got := Values(nil, true); len(got) != 0 {
		t.Fatal("未提交筛选时产生了空选项")
	}
}

func TestIntegersAndCatalogRejectAnyInvalidSelectedValue(t *testing.T) {
	got, err := Integers([]string{"2025", "2026", "2025"})
	if err != nil || !slices.Equal(got, []int{2025, 2026}) {
		t.Fatalf("多选整数解析失败：%v，%v", got, err)
	}
	if _, err := Integers([]string{"2025", "invalid"}); err == nil {
		t.Fatal("第二个无效整数被忽略")
	}
	if !ContainsOnly([]string{"enabled", "disabled"}, "enabled", "disabled") ||
		ContainsOnly([]string{"enabled", "invalid"}, "enabled", "disabled") {
		t.Fatal("多选目录未逐项校验")
	}
}
