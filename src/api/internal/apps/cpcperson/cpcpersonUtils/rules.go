package cpcpersonUtils

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// EvaluateTimeline 用同一套含边界规则生成详情异常和列表状态，缺失日期不视为时间冲突。
func EvaluateTimeline(
	nodes []cpctimelineModels.Node, values map[string]map[string]any,
	statuses map[string]string,
) (string, map[string][]cpcpersonModels.TimelineIssue) {
	status, issues, _ := EvaluateTimelineChecks(nodes, values, statuses)
	return status, issues
}

// EvaluateTimelineChecks 参考节点已有日期时生成对应规则，同时复用异常判定结果。
func EvaluateTimelineChecks(
	nodes []cpctimelineModels.Node,
	values map[string]map[string]any,
	statuses map[string]string,
) (string, map[string][]cpcpersonModels.TimelineIssue, map[string][]cpcpersonModels.TimelineRuleCheck) {
	status := TimelineNormal
	issues := make(map[string][]cpcpersonModels.TimelineIssue, len(nodes))
	checks := make(map[string][]cpcpersonModels.TimelineRuleCheck, len(nodes))
	byID := make(map[string]cpctimelineModels.Node, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
		issues[node.ID] = []cpcpersonModels.TimelineIssue{}
		checks[node.ID] = []cpcpersonModels.TimelineRuleCheck{}
	}
	for _, node := range nodes {
		// 跳过节点不校验自身，也不作为其他节点的有效时间来源。
		if statuses[node.ID] == TimelineSkipped {
			continue
		}
		actual, filled := nodeDate(node, values)
		for _, rule := range node.Rules {
			referenceNode, found := byID[rule.ReferenceNodeID]
			if !found || statuses[referenceNode.ID] == TimelineSkipped {
				continue
			}
			reference, ok := nodeDate(referenceNode, values)
			if !ok {
				continue
			}
			check := cpcpersonModels.TimelineRuleCheck{
				TimelineIssue: cpcpersonModels.TimelineIssue{
					RuleID:            rule.ID,
					ReferenceNodeName: referenceNode.Name,
					Message:           "请按预期范围核对发生时间",
				},
				ReferenceDate: reference.Format(
					time.DateOnly,
				),
				Result:      TimelinePending,
				ResultLabel: "待校验",
			}
			conflict := false
			if rule.MinOffset != nil {
				minimum := applyOffset(reference, *rule.MinOffset)
				check.MinDate = minimum.Format(time.DateOnly)
				conflict = filled && actual.Before(minimum)
			}
			if rule.MaxOffset != nil {
				maximum := applyOffset(reference, *rule.MaxOffset)
				check.MaxDate = maximum.Format(time.DateOnly)
				conflict = conflict || (filled && actual.After(maximum))
			}
			if filled {
				check.ActualDate = actual.Format(time.DateOnly)
				check.Result, check.ResultLabel = TimelineNormal, "符合范围"
			}
			if conflict {
				check.Result, check.ResultLabel = TimelineAbnormal, "时间异常"
				issues[node.ID] = append(issues[node.ID], check.TimelineIssue)
				status = TimelineAbnormal
			}
			checks[node.ID] = append(checks[node.ID], check)
		}
	}
	return status, issues, checks
}

func nodeDate(
	node cpctimelineModels.Node, values map[string]map[string]any,
) (time.Time, bool) {
	text, ok := values[node.ID][node.TimeFieldID].(string)
	if !ok || !validTimelineValue("date", text) {
		return time.Time{}, false
	}
	value, err := time.Parse(time.DateOnly, text)
	return value, err == nil
}

func applyOffset(value time.Time, offset cpctimelineModels.Offset) time.Time {
	// 依次计算年、月、自然日和周一至周五工作日；年月分别截断月末。
	value = clampedDate(value.Year()+offset.Years, value.Month(), value.Day())
	month := time.Date(
		value.Year(),
		value.Month()+time.Month(offset.Months),
		1,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	value = clampedDate(month.Year(), month.Month(), value.Day()).AddDate(0, 0, offset.Days)
	remaining, direction := offset.Workdays, 1
	if remaining < 0 {
		remaining, direction = -remaining, -1
	}
	for remaining > 0 {
		value = value.AddDate(0, 0, direction)
		if value.Weekday() != time.Saturday && value.Weekday() != time.Sunday {
			remaining--
		}
	}
	return value
}

func clampedDate(year int, month time.Month, day int) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
