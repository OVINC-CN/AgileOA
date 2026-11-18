package graduationSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Save 在工作空间锁内绑定或修改学生去向，不隐含学生详情读取权限。
func Save(
	ctx context.Context,
	workspaceID string,
	id uint64,
	request graduationModels.Request,
) (graduationModels.MutationData, error) {
	var result graduationModels.MutationData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationUpdate, event(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 校验固定业务字段，空类型作为待就业保存。
			input, err := cleanInput(request.Input)
			if err != nil {
				return err
			}
			before, err := detail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if err := update(tx, workspaceID, id, request.Revision, before, input); err != nil {
				return err
			}

			// 成功审计与字段更新处于同一事务。
			audit.Before = snapshot(before)
			before.Input, before.Revision = input, before.Revision+1
			audit.After = snapshot(before)
			result = graduationModels.MutationData{StudentID: id, Revision: before.Revision}
			return nil
		},
	)
	return result, err
}

// Clear 显式清除毕业去向字段，保留学生记录和历史审计。
func Clear(ctx context.Context, workspaceID string, id, revision uint64) error {
	return accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationDelete, event(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			before, err := detail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			cleared := graduationModels.Input{GraduationYear: before.GraduationYear}
			if err := update(
				tx,
				workspaceID,
				id,
				revision,
				before,
				cleared,
			); err != nil {
				return err
			}
			audit.Before = snapshot(before)
			before.Input, before.Revision = cleared, before.Revision+1
			audit.After = snapshot(before)
			return nil
		},
	)
}

func cleanInput(input graduationModels.Input) (graduationModels.Input, error) {
	input.GraduationYear = strings.TrimSpace(input.GraduationYear)
	input.TypeLabel = strings.TrimSpace(input.TypeLabel)
	input.Destination = strings.TrimSpace(input.Destination)
	if !validYear(input.GraduationYear) ||
		!validText(input.TypeLabel, graduationConstants.TypeLabelLength) ||
		!validText(input.Destination, graduationConstants.DestinationLength) {
		return input, invalid("请填写有效的毕业年份、类型和毕业去向")
	}

	// 待就业仅是空值的显示名称，不保存为独立类型。
	if input.TypeLabel == graduationConstants.PendingLabel {
		input.TypeLabel = ""
	}
	code := ""
	if input.TypeLabel != "" {
		digest := sha256.Sum256([]byte(input.TypeLabel))
		code = graduationConstants.CustomTypePrefix + hex.EncodeToString(digest[:])
	}
	if input.Type != "" && input.Type != code {
		return input, invalid("类型代码与名称不一致，请重新选择类型")
	}
	input.Type = code
	return input, nil
}

func detail(tx *gorm.DB, workspaceID string, id uint64) (graduationModels.Info, error) {
	var result graduationModels.Info
	err := tx.Table("students").Select(columns()).
		Where("workspace_id = ? AND id = ?", workspaceID, id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, &response.BusinessError{
			Status: http.StatusNotFound, Message: "学生不存在或不可访问",
		}
	}
	return result, err
}

func update(
	tx *gorm.DB,
	workspaceID string,
	id, revision uint64,
	before graduationModels.Info,
	input graduationModels.Input,
) error {
	if revision == 0 || revision != before.Revision {
		return stale()
	}
	changed := tx.Table("students").
		Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, revision).
		Updates(map[string]any{
			"graduation_year":        input.GraduationYear,
			"graduation_type":        input.Type,
			"graduation_type_label":  input.TypeLabel,
			"graduation_destination": input.Destination,
			"revision":               revision + 1,
		})
	if changed.Error != nil {
		return changed.Error
	}
	if changed.RowsAffected != 1 {
		return stale()
	}
	return nil
}

func columns() string {
	return "id AS student_id, student_no, name AS student_name, education_level, major, grade, " +
		"graduation_year, graduation_type AS type, graduation_type_label AS type_label, " +
		"graduation_destination AS destination, revision, created_at, updated_at"
}

func snapshot(info graduationModels.Info) auditModels.Snapshot {
	return auditModels.Snapshot{
		"graduation_year": info.GraduationYear,
		"student_id":      info.StudentID,
		"student_no":      info.StudentNo,
		"name":            info.StudentName,
		"type":            info.Type,
		"type_label":      info.TypeLabel,
		"destination":     info.Destination,
		"revision":        info.Revision,
	}
}

func event(workspaceID string, id uint64) auditModels.Event {
	result := auditModels.Event{
		TenantID: &workspaceID, ObjectType: graduationConstants.Object,
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID},
	}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}

func validText(value string, limit int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func validYear(value string) bool {
	if value == "" {
		return true
	}
	year, err := strconv.Atoi(value)
	return err == nil && len(value) == graduationConstants.YearLength &&
		year >= graduationConstants.MinYear && year <= graduationConstants.MaxYear
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "学生信息已被修改，请刷新后重试"}
}
