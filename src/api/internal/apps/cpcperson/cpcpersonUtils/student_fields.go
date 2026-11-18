package cpcpersonUtils

import (
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// HasStudentFields 判断节点是否需要读取学生出生日期。
func HasStudentFields(nodes []cpctimelineModels.Node) bool {
	for _, node := range nodes {
		for _, field := range node.Fields {
			if field.StudentField != "" {
				return true
			}
		}
	}
	return false
}

// TimelineBirthDates 批量读取同空间人员关联学生的出生日期，不读取身份证等敏感字段。
func TimelineBirthDates(
	tx *gorm.DB, workspaceID string, nodes []cpctimelineModels.Node, personIDs []uint64,
) (map[uint64]string, error) {
	// 未配置引用时沿用原有读取路径，空人员集合不执行查询。
	result := make(map[uint64]string, len(personIDs))
	if len(personIDs) == 0 || !HasStudentFields(nodes) {
		return result, nil
	}
	var records []struct {
		ID        uint64
		BirthDate string
	}
	if err := tx.Table("cpc_people AS p").Select("p.id, s.birth_date").
		Joins("JOIN students AS s ON s.workspace_id = p.workspace_id AND s.id = p.student_id").
		Where("p.workspace_id = ? AND p.id IN ?", workspaceID, personIDs).
		Scan(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		result[record.ID] = record.BirthDate
	}
	return result, nil
}

// ResolveTimelineValues 仅返回当前字段，引用值实时计算且始终覆盖历史填写值。
func ResolveTimelineValues(
	node cpctimelineModels.Node, stored map[string]any, birthDate string,
) map[string]any {
	values := make(map[string]any, len(node.Fields))
	for _, field := range node.Fields {
		if field.StudentField != "" {
			if value := studentFieldDate(field.StudentField, birthDate); value != "" {
				values[field.ID] = value
			}
			continue
		}
		if value, ok := stored[field.ID]; ok {
			values[field.ID] = value
		}
	}
	return values
}

func studentFieldDate(field, birthDate string) string {
	// 日期无效或成年日期超出四位年份时保持空值，闰日按目标年份月末截断。
	if !validTimelineValue(cpctimelineConstants.FieldDate, birthDate) {
		return ""
	}
	date, err := time.Parse(time.DateOnly, birthDate)
	if err != nil {
		return ""
	}
	switch field {
	case cpctimelineConstants.StudentBirthDate:
		return birthDate
	case cpctimelineConstants.StudentAdulthoodDate:
		year := date.Year() + cpctimelineConstants.AdulthoodYears
		if year > 9999 {
			return ""
		}
		return clampedDate(year, date.Month(), date.Day()).Format(time.DateOnly)
	default:
		return ""
	}
}
