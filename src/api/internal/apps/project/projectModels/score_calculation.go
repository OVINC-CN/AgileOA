package projectModels

import "github.com/OVINC-CN/AgileOA/src/api/internal/response"

// ScoreCondition 是仅支持 in 的学生基础维度条件及 AND、OR 条件组。
type ScoreCondition struct {
	// Type 是 condition 或 group。
	Type string `json:"type" example:"condition" enums:"condition,group"`
	// Logic 是条件组的 and 或 or，叶子条件不填写。
	Logic string `json:"logic,omitempty" example:"and" enums:"and,or"`
	// Children 是条件组内的非空子节点。
	Children []ScoreCondition `json:"children,omitempty"`
	// Field 仅允许 grade、education_level、major 或 class_name。
	Field string `json:"field,omitempty" example:"major" enums:"grade,education_level,major,class_name"`
	// Operator 固定为 in，按完整文本匹配候选值。
	Operator string `json:"operator,omitempty" example:"in" enums:"in"`
	// Values 是同一条件取并集的非空候选值数组。
	Values []string `json:"values,omitempty" example:"软件工程,人工智能"`
}

// ScoreRule 将一套条件与一个纯计算公式组合保存，按数组顺序匹配。
type ScoreRule struct {
	// Condition 是必填的条件或条件组，不匹配时继续下一组。
	Condition ScoreCondition `json:"condition"`
	// Formula 支持 [子项名称]、四则运算、括号、MAX 和 MIN，不接受 IF 或维度引用。
	Formula string `json:"formula" example:"MAX([志愿服务]*0.6,10)"`
}

// ScoreCalculation 是当前年度项目的学生申报总分计算配置。
type ScoreCalculation struct {
	// Enabled 开启后总分仅由公式计算，忽略请求中的总分和排名。
	Enabled bool `json:"enabled" example:"false"`
	// RankingDimensions 按年级、专业、班级的任意组合分组排名，空数组表示全奖项统一排名。
	// 仅启用总分计算时可配置；已通过申报按总分降序，同分并列且跳号，空总分按 0 分。
	RankingDimensions []string `json:"ranking_dimensions" example:"grade,major" enums:"grade,major,class_name"`
	// Rules 是按顺序使用首个匹配组的规则，不提供默认或兜底公式。
	Rules []ScoreRule `json:"rules"`
}

// ScoreDimensions 是试算使用的基础维度，字段不能扩展为任意学生资料。
type ScoreDimensions struct {
	// Grade 是试算年级，按完整文本匹配。
	Grade string `json:"grade" example:"2024"`
	// EducationLevel 是试算学历，按完整文本匹配，最长 32 字符。
	EducationLevel string `json:"education_level" example:"博士" maxLength:"32"`
	// Major 是试算专业。
	Major string `json:"major" example:"软件工程"`
	// ClassName 是试算班级。
	ClassName string `json:"class_name" example:"软件一班"`
}

// ScorePreviewRequest 使用当前未保存的项目配置试算，不写入业务记录。
type ScorePreviewRequest struct {
	// Calculation 是当前编辑的总分计算配置。
	Calculation ScoreCalculation `json:"score_calculation"`
	// DetailNames 是当前编辑的项目子项目录。
	DetailNames []string `json:"detail_names" example:"志愿服务,社会实践"`
	// Dimensions 是本次试算使用的固定维度。
	Dimensions ScoreDimensions `json:"dimensions"`
	// Details 是试算子项分数，未填写的分数为 null。
	Details []ScoreItem `json:"details"`
}

// ScorePreview 是试算和申报保存共用计算器的结果。
type ScorePreview struct {
	// MatchedRule 是首个匹配的规则组顺序号，从 1 开始。
	MatchedRule int `json:"matched_rule" example:"1"`
	// Score 是公式计算得到的总分，最终结果四舍五入到两位小数。
	Score float64 `json:"score" example:"84.25"`
}

type ScorePreviewResponse struct {
	response.Envelope[ScorePreview]
}
