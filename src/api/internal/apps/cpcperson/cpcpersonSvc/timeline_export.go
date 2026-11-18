package cpcpersonSvc

import (
	"encoding/json"
	"sort"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineUtils"
)

// timelineExport 仅平铺导出范围内人员绑定的模板字段，未填写节点保留空单元格。
func timelineExport(
	tx *gorm.DB, workspaceID string, people []cpcpersonModels.Info,
) ([]string, map[uint64][]string, error) {
	headers := []string{}
	rows := make(map[uint64][]string, len(people))
	templates := map[uint64]bool{}
	personIDs := make([]uint64, 0, len(people))
	for _, person := range people {
		personIDs = append(personIDs, person.ID)
		if person.TimelineTemplateID != nil {
			templates[*person.TimelineTemplateID] = true
		}
	}
	templateIDs := make([]uint64, 0, len(templates))
	for id := range templates {
		templateIDs = append(templateIDs, id)
	}
	sort.Slice(templateIDs, func(i, j int) bool { return templateIDs[i] < templateIDs[j] })
	// 同名表头合并为一列，每个模板仍按自身稳定字段标识读取，避免模板间串值。
	columns := map[uint64]map[string]map[string]int{}
	definitions := map[uint64]map[string]cpctimelineModels.Node{}
	allNodes := []cpctimelineModels.Node{}
	headerIndex := map[string]int{}
	for _, templateID := range templateIDs {
		nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, templateID)
		if err != nil {
			return nil, nil, err
		}
		columns[templateID] = map[string]map[string]int{}
		definitions[templateID] = map[string]cpctimelineModels.Node{}
		allNodes = append(allNodes, nodes...)
		for position, node := range nodes {
			definitions[templateID][node.ID] = node
			columns[templateID][node.ID] = map[string]int{}
			for _, field := range node.Fields {
				header := strconv.Itoa(position+1) + "-" + field.Name
				index, exists := headerIndex[header]
				if !exists {
					index = len(headers)
					headerIndex[header] = index
					headers = append(headers, header)
				}
				columns[templateID][node.ID][field.ID] = index
			}
		}
	}
	for _, person := range people {
		rows[person.ID] = make([]string, len(headers))
	}
	if len(templateIDs) == 0 {
		return headers, rows, nil
	}
	var records []cpcpersonModels.TimelineNode
	if err := tx.Where("workspace_id = ? AND person_id IN ? AND timeline_template_id IN ?",
		workspaceID, personIDs, templateIDs).Find(&records).Error; err != nil {
		return nil, nil, err
	}
	birthDates, err := cpcpersonUtils.TimelineBirthDates(tx, workspaceID, allNodes, personIDs)
	if err != nil {
		return nil, nil, err
	}
	for _, record := range records {
		values := map[string]any{}
		if err := json.Unmarshal([]byte(record.FieldValues), &values); err != nil {
			return nil, nil, err
		}
		values = cpcpersonUtils.ResolveTimelineValues(
			definitions[record.TimelineTemplateID][record.NodeID],
			values,
			birthDates[record.PersonID],
		)
		for field, index := range columns[record.TimelineTemplateID][record.NodeID] {
			text, err := timelineExportValue(values[field])
			if err != nil {
				return nil, nil, err
			}
			rows[record.PersonID][index] = text
		}
	}
	return headers, rows, nil
}

func timelineExportValue(value any) (string, error) {
	// 保留零与否，布尔值使用中文显示名称，日期文本沿用 yyyy-mm-dd。
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case bool:
		if typed {
			return "是", nil
		}
		return "否", nil
	default:
		return "", invalid("时间线字段值格式无效，请检查节点数据")
	}
}
