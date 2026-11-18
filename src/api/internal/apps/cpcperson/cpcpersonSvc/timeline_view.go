package cpcpersonSvc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func readableTimelineTemplates(
	tx *gorm.DB, workspaceID string, access personAccess, action string,
) *gorm.DB {
	// 模板目录仅包含当前操作范围内人员实际绑定的模板。
	people := access.limit(
		tx.Table("cpc_people AS readable").Select("1").
			Where("readable.workspace_id = cpc_timeline_templates.workspace_id").
			Where("readable.timeline_template_id = cpc_timeline_templates.id"),
		action,
		"readable.party_branch_id",
	)
	return tx.Model(&cpctimelineModels.Template{}).
		Where("workspace_id = ?", workspaceID).Where("EXISTS (?)", people)
}

// TimelineView 按人员列表权限提供已绑定模板的节点和字段目录。
func TimelineView(
	ctx context.Context, workspaceID string, templateID uint64,
) (cpcpersonModels.TimelineViewData, error) {
	result := cpcpersonModels.TimelineViewData{Nodes: []cpctimelineModels.Node{}}
	audit := event(workspaceID, 0)
	audit.Conditions["timeline_template_id"] = templateID
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, audit,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			var template cpctimelineModels.Template
			err := readableTimelineTemplates(
				tx,
				workspaceID,
				access,
				accessConstants.CPCPersonList,
			).
				Where("id = ?", templateID).
				Take(&template).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return missingTimelineView()
			}
			if err != nil {
				return err
			}
			result.TemplateID, result.TemplateName = template.ID, template.Name
			result.Nodes, err = cpctimelineUtils.LoadNodes(tx, workspaceID, template.ID)
			audit.ReturnedCount = int64(len(result.Nodes))
			return err
		})
	return result, err
}

// TimelineViewList 分页读取所选节点全部字段，与人员视图共用筛选和授权范围。
func TimelineViewList(
	ctx context.Context, workspaceID string,
	templateID, nodeID uint64, query cpcpersonModels.Query,
) (cpcpersonModels.TimelineViewListData, error) {
	result := cpcpersonModels.TimelineViewListData{
		Items: []cpcpersonModels.TimelineViewPerson{}, Page: query.Page, PageSize: query.PageSize,
	}
	query.TemplateID = templateID
	query.NodeID = nodeID
	audit := queryEvent(workspaceID, query)
	audit.Conditions["node_id"] = nodeID
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, audit,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			result.CanCreate = access.require(accessConstants.CPCPersonCreate) == nil
			result.CanImport = access.require(accessConstants.CPCPersonImport) == nil
			result.CanExport = access.require(accessConstants.CPCPersonExport) == nil
			node, scope, err := timelineViewScope(
				tx, workspaceID, templateID, nodeID, query, access, accessConstants.CPCPersonList,
			)
			if err != nil {
				return err
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			var people []cpcpersonModels.Info
			if err := scope.Order("p.id DESC").Limit(query.PageSize).
				Offset((query.Page - 1) * query.PageSize).Find(&people).Error; err != nil {
				return err
			}

			// 仅批量加载当前页的节点值和学生引用，避免逐人查询完整时间线。
			ids := make([]uint64, 0, len(people))
			for _, person := range people {
				ids = append(ids, person.ID)
			}
			if len(ids) == 0 {
				return nil
			}
			var records []cpcpersonModels.TimelineNode
			if err := tx.Where(
				"workspace_id = ? AND timeline_template_id = ? AND node_id = ? AND person_id IN ?",
				workspaceID, templateID, node.ID, ids,
			).Find(&records).Error; err != nil {
				return err
			}
			birthDates, err := cpcpersonUtils.TimelineBirthDates(
				tx, workspaceID, []cpctimelineModels.Node{node}, ids,
			)
			if err != nil {
				return err
			}
			byID := make(map[uint64]cpcpersonModels.TimelineNode, len(records))
			for _, record := range records {
				byID[record.PersonID] = record
			}
			for _, person := range people {
				record, ok := byID[person.ID]
				if !ok {
					return timelineStale()
				}
				values := map[string]any{}
				if err := json.Unmarshal([]byte(record.FieldValues), &values); err != nil {
					return err
				}
				setCapabilities(&person, access)
				label := "待处理"
				switch record.Status {
				case cpcpersonUtils.TimelineCompleted:
					label = "已完成"
				case cpcpersonUtils.TimelineSkipped:
					label = "已跳过"
				}
				result.Items = append(result.Items, cpcpersonModels.TimelineViewPerson{
					Person:      person,
					Status:      record.Status,
					StatusLabel: label,
					Values: cpcpersonUtils.ResolveTimelineValues(
						node,
						values,
						birthDates[person.ID],
					),
				})
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(person.ID, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// TimelineViewStatistics 按独立人员统计权限汇总所选节点全部筛选结果。
func TimelineViewStatistics(
	ctx context.Context, workspaceID string,
	templateID, nodeID uint64, query cpcpersonModels.Query,
) (cpcpersonModels.TimelineViewStatisticsData, error) {
	result := cpcpersonModels.TimelineViewStatisticsData{}
	query.TemplateID = templateID
	query.NodeID = nodeID
	audit := queryEvent(workspaceID, query)
	audit.Conditions["node_id"] = nodeID
	err := within(ctx, workspaceID, accessConstants.CPCPersonStatistics, audit,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			_, scope, err := timelineViewScope(
				tx,
				workspaceID,
				templateID,
				nodeID,
				query,
				access,
				accessConstants.CPCPersonStatistics,
			)
			if err != nil {
				return err
			}
			var counts []struct {
				Status string
				Count  int64
			}
			if err := scope.Select("selected_node.status, COUNT(*) AS count").
				Group("selected_node.status").Scan(&counts).Error; err != nil {
				return err
			}
			for _, count := range counts {
				result.Total += count.Count
				switch count.Status {
				case cpcpersonUtils.TimelineCompleted:
					result.Completed = count.Count
				case cpcpersonUtils.TimelineSkipped:
					result.Skipped = count.Count
				case cpcpersonUtils.TimelinePending:
					result.Pending = count.Count
				}
			}
			audit.Conditions["matched_people"] = result.Total
			audit.Conditions["group_by"] = "node_status"
			audit.ReturnedCount = int64(len(counts))
			return nil
		})
	return result, err
}

func timelineViewScope(
	tx *gorm.DB, workspaceID string, templateID, nodeID uint64,
	query cpcpersonModels.Query, access personAccess, action string,
) (cpctimelineModels.Node, *gorm.DB, error) {
	// 节点只能来自同空间的所选模板，所有关联同时限定人员、模板及租户。
	nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, templateID)
	if err != nil {
		return cpctimelineModels.Node{}, nil, err
	}
	for _, node := range nodes {
		if node.NumericID != nodeID {
			continue
		}
		query.TemplateID = templateID
		scope, err := filtered(tx, workspaceID, query, access, action)
		if err != nil {
			return cpctimelineModels.Node{}, nil, err
		}
		scope = scope.Joins("JOIN cpc_person_timeline_nodes AS selected_node ON "+
			"selected_node.workspace_id = p.workspace_id AND selected_node.person_id = p.id AND "+
			"selected_node.timeline_template_id = p.timeline_template_id AND selected_node.node_id = ?",
			node.ID)
		return node, scope, nil
	}
	return cpctimelineModels.Node{}, nil, missingTimelineView()
}

func missingTimelineView() error {
	return &response.BusinessError{
		Status: http.StatusNotFound, Message: "时间线模板或节点不存在或不可访问",
	}
}
