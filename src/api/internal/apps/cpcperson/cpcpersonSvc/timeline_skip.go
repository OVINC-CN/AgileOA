package cpcpersonSvc

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

// SkipTimelineNode 保留节点填写值及附件，跳过必填校验并将节点标记为已跳过。
func SkipTimelineNode(
	ctx context.Context, workspaceID string, id uint64, nodeID string,
	request cpcpersonModels.SkipTimelineNodeRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	changed := false
	auditEvent := event(workspaceID, id)
	auditEvent.ObjectType = "cpc_person_timeline_node"
	auditEvent.Conditions["node_id"] = nodeID
	auditEvent.Conditions["operation"] = "skip"
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate, auditEvent,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 复用节点编辑的人员范围、节点归属和两个版本检查，不读取表单草稿。
			before, data, err := loadTimelineNodeForUpdate(
				tx, access, workspaceID, id, nodeID, request.Revision, request.TemplateRevision,
			)
			if err != nil {
				return err
			}
			audit.Before = cpcpersonUtils.TimelineSnapshot(before)
			if before.Status == cpcpersonUtils.TimelineSkipped {
				audit.After = audit.Before
				result = data
				return nil
			}

			// 状态、依赖节点异常索引及成功审计在同一个事务内提交。
			after := before
			after.Status, after.Revision = cpcpersonUtils.TimelineSkipped, before.Revision+1
			written := tx.Model(&cpcpersonModels.TimelineNode{}).
				Where("workspace_id = ? AND person_id = ? AND node_id = ? AND revision = ?",
					workspaceID, id, nodeID, request.Revision).
				Updates(map[string]any{
					"status":   after.Status,
					"revision": after.Revision,
				})
			if written.Error != nil {
				return written.Error
			}
			if written.RowsAffected != 1 {
				return timelineStale()
			}
			audit.After = cpcpersonUtils.TimelineSnapshot(after)
			definitions := make([]cpctimelineModels.Node, 0, len(data.Nodes))
			for _, node := range data.Nodes {
				definitions = append(definitions, node.Node)
			}
			if err := cpcpersonUtils.RefreshTimelineStatus(
				tx, workspaceID, accessConstants.CPCPersonUpdate, id, *data.TemplateID, definitions,
			); err != nil {
				return err
			}
			person, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			result, err = timelineData(tx, workspaceID, person)
			changed = err == nil
			return err
		})
	if err == nil && changed {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline node skipped; workspace: %s; person: %d; node: %s",
			workspaceID, id, nodeID,
		)
	}
	return result, err
}
