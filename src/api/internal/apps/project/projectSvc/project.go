package projectSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// SaveProject 在工作空间锁内创建或修改年度项目。
func SaveProject(ctx context.Context, workspaceID string, id uint64,
	request projectModels.ProjectRequest,
) (projectModels.ProjectInfo, error) {
	var result projectModels.ProjectInfo
	var resetApplications []uint64
	var editedApplications []uint64
	var notifications applicationNotifications
	workflowChanged := false
	editingChanges := map[int]bool{}
	action := accessConstants.ProjectCreate
	if id != 0 {
		action = accessConstants.ProjectUpdate
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, "project", id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			input, err := cleanProject(request.ProjectInput)
			if err != nil {
				return err
			}
			record := projectModels.Project{
				WorkspaceID: workspaceID,
				Revision:    1,
			}
			stepsUnchanged := false
			wasOpen := false
			if id != 0 {
				before, err := projectDetail(tx, workspaceID, id)
				if err != nil {
					return err
				}
				if request.Revision != before.Revision {
					return stale()
				}
				stepsUnchanged = sameApprovalSteps(input.ApprovalSteps, before.ApprovalSteps)
				wasOpen = before.AllowSelfApply
				if input.ScoreCalculation == nil {
					input.ScoreCalculation = before.ScoreCalculation
				}
				if stepsUnchanged {
					requested := input.ApprovalSteps
					input.ApprovalSteps = append(
						[]projectModels.ApprovalStepInput{}, before.ApprovalSteps...,
					)
					for index, step := range requested {
						if step.AllowEdit == nil {
							continue
						}
						if approvalStepAllowsEdit(
							step,
						) != approvalStepAllowsEdit(
							before.ApprovalSteps[index],
						) {
							editingChanges[index+1] = *step.AllowEdit
						}
						input.ApprovalSteps[index].AllowEdit = step.AllowEdit
					}
				}
				audit.Before = projectSnapshot(before)
				record.ID = id
				record.Revision = before.Revision + 1
			}
			// 关闭申报不依赖历史审批人当前是否启用，重新开放时仍重新校验整个流程。
			if id == 0 || !stepsUnchanged || (input.AllowSelfApply && !wasOpen) {
				if err := cleanApprovalSteps(tx, workspaceID, &input); err != nil {
					return err
				}
			}
			record.Name, record.Year, record.Description = input.Name, input.Year, input.Description
			record.AllowSelfApply = input.AllowSelfApply
			record.SelfApplyDeadline = input.SelfApplyDeadline
			record.ScoreCalculation, err = prepareScoreCalculation(
				input.ScoreCalculation,
				input.DetailNames,
			)
			if err != nil {
				return err
			}
			if id == 0 {
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
			} else {
				if err := tx.Model(&projectModels.Project{}).
					Where("workspace_id = ? AND id = ?", workspaceID, id).
					Updates(map[string]any{
						"name":                record.Name,
						"year":                record.Year,
						"description":         record.Description,
						"allow_self_apply":    record.AllowSelfApply,
						"self_apply_deadline": record.SelfApplyDeadline,
						"score_calculation":   record.ScoreCalculation,
						"revision":            record.Revision,
					}).
					Error; err != nil {
					return writeError(err)
				}
			}
			if err := saveDetailOptions(tx, workspaceID, record.ID, input.DetailNames); err != nil {
				return err
			}
			if err := saveApprovalSteps(
				tx,
				workspaceID,
				record.ID,
				input.ApprovalSteps,
			); err != nil {
				return err
			}
			result, err = projectDetail(tx, workspaceID, record.ID)
			if err != nil {
				return err
			}

			// 流程实际变更才重置未完成申报，与项目配置及逐单审计原子提交。
			if id != 0 && !stepsUnchanged {
				workflowChanged = true
				resetApplications, notifications, err = resetProjectApplications(
					tx,
					workspaceID,
					result,
					applicationActor{UserID: principal.User.ID, Name: principal.User.DisplayName},
				)
				if err != nil {
					return err
				}
			} else if len(editingChanges) > 0 {
				editedApplications, err = syncProjectApplicationEditing(
					tx, workspaceID, result.ID, editingChanges,
				)
				if err != nil {
					return err
				}
			}
			audit.ObjectID, audit.After = strconv.FormatUint(record.ID, 10), projectSnapshot(result)
			if record.SelfApplyDeadline != nil {
				audit.Conditions["auto_submit_at"] = record.SelfApplyDeadline.Add(
					projectConstants.AutoSubmitDelay,
				)
			}
			if err := enqueueDraftAutoSubmit(tx, record); err != nil {
				return err
			}
			// 调整排名维度或重新保存时均可重排，失败后可通过再次保存重试。
			return enqueueApplicationRanking(tx, workspaceID, record.ID,
				result.ScoreCalculation != nil && result.ScoreCalculation.Enabled)
		})
	if err != nil && workflowChanged {
		logger.Logger.WithContext(ctx).Errorf(
			"Application workflow reset failed; workspace: %s; project: %d; error: %v",
			workspaceID, id, err,
		)
	}
	if err != nil && len(editingChanges) > 0 {
		logger.Logger.WithContext(ctx).Errorf(
			"Application editing settings update failed; workspace: %s; project: %d; error: %v",
			workspaceID, id, err,
		)
	}

	// 实际重置及通知在包含项目审计的事务提交后记录，失败不留下成功日志。
	if err == nil {
		for _, applicationID := range editedApplications {
			logger.Logger.WithContext(ctx).Infof(
				"Application editing settings updated; workspace: %s; project: %d; application: %d",
				workspaceID, result.ID, applicationID,
			)
		}
		for _, applicationID := range resetApplications {
			logger.Logger.WithContext(ctx).Infof(
				"Application workflow reset; workspace: %s; project: %d; application: %d",
				workspaceID, result.ID, applicationID,
			)
		}
		notifications.logCreated(ctx)
	}
	if err == nil && result.SelfApplyDeadline != nil {
		logger.Logger.WithContext(ctx).Infof(
			"Application auto-submit task queued; action: %s; workspace: %s; project: %d; deadline: %s",
			action, workspaceID, result.ID, result.SelfApplyDeadline.Format(time.RFC3339Nano),
		)
	}
	return result, err
}

// DeleteProject 显式检查申报、获奖及附件引用后，硬删除项目及其子项和审批流程配置。
func DeleteProject(ctx context.Context, workspaceID string, id, revision uint64) error {
	var optionCount int
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.ProjectDelete,
		event(workspaceID, "project", id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 校验租户及并发版本，删除前保留完整的自有配置快照。
			before, err := projectDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			// 申报属于独立业务，先显式检查关联，不能通过级联绕过审批及审计。
			for _, table := range []string{"project_applications", "project_participations", "project_attachments"} {
				var count int64
				if err := tx.Table(table).
					Where("workspace_id = ? AND project_id = ?", workspaceID, id).
					Count(&count).
					Error; err != nil {
					return err
				}
				if count > 0 {
					return &response.BusinessError{
						Status: http.StatusConflict, Message: "项目仍有关联申报、获奖记录或附件，不能删除",
					}
				}
			}
			audit.Before = projectSnapshot(before)
			optionCount = len(before.DetailNames)

			// 仅由所属项目外键清理子项配置，其他业务引用继续保护项目。
			return writeError(
				tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
					Delete(&projectModels.Project{}).
					Error,
			)
		})

	// 删除与审计提交后记录项目及自有配置的变更。
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Project deleted; action: %s; workspace: %s; project: %d; detail options: %d",
			accessConstants.ProjectDelete, workspaceID, id, optionCount,
		)
	}
	return err
}

func cleanProject(input projectModels.ProjectInput) (projectModels.ProjectInput, error) {
	input.Name, input.Description = strings.TrimSpace(
		input.Name,
	), strings.TrimSpace(
		input.Description,
	)
	if input.Name == "" || !validText(input.Name, projectConstants.NameLength, false) ||
		!validText(input.Description, projectConstants.DescriptionLength, true) ||
		input.Year < projectConstants.MinYear || input.Year > projectConstants.MaxYear {
		return input, invalid("项目名称、归属年份或说明无效")
	}
	if input.AllowSelfApply && input.SelfApplyDeadline == nil {
		return input, invalid("允许学生自行申报时必须填写截止时间")
	}
	if input.SelfApplyDeadline != nil {
		deadline := input.SelfApplyDeadline.UTC().Truncate(time.Microsecond)
		if deadline.Year() < projectConstants.MinYear ||
			deadline.Year() > projectConstants.MaxYear {
			return input, invalid("自行申报截止时间无效")
		}
		input.SelfApplyDeadline = &deadline
	}

	var err error
	input.DetailNames, err = cleanProjectDetailNames(input.DetailNames)
	if err != nil {
		return input, err
	}
	if _, err := prepareScoreCalculation(input.ScoreCalculation, input.DetailNames); err != nil {
		return input, err
	}
	return input, nil
}

func cleanProjectDetailNames(names []string) ([]string, error) {
	// 子项配置只保留有效且不重复的名称，空数组表示尚未配置。
	if names == nil {
		names = []string{}
	}
	seen := make(map[string]bool, len(names))
	for index, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || !validText(name, projectConstants.NameLength, false) || seen[name] {
			return names, invalid("子项名称不能为空、重复或超过 100 个字符")
		}
		names[index], seen[name] = name, true
	}
	return names, nil
}

func projectDetail(tx *gorm.DB, workspaceID string, id uint64) (projectModels.ProjectInfo, error) {
	var record projectModels.Project
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
		Take(&record).
		Error; err != nil {
		return projectModels.ProjectInfo{}, lookupError(err)
	}
	names, err := detailOptionNames(tx, workspaceID, id)
	if err != nil {
		return projectModels.ProjectInfo{}, err
	}
	steps, err := approvalSteps(tx, workspaceID, id)
	if err != nil {
		return projectModels.ProjectInfo{}, err
	}
	calculation, err := decodeScoreCalculation(record.ScoreCalculation)
	if err != nil {
		return projectModels.ProjectInfo{}, err
	}
	ids := []uint64{}
	if err := tx.Model(&attachmentModels.ProjectAttachment{}).
		Where(
			"workspace_id = ? AND project_id = ? AND state = ?",
			workspaceID,
			id,
			attachmentConstants.Ready,
		).
		Order("id ASC").
		Pluck("id", &ids).
		Error; err != nil {
		return projectModels.ProjectInfo{}, err
	}
	return projectModels.ProjectInfo{
		ProjectInput: projectModels.ProjectInput{
			Name:              record.Name,
			Year:              record.Year,
			Description:       record.Description,
			DetailNames:       names,
			AllowSelfApply:    record.AllowSelfApply,
			SelfApplyDeadline: record.SelfApplyDeadline,
			ApprovalSteps:     steps,
			ScoreCalculation:  calculation,
		},
		AttachmentIDs: ids,
		ID:            record.ID,
		Revision:      record.Revision,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}, nil
}

func projectSnapshot(info projectModels.ProjectInfo) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":                  info.ID,
		"name":                info.Name,
		"year":                info.Year,
		"description":         info.Description,
		"detail_names":        info.DetailNames,
		"allow_self_apply":    info.AllowSelfApply,
		"self_apply_deadline": info.SelfApplyDeadline,
		"approval_steps":      info.ApprovalSteps,
		"score_calculation":   info.ScoreCalculation,
		"attachment_ids":      info.AttachmentIDs,
		"revision":            info.Revision,
	}
}

func event(workspaceID, object string, id uint64) auditModels.Event {
	result := auditModels.Event{TenantID: &workspaceID, ObjectType: object,
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID}}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}

func validText(value string, limit int, multiline bool) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) &&
			(!multiline || (char != '\n' && char != '\r' && char != '\t')) {
			return false
		}
	}
	return true
}
func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}
func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "记录已被修改，请刷新后重试"}
}
func lookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &response.BusinessError{Status: http.StatusNotFound, Message: "记录不存在或不可访问"}
	}
	return err
}
func writeError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		switch database.Number {
		case 1062:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "同年同名项目或学生项目参与记录已存在",
			}
		case 1451:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "记录仍有关联数据，请先处理关联记录",
			}
		case 1452:
			return invalid("关联记录不存在或不可访问")
		}
	}
	return err
}
