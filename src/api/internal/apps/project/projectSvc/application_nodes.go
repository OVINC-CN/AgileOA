package projectSvc

import (
	"fmt"
	"slices"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func prepareDraftSubmission(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	input projectModels.ProjectInput,
	className string,
) (int, error) {
	// 手动和自动首次提交共用流程校验，关闭新建也必须有可用的审批流程。
	if len(input.ApprovalSteps) == 0 {
		return 0, invalid("申报必须配置审批流")
	}
	if err := cleanApprovalSteps(tx, workspaceID, &input); err != nil {
		return 0, err
	}
	if err := validateClassApprovers(tx, workspaceID, className, input.ApprovalSteps); err != nil {
		return 0, err
	}
	if err := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id).
		Delete(&projectModels.ApplicationNode{}).Error; err != nil {
		return 0, err
	}
	if err := initializeApplicationNodes(tx, workspaceID, id, input.ApprovalSteps); err != nil {
		return 0, err
	}
	return len(input.ApprovalSteps), nil
}

func initializeApplicationNodes(tx *gorm.DB, workspaceID string, id uint64,
	steps []projectModels.ApprovalStepInput,
) error {
	root := projectModels.ApplicationNode{
		WorkspaceID:   workspaceID,
		ApplicationID: id,
		Kind:          projectConstants.ApproverApplicant,
		SignMode:      projectConstants.SignAny,
		AllowEdit:     true,
		ReviewerLabel: "申报人",
	}
	if err := tx.Create(&root).Error; err != nil {
		return err
	}
	for index, step := range steps {
		targets, err := approvalTargets(tx, workspaceID, step)
		if err != nil {
			return err
		}
		for _, target := range targets {
			node := projectModels.ApplicationNode{
				WorkspaceID:   workspaceID,
				ApplicationID: id,
				Position:      index + 1,
				TargetOrder:   target.TargetOrder,
				SignMode:      target.SignMode,
				AllowEdit:     target.AllowEdit,
				Kind:          target.Kind,
				ClassPosition: target.ClassPosition,
				StudentID:     target.StudentID,
				UserID:        target.UserID,
				ReviewerLabel: target.ReviewerLabel,
			}
			if err := tx.Create(&node).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func applicationStepNodes(tx *gorm.DB, workspaceID string, id uint64, step int) (
	[]projectModels.ApplicationNode, error,
) {
	nodes := []projectModels.ApplicationNode{}
	err := tx.Where("workspace_id = ? AND application_id = ? AND position = ?", workspaceID, id, step).
		Order("target_order ASC").
		Find(&nodes).
		Error
	if err == nil && len(nodes) == 0 {
		return nodes, invalid("审批节点不存在")
	}
	return nodes, err
}

func matchingApplicationNode(actor applicationActor, info projectModels.ApplicationSummary,
	nodes []projectModels.ApplicationNode,
) (projectModels.ApplicationNode, bool) {
	for _, node := range nodes {
		if actorMatches(actor, info.StudentID, info.ClassName, node) {
			return node, true
		}
	}
	return projectModels.ApplicationNode{}, false
}

func identityKey(kind string, id uint64) string { return kind + ":" + strconv.FormatUint(id, 10) }

func actorIdentity(actor applicationActor) (string, uint64) {
	if actor.StudentID != 0 {
		return "student", actor.StudentID
	}
	return "user", actor.UserID
}

func classReviewers(tx *gorm.DB, workspaceID, className string, positions []string) (
	[]projectModels.ApplicationReviewer, map[string]bool, error,
) {
	result, coverage := []projectModels.ApplicationReviewer{}, map[string]bool{}
	if className == "" {
		return result, coverage, nil
	}
	var students []struct {
		ID                             uint64
		StudentNo, Name, ClassPosition string
	}
	if err := tx.Table("students").Select("id, student_no, name, class_position").
		Where("workspace_id = ? AND class_name = ? AND status = ?", workspaceID, className,
			studentConstants.StatusEnrolled).
		Order("id ASC").Limit(config.Config.Project.MaxOptions + 1).
		Scan(&students).Error; err != nil {
		return result, coverage, err
	}
	if len(students) > config.Config.Project.MaxOptions {
		return result, coverage, invalid("班级学生数量超过限制")
	}
	for _, student := range students {
		matched := false
		for _, position := range projectUtils.Positions(student.ClassPosition) {
			if slices.Contains(positions, position) {
				coverage[position], matched = true, true
			}
		}
		if matched {
			result = append(result, projectModels.ApplicationReviewer{IdentityType: "student",
				ID: student.ID, Label: student.Name + "(" + student.StudentNo + ")"})
		}
	}
	return result, coverage, nil
}

func nodeReviewers(tx *gorm.DB, workspaceID string, info projectModels.ApplicationSummary,
	nodes []projectModels.ApplicationNode,
) ([]projectModels.ApplicationReviewer, bool, error) {
	result := []projectModels.ApplicationReviewer{}
	if len(nodes) == 0 {
		return result, false, invalid("审批节点不存在")
	}
	if nodes[0].Kind == projectConstants.ApproverClassPosition {
		positions := make([]string, len(nodes))
		for index, node := range nodes {
			positions[index] = node.ClassPosition
		}
		var coverage map[string]bool
		var err error
		result, coverage, err = classReviewers(tx, workspaceID, info.ClassName, positions)
		if err != nil {
			return result, false, err
		}
		ready := len(result) > 0
		if nodes[0].SignMode == projectConstants.SignAll {
			ready = ready && len(coverage) == len(positions)
		}
		return result, ready, nil
	}
	seen := map[string]bool{}
	for _, node := range nodes {
		reviewer := projectModels.ApplicationReviewer{Label: node.ReviewerLabel}
		switch node.Kind {
		case projectConstants.ApproverApplicant:
			reviewer.IdentityType, reviewer.ID = "student", info.StudentID
			reviewer.Label = fmt.Sprintf("%s(%s)", info.StudentName, info.StudentNo)
		case projectConstants.ApproverStudent:
			if node.StudentID == nil {
				return result, false, invalid("学生审批节点无效")
			}
			reviewer.IdentityType, reviewer.ID = "student", *node.StudentID
		case projectConstants.ApproverUser:
			if node.UserID == nil {
				return result, false, invalid("组织审批节点无效")
			}
			reviewer.IdentityType, reviewer.ID = "user", *node.UserID
		default:
			return result, false, invalid("审批节点类型无效")
		}
		key := identityKey(reviewer.IdentityType, reviewer.ID)
		if !seen[key] {
			result = append(result, reviewer)
			seen[key] = true
		}
	}
	return result, len(result) > 0, nil
}

func loadNodeApprovals(tx *gorm.DB, workspaceID string, id uint64, step int) (
	map[string]bool, error,
) {
	var records []projectModels.ApplicationApproval
	if err := tx.Where("workspace_id = ? AND application_id = ? AND position = ?", workspaceID, id, step).
		Find(&records).
		Error; err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(records))
	for _, record := range records {
		result[identityKey(record.ActorType, record.ActorID)] = true
	}
	return result, nil
}

func allReviewersApproved(mode string, ready bool, reviewers []projectModels.ApplicationReviewer,
	votes map[string]bool,
) bool {
	if !ready || len(reviewers) == 0 {
		return false
	}
	for _, reviewer := range reviewers {
		signed := votes[identityKey(reviewer.IdentityType, reviewer.ID)]
		if mode == projectConstants.SignAny && signed {
			return true
		}
		if mode == projectConstants.SignAll && !signed {
			return false
		}
	}
	return mode == projectConstants.SignAll
}

func addNodeApproval(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	step int,
	actor applicationActor,
) error {
	kind, actorID := actorIdentity(actor)
	record := projectModels.ApplicationApproval{WorkspaceID: workspaceID, ApplicationID: id,
		Position: step, ActorType: kind, ActorID: actorID}
	if actor.StudentID != 0 {
		record.StudentID = &actorID
	} else {
		record.UserID = &actorID
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return invalid("本轮已通过该节点，请等待其他审批人")
	}
	return nil
}

func clearNodeApprovals(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	step int,
	following bool,
) error {
	scope := tx.Where("workspace_id = ? AND application_id = ?", workspaceID, id)
	if following {
		scope = scope.Where("position >= ?", step)
	} else {
		scope = scope.Where("position = ?", step)
	}
	return scope.Delete(&projectModels.ApplicationApproval{}).Error
}
