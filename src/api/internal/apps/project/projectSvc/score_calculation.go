package projectSvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func decodeScoreCalculation(raw *string) (*projectModels.ScoreCalculation, error) {
	// 数据库只保存固定模型，拒绝未知字段及多个 JSON 文档。
	if raw == nil {
		return nil, nil
	}
	var result projectModels.ScoreCalculation
	decoder := json.NewDecoder(strings.NewReader(*raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) {
		return nil, invalid("分数计算配置无效，请修正项目配置")
	}
	if err := cleanRankingDimensions(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func prepareScoreCalculation(
	value *projectModels.ScoreCalculation,
	names []string,
) (*string, error) {
	// 配置与年度项目、子项目录及并发版本共用同一次保存。
	if value == nil {
		return nil, nil
	}
	limits := config.Config.Project
	if err := cleanRankingDimensions(value); err != nil {
		return nil, err
	}
	if len(value.Rules) > limits.ScoreMaxRules || value.Enabled && len(value.Rules) == 0 {
		return nil, invalid("分数计算规则组数量无效")
	}
	if value.Rules == nil {
		value.Rules = []projectModels.ScoreRule{}
	}
	for index := range value.Rules {
		rule := &value.Rules[index]
		nodes := 0
		if err := cleanScoreCondition(&rule.Condition, 1, &nodes); err != nil {
			return nil, invalid(fmt.Sprintf("第 %d 组：%s", index+1, err))
		}
		rule.Formula = strings.TrimSpace(rule.Formula)
		if _, err := scoreFormula(rule.Formula, names, nil); err != nil {
			return nil, invalid(fmt.Sprintf("第 %d 组：%s", index+1, err))
		}
	}
	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	result := string(content)
	return &result, nil
}

func cleanRankingDimensions(value *projectModels.ScoreCalculation) error {
	// 固定目录及顺序，组合不受选择先后影响。
	if !value.Enabled && len(value.RankingDimensions) > 0 {
		return invalid("请先启用总分计算再配置排名维度")
	}
	seen := make(map[string]bool, len(value.RankingDimensions))
	for _, field := range value.RankingDimensions {
		if !slices.Contains(projectConstants.RankingDimensions, field) || seen[field] {
			return invalid("排名维度仅支持年级、专业、班级，且不能重复")
		}
		seen[field] = true
	}
	value.RankingDimensions = []string{}
	for _, field := range projectConstants.RankingDimensions {
		if seen[field] {
			value.RankingDimensions = append(value.RankingDimensions, field)
		}
	}
	return nil
}

func cleanScoreCondition(node *projectModels.ScoreCondition, depth int, count *int) error {
	// 条件树仅接受基础维度、in 和非空 AND、OR 组。
	*count += 1
	limits := config.Config.Project
	if depth > limits.ScoreRuleMaxDepth || *count > limits.ScoreRuleMaxNodes {
		return errors.New("分数条件超过层数或节点数量限制")
	}
	switch node.Type {
	case "group":
		if node.Logic != "and" && node.Logic != "or" || len(node.Children) == 0 ||
			node.Field != "" || node.Operator != "" || len(node.Values) != 0 {
			return errors.New("条件组须选择全部满足或任一满足，且不能留空")
		}
		for index := range node.Children {
			if err := cleanScoreCondition(&node.Children[index], depth+1, count); err != nil {
				return err
			}
		}
	case "condition":
		maxLength := scoreDimensionLength(node.Field)
		if maxLength == 0 ||
			node.Operator != "in" || node.Logic != "" || len(node.Children) != 0 ||
			len(node.Values) == 0 || len(node.Values) > limits.ScoreRuleMaxValues {
			return errors.New("条件只支持年级、学历、专业、班级的 in 匹配，每个条件至少填写一个候选值")
		}
		values := make([]string, 0, len(node.Values))
		seen := map[string]bool{}
		for _, value := range node.Values {
			value = strings.TrimSpace(value)
			if value == "" || !validText(value, maxLength, false) {
				return fmt.Errorf("条件候选值不能为空、包含控制字符或超过 %d 字符", maxLength)
			}
			if !seen[value] {
				values = append(values, value)
				seen[value] = true
			}
		}
		node.Values = values
	default:
		return errors.New("分数条件类型无效")
	}
	return nil
}

func scoreDimensionLength(field string) int {
	// 条件与试算共用固定维度目录，学历长度与学生基础档案一致。
	switch field {
	case "grade", "major", "class_name":
		return projectConstants.NameLength
	case "education_level":
		return 32
	default:
		return 0
	}
}

func matchesScoreCondition(node projectModels.ScoreCondition, fields map[string]string) bool {
	// 候选值精确匹配，同一 in 条件内取并集。
	if node.Type == "condition" {
		for _, value := range node.Values {
			if fields[node.Field] == value {
				return true
			}
		}
		return false
	}
	for _, child := range node.Children {
		matches := matchesScoreCondition(child, fields)
		if node.Logic == "and" && !matches {
			return false
		}
		if node.Logic == "or" && matches {
			return true
		}
	}
	return node.Logic == "and"
}

func projectScoreCalculation(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
) (*projectModels.ScoreCalculation, error) {
	// 使用租户和项目编号的唯一索引读取当前配置，不查询 JSON 内容。
	var record projectModels.Project
	if err := tx.Select("score_calculation").
		Where("workspace_id = ? AND id = ?", workspaceID, projectID).
		Take(&record).Error; err != nil {
		return nil, lookupError(err)
	}
	return decodeScoreCalculation(record.ScoreCalculation)
}

func calculateApplicationScore(
	tx *gorm.DB,
	workspaceID string,
	projectID, studentID uint64,
	calculation *projectModels.ScoreCalculation,
	content *projectModels.ApplicationContent,
) error {
	// 只在现有保存、提交和审批事务中计算，不创建异步计算任务。
	if calculation == nil || !calculation.Enabled {
		return nil
	}
	var student struct{ Grade, EducationLevel, Major, ClassName string }
	if err := tx.Table("students").Select("grade", "education_level", "major", "class_name").
		Where("workspace_id = ? AND id = ?", workspaceID, studentID).
		Take(&student).Error; err != nil {
		return lookupError(err)
	}
	names, err := detailOptionNames(tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	if _, err := prepareScoreCalculation(calculation, names); err != nil {
		return err
	}
	items := make([]projectModels.ScoreItem, len(content.Details))
	for index, detail := range content.Details {
		items[index] = detail.ScoreItem
	}
	canonical, err := canonicalDetailNames(tx, workspaceID, projectID, items)
	if err != nil {
		return err
	}
	scores := make(map[string]*float64, len(items))
	for index, item := range items {
		scores[canonical[index]] = item.Score
	}
	fields := map[string]string{
		"grade":           student.Grade,
		"education_level": student.EducationLevel,
		"major":           student.Major,
		"class_name":      student.ClassName,
	}
	result, err := evaluateScoreCalculation(calculation, names, fields, scores)
	if err != nil {
		return err
	}
	content.Score = &result.Score
	return nil
}

func evaluateScoreCalculation(
	calculation *projectModels.ScoreCalculation,
	names []string,
	fields map[string]string,
	scores map[string]*float64,
) (projectModels.ScorePreview, error) {
	for index, rule := range calculation.Rules {
		if !matchesScoreCondition(rule.Condition, fields) {
			continue
		}
		result, err := scoreFormula(rule.Formula, names, scores)
		if err != nil {
			return projectModels.ScorePreview{}, invalid(
				fmt.Sprintf("第 %d 组总分计算失败：%s", index+1, err),
			)
		}
		// 仅在完整公式计算结束后四舍五入，试算与实际保存使用同一结果。
		const scoreScale = 100
		if scaled := result * scoreScale; !math.IsInf(scaled, 0) {
			result = math.Round(scaled) / scoreScale
		}
		return projectModels.ScorePreview{MatchedRule: index + 1, Score: result}, nil
	}
	return projectModels.ScorePreview{}, invalid("未匹配任何分数计算条件，请联系项目管理员修改规则")
}

// PreviewScoreCalculation 复用项目新增或编辑权限，审计提交后返回未保存配置的试算结果。
func PreviewScoreCalculation(
	ctx context.Context,
	workspaceID string,
	id uint64,
	request projectModels.ScorePreviewRequest,
) (projectModels.ScorePreview, error) {
	var result projectModels.ScorePreview
	action := accessConstants.ProjectCreate
	if id != 0 {
		action = accessConstants.ProjectUpdate
	}
	err := accessUtils.Workspace(
		ctx, workspaceID, action, event(workspaceID, "project_score_preview", id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 编辑试算核验当前租户内项目，新建试算不创建占位项目。
			if id != 0 {
				var project projectModels.Project
				if err := tx.Select("id").Where("workspace_id = ? AND id = ?", workspaceID, id).
					Take(&project).Error; err != nil {
					return lookupError(err)
				}
			}
			if !request.Calculation.Enabled {
				return invalid("请先启用总分计算")
			}
			// 子项目录及公式使用当前草稿，试算不要求先保存项目。
			names, err := cleanProjectDetailNames(request.DetailNames)
			if err != nil {
				return err
			}
			fields := map[string]string{
				"grade":           request.Dimensions.Grade,
				"education_level": request.Dimensions.EducationLevel,
				"major":           request.Dimensions.Major,
				"class_name":      request.Dimensions.ClassName,
			}
			for field, value := range fields {
				maxLength := scoreDimensionLength(field)
				if !validText(value, maxLength, false) {
					return invalid(fmt.Sprintf("试算维度值无效或超过 %d 字符", maxLength))
				}
			}
			if _, err := prepareScoreCalculation(&request.Calculation, names); err != nil {
				return err
			}
			items, err := cleanScoreItems(request.Details)
			if err != nil {
				return err
			}
			allowed := make(map[string]bool, len(names))
			for _, name := range names {
				allowed[name] = true
			}
			scores := make(map[string]*float64, len(items))
			for _, item := range items {
				if !allowed[item.Name] {
					return invalid("试算子项必须选择当前项目子项目录中的名称")
				}
				scores[item.Name] = item.Score
			}
			result, err = evaluateScoreCalculation(&request.Calculation, names, fields, scores)
			if err != nil {
				return err
			}
			audit.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

func parseApplicationScores(
	request projectModels.ApplicationRequest,
	calculation *projectModels.ScoreCalculation,
	currentRank *int32,
) (projectModels.ApplicationContent, error) {
	content := request.ApplicationContent
	// 强制计算先剔除客户端总分和排名，不对这两个传入值执行业务校验。
	if calculation != nil && calculation.Enabled {
		content.Score, content.Rank = nil, currentRank
		return content, nil
	}
	if len(request.RawRank) > 0 && json.Unmarshal(request.RawRank, &content.Rank) != nil {
		return content, invalid("排名必须为正整数或留空")
	}
	if len(request.RawScore) > 0 && json.Unmarshal(request.RawScore, &content.Score) != nil {
		return content, invalid("总分必须为有效数字或留空")
	}
	return content, nil
}
