package cpctimelineUtils

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// LoadTransfers 按配置顺序读取同空间来源模板的转移规则。
func LoadTransfers(
	tx *gorm.DB, workspaceID string, templateID uint64,
) ([]cpctimelineModels.Transfer, error) {
	var records []cpctimelineModels.TemplateTransfer
	if err := tx.Where("workspace_id = ? AND source_template_id = ?", workspaceID, templateID).
		Order("position ASC, target_template_id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]cpctimelineModels.Transfer, 0, len(records))
	for _, record := range records {
		item := cpctimelineModels.Transfer{TargetTemplateID: record.TargetTemplateID}
		if err := json.Unmarshal([]byte(record.NodeMappings), &item.NodeMappings); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// ValidateTransfer 按当前节点目录校验映射，禁止引用字段、类型转换及目标覆盖。
func ValidateTransfer(
	source, target []cpctimelineModels.Node, mapping cpctimelineModels.Transfer,
) error {
	// 节点与字段均来自固定目录，不接受任意数据库名称。
	if len(mapping.NodeMappings) == 0 || len(mapping.NodeMappings) > len(source)*len(target) {
		return fmt.Errorf("请配置有效的转移节点映射")
	}
	sources, targets := transferNodes(source), transferNodes(target)
	pairs := map[[2]string]bool{}
	usedFields := map[[2]string]bool{}
	for _, pair := range mapping.NodeMappings {
		from, sourceOK := sources[pair.SourceNodeID]
		to, targetOK := targets[pair.TargetNodeID]
		key := [2]string{pair.SourceNodeID, pair.TargetNodeID}
		if !sourceOK || !targetOK || pairs[key] {
			return fmt.Errorf("转移节点必须存在，且同一来源和目标节点对不能重复")
		}
		pairs[key] = true
		if pair.Fields == nil || len(pair.Fields) > len(to.Fields) {
			return fmt.Errorf("请提供有效的转移字段映射列表")
		}
		fromFields, toFields := transferFields(from), transferFields(to)
		for _, field := range pair.Fields {
			left, leftOK := fromFields[field.SourceFieldID]
			right, rightOK := toFields[field.TargetFieldID]
			if !leftOK || !rightOK || left.StudentField != "" || right.StudentField != "" {
				return fmt.Errorf("转移字段必须存在且不能包含引用字段")
			}
			fieldKey := [2]string{to.ID, right.ID}
			if left.Type != right.Type || usedFields[fieldKey] {
				return fmt.Errorf("转移字段类型必须一致，且同一目标字段只能映射一次")
			}
			usedFields[fieldKey] = true
		}
	}
	return nil
}

func transferNodes(nodes []cpctimelineModels.Node) map[string]cpctimelineModels.Node {
	result := make(map[string]cpctimelineModels.Node, len(nodes))
	for _, node := range nodes {
		result[node.ID] = node
	}
	return result
}

func transferFields(node cpctimelineModels.Node) map[string]cpctimelineModels.Field {
	result := make(map[string]cpctimelineModels.Field, len(node.Fields))
	for _, field := range node.Fields {
		result[field.ID] = field
	}
	return result
}
