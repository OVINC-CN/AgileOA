package cpcpersonUtils

import (
	"math"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

func TestTimelineRequiredValuesPreserveFalseAndZero(t *testing.T) {
	node := cpctimelineModels.Node{Fields: []cpctimelineModels.Field{
		{ID: "date", Name: "发生时间", Type: "date", Required: true},
		{ID: "count", Name: "数量", Type: "number", Required: true},
		{ID: "confirmed", Name: "已确认", Type: "boolean", Required: true},
	}}
	values := map[string]any{"date": "2028-02-29", "count": float64(0), "confirmed": false}
	clean, err := ValidateTimelineValues(node, values)
	if err != nil || clean["count"] != float64(0) || clean["confirmed"] != false {
		t.Fatalf("零和否应视为有效填写：%v", err)
	}
	for _, invalid := range []map[string]any{
		{"date": "2027-02-29", "count": float64(0), "confirmed": false},
		{"date": "2028-02-29", "count": "0", "confirmed": false},
		{"date": "2028-02-29", "count": math.Inf(1), "confirmed": false},
		{"date": "2028-02-29", "count": float64(0)},
		{"date": "2028-02-29", "count": float64(0), "confirmed": false, "unknown": "x"},
		{"date": map[string]any{}, "count": float64(0), "confirmed": false},
	} {
		if _, err := ValidateTimelineValues(node, invalid); err == nil {
			t.Fatalf("无效日期、类型、缺失值或目录外字段被接受：%v", invalid)
		}
	}
}

func TestTimelineOffsetsClampEachMonthAndSkipWeekends(t *testing.T) {
	cases := []struct {
		date   string
		offset cpctimelineModels.Offset
		want   string
	}{
		{"2028-02-29", cpctimelineModels.Offset{Years: 1, Months: 1}, "2029-03-28"},
		{"2026-01-31", cpctimelineModels.Offset{Months: 1}, "2026-02-28"},
		{"2026-11-06", cpctimelineModels.Offset{Workdays: 1}, "2026-11-09"},
		{"2026-11-09", cpctimelineModels.Offset{Workdays: -1}, "2026-11-06"},
		{"2026-11-07", cpctimelineModels.Offset{Workdays: 1}, "2026-11-09"},
		{"2026-11-06", cpctimelineModels.Offset{Years: 1, Days: -7, Workdays: 1}, "2027-11-01"},
	}
	for _, item := range cases {
		date, err := time.Parse(time.DateOnly, item.date)
		if err != nil {
			t.Fatal(err)
		}
		if got := applyOffset(date, item.offset).Format(time.DateOnly); got != item.want {
			t.Errorf("%s 加 %v：期望 %s，实际 %s", item.date, item.offset, item.want, got)
		}
	}
}

func TestTimelineRulesAllowBoundariesAndReevaluateReferences(t *testing.T) {
	nodes := []cpctimelineModels.Node{
		{ID: "next", Name: "后续节点", TimeFieldID: "date", Rules: []cpctimelineModels.Rule{
			{
				ID:              "range",
				ReferenceNodeID: "reference",
				MinOffset: &cpctimelineModels.Offset{
					Days: 1,
				},
				MaxOffset: &cpctimelineModels.Offset{Days: 3},
			},
		}},
		{ID: "reference", Name: "参考节点", TimeFieldID: "date"},
	}
	values := map[string]map[string]any{"reference": {"date": "2026-11-06"}}
	for _, date := range []string{"2026-11-07", "2026-11-09"} {
		values["next"] = map[string]any{"date": date}
		if state, _ := EvaluateTimeline(nodes, values, nil); state != TimelineNormal {
			t.Fatal("包含边界的日期被误判异常")
		}
	}
	values["reference"]["date"] = "2026-11-10"
	state, issues := EvaluateTimeline(nodes, values, nil)
	if state != TimelineAbnormal || len(issues["next"]) != 1 ||
		issues["next"][0].MinDate != "2026-11-11" || issues["next"][0].MaxDate != "2026-11-13" {
		t.Fatal("参考节点修改后未同步生成异常及预期范围")
	}
	delete(values, "reference")
	if state, issues := EvaluateTimeline(
		nodes,
		values,
		nil,
	); state != TimelineNormal ||
		len(issues["next"]) != 0 {
		t.Fatal("缺少参考日期时不应产生时间冲突")
	}
}

func TestRuleVisibilityDependsOnReferenceDate(t *testing.T) {
	nodes := []cpctimelineModels.Node{
		{ID: "current", TimeFieldID: "date", Rules: []cpctimelineModels.Rule{
			{
				ID:              "visible",
				ReferenceNodeID: "filled",
				MinOffset:       &cpctimelineModels.Offset{Days: 1},
			},
			{ID: "hidden", ReferenceNodeID: "empty", MinOffset: &cpctimelineModels.Offset{Days: 1}},
		}},
		{ID: "filled", Name: "已填写参考节点", TimeFieldID: "date"},
		{ID: "empty", Name: "未填写参考节点", TimeFieldID: "date"},
	}
	values := map[string]map[string]any{"filled": {"date": "2026-11-06"}}
	state, issues, checks := EvaluateTimelineChecks(nodes, values, nil)
	if state != TimelineNormal || len(issues["current"]) != 0 || len(checks["current"]) != 1 ||
		checks["current"][0].Result != TimelinePending || checks["current"][0].MinDate != "2026-11-07" {
		t.Fatal("参考日期已填写时应展示待校验规则及预期范围")
	}
	values["current"] = map[string]any{"date": "2026-11-07"}
	_, _, checks = EvaluateTimelineChecks(nodes, values, nil)
	if len(checks["current"]) != 1 || checks["current"][0].Result != TimelineNormal {
		t.Fatal("符合范围的规则也应继续展示")
	}
	delete(values, "filled")
	_, _, checks = EvaluateTimelineChecks(nodes, values, nil)
	if len(checks["current"]) != 0 {
		t.Fatal("依赖节点没有日期时不应展示其规则")
	}
}

func TestSkippedNodesDoNotValidateOrProvideReferenceDates(t *testing.T) {
	nodes := []cpctimelineModels.Node{
		{ID: "reference", Name: "参考节点", TimeFieldID: "date"},
		{ID: "current", Name: "依赖节点", TimeFieldID: "date", Rules: []cpctimelineModels.Rule{
			{ID: "range", ReferenceNodeID: "reference", MinOffset: &cpctimelineModels.Offset{}},
		}},
	}
	values := map[string]map[string]any{
		"reference": {"date": "2026-11-07"}, "current": {"date": "2026-11-06"},
	}
	if status, _ := EvaluateTimeline(nodes, values, nil); status != TimelineAbnormal {
		t.Fatal("未跳过时应有时间冲突")
	}
	for _, skippedID := range []string{"current", "reference"} {
		status, issues, checks := EvaluateTimelineChecks(
			nodes, values, map[string]string{skippedID: TimelineSkipped},
		)
		if status != TimelineNormal || len(issues["current"]) != 0 ||
			len(checks["current"]) != 0 || checks["current"] == nil {
			t.Fatal("已跳过节点仍被校验、参与引用或返回空数组契约不一致")
		}
	}
	if values["reference"]["date"] != "2026-11-07" ||
		values["current"]["date"] != "2026-11-06" {
		t.Fatal("跳过节点不应修改已保存的填写值")
	}
}
