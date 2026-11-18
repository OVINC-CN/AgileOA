package cpcpersonSvc

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func determineBranch(tx *gorm.DB, workspaceID string, studentID uint64) (*uint64, error) {
	expression, err := matchingExpression(tx, workspaceID)
	if err != nil {
		return nil, err
	}
	var result struct{ PartyBranchID *uint64 }
	if err := tx.Table("students AS s").Select("? AS party_branch_id", expression).
		Where("s.workspace_id = ? AND s.id = ?", workspaceID, studentID).
		Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, missing()
		}
		return nil, err
	}
	if result.PartyBranchID == nil {
		return nil, invalid("该学生未匹配任何党支部，请先维护党支部准入规则")
	}
	return result.PartyBranchID, nil
}

func matchingExpression(tx *gorm.DB, workspaceID string) (clause.Expression, error) {
	// 先按优先级、编号倒序确定全空间唯一归属，再检查书记范围，不能先过滤支部。
	var branches []partybranchModels.Branch
	if err := tx.Where("workspace_id = ? AND admission_rule IS NOT NULL", workspaceID).
		Order("priority DESC, id DESC").
		Limit(config.Config.CPCPerson.MaxOptions + 1).Find(&branches).Error; err != nil {
		return nil, err
	}
	if len(branches) > config.Config.CPCPerson.MaxOptions {
		return nil, invalid("党支部数量超过人员匹配配置上限")
	}
	if len(branches) == 0 {
		return clause.Expr{SQL: "NULL"}, nil
	}
	var sql strings.Builder
	sql.WriteString("CASE")
	values := make([]any, 0, len(branches)*2)
	for _, branch := range branches {
		expression, err := admissionExpression(branch.AdmissionRule)
		if err != nil {
			return nil, err
		}
		sql.WriteString(" WHEN ? THEN CAST(? AS UNSIGNED)")
		values = append(values, expression, branch.ID)
	}
	sql.WriteString(" ELSE NULL END")
	return clause.Expr{SQL: sql.String(), Vars: values}, nil
}

func admissionExpression(raw *string) (clause.Expression, error) {
	// 未配置规则不能解释为允许所有学生，参数只进入占位符。
	if raw == nil {
		return clause.Expr{SQL: "1 = 0"}, nil
	}
	var node partybranchModels.AdmissionRuleNode
	decoder := json.NewDecoder(strings.NewReader(*raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&node); err != nil {
		return nil, invalid("党支部准入规则无效，请在数据管理中修正")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, invalid("党支部准入规则包含多余内容")
	}
	nodes := 0
	return compileRule(node, 1, &nodes)
}

func compileRule(
	node partybranchModels.AdmissionRuleNode, depth int, nodes *int,
) (clause.Expression, error) {
	*nodes++
	limits := config.Config.PartyBranch
	if depth > limits.MaxRuleDepth || *nodes > limits.MaxRuleNodes {
		return nil, invalid("党支部准入规则超出配置限制")
	}
	if node.Type == "condition" {
		limit, ok := partybranchConstants.RuleFieldLengths[node.Field]
		if !ok || node.Operator != "in" ||
			node.Logic != "" || len(node.Children) != 0 ||
			len(node.Values) == 0 || len(node.Values) > limits.MaxRuleValues {
			return nil, invalid("党支部准入条件无效")
		}
		values := make([]any, len(node.Values))
		for index, value := range node.Values {
			if value == "" || !validText(value, limit) {
				return nil, invalid("党支部准入条件候选值无效")
			}
			values[index] = value
		}
		return clause.IN{Column: clause.Column{Table: "s", Name: node.Field}, Values: values}, nil
	}
	if node.Type != "group" || len(node.Children) == 0 || node.Field != "" ||
		node.Operator != "" || len(node.Values) != 0 ||
		(node.Logic != "and" && node.Logic != "or") {
		return nil, invalid("党支部准入条件组无效")
	}
	children := make([]clause.Expression, 0, len(node.Children))
	for _, child := range node.Children {
		expression, err := compileRule(child, depth+1, nodes)
		if err != nil {
			return nil, err
		}
		children = append(children, expression)
	}
	if node.Logic == "or" {
		return clause.Or(children...), nil
	}
	return clause.And(children...), nil
}
