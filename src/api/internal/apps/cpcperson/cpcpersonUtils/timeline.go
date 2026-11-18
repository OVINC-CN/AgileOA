package cpcpersonUtils

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// TimelineSnapshot 按固定目录保存节点审计，不把自定义字段标识作为审计对象键。
func TimelineSnapshot(record cpcpersonModels.TimelineNode) auditModels.Snapshot {
	values := map[string]any{}
	_ = json.Unmarshal([]byte(record.FieldValues), &values)
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fields := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		fields = append(fields, map[string]any{"field_id": id, "value": values[id]})
	}
	return auditModels.Snapshot{
		"person_id":            record.PersonID,
		"timeline_template_id": record.TimelineTemplateID,
		"node_id":              record.NodeID,
		"fields":               fields,
		"status":               record.Status,
		"revision":             record.Revision,
	}
}

const (
	TimelinePending      = "pending"
	TimelineCompleted    = "completed"
	TimelineSkipped      = "skipped"
	TimelineNormal       = "normal"
	TimelineAbnormal     = "abnormal"
	TimelineTransferring = "transferring"
	TimelineTextLength   = 1000
)

// ValidateTimelineValues 只接受固定字段目录内的值，保留 false 和零等有效填写。
func ValidateTimelineValues(
	node cpctimelineModels.Node, values map[string]any,
) (map[string]any, error) {
	// 按模板逐项检查必填、类型及大小，不信任客户端提交的状态。
	clean := make(map[string]any, len(node.Fields))
	known := make(map[string]bool, len(node.Fields))
	for _, field := range node.Fields {
		known[field.ID] = true
		if field.StudentField != "" {
			if _, submitted := values[field.ID]; submitted {
				return nil, fmt.Errorf("“%s”引用学生信息，不能直接修改", field.Name)
			}
			continue
		}
		value := values[field.ID]
		if text, ok := value.(string); ok {
			value = strings.TrimSpace(text)
		}
		if value == nil || value == "" {
			if field.Required {
				return nil, fmt.Errorf("请填写“%s”", field.Name)
			}
			continue
		}
		if !validTimelineValue(field.Type, value) {
			return nil, fmt.Errorf("“%s”的格式无效或超过长度限制", field.Name)
		}
		clean[field.ID] = value
	}
	for id := range values {
		if !known[id] {
			return nil, fmt.Errorf("包含当前节点未定义的字段，请刷新后重试")
		}
	}
	return clean, nil
}

func validTimelineValue(kind string, value any) bool {
	// 日期仅接受有效公历日期；文本与多行文本使用相同的字符数上限。
	switch kind {
	case "number":
		number, ok := value.(float64)
		return ok && !math.IsInf(number, 0) && !math.IsNaN(number)
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "date":
		text, ok := value.(string)
		if !ok || len(text) != len(time.DateOnly) || text[:4] == "0000" {
			return false
		}
		_, err := time.Parse(time.DateOnly, text)
		return err == nil
	case "text", "textarea":
		text, ok := value.(string)
		if !ok || !utf8.ValidString(text) || utf8.RuneCountInString(text) > TimelineTextLength {
			return false
		}
		return strings.IndexFunc(text, func(char rune) bool {
			if kind == "textarea" && (char == '\n' || char == '\r' || char == '\t') {
				return false
			}
			return unicode.IsControl(char)
		}) < 0
	}
	return false
}
