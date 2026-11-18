package cpctimelineSvc

import (
	"encoding/json"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// syncPeople 在模板写入事务内同步绑定人员，节点删除会级联清理对应的附件元数据。
func syncPeople(
	tx *gorm.DB, workspaceID string, templateID uint64, nodes []cpctimelineModels.Node,
) error {
	// 读取同租户绑定关系与现有节点，不能根据人员可见范围遗漏其他绑定者。
	var people []uint64
	if err := tx.Model(&cpcpersonModels.Person{}).
		Where("workspace_id = ? AND timeline_template_id = ?", workspaceID, templateID).
		Order("id ASC").Pluck("id", &people).Error; err != nil {
		return err
	}
	var records []cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND timeline_template_id = ?", workspaceID, templateID).
		Order("person_id ASC, node_id ASC").Find(&records).Error; err != nil {
		return err
	}
	desired := make(map[string]cpctimelineModels.Node, len(nodes))
	for _, node := range nodes {
		desired[node.ID] = node
	}
	existing := make(map[uint64]map[string]bool, len(people))
	for _, record := range records {
		if existing[record.PersonID] == nil {
			existing[record.PersonID] = map[string]bool{}
		}
		existing[record.PersonID][record.NodeID] = true
		node, keep := desired[record.NodeID]
		if !keep {
			// 保存前已明确确认影响，删除填写内容与模板节点必须原子提交。
			if err := tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
				workspaceID, record.PersonID, record.NodeID).
				Delete(&cpcpersonModels.TimelineNode{}).Error; err != nil {
				return personNodeWriteError(err)
			}
			if err := syncAudit(tx, workspaceID, &record, nil); err != nil {
				return err
			}
			continue
		}
		values := map[string]any{}
		if err := json.Unmarshal([]byte(record.FieldValues), &values); err != nil {
			return err
		}
		visible := map[string]any{}
		cleared := false
		for _, field := range node.Fields {
			if field.StudentField != "" {
				if _, exists := values[field.ID]; exists {
					delete(values, field.ID)
					cleared = true
				}
				continue
			}
			if value, ok := values[field.ID]; ok {
				visible[field.ID] = value
			}
		}
		// 改为引用后清除所有历史填写值；手动字段失效时才退回待处理。
		after := record
		if cleared {
			encoded, err := json.Marshal(values)
			if err != nil {
				return err
			}
			after.FieldValues = string(encoded)
		}
		if record.Status == cpcpersonUtils.TimelineCompleted {
			if _, err := cpcpersonUtils.ValidateTimelineValues(node, visible); err != nil {
				after.Status = cpcpersonUtils.TimelinePending
			}
		}
		if !cleared && after.Status == record.Status {
			continue
		}
		after.Revision++
		if err := tx.Model(&cpcpersonModels.TimelineNode{}).
			Where("workspace_id = ? AND person_id = ? AND node_id = ?",
				workspaceID, record.PersonID, record.NodeID).
			Updates(map[string]any{
				"field_values": after.FieldValues,
				"status":       after.Status,
				"revision":     after.Revision,
			}).Error; err != nil {
			return err
		}
		if err := syncAudit(tx, workspaceID, &record, &after); err != nil {
			return err
		}
	}
	for _, personID := range people {
		for _, node := range nodes {
			if existing[personID][node.ID] {
				continue
			}
			record := cpcpersonModels.TimelineNode{
				WorkspaceID: workspaceID, PersonID: personID, TimelineTemplateID: templateID,
				NodeID: node.ID, FieldValues: "{}", Status: cpcpersonUtils.TimelinePending,
				Revision: 1,
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			if err := syncAudit(tx, workspaceID, nil, &record); err != nil {
				return err
			}
		}
	}
	for _, personID := range people {
		if err := cpcpersonUtils.RefreshTimelineStatus(
			tx,
			workspaceID,
			accessConstants.CPCTimelineUpdate,
			personID,
			templateID,
			nodes,
		); err != nil {
			return err
		}
	}
	return nil
}

func syncAudit(
	tx *gorm.DB, workspaceID string, before, after *cpcpersonModels.TimelineNode,
) error {
	record := after
	if record == nil {
		record = before
	}
	item := auditModels.Event{
		TenantID: &workspaceID, Action: accessConstants.CPCTimelineUpdate,
		ObjectType: "cpc_person_timeline_node", ObjectID: strconv.FormatUint(record.PersonID, 10),
		Result: auditConstants.Success,
		Conditions: auditModels.Snapshot{
			"operation": "sync_template", "timeline_template_id": record.TimelineTemplateID,
			"node_id": record.NodeID,
		},
	}
	if before != nil {
		item.Before = cpcpersonUtils.TimelineSnapshot(*before)
	}
	if after != nil {
		item.After = cpcpersonUtils.TimelineSnapshot(*after)
	}
	return auditSvc.Write(tx, item)
}
