package cpctimelineUtils

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// LoadNodes 按模板定义顺序读取节点，供模板及人员时间线共用。
func LoadNodes(
	tx *gorm.DB,
	workspaceID string,
	templateID uint64,
) ([]cpctimelineModels.Node, error) {
	// 以子表保存的位置为唯一展示顺序来源，编号仅在异常重复位置时提供稳定排序。
	var records []cpctimelineModels.TemplateNode
	if err := tx.Where("workspace_id = ? AND timeline_template_id = ?", workspaceID, templateID).
		Order("position ASC, node_id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	nodes := make([]cpctimelineModels.Node, 0, len(records))
	for _, record := range records {
		node := cpctimelineModels.Node{
			NumericID: record.NumericID,
			ID:        record.NodeID, Name: record.Name, Description: record.Description,
			GroupName:  record.GroupName,
			IsBaseline: record.IsBaseline, TimeFieldID: record.TimeFieldID,
		}
		if err := json.Unmarshal([]byte(record.FieldDefinitions), &node.Fields); err != nil {
			return nil, fmt.Errorf("timeline node fields decoding failed: %w", err)
		}
		if err := json.Unmarshal([]byte(record.ValidationRules), &node.Rules); err != nil {
			return nil, fmt.Errorf("timeline node rules decoding failed: %w", err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
