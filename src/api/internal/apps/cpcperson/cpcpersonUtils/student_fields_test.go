package cpcpersonUtils

import (
	"reflect"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

func TestStudentDatesResolveWithoutPersistingOrFallingBack(t *testing.T) {
	node := cpctimelineModels.Node{Fields: []cpctimelineModels.Field{
		{ID: "birth", Type: "date", StudentField: "birth_date"},
		{ID: "adult", Type: "date", StudentField: "adulthood_date"},
		{ID: "count", Type: "number"},
		{ID: "confirmed", Type: "boolean"},
	}}
	stored := map[string]any{
		"birth": "1990-01-01", "adult": "2008-01-01", "count": float64(0),
		"confirmed": false, "removed": "历史字段",
	}
	for _, test := range []struct {
		birth string
		adult string
	}{
		{"2008-02-29", "2026-02-28"},
		{"2008-03-01", "2026-03-01"},
		{"0001-01-01", "0019-01-01"},
		{"9981-12-31", "9999-12-31"},
		{"9982-01-01", ""},
		{"9999-12-31", ""},
		{"", ""},
		{"2007-02-29", ""},
		{"0000-01-01", ""},
		{"2008-02-30", ""},
		{" 2008-02-29", ""},
		{"无法使用", ""},
	} {
		want := map[string]any{"count": float64(0), "confirmed": false}
		if validTimelineValue("date", test.birth) {
			want["birth"] = test.birth
		}
		if test.adult != "" {
			want["adult"] = test.adult
		}
		if got := ResolveTimelineValues(node, stored, test.birth); !reflect.DeepEqual(got, want) {
			t.Errorf("出生日期 %q 的引用错误：%v，期望 %v", test.birth, got, want)
		}
	}
	if stored["birth"] != "1990-01-01" || stored["adult"] != "2008-01-01" {
		t.Fatal("读取引用时不应修改存储值")
	}
}

func TestStudentReferencesRejectWritesAndAllowMissingRequiredDates(t *testing.T) {
	node := cpctimelineModels.Node{Fields: []cpctimelineModels.Field{
		{ID: "date", Name: "出生日期", Type: "date", Required: true, StudentField: "birth_date"},
		{ID: "note", Name: "备注", Type: "text", Required: true},
	}}
	clean, err := ValidateTimelineValues(node, map[string]any{"note": "保留手动填写"})
	if err != nil || !reflect.DeepEqual(clean, map[string]any{"note": "保留手动填写"}) {
		t.Fatalf("缺少必填引用日期不应阻止保存：%v", err)
	}
	for _, value := range []any{nil, "", "2008-02-29", false} {
		if _, err := ValidateTimelineValues(node, map[string]any{
			"date": value, "note": "保留手动填写",
		}); err == nil {
			t.Fatalf("直接写入引用字段应被拒绝：%v", value)
		}
	}
	if _, err := ValidateTimelineValues(node, map[string]any{}); err == nil {
		t.Fatal("引用字段不应放宽手动必填字段的校验")
	}
}

func TestStudentReferenceChangesRecalculateDependentRules(t *testing.T) {
	nodes := []cpctimelineModels.Node{
		{ID: "adult", TimeFieldID: "date", Fields: []cpctimelineModels.Field{
			{ID: "date", Type: "date", StudentField: "adulthood_date"},
		}},
		{ID: "application", TimeFieldID: "date", Rules: []cpctimelineModels.Rule{
			{ID: "after_adult", ReferenceNodeID: "adult", MinOffset: &cpctimelineModels.Offset{}},
		}},
	}
	values := map[string]map[string]any{"application": {"date": "2026-02-28"}}
	for _, test := range []struct {
		birth   string
		status  string
		minimum string
	}{
		{"2008-02-29", TimelineNormal, "2026-02-28"},
		{"2008-03-01", TimelineAbnormal, "2026-03-01"},
		{"", TimelineNormal, ""},
	} {
		values["adult"] = ResolveTimelineValues(nodes[0], nil, test.birth)
		status, _, checks := EvaluateTimelineChecks(nodes, values, nil)
		if status != test.status {
			t.Fatalf("更新学生日期后依赖节点的异常状态未更新：%s", status)
		}
		if test.minimum == "" {
			if len(checks["application"]) != 0 {
				t.Fatal("引用缺失时不应使用旧日期校验")
			}
		} else if len(checks["application"]) != 1 ||
			checks["application"][0].MinDate != test.minimum {
			t.Fatal("依赖规则未使用当前学生日期")
		}
	}
}
