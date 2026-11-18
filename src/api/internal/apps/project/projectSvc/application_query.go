package projectSvc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

const applicationColumns = "a.*, p.name AS project_name, p.year, p.allow_self_apply, " +
	"p.self_apply_deadline, " +
	"s.student_no, s.name AS student_name, s.class_name"

// ListApplications 按管理、本人及节点指派范围查询，待我审批仅包含当前轮次尚未签署的节点。
func ListApplications(
	ctx context.Context,
	workspaceID, mode string,
	query projectModels.ApplicationQuery,
) (projectModels.ApplicationList, error) {
	result := projectModels.ApplicationList{
		Items:    []projectModels.ApplicationSummary{},
		Page:     query.Page,
		PageSize: query.PageSize,
	}
	err := applicationAccess(
		ctx,
		workspaceID,
		applicationAction(isStudentActor(ctx), mode, ""),
		event(workspaceID, "project_application", 0),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			if err := validateApplicationQuery(query); err != nil {
				return err
			}
			audit.Conditions = applicationFilterSnapshot(query, mode)
			audit.Conditions["page"], audit.Conditions["page_size"] = query.Page, query.PageSize
			scope := applicationQueryScope(tx, workspaceID, mode, actor, query)
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			if err := scope.Select(applicationColumns).
				Order("a.id DESC").
				Offset((query.Page - 1) * query.PageSize).
				Limit(query.PageSize).
				Scan(&result.Items).
				Error; err != nil {
				return err
			}
			for index := range result.Items {
				if err := fillApplicationSummary(
					tx,
					workspaceID,
					mode,
					actor,
					&result.Items[index],
				); err != nil {
					return err
				}
				audit.ReturnedIDs = append(
					audit.ReturnedIDs,
					strconv.FormatUint(result.Items[index].ID, 10),
				)
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

func fillApplicationSummary(
	tx *gorm.DB,
	workspaceID, mode string,
	actor applicationActor,
	info *projectModels.ApplicationSummary,
) error {
	if info.Status == projectConstants.ApplicationApproved {
		return nil
	}
	nodes, err := applicationStepNodes(tx, workspaceID, info.ID, info.CurrentStep)
	if err != nil {
		return err
	}
	labels := make([]string, len(nodes))
	for index, node := range nodes {
		labels[index] = node.ReviewerLabel
	}
	info.CurrentReviewer = strings.Join(labels, "、")
	if node, matched := matchingApplicationNode(actor, *info, nodes); matched {
		applicationPermissions(info, actor, mode, node)
	}
	if info.CanApprove {
		votes, err := loadNodeApprovals(tx, workspaceID, info.ID, info.CurrentStep)
		if err != nil {
			return err
		}
		kind, actorID := actorIdentity(actor)
		if votes[identityKey(kind, actorID)] {
			info.CanApprove = false
		}
		var pending int64
		if err := tx.Model(&attachmentModels.ApplicationDetailAttachment{}).
			Where("workspace_id = ? AND application_id = ? AND state <> ?", workspaceID, info.ID, "ready").
			Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			info.CanApprove = false
		}

	}
	return nil
}

func applicationDetail(
	tx *gorm.DB,
	workspaceID, mode string,
	actor applicationActor,
	id uint64,
) (projectModels.ApplicationInfo, error) {
	result := projectModels.ApplicationInfo{
		Steps:   []projectModels.ApplicationStepInfo{},
		Details: []projectModels.ApplicationDetailInfo{},
		History: []projectModels.ApplicationHistory{},
	}
	var row struct {
		projectModels.ApplicationSummary
		projectModels.ApplicationValues
		SelfApplyDeadline  *time.Time
		ProjectDescription string
	}
	if err := applicationScope(tx, workspaceID, mode, actor).
		Select(applicationColumns+", p.description AS project_description").
		Where("a.id = ?", id).Take(&row).Error; err != nil {
		return result, lookupError(err)
	}
	result.ApplicationSummary, result.ApplicationValues = row.ApplicationSummary, row.ApplicationValues
	result.SelfApplyDeadline = row.SelfApplyDeadline
	result.ProjectDescription = row.ProjectDescription
	if err := fillApplicationSummary(
		tx,
		workspaceID,
		mode,
		actor,
		&result.ApplicationSummary,
	); err != nil {
		return result, err
	}
	var nodes []projectModels.ApplicationNode
	if err := tx.Where("workspace_id = ? AND application_id = ? AND position > 0", workspaceID, id).
		Order("position ASC, target_order ASC").Find(&nodes).Error; err != nil {
		return result, err
	}
	if result.Status == projectConstants.ApplicationDraft {
		var configured []projectModels.ApprovalStep
		if err := tx.Where("workspace_id = ? AND project_id = ?", workspaceID, result.ProjectID).
			Order("position ASC, target_order ASC").Find(&configured).Error; err != nil {
			return result, err
		}
		for _, step := range configured {
			nodes = append(nodes, projectModels.ApplicationNode{
				Position:      step.Position,
				TargetOrder:   step.TargetOrder,
				Kind:          step.Kind,
				SignMode:      step.SignMode,
				AllowEdit:     step.AllowEdit,
				ClassPosition: step.ClassPosition,
				StudentID:     step.StudentID,
				UserID:        step.UserID,
				ReviewerLabel: step.ReviewerLabel,
			})
		}
	}
	groups := map[int][]projectModels.ApplicationNode{}
	configured := []projectModels.ApprovalStep{}
	for _, node := range nodes {
		groups[node.Position] = append(groups[node.Position], node)
		configured = append(
			configured,
			projectModels.ApprovalStep{Position: node.Position, TargetOrder: node.TargetOrder,
				AllowEdit: node.AllowEdit,
				Kind:      node.Kind, SignMode: node.SignMode, ClassPosition: node.ClassPosition,
				StudentID: node.StudentID, UserID: node.UserID, ReviewerLabel: node.ReviewerLabel},
		)
	}
	for index, input := range groupApprovalSteps(configured) {
		reviewers, ready, err := nodeReviewers(
			tx,
			workspaceID,
			result.ApplicationSummary,
			groups[index+1],
		)
		if err != nil {
			return result, err
		}
		votes, err := loadNodeApprovals(tx, workspaceID, id, index+1)
		if err != nil {
			return result, err
		}
		for i := range reviewers {
			reviewers[i].Approved = votes[identityKey(reviewers[i].IdentityType, reviewers[i].ID)]
		}
		result.Steps = append(
			result.Steps,
			projectModels.ApplicationStepInfo{ApprovalStepInput: input,
				Position: index + 1, Reviewers: reviewers, Ready: ready},
		)
	}
	var details []projectModels.ApplicationDetail
	if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id).
		Order("sort_order ASC, id ASC").Find(&details).Error; err != nil {
		return result, err
	}
	for _, detail := range details {
		info := projectModels.ApplicationDetailInfo{ID: detail.ID,
			ApplicationDetailInput: projectModels.ApplicationDetailInput{
				Key: detail.DetailKey,
				ScoreItem: projectModels.ScoreItem{
					Name:        detail.Name,
					Description: detail.Description,
					Score:       detail.Score,
				},
			},
			Attachments: []attachmentModels.Info{}}
		var files []attachmentModels.ApplicationDetailAttachment
		if err := tx.Where("workspace_id = ? AND application_id = ? AND detail_id = ?", workspaceID, id, detail.ID).
			Order("id ASC").
			Find(&files).
			Error; err != nil {
			return result, err
		}
		for _, file := range files {
			info.Attachments = append(info.Attachments, applicationAttachmentInfo(file))
		}
		result.Details = append(result.Details, info)
	}
	if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id).
		Order("id ASC").
		Find(&result.History).
		Error; err != nil {
		return result, err
	}
	if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id).
		Order("position ASC, actor_type ASC, actor_id ASC").
		Find(&result.ApprovalRecords).Error; err != nil {
		return result, err
	}
	var err error
	result.DetailNames, err = detailOptionNames(tx, workspaceID, result.ProjectID)
	if err != nil {
		return result, err
	}
	calculation, err := projectScoreCalculation(tx, workspaceID, result.ProjectID)
	if err != nil {
		return result, err
	}
	if calculation != nil {
		result.ScoreCalculationEnabled = calculation.Enabled
	}
	return result, nil
}

// ApplicationDetail 审计持久化后返回允许范围内的正文、附件及各级签署进度。
func ApplicationDetail(
	ctx context.Context,
	workspaceID, mode string,
	id uint64,
) (projectModels.ApplicationInfo, error) {
	var result projectModels.ApplicationInfo
	err := applicationAccess(
		ctx,
		workspaceID,
		applicationAction(isStudentActor(ctx), mode, ""),
		event(workspaceID, "project_application", id),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			var err error
			result, err = applicationDetail(tx, workspaceID, mode, actor, id)
			if err != nil {
				return err
			}
			audit.ReturnedIDs, audit.ReturnedCount = []string{strconv.FormatUint(id, 10)}, 1
			return nil
		},
	)
	return result, err
}

// StudentProjects 仅返回允许自行申报的项目，沿用固定项目字段和分页上限。
func StudentProjects(ctx context.Context, workspaceID string, query projectModels.Query) (
	projectModels.ProjectList, error,
) {
	result := projectModels.ProjectList{Items: []projectModels.ProjectInfo{},
		Page: query.Page, PageSize: query.PageSize}
	err := applicationAccess(ctx, workspaceID, accessConstants.StudentProjectList,
		event(workspaceID, "project", 0),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			if err := validQuery(query); err != nil {
				return err
			}
			// 同一项目只允许一张申报，已有申报或获奖记录不再作为新建选项。
			applications := tx.Model(&projectModels.Application{}).Select("1").
				Where("workspace_id = ? AND project_id = projects.id AND student_id = ?", workspaceID, actor.StudentID)
			awards := tx.Model(&projectModels.Participation{}).Select("1").
				Where("workspace_id = ? AND project_id = projects.id AND student_id = ?", workspaceID, actor.StudentID)
			scope := projectListScope(tx, workspaceID, query).Where("allow_self_apply = ?", true).
				Where("self_apply_deadline IS NULL OR self_apply_deadline > ?", time.Now().UTC()).
				Where("NOT EXISTS (?) AND NOT EXISTS (?)", applications, awards)
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			var ids []uint64
			if err := scope.Order("id DESC").Offset((query.Page-1)*query.PageSize).
				Limit(query.PageSize).Pluck("id", &ids).Error; err != nil {
				return err
			}
			for _, id := range ids {
				info, err := projectDetail(tx, workspaceID, id)
				if err != nil {
					return err
				}
				// 学生不读取项目私有附件编号或其他审批人的账号编号。
				info.AttachmentIDs = []uint64{}
				for index := range info.ApprovalSteps {
					info.ApprovalSteps[index].StudentIDs, info.ApprovalSteps[index].UserIDs = []uint64{}, []uint64{}
					info.ApprovalSteps[index].ReviewerOptions = []projectModels.NameOption{}
				}
				result.Items = append(result.Items, info)
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(id, 10))
			}
			audit.Conditions["keyword"] = query.Keyword
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// ApplicationProjectOptions 与奖项管理、获奖学生复用同一项目名称和归属年份枚举。
func ApplicationProjectOptions(
	ctx context.Context,
	workspaceID, mode string,
) (projectModels.FilterOptionsData, error) {
	result := projectModels.FilterOptionsData{
		Names:           []projectModels.FilterOption{},
		Years:           []projectModels.FilterOption{},
		EducationLevels: []projectModels.FilterOption{},
		Majors:          []projectModels.FilterOption{},
		Grades:          []projectModels.FilterOption{},
	}
	err := applicationAccess(ctx, workspaceID, applicationAction(isStudentActor(ctx), mode, ""),
		event(workspaceID, "project", 0),
		func(tx *gorm.DB, _ applicationActor, audit *auditModels.Event) error {
			return loadProjectFilterOptions(tx, workspaceID, &result, audit)
		})
	return result, err
}
