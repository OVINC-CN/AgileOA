package partybranchSvc

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func ruleCondition(field string, values ...string) partybranchModels.AdmissionRuleNode {
	return partybranchModels.AdmissionRuleNode{
		Type: "condition", Field: field, Operator: "in", Values: values,
	}
}

func TestAdmissionRulePreservesNestedAndOrAndNormalizesValues(t *testing.T) {
	rule := partybranchModels.AdmissionRuleNode{
		Type: "group", Logic: "and",
		Children: []partybranchModels.AdmissionRuleNode{
			ruleCondition("advisor", " 张三 ", "王五", "张三"),
			ruleCondition("education_level", "硕士", "博士"),
			{Type: "group", Logic: "or", Children: []partybranchModels.AdmissionRuleNode{
				ruleCondition("major", "会计"),
				{Type: "group", Logic: "and", Children: []partybranchModels.AdmissionRuleNode{
					ruleCondition("advisor", "李四"), ruleCondition("major", "审计"),
				}},
			}},
		},
	}
	cleaned, err := cleanAdmissionRule(&rule)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cleaned.Children[0].Values, []string{"张三", "王五"}) ||
		cleaned.Logic != "and" || cleaned.Children[2].Logic != "or" ||
		cleaned.Children[2].Children[1].Logic != "and" ||
		cleaned.Children[1].Field != "education_level" {
		t.Fatal("规则树的嵌套逻辑或候选值集合发生改变")
	}
	if rule.Children[0].Values[0] != " 张三 " {
		t.Fatal("校验修改了调用方的原规则")
	}
	if result, err := cleanAdmissionRule(nil); err != nil || result != nil {
		t.Fatal("不允许显式清空规则")
	}
}

func TestAdmissionRuleRejectsInvalidNodesAndArbitraryFields(t *testing.T) {
	for _, node := range []partybranchModels.AdmissionRuleNode{
		{},
		{Type: "group", Logic: "and"},
		{Type: "group", Logic: "xor", Children: []partybranchModels.AdmissionRuleNode{ruleCondition("major", "会计")}},
		{Type: "group", Logic: "or", Field: "major", Children: []partybranchModels.AdmissionRuleNode{ruleCondition("major", "会计")}},
		ruleCondition("users.password_hash", "虚构值"),
		ruleCondition("advisor"),
		ruleCondition("advisor", " "),
		ruleCondition("advisor", "张\x00三"),
		ruleCondition("advisor", strings.Repeat("中", 51)),
		ruleCondition("major", strings.Repeat("中", 101)),
		ruleCondition("education_level", strings.Repeat("中", 33)),
		ruleCondition("student_no", strings.Repeat("0", 33)),
		ruleCondition("student_no", " "),
		ruleCondition("student_no", "000\x001"),
		{Type: "condition", Field: "major", Operator: "eq", Values: []string{"会计"}},
		{Type: "condition", Field: "major", Operator: "in", Logic: "or", Values: []string{"会计"}},
	} {
		if _, err := cleanAdmissionRule(&node); err == nil {
			t.Fatalf("接受了非法节点：%+v", node)
		}
	}
}

func TestAdmissionRulePreservesStudentNumbersAndLeadingZeros(t *testing.T) {
	rule := ruleCondition("student_no", " 000001 ", "S20260001", "000001", strings.Repeat("0", 32))
	cleaned, err := cleanAdmissionRule(&rule)
	if err != nil {
		t.Fatal(err)
	}
	if cleaned.Field != "student_no" ||
		!reflect.DeepEqual(
			cleaned.Values,
			[]string{"000001", "S20260001", strings.Repeat("0", 32)},
		) {
		t.Fatal("学号候选值未正确去重，或前导零及长度边界发生改变")
	}
}

func TestAdmissionRuleEnforcesConfiguredTreeAndValueLimits(t *testing.T) {
	original := config.Config.PartyBranch
	t.Cleanup(func() { config.Config.PartyBranch = original })
	leaf := ruleCondition("major", "会计", "审计")
	rule := partybranchModels.AdmissionRuleNode{
		Type: "group", Logic: "and", Children: []partybranchModels.AdmissionRuleNode{
			{Type: "group", Logic: "or", Children: []partybranchModels.AdmissionRuleNode{leaf}},
		},
	}
	config.Config.PartyBranch.MaxRuleDepth = 2
	if _, err := cleanAdmissionRule(&rule); err == nil {
		t.Fatal("超过配置层数的规则被接受")
	}
	config.Config.PartyBranch = original
	config.Config.PartyBranch.MaxRuleNodes = 2
	if _, err := cleanAdmissionRule(&rule); err == nil {
		t.Fatal("超过配置节点数的规则被接受")
	}
	config.Config.PartyBranch = original
	config.Config.PartyBranch.MaxRuleValues = 1
	if _, err := cleanAdmissionRule(&leaf); err == nil {
		t.Fatal("超过配置候选值数的规则被接受")
	}
}
