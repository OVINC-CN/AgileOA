package cpctimelineSvc

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func syncNodes(
	tx *gorm.DB,
	workspaceID string,
	templateID uint64,
	nodes []cpctimelineModels.Node,
) error {
	// 节点定义及展示顺序只存子表，稳定标识不随重命名和排序改变。
	records, err := nodeRecords(workspaceID, templateID, nodes)
	if err != nil {
		return err
	}
	var current []cpctimelineModels.TemplateNode
	if err := tx.Select("node_id").
		Where("workspace_id = ? AND timeline_template_id = ?", workspaceID, templateID).
		Find(&current).Error; err != nil {
		return err
	}
	existing := make(map[string]bool, len(current))
	for _, node := range current {
		existing[node.NodeID] = true
	}
	desired := make(map[string]bool, len(nodes))
	for _, record := range records {
		desired[record.NodeID] = true
		if !existing[record.NodeID] {
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			continue
		}
		if err := tx.Model(&cpctimelineModels.TemplateNode{}).
			Where("workspace_id = ? AND timeline_template_id = ? AND node_id = ?",
				workspaceID, templateID, record.NodeID).
			Updates(map[string]any{
				"name": record.Name, "description": record.Description,
				"group_name":        record.GroupName,
				"is_baseline":       record.IsBaseline,
				"time_field_id":     record.TimeFieldID,
				"field_definitions": record.FieldDefinitions,
				"validation_rules":  record.ValidationRules,
				"position":          record.Position,
			}).Error; err != nil {
			return err
		}
	}

	// 先同步全部绑定人员，再删除模板节点；任一步失败回滚整个模板修改。
	if err := syncPeople(tx, workspaceID, templateID, nodes); err != nil {
		return err
	}

	// 移除节点时按外键级联清理对应的人员节点及附件记录。
	for _, node := range current {
		if desired[node.NodeID] {
			continue
		}
		err := tx.Where("workspace_id = ? AND timeline_template_id = ? AND node_id = ?",
			workspaceID, templateID, node.NodeID).Delete(&cpctimelineModels.TemplateNode{}).Error
		var database *mysql.MySQLError
		if errors.As(err, &database) && database.Number == 1451 {
			return &response.BusinessError{
				Status: http.StatusConflict, Message: "删除节点失败，请检查关联业务后重试",
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func nodeRecords(
	workspaceID string,
	templateID uint64,
	nodes []cpctimelineModels.Node,
) ([]cpctimelineModels.TemplateNode, error) {
	// 数组顺序转换为从一开始的显式位置，字段和规则各自保存为节点 JSON 列。
	records := make([]cpctimelineModels.TemplateNode, 0, len(nodes))
	for index, node := range nodes {
		fields, err := json.Marshal(node.Fields)
		if err != nil {
			return nil, err
		}
		rules, err := json.Marshal(node.Rules)
		if err != nil {
			return nil, err
		}
		records = append(records, cpctimelineModels.TemplateNode{
			WorkspaceID: workspaceID, TimelineTemplateID: templateID, NodeID: node.ID,
			Name: node.Name, Description: node.Description, TimeFieldID: node.TimeFieldID,
			GroupName:        node.GroupName,
			IsBaseline:       node.IsBaseline,
			FieldDefinitions: string(fields), ValidationRules: string(rules), Position: index + 1,
		})
	}
	return records, nil
}

func personNodeWriteError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) && database.Number == 1451 {
		return &response.BusinessError{
			Status:  http.StatusConflict,
			Message: "删除人员节点失败，请检查关联业务后重试",
		}
	}
	return err
}
