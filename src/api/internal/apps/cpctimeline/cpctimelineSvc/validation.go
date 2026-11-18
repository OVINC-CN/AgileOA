package cpctimelineSvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

var stableID = regexp.MustCompile(cpctimelineConstants.StableIDPattern)

func prepare(input cpctimelineModels.Input) (cpctimelineModels.Input, error) {
	// 校验模板及定义大小，不将任何业务时间关系作为入库前置条件。
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !validText(input.Name, cpctimelineConstants.NameLength, true, false) ||
		!validText(input.Description, cpctimelineConstants.DescriptionLength, false, true) {
		return input, invalid("请检查模板名称和说明的格式及长度")
	}
	limits := config.Config.CPCTimeline
	if input.Transfers == nil {
		input.Transfers = []cpctimelineModels.Transfer{}
	}
	if len(input.Transfers) > config.Config.CPCPerson.MaxOptions {
		return input, invalid("转移目标数量超过限制")
	}
	targets := make(map[uint64]bool, len(input.Transfers))
	for _, transfer := range input.Transfers {
		if transfer.TargetTemplateID == 0 || targets[transfer.TargetTemplateID] ||
			len(
				transfer.NodeMappings,
			) == 0 || len(transfer.NodeMappings) > limits.MaxNodes*limits.MaxNodes {
			return input, invalid("转移目标必须存在且不能重复，并须配置节点映射")
		}
		targets[transfer.TargetTemplateID] = true
	}
	if len(input.Definition.Nodes) == 0 || len(input.Definition.Nodes) > limits.MaxNodes {
		return input, invalid("模板节点数为空或超过限制")
	}
	nodes := make(map[string]bool, len(input.Definition.Nodes))
	for i := range input.Definition.Nodes {
		node := &input.Definition.Nodes[i]
		if !stableID.MatchString(node.ID) || nodes[node.ID] {
			return input, invalid("节点标识必须是模板内唯一的英文标识")
		}
		nodes[node.ID] = true
		if err := prepareNode(node); err != nil {
			return input, invalid(fmt.Sprintf("第 %d 个节点：%s", i+1, err.Error()))
		}
	}
	for _, node := range input.Definition.Nodes {
		for _, rule := range node.Rules {
			if rule.ReferenceNodeID == node.ID || !nodes[rule.ReferenceNodeID] {
				return input, invalid("提示规则必须引用同模板内存在的其他节点")
			}
		}
	}

	// 按固定协议校验整体大小，自定义字段不会转换为数据库表名或列名。
	content, err := json.Marshal(input)
	if err != nil {
		return input, err
	}
	if len(content) > limits.MaxDefinitionBytes {
		return input, invalid("模板定义超过大小限制")
	}
	return input, nil
}

func prepareNode(node *cpctimelineModels.Node) error {
	// 校验节点字段，唯一指定的日期字段必须存在且必填。
	node.NumericID = 0
	node.Name = strings.TrimSpace(node.Name)
	node.Description = strings.TrimSpace(node.Description)
	node.GroupName = strings.TrimSpace(node.GroupName)
	if !validText(node.Name, cpctimelineConstants.NameLength, true, false) ||
		!validText(node.Description, cpctimelineConstants.DescriptionLength, false, true) {
		return fmt.Errorf("请检查节点名称和说明的格式及长度")
	}
	if !validText(node.GroupName, cpctimelineConstants.NameLength, false, false) {
		return fmt.Errorf("节点分组最多 100 字符，且不能包含控制字符")
	}
	limits := config.Config.CPCTimeline
	if len(node.Fields) == 0 || len(node.Fields) > limits.MaxFieldsPerNode ||
		node.Rules == nil || len(node.Rules) > limits.MaxRulesPerNode {
		return fmt.Errorf("字段或规则列表缺失，或数量超过限制")
	}
	fields := make(map[string]bool, len(node.Fields))
	names := make(map[string]bool, len(node.Fields))
	foundTime := false
	for i := range node.Fields {
		field := &node.Fields[i]
		field.Name = strings.TrimSpace(field.Name)
		if !stableID.MatchString(field.ID) || fields[field.ID] || names[field.Name] ||
			!validText(field.Name, cpctimelineConstants.NameLength, true, false) {
			return fmt.Errorf("字段标识和名称须在节点内唯一，名称最多 100 字符")
		}
		fields[field.ID], names[field.Name] = true, true
		switch field.Type {
		case cpctimelineConstants.FieldText, cpctimelineConstants.FieldTextarea,
			cpctimelineConstants.FieldNumber, cpctimelineConstants.FieldDate,
			cpctimelineConstants.FieldBoolean:
		default:
			return fmt.Errorf("字段类型无效")
		}
		if field.StudentField != "" {
			if field.StudentField != cpctimelineConstants.StudentBirthDate &&
				field.StudentField != cpctimelineConstants.StudentAdulthoodDate {
				return fmt.Errorf("学生引用字段无效，仅支持出生日期和成年日期")
			}
			if field.Type != cpctimelineConstants.FieldDate {
				return fmt.Errorf("引用学生信息的字段必须为日期类型")
			}
		}
		if field.ID == node.TimeFieldID {
			foundTime = field.Type == cpctimelineConstants.FieldDate && field.Required
		}
	}
	if !foundTime {
		return fmt.Errorf("每个节点必须指定一个必填日期字段作为发生时间")
	}

	// 规则始终为提示语义，仅校验结构、引用和明确可比较的上下界。
	rules := make(map[string]bool, len(node.Rules))
	for i := range node.Rules {
		rule := &node.Rules[i]
		if !stableID.MatchString(rule.ID) || rules[rule.ID] {
			return fmt.Errorf("规则标识必须是节点内唯一的英文标识")
		}
		rules[rule.ID] = true
		if rule.MinOffset == nil && rule.MaxOffset == nil {
			return fmt.Errorf("每条提示规则至少配置一个时间边界")
		}
		for _, bound := range []*cpctimelineModels.Offset{rule.MinOffset, rule.MaxOffset} {
			if bound == nil {
				continue
			}
			for _, value := range []int{bound.Years, bound.Months, bound.Days, bound.Workdays} {
				if value < -limits.MaxOffset || value > limits.MaxOffset {
					return fmt.Errorf("相对时间偏移超过限制")
				}
			}
		}
		if reversed(rule.MinOffset, rule.MaxOffset) {
			return fmt.Errorf("最早时间不能晚于最晚时间，请检查偏移范围")
		}
	}
	return nil
}

func reversed(minimum, maximum *cpctimelineModels.Offset) bool {
	if minimum == nil || maximum == nil {
		return false
	}
	if minimum.Years == maximum.Years && minimum.Months == maximum.Months {
		if minimum.Workdays == maximum.Workdays {
			return minimum.Days > maximum.Days
		}
		if minimum.Days == maximum.Days {
			return minimum.Workdays > maximum.Workdays
		}
	}
	return minimum.Days == maximum.Days && minimum.Workdays == maximum.Workdays &&
		minimum.Years*12+minimum.Months > maximum.Years*12+maximum.Months
}

func validText(value string, limit int, required, multiline bool) bool {
	return (!required || value != "") && utf8.ValidString(value) &&
		utf8.RuneCountInString(value) <= limit && strings.IndexFunc(value, func(r rune) bool {
		if multiline && (r == '\n' || r == '\r' || r == '\t') {
			return false
		}
		return unicode.IsControl(r)
	}) < 0
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}
