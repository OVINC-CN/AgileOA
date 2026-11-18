package cpcpersonUtils

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// RequireTimelineWritable 阻止转移期间修改人员、节点及附件。
func RequireTimelineWritable(status *string) error {
	if status != nil && *status == TimelineTransferring {
		return &response.BusinessError{
			Status:  http.StatusConflict,
			Message: "人员时间线正在转移中，请先取消转移后再操作",
		}
	}
	return nil
}

// RefreshTimelineStatus 在调用方空间事务中重算异常索引，使筛选和详情结果一致。
func RefreshTimelineStatus(
	tx *gorm.DB, workspaceID, action string, personID, templateID uint64,
	nodes []cpctimelineModels.Node,
) error {
	_, err := refreshTimelineStatus(tx, workspaceID, action, personID, templateID, nodes)
	return err
}

func refreshTimelineStatus(
	tx *gorm.DB, workspaceID, action string, personID, templateID uint64,
	nodes []cpctimelineModels.Node,
) (bool, error) {
	var person cpcpersonModels.Person
	if err := tx.Where("workspace_id = ? AND id = ? AND timeline_template_id = ?",
		workspaceID, personID, templateID).Take(&person).Error; err != nil {
		return false, err
	}
	// 学生引用和模板重算不能覆盖待审核转移状态。
	if person.TimelineStatus != nil && *person.TimelineStatus == TimelineTransferring {
		return false, nil
	}
	var records []cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id = ? AND timeline_template_id = ?",
		workspaceID, personID, templateID).Find(&records).Error; err != nil {
		return false, err
	}
	birthDates, err := TimelineBirthDates(tx, workspaceID, nodes, []uint64{personID})
	if err != nil {
		return false, err
	}
	definitions := make(map[string]cpctimelineModels.Node, len(nodes))
	for _, node := range nodes {
		definitions[node.ID] = node
	}
	values := make(map[string]map[string]any, len(records))
	statuses := make(map[string]string, len(records))
	for _, record := range records {
		statuses[record.NodeID] = record.Status
		fields := map[string]any{}
		if err := json.Unmarshal([]byte(record.FieldValues), &fields); err != nil {
			return false, err
		}
		values[record.NodeID] = ResolveTimelineValues(
			definitions[record.NodeID], fields, birthDates[personID],
		)
	}
	status, _ := EvaluateTimeline(nodes, values, statuses)
	if person.TimelineStatus != nil && *person.TimelineStatus == status {
		return false, nil
	}
	if err := tx.Model(&cpcpersonModels.Person{}).
		Where("workspace_id = ? AND id = ? AND timeline_template_id = ?",
			workspaceID, personID, templateID).
		Updates(map[string]any{
			"timeline_status": status,
			"revision":        person.Revision + 1,
		}).Error; err != nil {
		return false, err
	}
	err = auditSvc.Write(tx, auditModels.Event{
		TenantID:   &workspaceID,
		Action:     action,
		ObjectType: "cpc_person",
		ObjectID:   strconv.FormatUint(personID, 10),
		Result:     auditConstants.Success,
		Conditions: auditModels.Snapshot{
			"operation":            "refresh_timeline_status",
			"timeline_template_id": templateID,
		},
		Before: auditModels.Snapshot{
			"timeline_status": person.TimelineStatus,
			"revision":        person.Revision,
		},
		After: auditModels.Snapshot{
			"timeline_status": status,
			"revision":        person.Revision + 1,
		},
	})
	return err == nil, err
}

// RefreshStudentTimelineStatuses 在学生资料修改事务中重算引用日期的时间线异常索引。
func RefreshStudentTimelineStatuses(
	tx *gorm.DB, workspaceID, action string, studentID uint64,
) ([]uint64, error) {
	// 只处理同租户绑定此学生的时间线，不改变节点填写值及完成状态。
	var people []cpcpersonModels.Person
	if err := tx.Select("id", "timeline_template_id").
		Where("workspace_id = ? AND student_id = ? AND timeline_template_id IS NOT NULL",
			workspaceID, studentID).
		Order("id ASC").Find(&people).Error; err != nil {
		return nil, err
	}
	changed := []uint64{}
	for _, person := range people {
		nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, *person.TimelineTemplateID)
		if err != nil {
			return nil, err
		}
		if !HasStudentFields(nodes) {
			continue
		}
		updated, err := refreshTimelineStatus(
			tx, workspaceID, action, person.ID, *person.TimelineTemplateID, nodes,
		)
		if err != nil {
			return nil, err
		}
		if updated {
			changed = append(changed, person.ID)
		}
	}
	return changed, nil
}
