package cpcpersonSvc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// TimelineFilterFields 仅枚举当前读取范围内人员实际绑定模板的字段定义。
func TimelineFilterFields(
	ctx context.Context, workspaceID, keyword string,
) ([]cpcpersonModels.TimelineFilterField, error) {
	result := []cpcpersonModels.TimelineFilterField{}
	audit := event(workspaceID, 0)
	audit.Conditions["keyword"] = keyword
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, audit,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			// 字段元数据同样受空间和书记读取范围约束，不要求模板管理权限。
			keyword = strings.TrimSpace(keyword)
			if !validText(keyword, cpcpersonConstants.KeywordLength) {
				return invalid("时间线字段搜索条件无效")
			}
			var records []struct {
				cpctimelineModels.TemplateNode
				TemplateName string
			}
			if err := timelineFieldScope(tx, workspaceID, access, accessConstants.CPCPersonList).
				Select("n.*, t.name AS template_name").
				Joins("JOIN cpc_timeline_templates AS t ON t.workspace_id = n.workspace_id AND t.id = n.timeline_template_id").
				Order("n.timeline_template_id ASC, n.position ASC, n.node_id ASC").
				Scan(&records).Error; err != nil {
				return err
			}
			positions := map[uint64]int{}
			for _, record := range records {
				positions[record.TimelineTemplateID]++
				var fields []cpctimelineModels.Field
				if err := json.Unmarshal([]byte(record.FieldDefinitions), &fields); err != nil {
					return err
				}
				for _, field := range fields {
					label := record.TemplateName + " / " +
						strconv.Itoa(positions[record.TimelineTemplateID]) +
						"-" + record.Name + " / " + field.Name
					if !strings.Contains(strings.ToLower(label), strings.ToLower(keyword)) {
						continue
					}
					result = append(result, cpcpersonModels.TimelineFilterField{
						NodeID: record.NumericID, FieldID: field.ID, Label: label, Type: field.Type,
					})
					if len(result) > config.Config.CPCPerson.MaxOptions {
						return invalid("时间线字段较多，请输入模板、节点或字段名称搜索")
					}
				}
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

func timelineFieldScope(
	tx *gorm.DB, workspaceID string, access personAccess, action string,
) *gorm.DB {
	// 通过人员存在性检查限定模板，避免重复节点或泄露其他范围的模板定义。
	people := access.limit(
		tx.Table("cpc_people AS readable").Select("1").
			Where("readable.workspace_id = n.workspace_id").
			Where("readable.timeline_template_id = n.timeline_template_id"),
		action,
		"readable.party_branch_id",
	)
	return tx.Table("cpc_timeline_template_nodes AS n").
		Where("n.workspace_id = ?", workspaceID).Where("EXISTS (?)", people)
}

func filterTimelineNode(
	tx, scope *gorm.DB, query cpcpersonModels.Query,
) (*gorm.DB, error) {
	// 节点状态必须限定当前模板及节点，多选取并集，不影响人员视图的时间线状态条件。
	if query.NodeID == 0 && len(query.NodeStatuses) == 0 {
		return scope, nil
	}
	if query.TemplateID == 0 || query.NodeID == 0 {
		return nil, invalid("节点状态筛选须同时选择时间线模板和节点")
	}
	if len(query.NodeStatuses) > config.Config.CPCPerson.MaxOptions {
		return nil, invalid("节点状态筛选超过限制")
	}
	for _, status := range query.NodeStatuses {
		switch status {
		case cpcpersonUtils.TimelinePending, cpcpersonUtils.TimelineSkipped,
			cpcpersonUtils.TimelineCompleted:
		default:
			return nil, invalid("请选择有效的节点状态")
		}
	}
	matched := tx.Table("cpc_person_timeline_nodes AS filtered_node").Select("1").
		Joins("JOIN cpc_timeline_template_nodes AS node_definition ON "+
			"node_definition.workspace_id = filtered_node.workspace_id AND "+
			"node_definition.timeline_template_id = filtered_node.timeline_template_id AND "+
			"node_definition.node_id = filtered_node.node_id").
		Where("filtered_node.workspace_id = p.workspace_id AND filtered_node.person_id = p.id").
		Where("filtered_node.timeline_template_id = p.timeline_template_id").
		Where("filtered_node.timeline_template_id = ? AND node_definition.id = ?",
			query.TemplateID, query.NodeID)
	if len(query.NodeStatuses) > 0 {
		matched = matched.Where("filtered_node.status IN ?", query.NodeStatuses)
	}
	return scope.Where("EXISTS (?)", matched), nil
}

func filterTimeline(
	tx, scope *gorm.DB,
	workspaceID string,
	filters []cpcpersonModels.TimelineFilter,
	access personAccess,
	action string,
) (*gorm.DB, error) {
	// 先按当前操作范围加载目录，客户端不能指定任意 JSON 路径或数据库标识。
	if len(filters) == 0 {
		return scope, nil
	}
	if len(filters) > config.Config.CPCPerson.MaxOptions {
		return nil, invalid("时间线筛选条件超过限制")
	}
	if !access.hasScope(action) {
		return scope, nil
	}
	ids := make([]uint64, 0, len(filters))
	for _, filter := range filters {
		if filter.NodeID == 0 || filter.FieldID == "" {
			return nil, invalid("请选择有效的时间线字段")
		}
		ids = append(ids, filter.NodeID)
	}
	var records []cpctimelineModels.TemplateNode
	if err := timelineFieldScope(tx, workspaceID, access, action).
		Where("n.id IN ?", ids).Find(&records).Error; err != nil {
		return nil, err
	}
	nodes := make(map[uint64]cpctimelineModels.TemplateNode, len(records))
	fields := make(map[uint64]map[string]cpctimelineModels.Field, len(records))
	for _, record := range records {
		var definitions []cpctimelineModels.Field
		if err := json.Unmarshal([]byte(record.FieldDefinitions), &definitions); err != nil {
			return nil, err
		}
		nodes[record.NumericID] = record
		fields[record.NumericID] = make(map[string]cpctimelineModels.Field, len(definitions))
		for _, field := range definitions {
			fields[record.NumericID][field.ID] = field
		}
	}

	// 每个字段条件使用独立存在性子查询，多个条件取交集且不增加人员计数。
	for _, filter := range filters {
		field, ok := fields[filter.NodeID][filter.FieldID]
		if !ok {
			return nil, invalid("时间线字段不存在或不可用，请刷新筛选条件")
		}
		value, err := timelineFilterValue(filter, field)
		if err != nil {
			return nil, err
		}
		node := nodes[filter.NodeID]
		matched := tx.Table("cpc_person_timeline_nodes AS n").Select("1").
			Where("n.workspace_id = p.workspace_id AND n.person_id = p.id").
			Where("n.timeline_template_id = p.timeline_template_id").
			Where("n.timeline_template_id = ? AND n.node_id = ?", node.TimelineTemplateID, node.NodeID)
		if field.StudentField != "" {
			matched = timelineStudentFieldPredicate(matched, filter, field.StudentField, value)
		} else {
			path := "$." + strconv.Quote(field.ID)
			matched = timelineFilterPredicate(matched, filter, field.Type, path, value)
		}
		scope = scope.Where("EXISTS (?)", matched)
	}
	return scope, nil
}

func timelineFilterValue(
	filter cpcpersonModels.TimelineFilter, field cpctimelineModels.Field,
) (any, error) {
	// 空值条件不携带比较值；比较条件按当前字段类型重新验证。
	if filter.Operator == "empty" || filter.Operator == "not_empty" {
		if filter.Value != "" {
			return nil, invalid("时间线空值筛选不能包含比较值")
		}
		return nil, nil
	}
	switch filter.Operator {
	case "eq":
	case "contains":
		if field.Type != "text" && field.Type != "textarea" {
			return nil, invalid("仅文本时间线字段支持包含筛选")
		}
	case "gt", "gte", "lt", "lte":
		if field.Type != "number" && field.Type != "date" {
			return nil, invalid("仅数字和日期时间线字段支持范围筛选")
		}
	default:
		return nil, invalid("不支持的时间线筛选运算符")
	}
	var value any = filter.Value
	switch field.Type {
	case "number":
		number, err := strconv.ParseFloat(strings.TrimSpace(filter.Value), 64)
		if err != nil {
			return nil, invalid("时间线筛选值必须为有效数字")
		}
		value = number
	case "boolean":
		if filter.Value != "true" && filter.Value != "false" {
			return nil, invalid("时间线是否筛选必须使用 true 或 false")
		}
		value = filter.Value == "true"
	}
	field.Required = true
	// 比较值仍按日期校验，引用字段不作为人员填写值提交。
	field.StudentField = ""
	clean, err := cpcpersonUtils.ValidateTimelineValues(
		cpctimelineModels.Node{Fields: []cpctimelineModels.Field{field}},
		map[string]any{field.ID: value},
	)
	if err != nil {
		return nil, invalid(err.Error())
	}
	return clean[field.ID], nil
}

func timelineStudentFieldPredicate(
	scope *gorm.DB, filter cpcpersonModels.TimelineFilter, field string, value any,
) *gorm.DB {
	// 出生日期由学生表生成列保证有效，DATE_ADD 与展示共用月末截断语义。
	expression := "NULLIF(s.birth_date, '')"
	switch field {
	case cpctimelineConstants.StudentBirthDate:
	case cpctimelineConstants.StudentAdulthoodDate:
		expression = "DATE_FORMAT(DATE_ADD(" + expression + ", INTERVAL " +
			strconv.Itoa(cpctimelineConstants.AdulthoodYears) + " YEAR), '%Y-%m-%d')"
	default:
		return scope.Where("1 = 0")
	}
	if filter.Operator == "empty" {
		return scope.Where(expression + " IS NULL")
	}
	if filter.Operator == "not_empty" {
		return scope.Where(expression + " IS NOT NULL")
	}
	operator := map[string]string{
		"eq": "=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=",
	}[filter.Operator]
	return scope.Where(expression+" "+operator+" ?", value)
}

func timelineFilterPredicate(
	scope *gorm.DB, filter cpcpersonModels.TimelineFilter, kind, path string, value any,
) *gorm.DB {
	// 缺失、JSON null 和空文本均为空，数字零和 false 保持已填写语义。
	const empty = "(JSON_EXTRACT(n.field_values, ?) IS NULL OR " +
		"JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) = 'NULL' OR " +
		"(JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) = 'STRING' AND " +
		"TRIM(JSON_UNQUOTE(JSON_EXTRACT(n.field_values, ?))) = ''))"
	if filter.Operator == "empty" {
		return scope.Where(empty, path, path, path, path)
	}
	if filter.Operator == "not_empty" {
		return scope.Where("NOT "+empty, path, path, path, path)
	}
	if filter.Operator == "contains" {
		return scope.Where("JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) = 'STRING'", path).
			Where("LOCATE(?, JSON_UNQUOTE(JSON_EXTRACT(n.field_values, ?))) > 0", value, path)
	}
	// SQL 运算符来自固定映射，JSON 路径和比较值始终使用占位符。
	operator := map[string]string{"eq": "=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[filter.Operator]
	expression := "JSON_UNQUOTE(JSON_EXTRACT(n.field_values, ?))"
	switch kind {
	case "number":
		scope = scope.Where(
			"JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) IN ('INTEGER', 'DOUBLE')",
			path,
		)
		expression = "CAST(" + expression + " AS DOUBLE)"
	case "boolean":
		scope = scope.Where("JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) = 'BOOLEAN'", path)
		value = strconv.FormatBool(value.(bool))
	default:
		scope = scope.Where("JSON_TYPE(JSON_EXTRACT(n.field_values, ?)) = 'STRING'", path)
	}
	return scope.Where(expression+" "+operator+" ?", path, value)
}
