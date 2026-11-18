package projectSvc

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

var applicationDetailKeyPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// CreateApplication 保存唯一申报或草稿，正式提交时复制当前审批配置。
func CreateApplication(
	ctx context.Context,
	workspaceID string,
	request projectModels.ApplicationRequest,
) (projectModels.ApplicationInfo, error) {
	var result projectModels.ApplicationInfo
	var notifications applicationNotifications
	err := applicationAccess(
		ctx,
		workspaceID,
		accessConstants.StudentApplicationCreate,
		event(workspaceID, "project_application", 0),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			if actor.StudentID == 0 || request.Revision != 0 {
				return accessUtils.Denied()
			}
			project, err := projectDetail(tx, workspaceID, request.ProjectID)
			if err != nil {
				return err
			}
			if err := requireNewApplication(project, time.Now().UTC()); err != nil {
				return err
			}
			content, err := cleanApplication(
				tx,
				workspaceID,
				project.ID,
				actor.StudentID,
				request,
				nil,
			)
			if err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&projectModels.Participation{}).
				Where("workspace_id = ? AND project_id = ? AND student_id = ?",
					workspaceID, project.ID, actor.StudentID).
				Count(&count).
				Error; err != nil {
				return err
			}
			if count != 0 {
				return invalid("该项目已存在本人的获奖记录，不能重复申报")
			}
			status, current, operation := projectConstants.ApplicationDraft, 0, projectConstants.ApplicationDraft
			steps := []projectModels.ApprovalStepInput{}
			if !request.SaveAsDraft {
				input := project.ProjectInput
				if err := cleanApprovalSteps(tx, workspaceID, &input); err != nil {
					return err
				}
				if err := validateClassApprovers(
					tx,
					workspaceID,
					actor.ClassName,
					input.ApprovalSteps,
				); err != nil {
					return err
				}
				status, current, operation, steps = projectConstants.ApplicationPending, 1, projectConstants.ApplicationSubmit, input.ApprovalSteps
			}

			// 草稿与正式申报共用唯一约束，附件始终按该申报编号归档。
			record := projectModels.Application{
				WorkspaceID: workspaceID,
				ProjectID:   project.ID,
				StudentID:   actor.StudentID,
				Statement:   content.Statement,
				Rank:        content.Rank,
				Score:       content.Score,
				Status:      status,
				CurrentStep: current,
				Revision:    1,
			}
			if err := tx.Create(&record).Error; err != nil {
				return applicationWriteError(err)
			}
			if err := initializeApplicationNodes(tx, workspaceID, record.ID, steps); err != nil {
				return err
			}
			if err := saveApplicationDetails(
				tx,
				workspaceID,
				record.ID,
				project.ID,
				content.Details,
			); err != nil {
				return err
			}
			if err := applicationHistory(
				tx,
				workspaceID,
				record.ID,
				0,
				actor,
				operation,
				request.Comment,
			); err != nil {
				return err
			}
			result, err = applicationDetail(
				tx,
				workspaceID,
				projectConstants.ApplicationMine,
				actor,
				record.ID,
			)
			if err != nil {
				return err
			}
			audit.ObjectID, audit.After = strconv.FormatUint(
				record.ID,
				10,
			), applicationSnapshot(
				result,
			)
			notifications, err = notifyApplication(
				tx, workspaceID, projectModels.ApplicationInfo{}, result,
			)
			return err
		},
	)
	if err == nil {
		notifications.logCreated(ctx)
	}
	return result, err
}

func requireNewApplication(project projectModels.ProjectInfo, now time.Time) error {
	// 开放状态和截止时间仅限制新建，已有单据始终按自身流程处理。
	if !project.AllowSelfApply {
		return invalid("该项目未开放新建申报")
	}
	if project.SelfApplyDeadline != nil && !now.Before(*project.SelfApplyDeadline) {
		return invalid("该项目已超过自行申报截止时间")
	}
	return nil
}

func cleanApplication(
	tx *gorm.DB,
	workspaceID string,
	projectID, studentID uint64,
	request projectModels.ApplicationRequest,
	currentRank *int32,
) (projectModels.ApplicationContent, error) {
	calculation, err := projectScoreCalculation(tx, workspaceID, projectID)
	if err != nil {
		return projectModels.ApplicationContent{}, err
	}
	content, err := parseApplicationScores(request, calculation, currentRank)
	if err != nil {
		return content, err
	}
	content.Statement = strings.TrimSpace(content.Statement)
	if !validText(content.Statement, projectConstants.DescriptionLength, true) ||
		!validText(request.Comment, projectConstants.DescriptionLength, true) {
		return content, invalid("申报说明及审批意见不能超过 10000 字符")
	}
	scores := make([]projectModels.ScoreItem, len(content.Details))
	seen := map[string]bool{}
	for index, detail := range content.Details {
		if !applicationDetailKeyPattern.MatchString(detail.Key) || seen[detail.Key] {
			return content, invalid("子项标识无效或重复")
		}
		seen[detail.Key], scores[index] = true, detail.ScoreItem
	}
	input, err := cleanParticipation(
		projectModels.ParticipationInput{ParticipationFields: projectModels.ParticipationFields{
			ProjectID: projectID,
			StudentID: studentID,
			Rank:      content.Rank,
			Score:     content.Score,
		}, Details: scores},
	)
	if err != nil {
		return content, err
	}
	if err := validateDetailNames(tx, workspaceID, projectID, input.Details); err != nil {
		return content, err
	}
	if content.Details == nil {
		content.Details = []projectModels.ApplicationDetailInput{}
	}
	for index := range content.Details {
		content.Details[index].ScoreItem = input.Details[index]
	}
	if err := calculateApplicationScore(
		tx, workspaceID, projectID, studentID, calculation, &content,
	); err != nil {
		return content, err
	}
	return content, nil
}

func validateClassApprovers(
	tx *gorm.DB,
	workspaceID, className string,
	steps []projectModels.ApprovalStepInput,
) error {
	for _, step := range steps {
		if step.Kind != projectConstants.ApproverClassPosition {
			continue
		}
		reviewers, coverage, err := classReviewers(tx, workspaceID, className, step.ClassPositions)
		if err != nil {
			return err
		}
		if len(reviewers) == 0 ||
			(step.SignMode == projectConstants.SignAll && len(coverage) != len(step.ClassPositions)) {
			return invalid("所在班级的所选班委尚未配置完整，请联系管理员完善任职信息")
		}
	}
	return nil
}

// ChangeApplication 保存内容或处理当前轮次；并签全部通过后才流向下一级。
func ChangeApplication(
	ctx context.Context,
	workspaceID, mode, operation string,
	id uint64,
	request projectModels.ApplicationRequest,
) (projectModels.ApplicationInfo, error) {
	var result projectModels.ApplicationInfo
	var notifications applicationNotifications
	action := applicationAction(isStudentActor(ctx), mode, operation)
	err := applicationAccess(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_application", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			before, err := applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			if request.Revision != before.Revision {
				return stale()
			}
			if before.Status == projectConstants.ApplicationApproved {
				return invalid("已通过的申报不能修改")
			}
			if mode == projectConstants.ApplicationMine &&
				before.Status != projectConstants.ApplicationRejected &&
				before.Status != projectConstants.ApplicationDraft {
				return invalid("提交后只能修改被驳回的申报单")
			}
			advancing := operation == projectConstants.ApplicationSubmit ||
				operation == projectConstants.ApplicationApprove
			nodes, err := applicationStepNodes(tx, workspaceID, id, before.CurrentStep)
			if err != nil {
				return err
			}
			node, matches := matchingApplicationNode(actor, before.ApplicationSummary, nodes)
			if !matches ||
				(mode == projectConstants.ApplicationMine && node.Kind != projectConstants.ApproverApplicant) ||
				(mode == projectConstants.ApplicationReviews && node.Position == 0) {
				return accessUtils.Denied()
			}
			if request.ProjectID != 0 && request.ProjectID != before.ProjectID {
				return invalid("申报不能更换所属项目")
			}
			if err := validateApplicationEditing(before, request, operation); err != nil {
				return err
			}
			content, err := cleanApplication(
				tx,
				workspaceID,
				before.ProjectID,
				before.StudentID,
				request,
				before.Rank,
			)
			if err != nil {
				return err
			}
			if operation == projectConstants.ApplicationReject &&
				strings.TrimSpace(request.Comment) == "" {
				return invalid("驳回时请填写审批意见")
			}
			audit.Before = applicationSnapshot(before)
			total := len(before.Steps)

			// 草稿首次正式提交读取最新流程，后续项目流程变更由项目保存事务同步重置。
			if before.Status == projectConstants.ApplicationDraft && advancing {
				project, err := projectDetail(tx, workspaceID, before.ProjectID)
				if err != nil {
					return err
				}
				total, err = prepareDraftSubmission(
					tx, workspaceID, id, project.ProjectInput, before.ClassName,
				)
				if err != nil {
					return err
				}
			}
			changed := !reflect.DeepEqual(applicationInput(before), content)
			if changed {
				if err := clearNodeApprovals(
					tx,
					workspaceID,
					id,
					before.CurrentStep,
					false,
				); err != nil {
					return err
				}
			}
			step, status := before.CurrentStep, before.Status
			switch operation {
			case projectConstants.ApplicationUpdate:
			case projectConstants.ApplicationReject:
				step, status, err = nextApplicationState(step, total, status, operation)
				if err != nil {
					return err
				}
				if err := clearNodeApprovals(tx, workspaceID, id, step, true); err != nil {
					return err
				}
			case projectConstants.ApplicationApprove, projectConstants.ApplicationSubmit:
				if err := requireApplicationFilesReady(tx, workspaceID, id); err != nil {
					return err
				}
				if err := addNodeApproval(
					tx,
					workspaceID,
					id,
					before.CurrentStep,
					actor,
				); err != nil {
					return err
				}
				reviewers, ready, err := nodeReviewers(
					tx,
					workspaceID,
					before.ApplicationSummary,
					nodes,
				)
				if err != nil {
					return err
				}
				votes, err := loadNodeApprovals(tx, workspaceID, id, before.CurrentStep)
				if err != nil {
					return err
				}
				if allReviewersApproved(node.SignMode, ready, reviewers, votes) {
					step, status, err = nextApplicationState(step, total, status, operation)
					if err != nil {
						return err
					}
				}
			default:
				return invalid("申报操作无效")
			}

			// 内容、签署、历史和审计共用事务；末级通过仅完成原申报单。
			if err := saveApplicationDetails(
				tx,
				workspaceID,
				id,
				before.ProjectID,
				content.Details,
			); err != nil {
				return err
			}
			if err := tx.Model(&projectModels.Application{}).
				Where("workspace_id = ? AND id = ?", workspaceID, id).
				Updates(map[string]any{
					"statement":    content.Statement,
					"rank":         content.Rank,
					"score":        content.Score,
					"current_step": step,
					"status":       status,
					"revision":     before.Revision + 1,
				}).
				Error; err != nil {
				return err
			}
			historyOperation := operation
			if before.Status == projectConstants.ApplicationDraft &&
				operation == projectConstants.ApplicationUpdate {
				historyOperation = projectConstants.ApplicationDraft
			}
			if err := applicationHistory(
				tx,
				workspaceID,
				id,
				before.CurrentStep,
				actor,
				historyOperation,
				request.Comment,
			); err != nil {
				return err
			}
			result, err = applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.After = applicationSnapshot(result)
			notifications, err = notifyApplication(tx, workspaceID, before, result)
			if err != nil {
				return err
			}
			if result.Status == projectConstants.ApplicationApproved {
				return enqueueApplicationRanking(
					tx, workspaceID, result.ProjectID, result.ScoreCalculationEnabled,
				)
			}
			return nil
		},
	)
	if err == nil {
		notifications.logCreated(ctx)
	}
	return result, err
}

func nextApplicationState(step, total int, status, operation string) (int, string, error) {
	if status == projectConstants.ApplicationApproved || step < 0 || step > total || total < 1 {
		return step, status, invalid("申报流程状态无效")
	}
	switch operation {
	case projectConstants.ApplicationUpdate:
		return step, status, nil
	case projectConstants.ApplicationReject:
		if step == 0 {
			return step, status, invalid("申报人节点不能继续驳回")
		}
		return 0, projectConstants.ApplicationRejected, nil
	case projectConstants.ApplicationApprove, projectConstants.ApplicationSubmit:
		if step == total {
			return step, projectConstants.ApplicationApproved, nil
		}
		return step + 1, projectConstants.ApplicationPending, nil
	default:
		return step, status, invalid("申报操作无效")
	}
}

func applicationInput(info projectModels.ApplicationInfo) projectModels.ApplicationContent {
	details := make([]projectModels.ApplicationDetailInput, len(info.Details))
	for index, detail := range info.Details {
		details[index] = detail.ApplicationDetailInput
	}
	return projectModels.ApplicationContent{
		ApplicationValues: info.ApplicationValues,
		Details:           details,
	}
}

func validateApplicationEditing(
	before projectModels.ApplicationInfo,
	request projectModels.ApplicationRequest,
	operation string,
) error {
	// 只读审批仍可提交意见及流转，不能通过审批请求夹带字段修改。
	if before.CanEdit {
		return nil
	}
	denied := &response.BusinessError{
		Status: http.StatusForbidden, Message: "当前审批级别不允许编辑申报内容",
	}
	if operation != projectConstants.ApplicationApprove &&
		operation != projectConstants.ApplicationReject {
		return denied
	}
	content, err := parseApplicationScores(request, &projectModels.ScoreCalculation{
		Enabled: before.ScoreCalculationEnabled,
	}, before.Rank)
	if err != nil {
		return err
	}
	content.Statement = strings.TrimSpace(content.Statement)
	existing := applicationInput(before)
	if before.ScoreCalculationEnabled {
		// 总分由后续共用计算器更新，客户端总分及排名继续按强制计算契约忽略。
		existing.Score = nil
	}
	if !reflect.DeepEqual(existing, content) {
		return denied
	}
	return nil
}

type applicationDetailRow struct {
	projectModels.ApplicationDetail
	CanonicalName string
}

func saveApplicationDetails(
	tx *gorm.DB, workspaceID string, id, projectID uint64,
	items []projectModels.ApplicationDetailInput,
) error {
	// 读取配置中的名称以沿用数据库排序规则，保留明细编号和已有附件引用。
	var rows []applicationDetailRow
	if err := tx.Table("project_application_details AS d").
		Select("d.*, o.name AS canonical_name").
		Joins("JOIN project_detail_options AS o ON o.workspace_id = d.workspace_id AND o.project_id = d.project_id AND o.name = d.name").
		Where("d.workspace_id = ? AND d.application_id = ?", workspaceID, id).
		Find(&rows).Error; err != nil {
		return err
	}
	existing := make(map[string]projectModels.ApplicationDetail, len(rows))
	for _, row := range rows {
		row.Name = row.CanonicalName
		existing[row.DetailKey] = row.ApplicationDetail
	}
	positions := make(map[string]int, len(items))
	for index, item := range items {
		positions[item.Key] = index
	}
	for _, row := range rows {
		if _, kept := positions[row.DetailKey]; kept {
			continue
		}
		var count int64
		if err := tx.Model(&attachmentModels.ApplicationDetailAttachment{}).
			Where("workspace_id = ? AND application_id = ? AND detail_id = ?", workspaceID, id, row.ID).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return invalid("请先删除子项附件，再移除该子项")
		}
		if err := tx.Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, row.ID).
			Delete(&projectModels.ApplicationDetail{}).
			Error; err != nil {
			return err
		}
		delete(existing, row.DetailKey)
	}
	scores := make([]projectModels.ScoreItem, len(items))
	for index, item := range items {
		scores[index] = item.ScoreItem
	}
	names, err := canonicalDetailNames(tx, workspaceID, projectID, scores)
	if err != nil {
		return err
	}
	normalized := append([]projectModels.ApplicationDetailInput(nil), items...)
	for index := range normalized {
		normalized[index].Name = names[index]
	}
	ordered, err := orderApplicationDetails(existing, normalized)
	if err != nil {
		return err
	}

	// 先释放旧名称再写入依赖它的子项，避免唯一约束阻止合法的更换或移除后重新新增。
	for _, next := range ordered {
		item := items[positions[next.Key]]
		record, found := existing[item.Key]
		if !found {
			record = projectModels.ApplicationDetail{
				WorkspaceID:   workspaceID,
				ApplicationID: id,
				ProjectID:     projectID,
				DetailKey:     item.Key,
			}
		}
		record.SortOrder, record.Name, record.Description, record.Score = positions[item.Key], item.Name, item.Description, item.Score
		if !found {
			if err := tx.Create(&record).Error; err != nil {
				return detailWriteError(err)
			}
		} else if err := tx.Model(&projectModels.ApplicationDetail{}).
			Where("workspace_id = ? AND application_id = ? AND id = ?", workspaceID, id, record.ID).
			Updates(map[string]any{"sort_order": positions[item.Key], "name": item.Name, "description": item.Description, "score": item.Score}).
			Error; err != nil {
			return detailWriteError(err)
		}
	}
	return nil
}

func orderApplicationDetails(
	existing map[string]projectModels.ApplicationDetail,
	items []projectModels.ApplicationDetailInput,
) ([]projectModels.ApplicationDetailInput, error) {
	occupied := make(map[string]string, len(existing))
	for key, item := range existing {
		occupied[item.Name] = key
	}
	pending := append([]projectModels.ApplicationDetailInput(nil), items...)
	ordered := make([]projectModels.ApplicationDetailInput, 0, len(items))
	for len(pending) > 0 {
		remaining := make([]projectModels.ApplicationDetailInput, 0, len(pending))
		for _, item := range pending {
			if owner, used := occupied[item.Name]; used && owner != item.Key {
				remaining = append(remaining, item)
				continue
			}
			if before, found := existing[item.Key]; found {
				delete(occupied, before.Name)
			}
			occupied[item.Name] = item.Key
			ordered = append(ordered, item)
		}
		if len(remaining) == len(pending) {
			return nil, invalid("子项名称互换冲突，请分次修改并保存")
		}
		pending = remaining
	}
	return ordered, nil
}

func requireApplicationFilesReady(tx *gorm.DB, workspaceID string, id uint64) error {
	var count int64
	if err := tx.Model(&attachmentModels.ApplicationDetailAttachment{}).
		Where("workspace_id = ? AND application_id = ? AND state <> ?", workspaceID, id, attachmentConstants.Ready).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return invalid("存在未完成上传或删除的附件，请先处理附件")
	}
	return nil
}

func applicationHistory(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	step int,
	actor applicationActor,
	operation, comment string,
) error {
	kind, actorID := actorIdentity(actor)
	return tx.Create(
		&projectModels.ApplicationHistory{
			WorkspaceID:   workspaceID,
			ApplicationID: id,
			Step:          step,
			Operation:     operation,
			ActorID:       actorID,
			ActorType:     kind,
			ActorName:     actor.Name,
			Comment:       strings.TrimSpace(comment),
		},
	).Error
}

func applicationSnapshot(info projectModels.ApplicationInfo) auditModels.Snapshot {
	// 子项标识属于业务代码，审计字段不能使用 key 或以 _key 结尾的凭据保留名称。
	details := make([]auditModels.Snapshot, len(info.Details))
	for index, detail := range info.Details {
		details[index] = auditModels.Snapshot{"id": detail.ID, "detail_code": detail.Key,
			"name": detail.Name, "description": detail.Description, "score": detail.Score,
			"attachments": detail.Attachments}
	}
	approvals := make([]auditModels.Snapshot, len(info.ApprovalRecords))
	for index, row := range info.ApprovalRecords {
		approvals[index] = auditModels.Snapshot{
			"workspace_id":   row.WorkspaceID,
			"application_id": row.ApplicationID,
			"position":       row.Position,
			"actor_type":     row.ActorType,
			"actor_id":       row.ActorID,
			"student_id":     row.StudentID,
			"user_id":        row.UserID,
			"created_at":     row.CreatedAt,
			"updated_at":     row.UpdatedAt,
		}
	}
	return auditModels.Snapshot{
		"id":           info.ID,
		"project_id":   info.ProjectID,
		"project_name": info.ProjectName,
		"year":         info.Year,
		"student_id":   info.StudentID,
		"student_no":   info.StudentNo,
		"student_name": info.StudentName,
		"class_name":   info.ClassName,
		"statement":    info.Statement,
		"rank":         info.Rank,
		"score":        info.Score,
		"details":      details,
		"status":       info.Status,
		"current_step": info.CurrentStep,
		"steps":        info.Steps,
		"history":      info.History,
		"approvals":    approvals,
		"revision":     info.Revision,
		"created_at":   info.CreatedAt,
		"updated_at":   info.UpdatedAt,
	}
}

func applicationWriteError(err error) error {
	converted := writeError(err)
	var business *response.BusinessError
	if errors.As(converted, &business) && business.Status == http.StatusConflict {
		return &response.BusinessError{
			Status:  http.StatusConflict,
			Message: "该项目已存在申报或草稿，请在我的申报中继续处理",
		}
	}
	return converted
}
