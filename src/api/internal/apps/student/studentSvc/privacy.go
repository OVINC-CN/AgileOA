package studentSvc

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

// Sensitive 使用唯一的敏感信息权限返回固定敏感字段，不按字段细分授权。
func Sensitive(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (studentModels.SensitiveData, error) {
	result := studentModels.SensitiveData{StudentID: id, Values: []studentModels.FieldValue{}}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentSensitiveHandle,
		studentEvent(
			workspaceID,
			id,
		),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			record, err := studentVersion(tx, workspaceID, id)
			if err != nil {
				return err
			}
			values, err := loadSensitive(tx, workspaceID, id)
			if err != nil {
				return err
			}
			result.Revision = record.Revision
			for _, field := range studentConstants.Fields {
				if field.Group != studentConstants.Sensitive {
					continue
				}
				value, _ := values[field.Key].(string)
				if value == "" && field.Type == "enum" {
					value = "unknown"
				}
				result.Values = append(
					result.Values,
					studentModels.FieldValue{Field: field.Key, Value: value},
				)
			}
			event.ReturnedIDs = []string{event.ObjectID}
			event.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

// UpdateSensitive 统一使用处理敏感信息权限，全量更新十四个敏感字段。
func UpdateSensitive(ctx context.Context, workspaceID string, id uint64,
	request studentModels.SensitiveRequest,
) (studentModels.MutationData, error) {
	var result studentModels.MutationData
	var timelinePersonIDs []uint64
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentSensitiveHandle,
		studentEvent(
			workspaceID,
			id,
		),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			record, err := studentVersion(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if request.Revision != record.Revision {
				return stale()
			}
			updates, err := validateSensitiveReplacement(request.Values)
			if err != nil {
				return err
			}
			before, err := loadSensitive(tx, workspaceID, id)
			if err != nil {
				return err
			}
			after := auditModels.Snapshot{}
			for key, value := range updates {
				after[key] = value
			}
			birthDate, err := saveSensitive(tx, workspaceID, id, updates)
			if err != nil {
				return err
			}
			if err := bumpRevision(tx, workspaceID, id, record.Revision); err != nil {
				return err
			}
			if birthDate != record.BirthDate {
				timelinePersonIDs, err = cpcpersonUtils.RefreshStudentTimelineStatuses(
					tx, workspaceID, accessConstants.StudentSensitiveHandle, id,
				)
				if err != nil {
					return err
				}
			}
			event.SensitiveBefore, event.SensitiveAfter = before, after
			event.Before = auditModels.Snapshot{
				"revision":   record.Revision,
				"birth_date": record.BirthDate,
			}
			event.After = auditModels.Snapshot{
				"revision":   record.Revision + 1,
				"birth_date": birthDate,
			}
			result = studentModels.MutationData{ID: id, Revision: record.Revision + 1}
			return nil
		},
	)
	if err == nil && len(timelinePersonIDs) > 0 {
		logger.Logger.WithContext(ctx).Infof(
			"CPC timeline statuses refreshed; workspace: %s; student: %d; people: %v",
			workspaceID, id, timelinePersonIDs,
		)
	}
	return result, err
}

func studentVersion(tx *gorm.DB, workspaceID string, id uint64) (studentModels.BasicRecord, error) {
	var record studentModels.BasicRecord
	err := basicScope(tx, workspaceID).Select("s.id", "s.revision", "s.birth_date").
		Where("s.id = ?", id).
		Take(&record).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, missing()
	}
	return record, err
}

func validateSensitive(values []studentModels.FieldValue) (map[string]string, error) {
	if len(values) == 0 {
		return nil, invalid("请提交需要修改的敏感字段")
	}
	result := map[string]string{}
	for _, item := range values {
		field, ok := studentConstants.Lookup(item.Field)
		if !ok || field.Group != studentConstants.Sensitive {
			return nil, invalid("敏感信息字段无效")
		}
		if _, exists := result[item.Field]; exists {
			return nil, invalid("敏感字段不能重复")
		}
		value := strings.TrimSpace(item.Value)
		if err := validateValue(field, value); err != nil {
			return nil, err
		}
		result[item.Field] = value
	}
	return result, nil
}

func validateSensitiveReplacement(values []studentModels.FieldValue) (map[string]string, error) {
	// 编辑请求必须携带固定目录中的全部字段，导入仍按实际列单独校验。
	result, err := validateSensitive(values)
	if err != nil {
		return nil, err
	}
	if len(result) != len(sensitiveColumns()) {
		return nil, invalid("请提交全部敏感字段")
	}
	return result, nil
}

func sensitiveColumns() []string {
	columns := []string{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive {
			columns = append(columns, field.Key)
		}
	}
	return columns
}

func loadSensitive(tx *gorm.DB, workspaceID string, id uint64) (auditModels.Snapshot, error) {
	var record studentModels.Student
	if err := tx.Select(sensitiveColumns()).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Take(&record).
		Error; err != nil {
		return nil, err
	}
	values := sensitiveValues(record)
	result := auditModels.Snapshot{}
	for key, value := range values {
		field, _ := studentConstants.Lookup(key)
		if value != "" && (field.Type != "enum" || value != "unknown") {
			result[key] = value
		}
	}
	return result, nil
}

func sensitiveValues(record studentModels.Student) map[string]string {
	return map[string]string{
		"identity_number": record.IdentityNumber,
		"home_address":    record.HomeAddress,
		"only_child":      studentConstants.EnumValue("only_child", record.OnlyChild),
		"single_parent":   studentConstants.EnumValue("single_parent", record.SingleParent),
		"orphan":          studentConstants.EnumValue("orphan", record.Orphan),
		"student_loan":    studentConstants.EnumValue("student_loan", record.StudentLoan),
		"registered_poverty": studentConstants.EnumValue(
			"registered_poverty",
			record.RegisteredPoverty,
		),
		"has_religion":            studentConstants.EnumValue("has_religion", record.HasReligion),
		"religion_category":       record.ReligionCategory,
		"religion_identification": record.ReligionIdentification,
		"religion_start_year":     record.ReligionStartYear,
		"family_religion":         record.FamilyReligion,

		"medical_history":       record.MedicalHistory,
		"special_circumstances": record.SpecialCircumstances,
	}
}

func saveSensitive(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	values map[string]string,
) (string, error) {
	updates := map[string]any{}
	for key, value := range values {
		field, ok := studentConstants.Lookup(key)
		if !ok || field.Group != studentConstants.Sensitive {
			return "", invalid("敏感信息字段无效")
		}
		stored, err := storageValue(field, value)
		if err != nil {
			return "", err
		}
		updates[key] = stored
	}
	if len(updates) == 0 {
		return "", nil
	}
	if err := tx.Model(&studentModels.Student{}).
		Where("workspace_id = ? AND id = ?", workspaceID, id).
		Updates(updates).Error; err != nil {
		return "", err
	}

	// 完整更新提交身份证号码，导入只在提供号码时需要读取派生值。
	if values["identity_number"] == "" {
		return "", nil
	}
	record, err := studentVersion(tx, workspaceID, id)
	return record.BirthDate, err
}

func bumpRevision(tx *gorm.DB, workspaceID string, id, revision uint64) error {
	changed := tx.Model(&studentModels.Student{}).
		Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, revision).
		Updates(map[string]any{"revision": revision + 1})
	if changed.Error != nil {
		return changed.Error
	}
	if changed.RowsAffected != 1 {
		return stale()
	}
	return nil
}

func sensitiveSnapshot(values map[string]string) auditModels.Snapshot {
	result := auditModels.Snapshot{}
	for key, value := range values {
		result[key] = value
	}
	return result
}
