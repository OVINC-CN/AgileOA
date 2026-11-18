package projectSvc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func TestScoreFormulaAllowedMathAndMissingValues(t *testing.T) {
	a, b := 80.0, 90.0
	names := []string{"子项A", "子项B", "含]括号\\名称"}
	scores := map[string]*float64{"子项A": &a, "子项B": &b, "含]括号\\名称": &a}
	for _, item := range []struct {
		formula string
		want    float64
	}{
		{"[子项A]*0.6+[子项B]*0.4", 84},
		{"max([子项A]*0.6,10)", 48},
		{"MIN(MAX([子项A],10),[子项B],100)", 80},
		{"-(2+3)*4/2", -10},
		{`[含\]括号\\名称]`, 80},
		{"0", 0},
	} {
		got, err := scoreFormula(item.formula, names, scores)
		if err != nil || math.Abs(got-item.want) > 1e-10 {
			t.Fatalf("公式 %s：结果 %v，期望 %v，错误 %v", item.formula, got, item.want, err)
		}
	}
	for _, formula := range []string{
		"IF([专业]=1,2,3)", "[专业]+1", "[未知子项]", "MAX(1)",
		"[子项A]/0", "0/0", "1+", "(1+2", "SUM(1,2)", "1;2", "1e999",
	} {
		if _, err := scoreFormula(formula, names, scores); err == nil {
			t.Fatalf("无效公式被接受：%s", formula)
		}
	}
	if result, err := scoreFormula(
		"MAX([子项A]*0.6,10)",
		names,
		map[string]*float64{},
	); err != nil ||
		result != 10 {
		t.Fatal("缺少或未填的子项分数应按零参与计算")
	}
	if result, err := scoreFormula(
		"[子项A]+[子项B]",
		names,
		map[string]*float64{"子项A": nil, "子项B": &b},
	); err != nil ||
		result != 90 {
		t.Fatal("未填写与已填写子项应共同参与计算")
	}
	if _, err := scoreFormula(
		strings.Repeat("(", 100)+"1"+strings.Repeat(")", 100),
		names,
		nil,
	); err == nil {
		t.Fatal("过度嵌套的公式不应被接受")
	}
}

func TestScoreFormulaComprehensiveAssessment(t *testing.T) {
	// 保留实际使用的多行公式，覆盖中文名称、连字符、换行与多层权重。
	const formula = `min(75+[德育加减分项],100)*0.25+
min(65+[体美劳加减分项],100)*0.15+
0.6*(
  0.2*(0.7*[M1-1学位课]+0.3*[M1-2非学位课])+
  0.4*([M2-1论文]+[M2-2科研]+[M2-3专利])+
  0.2*([M3-1会议]+[M3-2竞赛])+
  0.2*[M4项目]
)`
	names := []string{
		"德育加减分项", "体美劳加减分项", "M1-1学位课", "M1-2非学位课", "M2-1论文",
		"M2-2科研", "M2-3专利", "M3-1会议", "M3-2竞赛", "M4项目",
	}
	calculation := &projectModels.ScoreCalculation{
		Enabled: true,
		Rules: []projectModels.ScoreRule{{
			Condition: projectModels.ScoreCondition{
				Type: "condition", Field: "grade", Operator: "in", Values: []string{"2026"},
			},
			Formula: formula,
		}},
	}
	if _, err := prepareScoreCalculation(calculation, names); err != nil {
		t.Fatalf("完整公式应能保存：%v", err)
	}

	// 期望值由各部分权重独立核算；未选择及显式空分数分别验证。
	for _, item := range []struct {
		name   string
		values map[string]float64
		want   float64
	}{
		{name: "全部未填写", want: 28.5},
		{
			name: "全部显式零分",
			values: map[string]float64{
				"德育加减分项": 0, "体美劳加减分项": 0, "M1-1学位课": 0, "M1-2非学位课": 0,
				"M2-1论文": 0, "M2-2科研": 0, "M2-3专利": 0,
				"M3-1会议": 0, "M3-2竞赛": 0, "M4项目": 0,
			},
			want: 28.5,
		},
		{
			name:   "只填写学业成绩",
			values: map[string]float64{"M1-1学位课": 80, "M1-2非学位课": 90},
			want:   38.46,
		},
		{
			name:   "只填写加分项",
			values: map[string]float64{"德育加减分项": 10, "体美劳加减分项": 20},
			want:   34,
		},
		{
			name:   "只填写科研成果",
			values: map[string]float64{"M2-1论文": 10, "M2-2科研": 20, "M2-3专利": 30},
			want:   42.9,
		},
		{
			name: "跨组部分填写",
			values: map[string]float64{
				"M2-1论文": 10, "M2-2科研": 20, "M3-2竞赛": 5, "M4项目": 40,
			},
			want: 41.1,
		},
		{
			name:   "减分项为负数",
			values: map[string]float64{"德育加减分项": -25, "体美劳加减分项": -15},
			want:   20,
		},
		{
			name:   "两项加分分别封顶",
			values: map[string]float64{"德育加减分项": 50, "体美劳加减分项": 60},
			want:   40,
		},
		{
			name: "全部填写不同分数",
			values: map[string]float64{
				"德育加减分项": 15, "体美劳加减分项": 25, "M1-1学位课": 90, "M1-2非学位课": 80,
				"M2-1论文": 10, "M2-2科研": 20, "M2-3专利": 5,
				"M3-1会议": 15, "M3-2竞赛": 25, "M4项目": 30,
			},
			want: 63.24,
		},
		{
			name: "总分不额外封顶",
			values: map[string]float64{
				"德育加减分项": 30, "体美劳加减分项": 40, "M1-1学位课": 100, "M1-2非学位课": 100,
				"M2-1论文": 100, "M2-2科研": 100, "M2-3专利": 100,
				"M3-1会议": 100, "M3-2竞赛": 100, "M4项目": 100,
			},
			want: 160,
		},
		{
			name:   "封顶与负分同时出现",
			values: map[string]float64{"德育加减分项": 100, "体美劳加减分项": -80},
			want:   22.75,
		},
	} {
		t.Run(item.name, func(t *testing.T) {
			for _, missing := range []string{"未选择子项", "分数为空"} {
				t.Run(missing, func(t *testing.T) {
					scores := make(map[string]*float64, len(names))
					if missing == "分数为空" {
						for _, name := range names {
							scores[name] = nil
						}
					}
					for name, value := range item.values {
						scores[name] = &value
					}
					result, err := evaluateScoreCalculation(
						calculation,
						names,
						map[string]string{"grade": "2026"},
						scores,
					)
					if err != nil {
						t.Fatalf("完整公式计算失败：%v", err)
					}
					if result.MatchedRule != 1 || math.Abs(result.Score-item.want) > 1e-9 {
						t.Fatalf("结果 %v，期望 %v，命中规则 %d", result.Score, item.want, result.MatchedRule)
					}
				})
			}
		})
	}
}

func TestScoreConditionInAndOrAndExactText(t *testing.T) {
	condition := projectModels.ScoreCondition{
		Type: "group", Logic: "and", Children: []projectModels.ScoreCondition{
			{Type: "condition", Field: "major", Operator: "in", Values: []string{"软件工程", "人工智能"}},
			{
				Type:     "condition",
				Field:    "education_level",
				Operator: "in",
				Values:   []string{"硕士", "博士"},
			},
			{Type: "group", Logic: "or", Children: []projectModels.ScoreCondition{
				{Type: "condition", Field: "grade", Operator: "in", Values: []string{"2024"}},
				{Type: "condition", Field: "class_name", Operator: "in", Values: []string{"01班"}},
			}},
		},
	}
	nodes := 0
	if err := cleanScoreCondition(&condition, 1, &nodes); err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"major": "人工智能", "class_name": "01班", "education_level": "博士"}
	if !matchesScoreCondition(condition, fields) {
		t.Fatal("in 候选值和 OR 子组应匹配")
	}
	fields["education_level"] = "本科"
	if matchesScoreCondition(condition, fields) {
		t.Fatal("AND 组必须满足学历条件")
	}
	fields["education_level"], fields["class_name"] = "硕士", "1班"
	if matchesScoreCondition(condition, fields) {
		t.Fatal("文本匹配不应丢失前导零")
	}
	for _, node := range []projectModels.ScoreCondition{
		{Type: "condition", Field: "major", Operator: "eq", Values: []string{"软件工程"}},
		{Type: "condition", Field: "student_no", Operator: "in", Values: []string{"01"}},
		{Type: "condition", Field: "grade", Operator: "in"},
		{
			Type: "condition", Field: "education_level", Operator: "in",
			Values: []string{strings.Repeat("学", 33)},
		},
		{Type: "group", Logic: "and"},
	} {
		count := 0
		if err := cleanScoreCondition(&node, 1, &count); err == nil {
			t.Fatal("不支持的条件被接受")
		}
	}
}

func TestScoreCalculationRoundsFinalTotal(t *testing.T) {
	// 检查浮点尾差、进位、负数和仅对最终结果舍入，保留大数的有限性。
	largeScore := math.MaxFloat64
	for _, item := range []struct {
		formula string
		want    float64
	}{
		{"46.480000000000004", 46.48},
		{"46.484", 46.48},
		{"46.485", 46.49},
		{"-46.485", -46.49},
		{"0.004+0.004", 0.01},
		{"0", 0},
		{"[子项A]", math.MaxFloat64},
	} {
		t.Run(item.formula, func(t *testing.T) {
			result, err := evaluateScoreCalculation(
				&projectModels.ScoreCalculation{
					Enabled: true,
					Rules: []projectModels.ScoreRule{{
						Condition: projectModels.ScoreCondition{
							Type:     "condition",
							Field:    "grade",
							Operator: "in",
							Values:   []string{"2026"},
						},
						Formula: item.formula,
					}},
				},
				[]string{"子项A"},
				map[string]string{"grade": "2026"},
				map[string]*float64{"子项A": &largeScore},
			)
			if err != nil || result.Score != item.want {
				t.Fatalf("结果 %v，期望 %v，错误 %v", result.Score, item.want, err)
			}
		})
	}
}

func TestForcedScoreCalculationIgnoresRawScoreAndRank(t *testing.T) {
	previousRank := int32(2)
	request := projectModels.ApplicationRequest{
		RawRank: json.RawMessage(`{"invalid":true}`), RawScore: json.RawMessage(`"不是数字"`),
	}
	content, err := parseApplicationScores(request, &projectModels.ScoreCalculation{
		Enabled: true,
	}, &previousRank)
	if err != nil || content.Score != nil || content.Rank == nil || *content.Rank != previousRank {
		t.Fatal("强制计算必须忽略传入总分、排名并保留已有排名")
	}
	if _, err := parseApplicationScores(request, nil, nil); err == nil {
		t.Fatal("普通模式仍须拒绝无效的手工总分或排名")
	}
	var payload projectModels.ApplicationPayload
	if err := json.Unmarshal(
		[]byte(`{"rank":"无效排名","score":{},"details":[]}`),
		&payload,
	); err != nil {
		t.Fatal("入口不能在读取强制计算配置前校验总分、排名类型")
	}
}
