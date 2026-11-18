package cpcpersonSvc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

type timelineFilePlacement struct {
	Source attachmentModels.PersonTimelineAttachment
	NodeID string
}

type timelineTransferPlan struct {
	Person  cpcpersonModels.Info
	Source  cpcpersonModels.TimelineData
	Preview cpcpersonModels.TimelineTransferPreview
	Mapping cpctimelineModels.Transfer
	Files   []timelineFilePlacement
}

func transferSource(
	tx *gorm.DB, access personAccess, workspaceID string, id uint64, action string,
) (cpcpersonModels.Info, cpcpersonModels.TimelineData, error) {
	// 按发起或审核的独立权限复核范围，再实时计算全部节点，不能信任列表状态。
	var data cpcpersonModels.TimelineData
	if err := access.require(action); err != nil {
		return cpcpersonModels.Info{}, data, err
	}
	person, err := detail(tx, workspaceID, id, access, action)
	if err != nil {
		return person, data, err
	}
	if person.TimelineTemplateID == nil {
		return person, data, invalid("请先绑定时间线模板")
	}
	if action == accessConstants.CPCPersonUpdate {
		if err := cpcpersonUtils.RequireTimelineWritable(person.TimelineStatus); err != nil {
			return person, data, err
		}
	} else if person.TimelineTransferTargetID == nil {
		return person, data, invalid("当前人员没有待审核的时间线转移")
	}
	data, err = timelineData(tx, workspaceID, person)
	if err != nil {
		return person, data, err
	}
	if timelineActualStatus(data) == cpcpersonUtils.TimelineAbnormal {
		return person, data, invalid("当前时间线存在异常时间节点，请先处理后再转移")
	}
	return person, data, nil
}

func prepareTimelineTransfer(
	tx *gorm.DB, access personAccess, workspaceID string, id, targetID uint64, action string,
) (timelineTransferPlan, error) {
	var plan timelineTransferPlan
	person, source, err := transferSource(tx, access, workspaceID, id, action)
	if err != nil {
		return plan, err
	}
	if action == accessConstants.CPCPersonTimelineTransferReview &&
		*person.TimelineTransferTargetID != targetID {
		return plan, invalid("转移目标已发生变化，请重新预览后审核")
	}
	transfers, err := cpctimelineUtils.LoadTransfers(tx, workspaceID, *source.TemplateID)
	if err != nil {
		return plan, err
	}
	var mapping *cpctimelineModels.Transfer
	for index := range transfers {
		if transfers[index].TargetTemplateID == targetID {
			mapping = &transfers[index]
		}
	}
	if mapping == nil || targetID == *source.TemplateID {
		return plan, invalid("当前模板未配置此转移目标，请刷新后重试")
	}
	target, err := timelineTemplate(tx, workspaceID, targetID)
	if err != nil {
		return plan, err
	}
	nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, targetID)
	if err != nil {
		return plan, err
	}
	sourceNodes := make([]cpctimelineModels.Node, 0, len(source.Nodes))
	for _, node := range source.Nodes {
		sourceNodes = append(sourceNodes, node.Node)
	}
	if err := cpctimelineUtils.ValidateTransfer(sourceNodes, nodes, *mapping); err != nil {
		return plan, invalid(err.Error())
	}
	birthDates, err := cpcpersonUtils.TimelineBirthDates(tx, workspaceID, nodes, []uint64{id})
	if err != nil {
		return plan, err
	}
	var files []attachmentModels.PersonTimelineAttachment
	if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, id).
		Order("id ASC").Find(&files).Error; err != nil {
		return plan, err
	}
	plan, err = buildTimelineTransfer(
		person,
		source,
		target,
		nodes,
		*mapping,
		files,
		birthDates[id],
	)
	if err != nil {
		return plan, err
	}
	if err := timelineTemplateFiles(tx, workspaceID, &plan.Preview.Timeline); err != nil {
		return plan, err
	}

	// 快照包含节点版本、来源附件、引用结果、映射及两个模板，确认时重新计算比对。
	encoded, err := json.Marshal(struct {
		WorkspaceID string
		Source      cpcpersonModels.TimelineData
		Preview     cpcpersonModels.TimelineTransferPreview
		Mapping     cpctimelineModels.Transfer
	}{workspaceID, plan.Source, plan.Preview, plan.Mapping})
	if err != nil {
		return plan, err
	}
	digest := sha256.Sum256(encoded)
	plan.Preview.PreviewVersion = hex.EncodeToString(digest[:])
	return plan, nil
}

func timelineActualStatus(data cpcpersonModels.TimelineData) string {
	for _, node := range data.Nodes {
		if len(node.Issues) > 0 {
			return cpcpersonUtils.TimelineAbnormal
		}
	}
	return cpcpersonUtils.TimelineNormal
}

func buildTimelineTransfer(
	person cpcpersonModels.Info,
	source cpcpersonModels.TimelineData,
	target cpctimelineModels.Template,
	nodes []cpctimelineModels.Node,
	mapping cpctimelineModels.Transfer,
	files []attachmentModels.PersonTimelineAttachment,
	birthDate string,
) (timelineTransferPlan, error) {
	plan := timelineTransferPlan{
		Person: person, Source: source, Mapping: mapping,
		Preview: cpcpersonModels.TimelineTransferPreview{
			SourceTemplateID: *source.TemplateID, SourceTemplateName: source.TemplateName,
			DiscardedFields: []cpcpersonModels.TimelineDiscardedField{},
			Timeline: cpcpersonModels.TimelineData{
				PersonID:         person.ID,
				PersonRevision:   person.Revision + 1,
				TemplateID:       &target.ID,
				TemplateName:     target.Name,
				TemplateRevision: target.Revision,
				CanUpdate:        person.CanUpdate,
				Nodes:            []cpcpersonModels.TimelineNodeInfo{},
			},
		},
	}
	byID := make(map[string]cpcpersonModels.TimelineNodeInfo, len(source.Nodes))
	revision := person.Revision + 1
	for _, node := range source.Nodes {
		byID[node.ID] = node
		revision = max(revision, node.Revision+1)
	}
	values := map[string]map[string]any{}
	statuses := map[string]string{}
	used := map[[2]string]bool{}
	for _, node := range nodes {
		// 多来源合并字段，完成状态仅在全部来源已完成且目标必填完整时保留。
		stored := map[string]any{}
		mapped, allCompleted, allSkipped := false, true, true
		for _, pair := range mapping.NodeMappings {
			if pair.TargetNodeID != node.ID {
				continue
			}
			from := byID[pair.SourceNodeID]
			mapped = true
			allCompleted = allCompleted && from.Status == cpcpersonUtils.TimelineCompleted
			allSkipped = allSkipped && from.Status == cpcpersonUtils.TimelineSkipped
			for _, field := range pair.Fields {
				used[[2]string{from.ID, field.SourceFieldID}] = true
				if value, ok := from.Values[field.SourceFieldID]; ok {
					stored[field.TargetFieldID] = value
				}
			}
		}
		partial := node
		partial.Fields = append([]cpctimelineModels.Field{}, node.Fields...)
		for index := range partial.Fields {
			partial.Fields[index].Required = false
		}
		clean, err := cpcpersonUtils.ValidateTimelineValues(partial, stored)
		if err != nil {
			return plan, invalid(fmt.Sprintf("目标节点“%s”：%s", node.Name, err.Error()))
		}
		encoded, err := json.Marshal(clean)
		if err != nil {
			return plan, err
		}
		if len(encoded) > config.Config.CPCTimeline.MaxDefinitionBytes {
			return plan, invalid("映射后的节点填写内容超过大小限制")
		}
		status, label := cpcpersonUtils.TimelinePending, "待处理"
		if mapped && allSkipped {
			status, label = cpcpersonUtils.TimelineSkipped, "已跳过"
		} else if mapped && allCompleted {
			if _, err := cpcpersonUtils.ValidateTimelineValues(node, clean); err == nil {
				status, label = cpcpersonUtils.TimelineCompleted, "已完成"
			}
		}
		visible := cpcpersonUtils.ResolveTimelineValues(node, clean, birthDate)
		values[node.ID], statuses[node.ID] = visible, status
		plan.Preview.Timeline.Nodes = append(
			plan.Preview.Timeline.Nodes,
			cpcpersonModels.TimelineNodeInfo{
				Node:                node,
				Values:              visible,
				Status:              status,
				StatusLabel:         label,
				Revision:            revision,
				TemplateAttachments: []attachmentModels.Info{},
				Attachments:         []attachmentModels.Info{},
			},
		)
	}
	for _, node := range source.Nodes {
		for _, field := range node.Fields {
			value := node.Values[field.ID]
			if field.StudentField == "" && !used[[2]string{node.ID, field.ID}] &&
				transferValueFilled(value) {
				plan.Preview.DiscardedFields = append(plan.Preview.DiscardedFields,
					cpcpersonModels.TimelineDiscardedField{
						NodeName: node.Name, FieldName: field.Name, Value: value,
					})
			}
		}
	}
	status, issues, checks := cpcpersonUtils.EvaluateTimelineChecks(nodes, values, statuses)
	plan.Preview.Timeline.Status = status
	for index := range plan.Preview.Timeline.Nodes {
		node := &plan.Preview.Timeline.Nodes[index]
		node.Issues, node.Checks = issues[node.ID], checks[node.ID]
	}
	err := transferFilePlacements(&plan, files)
	return plan, err
}

func transferValueFilled(value any) bool {
	if value == nil {
		return false
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	return true
}

func transferFilePlacements(
	plan *timelineTransferPlan, files []attachmentModels.PersonTimelineAttachment,
) error {
	// 同一对象合并到同一目标时去重，分发到多个目标时建立各自独立的附件引用。
	seen := map[[2]string]bool{}
	for _, file := range files {
		if file.State != attachmentConstants.Ready {
			return invalid("存在未完成上传或待删除的人员附件，请先处理后再转移")
		}
		mapped := false
		for _, pair := range plan.Mapping.NodeMappings {
			if pair.SourceNodeID != file.NodeID {
				continue
			}
			mapped = true
			key := [2]string{pair.TargetNodeID, file.ObjectHash}
			if seen[key] {
				continue
			}
			seen[key] = true
			plan.Files = append(
				plan.Files,
				timelineFilePlacement{Source: file, NodeID: pair.TargetNodeID},
			)
			for index := range plan.Preview.Timeline.Nodes {
				node := &plan.Preview.Timeline.Nodes[index]
				if node.ID == pair.TargetNodeID {
					info := timelineFileInfo(file.Record, node.ID)
					info.PersonID, info.Category = file.PersonID, attachmentConstants.CategoryCPCPersonTimelineNode
					info.Revision++
					node.Attachments = append(node.Attachments, info)
					if len(node.Attachments) > config.Config.CPCTimeline.MaxAttachments {
						return invalid("合并后的节点附件数量超过限制，请调整节点映射")
					}
				}
			}
		}
		if !mapped {
			return invalid("带有人员附件的来源节点未配置目标节点，请先补充节点映射")
		}
	}
	return nil
}
