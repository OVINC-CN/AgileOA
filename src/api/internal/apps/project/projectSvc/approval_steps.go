package projectSvc

import (
	"cmp"
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func approvalSteps(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
) ([]projectModels.ApprovalStepInput, error) {
	rows := []projectModels.ApprovalStep{}
	if err := tx.Where("workspace_id = ? AND project_id = ?", workspaceID, projectID).
		Order("position ASC, target_order ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return groupApprovalSteps(rows), nil
}

func emptyApprovalStep(kind, signMode string) projectModels.ApprovalStepInput {
	return projectModels.ApprovalStepInput{
		Kind:            kind,
		SignMode:        signMode,
		ClassPositions:  []string{},
		StudentIDs:      []uint64{},
		UserIDs:         []uint64{},
		ReviewerOptions: []projectModels.NameOption{},
	}
}

func groupApprovalSteps(rows []projectModels.ApprovalStep) []projectModels.ApprovalStepInput {
	result := []projectModels.ApprovalStepInput{}
	positions := map[int]int{}
	for _, row := range rows {
		index, exists := positions[row.Position]
		if !exists {
			index = len(result)
			positions[row.Position] = index
			result = append(result, emptyApprovalStep(row.Kind, row.SignMode))
		}
		step := &result[index]
		allowEdit := row.AllowEdit
		step.AllowEdit = &allowEdit
		if row.ClassPosition != "" {
			step.ClassPositions = append(step.ClassPositions, row.ClassPosition)
		}
		if row.StudentID != nil {
			step.StudentIDs = append(step.StudentIDs, *row.StudentID)
		}
		if row.UserID != nil {
			step.UserIDs = append(step.UserIDs, *row.UserID)
		}
		if step.ReviewerLabel != "" {
			step.ReviewerLabel += "、"
		}
		step.ReviewerLabel += row.ReviewerLabel
		value := row.ClassPosition
		if row.StudentID != nil {
			value = strconv.FormatUint(*row.StudentID, 10)
		}
		if row.UserID != nil {
			value = strconv.FormatUint(*row.UserID, 10)
		}
		if value != "" {
			step.ReviewerOptions = append(
				step.ReviewerOptions,
				projectModels.NameOption{Value: value, Label: row.ReviewerLabel},
			)
		}

	}
	return result
}

func approvalTargets(
	tx *gorm.DB,
	workspaceID string,
	step projectModels.ApprovalStepInput,
) ([]projectModels.ApprovalStep, error) {
	rows := []projectModels.ApprovalStep{}
	if step.SignMode != projectConstants.SignAny && step.SignMode != projectConstants.SignAll {
		return nil, invalid("请选择或签或并签")
	}
	count := len(step.ClassPositions) + len(step.StudentIDs) + len(step.UserIDs)
	if count > config.Config.Project.MaxApprovalTargets {
		return nil, invalid("同级审批人数量超过限制")
	}
	switch step.Kind {
	case projectConstants.ApproverApplicant:
		if count != 0 || step.SignMode != projectConstants.SignAny {
			return nil, invalid("申报人节点不能选择其他人员")
		}
		rows = append(
			rows,
			projectModels.ApprovalStep{
				Kind:          step.Kind,
				SignMode:      step.SignMode,
				ReviewerLabel: "申报人",
			},
		)
	case projectConstants.ApproverClassPosition:
		if len(step.ClassPositions) == 0 || len(step.StudentIDs)+len(step.UserIDs) != 0 {
			return nil, invalid("请选择班委")
		}
		options, err := positionOptions(tx, workspaceID)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, position := range step.ClassPositions {
			if position == "" || !validText(position, 64, false) || seen[position] ||
				!slices.Contains(options, position) {
				return nil, invalid("班委名称无效、重复或不存在")
			}
			seen[position] = true
			rows = append(rows, projectModels.ApprovalStep{Kind: step.Kind, SignMode: step.SignMode,
				ClassPosition: position, ReviewerLabel: position})
		}
	case projectConstants.ApproverStudent, projectConstants.ApproverUser:
		ids := step.StudentIDs
		if step.Kind == projectConstants.ApproverUser {
			ids = step.UserIDs
		}
		if len(ids) == 0 || len(step.ClassPositions) != 0 ||
			(step.Kind == projectConstants.ApproverStudent && len(step.UserIDs) != 0) ||
			(step.Kind == projectConstants.ApproverUser && len(step.StudentIDs) != 0) {
			return nil, invalid("请选择相应类型的用户")
		}
		seen := map[uint64]bool{}
		for _, id := range ids {
			if id == 0 || seen[id] {
				return nil, invalid("审批用户编号无效或重复")
			}
			seen[id] = true
			var record struct{ Number, Name string }
			row := projectModels.ApprovalStep{Kind: step.Kind, SignMode: step.SignMode}
			value := id
			if step.Kind == projectConstants.ApproverStudent {
				row.StudentID = &value
				if err := tx.Table("students").Select("student_no AS number, name").
					Where("workspace_id = ? AND id = ? AND status = ?", workspaceID, id, studentConstants.StatusEnrolled).
					Take(&record).Error; err != nil {
					return nil, lookupError(err)
				}
			} else {
				row.UserID = &value
				if err := accessUtils.WorkspaceUsers(tx, workspaceID).
					Select("users.username AS number, users.display_name AS name").
					Where("users.id = ?", id).Take(&record).Error; err != nil {
					return nil, lookupError(err)
				}
			}
			// 配置及申报节点共用名称(账号或学号)格式的身份快照。
			row.ReviewerLabel = record.Name + "(" + record.Number + ")"
			rows = append(rows, row)
		}
	default:
		return nil, invalid("审批节点类型无效")
	}
	for index := range rows {
		rows[index].TargetOrder = index
		rows[index].AllowEdit = approvalStepAllowsEdit(step)
	}
	return rows, nil
}

func cleanApprovalSteps(tx *gorm.DB, workspaceID string, input *projectModels.ProjectInput) error {
	// 类型、签署方式及多选对象均使用固定目录，显示名称不信任请求中的值。
	if input.ApprovalSteps == nil {
		input.ApprovalSteps = []projectModels.ApprovalStepInput{}
	}
	if len(input.ApprovalSteps) > config.Config.Project.MaxApprovalSteps ||
		(input.AllowSelfApply && len(input.ApprovalSteps) == 0) {
		return invalid("开放申报必须配置审批流，审批级数不能超过限制")
	}
	for index := range input.ApprovalSteps {
		step := &input.ApprovalSteps[index]
		if step.SignMode == "" {
			step.SignMode = projectConstants.SignAny
		}
		if step.ClassPositions == nil {
			step.ClassPositions = []string{}
		}
		if step.StudentIDs == nil {
			step.StudentIDs = []uint64{}
		}
		if step.UserIDs == nil {
			step.UserIDs = []uint64{}
		}
		for i := range step.ClassPositions {
			step.ClassPositions[i] = strings.TrimSpace(step.ClassPositions[i])
		}
		rows, err := approvalTargets(tx, workspaceID, *step)
		if err != nil {
			return err
		}
		step.ReviewerLabel = ""
		normalized := groupApprovalSteps(rows)
		if len(normalized) > 0 {
			step.ReviewerLabel = normalized[0].ReviewerLabel
			step.ReviewerOptions = normalized[0].ReviewerOptions
		}

	}
	return nil
}

func saveApprovalSteps(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
	steps []projectModels.ApprovalStepInput,
) error {
	// 每个多选对象保留独立级联外键，同级顺序不承担签署先后含义。
	current, err := approvalSteps(tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(current, steps) {
		return nil
	}
	// 单独修改编辑开关不重新关联审批人，也不要求已保存的处理人仍然启用。
	if sameApprovalSteps(current, steps) {
		for index, step := range steps {
			allowEdit := approvalStepAllowsEdit(step)
			if approvalStepAllowsEdit(current[index]) == allowEdit {
				continue
			}
			if err := tx.Model(&projectModels.ApprovalStep{}).
				Where("workspace_id = ? AND project_id = ? AND position = ?",
					workspaceID, projectID, index+1).
				Update("allow_edit", allowEdit).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if err := tx.Where("workspace_id = ? AND project_id = ?", workspaceID, projectID).
		Delete(&projectModels.ApprovalStep{}).Error; err != nil {
		return err
	}
	for index, step := range steps {
		rows, err := approvalTargets(tx, workspaceID, step)
		if err != nil {
			return err
		}
		for i := range rows {
			rows[i].WorkspaceID, rows[i].ProjectID, rows[i].Position = workspaceID, projectID, index+1
		}
		if err := tx.Create(&rows).Error; err != nil {
			return writeError(err)
		}
	}
	return nil
}

func positionOptions(tx *gorm.DB, workspaceID string) ([]string, error) {
	values := []string{}
	if err := tx.Table("students").Where("workspace_id = ? AND class_position <> ''", workspaceID).
		Distinct("class_position").Order("class_position ASC").
		Limit(config.Config.Project.MaxOptions+1).
		Pluck("class_position", &values).Error; err != nil {
		return nil, err
	}
	if len(values) > config.Config.Project.MaxOptions {
		return nil, invalid("班委选项超过限制")
	}
	unique := map[string]bool{}
	for _, value := range values {
		for _, position := range projectUtils.Positions(value) {
			unique[position] = true
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

// ApprovalOptions 枚举班委、在籍学生或当前空间拥有进入权限的启用组织账号。
func ApprovalOptions(ctx context.Context, workspaceID, kind, keyword string) (
	projectModels.ApprovalOptions, error,
) {
	result := projectModels.ApprovalOptions{Items: []projectModels.NameOption{}}
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.ProjectList,
		event(workspaceID, "project_approval", 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			if !validText(keyword, projectConstants.NameLength, false) {
				return invalid("审批人搜索关键词无效")
			}
			audit.Conditions["kind"], audit.Conditions["keyword"] = kind, keyword
			if kind == projectConstants.ApproverClassPosition {
				positions, err := positionOptions(tx, workspaceID)
				if err != nil {
					return err
				}
				for _, position := range positions {
					if strings.Contains(position, keyword) {
						result.Items = append(result.Items, projectModels.NameOption{
							Value: position, Label: position})
					}
				}
			} else {
				var rows []struct {
					ID           uint64
					Number, Name string
				}
				var scope *gorm.DB
				switch kind {
				case projectConstants.ApproverStudent:
					scope = tx.Table("students").Select("id, student_no AS number, name").
						Where("workspace_id = ? AND status = ?", workspaceID, studentConstants.StatusEnrolled).
						Where("LOCATE(?, student_no) > 0 OR LOCATE(?, name) > 0", keyword, keyword)
				case projectConstants.ApproverUser:
					scope = accessUtils.WorkspaceUsers(tx, workspaceID).
						Select("users.id, users.username AS number, users.display_name AS name").
						Where("LOCATE(?, users.username) > 0 OR LOCATE(?, users.display_name) > 0",
							keyword, keyword)
				default:
					return invalid("审批人选项类型无效")
				}
				if err := scope.Order("id DESC").Limit(config.Config.Project.MaxOptions + 1).
					Scan(&rows).Error; err != nil {
					return err
				}
				if len(rows) > config.Config.Project.MaxOptions {
					return invalid("审批人选项过多，请输入关键词搜索")
				}
				for _, row := range rows {
					result.Items = append(result.Items, projectModels.NameOption{
						Value: strconv.FormatUint(
							row.ID,
							10,
						),
						Label: fmt.Sprintf("%s(%s)", row.Name, row.Number),
					})
				}
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

func sameApprovalSteps(left, right []projectModels.ApprovalStepInput) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a.SignMode == "" {
			a.SignMode = projectConstants.SignAny
		}
		if b.SignMode == "" {
			b.SignMode = projectConstants.SignAny
		}
		if a.Kind != b.Kind || a.SignMode != b.SignMode ||
			!sameApprovalTargets(a.ClassPositions, b.ClassPositions) ||
			!sameApprovalTargets(a.StudentIDs, b.StudentIDs) ||
			!sameApprovalTargets(a.UserIDs, b.UserIDs) {
			return false
		}
	}
	return true
}

func approvalStepAllowsEdit(step projectModels.ApprovalStepInput) bool {
	return step.AllowEdit == nil || *step.AllowEdit
}

func sameApprovalTargets[T cmp.Ordered](left, right []T) bool {
	// 同级多选顺序不影响审批语义，不能因重新排序清空已完成的本轮签署。
	a, b := slices.Clone(left), slices.Clone(right)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
