package cpctimelineSvc

import (
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func fixtureInput() cpctimelineModels.Input {
	return cpctimelineModels.Input{
		Name: "虚构时间线",
		Definition: cpctimelineModels.Definition{Nodes: []cpctimelineModels.Node{
			{
				ID: "node_first", Name: "前置材料", TimeFieldID: "occurred_at",
				Fields: []cpctimelineModels.Field{
					{ID: "occurred_at", Name: "材料日期", Type: "date", Required: true},
					{ID: "note", Name: "备注", Type: "textarea"},
				},
				Rules: []cpctimelineModels.Rule{
					{
						ID: "rule_before", ReferenceNodeID: "node_second",
						MinOffset: &cpctimelineModels.Offset{Years: 1, Days: -7},
						MaxOffset: &cpctimelineModels.Offset{Years: 1},
					},
					{
						ID: "rule_notice", ReferenceNodeID: "node_second",
						MaxOffset: &cpctimelineModels.Offset{Workdays: 5},
					},
				},
			},
			{
				ID: "node_second", Name: "参考材料", TimeFieldID: "date",
				Fields: []cpctimelineModels.Field{{
					ID: "date", Name: "发生日期", Type: "date", Required: true,
				}},
				Rules: []cpctimelineModels.Rule{},
			},
		}},
	}
}

func TestDefinitionPreservesOrderStableReferencesAndCompoundRanges(t *testing.T) {
	input := fixtureInput()
	input.Definition.Nodes[0], input.Definition.Nodes[1] = input.Definition.Nodes[1], input.Definition.Nodes[0]
	cleaned, err := prepare(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded := cleaned.Definition
	if decoded.Nodes[0].ID != "node_second" ||
		decoded.Nodes[1].Rules[0].ReferenceNodeID != "node_second" ||
		decoded.Nodes[1].Rules[0].MinOffset.Years != 1 ||
		decoded.Nodes[1].Rules[0].MinOffset.Days != -7 ||
		len(cleaned.Definition.Nodes[1].Rules) != 2 {
		t.Fatal("排序、复合偏移或多规则引用发生改变")
	}
}

func TestStudentFieldReferencesOnlyAcceptRegisteredDateFields(t *testing.T) {
	for _, field := range []string{"", "birth_date", "adulthood_date"} {
		input := fixtureInput()
		input.Definition.Nodes[0].Fields[0].StudentField = field
		cleaned, err := prepare(input)
		if err != nil || cleaned.Definition.Nodes[0].Fields[0].StudentField != field {
			t.Fatalf("有效学生日期引用未保留：%s，%v", field, err)
		}
	}
	for _, field := range []cpctimelineModels.Field{
		{ID: "note", Name: "备注", Type: "text", StudentField: "birth_date"},
		{ID: "note", Name: "备注", Type: "date", StudentField: "identity_number"},
		{ID: "note", Name: "备注", Type: "date", StudentField: "s.birth_date"},
	} {
		input := fixtureInput()
		input.Definition.Nodes[0].Fields[1] = field
		if _, err := prepare(input); err == nil {
			t.Fatal("非日期类型或未登记学生字段通过引用校验")
		}
	}
}

func TestDefinitionRejectsInvalidTimeFieldsReferencesAndRanges(t *testing.T) {
	for name, mutate := range map[string]func(*cpctimelineModels.Input){
		"missing_time": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].TimeFieldID = "missing"
		},
		"optional_time": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Fields[0].Required = false
		},
		"text_time": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Fields[0].Type = "text"
		},
		"duplicate_field": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Fields[1].Name = "材料日期"
		},
		"self_reference": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Rules[0].ReferenceNodeID = "node_first"
		},
		"dangling_reference": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Rules[0].ReferenceNodeID = "missing"
		},
		"empty_range": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Rules[0].MinOffset = nil
			input.Definition.Nodes[0].Rules[0].MaxOffset = nil
		},
		"reversed_range": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Rules[0].MinOffset = &cpctimelineModels.Offset{Months: 6}
			input.Definition.Nodes[0].Rules[0].MaxOffset = &cpctimelineModels.Offset{Months: 2}
		},
		"duplicate_node": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[1].ID = "node_first"
		},
		"unknown_type": func(input *cpctimelineModels.Input) {
			input.Definition.Nodes[0].Fields[1].Type = "sql"
		},
		"control_character": func(input *cpctimelineModels.Input) {
			input.Name = "模板\x00"
		},
		"long_name": func(input *cpctimelineModels.Input) {
			input.Name = strings.Repeat("中", 101)
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := fixtureInput()
			mutate(&input)
			if _, err := prepare(input); err == nil {
				t.Fatal("无效模板被接受")
			}
		})
	}
}

func TestDefinitionEnforcesConfiguredLimits(t *testing.T) {
	original := config.Config.CPCTimeline
	t.Cleanup(func() { config.Config.CPCTimeline = original })
	for _, change := range []func(){
		func() { config.Config.CPCTimeline.MaxNodes = 1 },
		func() { config.Config.CPCTimeline.MaxFieldsPerNode = 1 },
		func() { config.Config.CPCTimeline.MaxRulesPerNode = 1 },
		func() { config.Config.CPCTimeline.MaxDefinitionBytes = 10 },
		func() { config.Config.CPCTimeline.MaxOffset = 1 },
	} {
		config.Config.CPCTimeline = original
		change()
		if _, err := prepare(fixtureInput()); err == nil {
			t.Fatal("超过配置限制的定义被接受")
		}
	}
}

func TestNodeGroupsAreOptionalAndValidateNames(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"  申请阶段  ", "申请阶段"},
		{strings.Repeat("中", 100), strings.Repeat("中", 100)},
	} {
		input := fixtureInput()
		input.Definition.Nodes[0].GroupName = test.name
		cleaned, err := prepare(input)
		if err != nil || cleaned.Definition.Nodes[0].GroupName != test.want {
			t.Fatalf("节点分组未正确保留或清理：%q，%v", test.name, err)
		}
	}
	for _, name := range []string{strings.Repeat("中", 101), "申请\n阶段", "申请\x00阶段"} {
		input := fixtureInput()
		input.Definition.Nodes[0].GroupName = name
		if _, err := prepare(input); err == nil {
			t.Fatalf("无效分组名称被接受：%q", name)
		}
	}
}
