package partybranchSvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// ReadAdmissionRule 复用党支部查看权限读取规则树，审计持久化后返回。
func ReadAdmissionRule(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (partybranchModels.AdmissionRuleInfo, error) {
	var result partybranchModels.AdmissionRuleInfo
	err := accessUtils.Workspace(ctx, workspaceID,
		accessConstants.CPCPartyBranchList, admissionRuleEvent(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			var err error
			result, err = admissionRuleDetail(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = 1
			return nil
		})
	return result, err
}

// SaveAdmissionRule 替换或清空准入规则，和党支部基础信息共用并发版本。
func SaveAdmissionRule(
	ctx context.Context,
	workspaceID string,
	id uint64,
	request partybranchModels.AdmissionRuleRequest,
) (partybranchModels.AdmissionRuleInfo, error) {
	var result partybranchModels.AdmissionRuleInfo
	err := accessUtils.Workspace(ctx, workspaceID,
		accessConstants.CPCPartyBranchUpdate, admissionRuleEvent(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验租户、版本、树结构、固定字段及配置边界。
			before, err := admissionRuleDetail(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			if request.Revision != before.Revision {
				return stale()
			}
			_, content, err := prepareAdmissionRule(request.Rule)
			if err != nil {
				return err
			}
			// 仅更新规则列，避免覆盖支部基础信息；写入与审计共用事务。
			if err := tx.Model(&partybranchModels.Branch{}).
				Where("workspace_id = ? AND id = ?", principal.Workspace.ID, id).
				Updates(map[string]any{
					"admission_rule": content,
					"revision":       before.Revision + 1,
				}).Error; err != nil {
				return writeError(err)
			}
			result, err = admissionRuleDetail(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			audit.Before = admissionRuleSnapshot(before)
			audit.After = admissionRuleSnapshot(result)
			return nil
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Party branch admission rule updated; action: %s; workspace: %s; branch: %d",
			accessConstants.CPCPartyBranchUpdate, workspaceID, id,
		)
	}
	return result, err
}

func admissionRuleDetail(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
) (partybranchModels.AdmissionRuleInfo, error) {
	var record partybranchModels.Branch
	if err := tx.Select("id", "admission_rule", "revision", "updated_at").
		Where("workspace_id = ? AND id = ?", workspaceID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return partybranchModels.AdmissionRuleInfo{}, invalidBranch()
		}
		return partybranchModels.AdmissionRuleInfo{}, err
	}
	result := partybranchModels.AdmissionRuleInfo{
		ID:        record.ID,
		Revision:  record.Revision,
		UpdatedAt: record.UpdatedAt,
		Fields:    append([]partybranchModels.RuleField{}, partybranchConstants.RuleFields...),
		Logics:    append([]partybranchModels.RuleOption{}, partybranchConstants.RuleLogics...),
		Operators: append([]partybranchModels.RuleOption{}, partybranchConstants.RuleOperators...),
	}
	if record.AdmissionRule != nil {
		// 按固定节点模型读取，存储异常不能静默解释为允许准入。
		decoder := json.NewDecoder(strings.NewReader(*record.AdmissionRule))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result.Rule); err != nil {
			return result, fmt.Errorf("party branch admission rule decoding failed: %w", err)
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return result, errors.New("party branch admission rule has trailing data")
		}
	}
	return result, nil
}

func admissionRuleSnapshot(info partybranchModels.AdmissionRuleInfo) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":             info.ID,
		"admission_rule": info.Rule,
		"revision":       info.Revision,
	}
}

func admissionRuleEvent(workspaceID string, id uint64) auditModels.Event {
	result := event(workspaceID, id)
	result.Conditions["resource"] = "admission_rule"
	return result
}

func prepareAdmissionRule(
	rule *partybranchModels.AdmissionRuleNode,
) (*partybranchModels.AdmissionRuleNode, *string, error) {
	cleaned, err := cleanAdmissionRule(rule)
	if err != nil || cleaned == nil {
		return nil, nil, err
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return nil, nil, err
	}
	content := string(encoded)
	return cleaned, &content, nil
}

func cleanAdmissionRule(
	rule *partybranchModels.AdmissionRuleNode,
) (*partybranchModels.AdmissionRuleNode, error) {
	if rule == nil {
		return nil, nil
	}
	count := 0
	result, err := cleanRuleNode(*rule, 1, &count)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func cleanRuleNode(
	node partybranchModels.AdmissionRuleNode,
	depth int,
	count *int,
) (partybranchModels.AdmissionRuleNode, error) {
	// 在递归前检查层数及总节点数，条件组和条件均计入配置上限。
	*count += 1
	if depth > config.Config.PartyBranch.MaxRuleDepth ||
		*count > config.Config.PartyBranch.MaxRuleNodes {
		return node, invalid("准入规则超过层数或节点数量限制")
	}
	switch node.Type {
	case partybranchConstants.RuleGroup:
		validLogic := node.Logic == partybranchConstants.RuleAnd ||
			node.Logic == partybranchConstants.RuleOr
		mixedFields := node.Field != "" || node.Operator != "" || len(node.Values) != 0
		if !validLogic || mixedFields || len(node.Children) == 0 {
			return node, invalid("条件组须选择 AND 或 OR，且至少包含一个条件或子组")
		}
		result := partybranchModels.AdmissionRuleNode{Type: node.Type, Logic: node.Logic}
		for _, child := range node.Children {
			cleaned, err := cleanRuleNode(child, depth+1, count)
			if err != nil {
				return node, err
			}
			result.Children = append(result.Children, cleaned)
		}
		return result, nil
	case partybranchConstants.RuleCondition:
		if node.Logic != "" || len(node.Children) != 0 ||
			node.Operator != partybranchConstants.RuleIn {
			return node, invalid("请选择有效的匹配条件")
		}
		limit, ok := partybranchConstants.RuleFieldLengths[node.Field]
		if !ok {
			return node, invalid("准入条件仅支持导师姓名、专业、学历和学号")
		}
		if len(node.Values) == 0 || len(node.Values) > config.Config.PartyBranch.MaxRuleValues {
			return node, invalid("每个准入条件须填写候选值，且不能超过数量上限")
		}
		result := partybranchModels.AdmissionRuleNode{
			Type: node.Type, Field: node.Field, Operator: node.Operator,
		}
		seen := make(map[string]bool, len(node.Values))
		for _, value := range node.Values {
			value = strings.TrimSpace(value)
			if value == "" || !validText(value, limit) {
				return node, invalid("准入条件候选值不能为空、包含控制字符或超过字段长度限制")
			}
			if !seen[value] {
				result.Values = append(result.Values, value)
				seen[value] = true
			}
		}
		return result, nil
	default:
		return node, invalid("准入规则节点类型无效")
	}
}

func invalidBranch() error {
	return &response.BusinessError{Status: http.StatusNotFound, Message: "党支部不存在或不可访问"}
}
