package cpcpersonSvc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// TimelineOptions 按列表、新增、编辑或导入权限提供模板选项，不要求模板管理权限。
func TimelineOptions(
	ctx context.Context, workspaceID, action, keyword string,
) ([]cpcpersonModels.Option, error) {
	result := []cpcpersonModels.Option{}
	err := within(ctx, workspaceID, action, event(workspaceID, 0),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 仅开放固定用途，并要求具有相应用途的人员操作范围。
			if action != accessConstants.CPCPersonList &&
				action != accessConstants.CPCPersonCreate &&
				action != accessConstants.CPCPersonUpdate &&
				action != accessConstants.CPCPersonImport {
				return invalid("模板选项用途无效")
			}
			if action != accessConstants.CPCPersonList {
				if err := access.require(action); err != nil {
					return err
				}
			}
			keyword = strings.TrimSpace(keyword)
			if !validText(keyword, 100) {
				return invalid("模板名称搜索条件无效")
			}
			scope := tx.Model(&cpctimelineModels.Template{}).Where("workspace_id = ?", workspaceID)
			if action == accessConstants.CPCPersonList {
				scope = readableTimelineTemplates(tx, workspaceID, access, action)
			}
			if keyword != "" {
				scope = scope.Where("INSTR(name, ?) > 0", keyword)
			}
			var templates []cpctimelineModels.Template
			if err := scope.Order("id DESC").Limit(config.Config.CPCPerson.MaxOptions + 1).
				Find(&templates).Error; err != nil {
				return err
			}
			if len(templates) > config.Config.CPCPerson.MaxOptions {
				return invalid("时间线模板较多，请输入名称搜索")
			}
			for _, template := range templates {
				id := strconv.FormatUint(template.ID, 10)
				result = append(result, cpcpersonModels.Option{Value: id, Label: template.Name})
				audit.ReturnedIDs = append(audit.ReturnedIDs, id)
			}
			audit.ReturnedCount = int64(len(result))
			audit.Conditions["keyword"] = keyword
			return nil
		})
	return result, err
}

// Timeline 按人员详情或编辑权限读取对应范围的节点与附件元数据。
func Timeline(
	ctx context.Context, workspaceID string, id uint64, editing bool,
) (cpcpersonModels.TimelineData, error) {
	action := accessConstants.CPCPersonDetail
	if editing {
		action = accessConstants.CPCPersonUpdate
	}
	return readTimeline(ctx, workspaceID, id, action)
}

// TimelineForReview 按独立审核权限读取人员时间线，不要求人员编辑权限。
func TimelineForReview(
	ctx context.Context, workspaceID string, id uint64,
) (cpcpersonModels.TimelineData, error) {
	return readTimeline(
		ctx,
		workspaceID,
		id,
		accessConstants.CPCPersonTimelineTransferReview,
	)
}

func readTimeline(
	ctx context.Context, workspaceID string, id uint64, action string,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	err := within(ctx, workspaceID, action, event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(action); err != nil {
				return err
			}
			person, err := detail(tx, workspaceID, id, access, action)
			if err != nil {
				return err
			}
			result, err = timelineData(tx, workspaceID, person)
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = int64(len(result.Nodes))
			return err
		})
	return result, err
}

// BindTimeline 为存量未绑定人员一次性绑定模板并建立全部待处理节点。
func BindTimeline(
	ctx context.Context, workspaceID string, id uint64,
	request cpcpersonModels.BindTimelineRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate, event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(accessConstants.CPCPersonUpdate); err != nil {
				return err
			}
			before, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			if before.Revision != request.Revision {
				return stale()
			}
			if before.TimelineTemplateID != nil {
				return invalid("人员已绑定时间线模板，不能重复绑定或更换")
			}
			if _, err := timelineTemplate(tx, workspaceID, request.TemplateID); err != nil {
				return err
			}

			// 绑定与节点创建共用空间锁及同一事务，不能出现部分节点。
			written := tx.Model(&cpcpersonModels.Person{}).
				Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, request.Revision).
				Updates(map[string]any{
					"timeline_template_id": request.TemplateID,
					"timeline_status":      cpcpersonUtils.TimelineNormal,
					"revision":             request.Revision + 1,
				})
			if written.Error != nil {
				return writeError(written.Error)
			}
			if written.RowsAffected != 1 {
				return stale()
			}
			if err := createTimelineNodes(
				tx, workspaceID, accessConstants.CPCPersonUpdate, id, request.TemplateID,
			); err != nil {
				return err
			}
			after, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			audit.Before, audit.After = snapshot(before), snapshot(after)
			result, err = timelineData(tx, workspaceID, after)
			return err
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline bound; workspace: %s; person: %d; template: %d",
			workspaceID, id, request.TemplateID,
		)
	}
	return result, err
}

// SaveTimelineNode 校验当前模板与全部必填值后将人员节点标记为已完成。
func SaveTimelineNode(
	ctx context.Context, workspaceID string, id uint64, nodeID string,
	request cpcpersonModels.TimelineNodeRequest,
) (cpcpersonModels.TimelineData, error) {
	var result cpcpersonModels.TimelineData
	err := within(ctx, workspaceID, accessConstants.CPCPersonUpdate, event(workspaceID, id),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(accessConstants.CPCPersonUpdate); err != nil {
				return err
			}
			person, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			if err := cpcpersonUtils.RequireTimelineWritable(person.TimelineStatus); err != nil {
				return err
			}
			data, err := timelineData(tx, workspaceID, person)
			if err != nil {
				return err
			}
			if data.TemplateID == nil || data.TemplateRevision != request.TemplateRevision {
				return timelineStale()
			}
			var node *cpcpersonModels.TimelineNodeInfo
			for index := range data.Nodes {
				if data.Nodes[index].ID == nodeID {
					node = &data.Nodes[index]
				}
			}
			if node == nil {
				return invalid("人员时间线节点不存在，请刷新后重试")
			}
			if node.Revision != request.Revision {
				return timelineStale()
			}
			values, err := cpcpersonUtils.ValidateTimelineValues(node.Node, request.Values)
			if err != nil {
				return invalid(err.Error())
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				return err
			}
			if len(encoded) > config.Config.CPCTimeline.MaxDefinitionBytes {
				return invalid("节点填写内容超过大小限制")
			}

			// 状态由服务端生成，不接受客户端指定完成或绕过必填校验。
			var before cpcpersonModels.TimelineNode
			if err := tx.Where(
				"workspace_id = ? AND person_id = ? AND timeline_template_id = ? AND node_id = ?",
				workspaceID, id, *data.TemplateID, nodeID,
			).Take(&before).Error; err != nil {
				return err
			}
			after := before
			after.FieldValues, after.Status = string(encoded), cpcpersonUtils.TimelineCompleted
			after.Revision++
			written := tx.Model(&cpcpersonModels.TimelineNode{}).
				Where("workspace_id = ? AND person_id = ? AND node_id = ? AND revision = ?",
					workspaceID, id, nodeID, request.Revision).
				Updates(map[string]any{
					"field_values": after.FieldValues, "status": after.Status,
					"revision": after.Revision,
				})
			if written.Error != nil {
				return written.Error
			}
			if written.RowsAffected != 1 {
				return timelineStale()
			}
			audit.ObjectType = "cpc_person_timeline_node"
			audit.Conditions["node_id"] = nodeID
			audit.Before = cpcpersonUtils.TimelineSnapshot(before)
			audit.After = cpcpersonUtils.TimelineSnapshot(after)
			definitions := make([]cpctimelineModels.Node, 0, len(data.Nodes))
			for _, item := range data.Nodes {
				definitions = append(definitions, item.Node)
			}
			if err := cpcpersonUtils.RefreshTimelineStatus(
				tx,
				workspaceID,
				accessConstants.CPCPersonUpdate,
				id,
				*data.TemplateID,
				definitions,
			); err != nil {
				return err
			}
			person, err = detail(tx, workspaceID, id, access, accessConstants.CPCPersonUpdate)
			if err != nil {
				return err
			}
			result, err = timelineData(tx, workspaceID, person)
			return err
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person timeline node completed; workspace: %s; person: %d; node: %s",
			workspaceID, id, nodeID,
		)
	}
	return result, err
}

// RequireTimelineNode 在调用方持有的空间锁中复核人员权限及节点归属，供附件共用。
func RequireTimelineNode(
	tx *gorm.DB, principal accessModels.Principal, personID uint64, nodeID, action string,
) (uint64, error) {
	access, err := loadAccess(tx, principal)
	if err != nil {
		return 0, err
	}
	if err := access.require(action); err != nil {
		return 0, err
	}
	person, err := detail(tx, principal.Workspace.ID, personID, access, action)
	if err != nil {
		return 0, err
	}
	if person.TimelineTemplateID == nil {
		return 0, invalid("请先绑定时间线模板")
	}
	var node cpcpersonModels.TimelineNode
	err = tx.Where("workspace_id = ? AND person_id = ? AND node_id = ?",
		principal.Workspace.ID, personID, nodeID).Take(&node).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, invalid("人员时间线节点不存在，请刷新后重试")
	}
	return node.TimelineTemplateID, err
}

func timelineTemplate(
	tx *gorm.DB, workspaceID string, id uint64,
) (cpctimelineModels.Template, error) {
	var template cpctimelineModels.Template
	if id == 0 {
		return template, invalid("请选择时间线模板")
	}
	err := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).Take(&template).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return template, invalid("时间线模板不存在或不可访问")
	}
	return template, err
}

func createTimelineNodes(
	tx *gorm.DB, workspaceID, action string, personID, templateID uint64,
) error {
	nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, templateID)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return invalid("时间线模板没有节点，请先维护模板")
	}
	records := make([]cpcpersonModels.TimelineNode, 0, len(nodes))
	for _, node := range nodes {
		records = append(records, cpcpersonModels.TimelineNode{
			WorkspaceID: workspaceID, PersonID: personID, TimelineTemplateID: templateID,
			NodeID: node.ID, FieldValues: "{}", Status: cpcpersonUtils.TimelinePending,
			Revision: 1,
		})
	}
	if err := tx.Create(&records).Error; err != nil {
		return err
	}
	if cpcpersonUtils.HasStudentFields(nodes) {
		return cpcpersonUtils.RefreshTimelineStatus(
			tx, workspaceID, action, personID, templateID, nodes,
		)
	}
	return nil
}

func timelineData(
	tx *gorm.DB, workspaceID string, person cpcpersonModels.Info,
) (cpcpersonModels.TimelineData, error) {
	result := cpcpersonModels.TimelineData{
		PersonID: person.ID, PersonRevision: person.Revision, TemplateID: person.TimelineTemplateID,
		CanUpdate: person.CanUpdate, Nodes: []cpcpersonModels.TimelineNodeInfo{},
		CanReviewTransfer: person.CanReviewTimelineTransfer,
		TransferTargetID:  person.TimelineTransferTargetID,
	}
	if person.TimelineStatus != nil &&
		*person.TimelineStatus == cpcpersonUtils.TimelineTransferring {
		result.CanUpdate = false
		result.CanCancelTransfer = person.CanUpdate
	}
	if person.TimelineTransferTargetID != nil {
		target, err := timelineTemplate(tx, workspaceID, *person.TimelineTransferTargetID)
		if err != nil {
			return result, err
		}
		result.TransferTargetName = target.Name
	}
	if person.TimelineTemplateID == nil {
		return result, nil
	}
	template, err := timelineTemplate(tx, workspaceID, *person.TimelineTemplateID)
	if err != nil {
		return result, err
	}
	result.TemplateName, result.TemplateRevision = template.Name, template.Revision
	var transfers int64
	if err := tx.Model(&cpctimelineModels.TemplateTransfer{}).
		Where("workspace_id = ? AND source_template_id = ?", workspaceID, template.ID).
		Count(&transfers).Error; err != nil {
		return result, err
	}
	result.HasTransferTargets = transfers > 0
	nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, template.ID)
	if err != nil {
		return result, err
	}
	var records []cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id = ? AND timeline_template_id = ?",
		workspaceID, person.ID, template.ID).Find(&records).Error; err != nil {
		return result, err
	}
	byID := make(map[string]cpcpersonModels.TimelineNode, len(records))
	for _, record := range records {
		byID[record.NodeID] = record
	}
	birthDates, err := cpcpersonUtils.TimelineBirthDates(
		tx, workspaceID, nodes, []uint64{person.ID},
	)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		record, ok := byID[node.ID]
		if !ok {
			return result, timelineStale()
		}
		values := map[string]any{}
		if err := json.Unmarshal([]byte(record.FieldValues), &values); err != nil {
			return result, err
		}
		// 引用实时读取学生资料，缺失时不回退到历史填写值。
		visible := cpcpersonUtils.ResolveTimelineValues(node, values, birthDates[person.ID])
		label := "待处理"
		switch record.Status {
		case cpcpersonUtils.TimelineCompleted:
			label = "已完成"
		case cpcpersonUtils.TimelineSkipped:
			label = "已跳过"
		}
		result.Nodes = append(result.Nodes, cpcpersonModels.TimelineNodeInfo{
			Node: node, Values: visible, Status: record.Status, StatusLabel: label,
			Revision: record.Revision, TemplateAttachments: []attachmentModels.Info{},
			Attachments: []attachmentModels.Info{},
		})
	}
	allValues := make(map[string]map[string]any, len(result.Nodes))
	statuses := make(map[string]string, len(result.Nodes))
	for _, node := range result.Nodes {
		allValues[node.ID] = node.Values
		statuses[node.ID] = node.Status
	}
	status, issues, checks := cpcpersonUtils.EvaluateTimelineChecks(nodes, allValues, statuses)
	result.Status = status
	if person.TimelineStatus != nil &&
		*person.TimelineStatus == cpcpersonUtils.TimelineTransferring {
		result.Status = cpcpersonUtils.TimelineTransferring
	}
	for index := range result.Nodes {
		result.Nodes[index].Issues = issues[result.Nodes[index].ID]
		result.Nodes[index].Checks = checks[result.Nodes[index].ID]
	}
	return result, timelineFiles(tx, workspaceID, &result)
}

func timelineFiles(tx *gorm.DB, workspaceID string, result *cpcpersonModels.TimelineData) error {
	if err := timelineTemplateFiles(tx, workspaceID, result); err != nil {
		return err
	}
	var personal []attachmentModels.PersonTimelineAttachment
	if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, result.PersonID).
		Order("id DESC").Find(&personal).Error; err != nil {
		return err
	}
	for index := range result.Nodes {
		node := &result.Nodes[index]
		for _, file := range personal {
			if file.NodeID == node.ID {
				info := timelineFileInfo(file.Record, file.NodeID)
				info.Category, info.PersonID = attachmentConstants.CategoryCPCPersonTimelineNode, result.PersonID
				node.Attachments = append(node.Attachments, info)
			}
		}
	}
	return nil
}

func timelineTemplateFiles(
	tx *gorm.DB,
	workspaceID string,
	result *cpcpersonModels.TimelineData,
) error {
	// 批量读取两个固定类别的元数据，模板附件只提供已完成上传的文件。
	var templates []attachmentModels.TimelineNodeAttachment
	if err := tx.Where("workspace_id = ? AND timeline_template_id = ? AND state = ?",
		workspaceID, *result.TemplateID, attachmentConstants.Ready).
		Order("id DESC").Find(&templates).Error; err != nil {
		return err
	}
	for index := range result.Nodes {
		node := &result.Nodes[index]
		for _, file := range templates {
			if file.NodeID == node.ID {
				info := timelineFileInfo(file.Record, file.NodeID)
				info.Category = attachmentConstants.CategoryCPCTimelineNode
				info.TimelineTemplateID = *result.TemplateID
				node.TemplateAttachments = append(node.TemplateAttachments, info)
			}
		}
	}
	return nil
}

func timelineFileInfo(file attachmentModels.Record, nodeID string) attachmentModels.Info {
	return attachmentModels.Info{
		ID: file.ID, NodeID: nodeID, Filename: file.Filename, Size: file.Size,
		State: file.State, Revision: file.Revision,
		CreatedAt: file.CreatedAt, UpdatedAt: file.UpdatedAt,
	}
}

func timelineStale() error {
	return &response.BusinessError{
		Status: http.StatusConflict, Message: "人员时间线或模板已被修改，请刷新后重试",
	}
}
