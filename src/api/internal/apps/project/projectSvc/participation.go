package projectSvc

import (
	"context"
	"math"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

type participationRow struct {
	projectModels.Participation
	ProjectName    string
	Year           int
	StudentNo      string
	StudentName    string
	EducationLevel string
	Major          string
	Grade          string
	DetailCount    int64
}

// SaveParticipation 独立授权并保存排名、总分及有序子项，不计算或覆盖任一分数。
func SaveParticipation(ctx context.Context, workspaceID string, id uint64,
	request projectModels.ParticipationRequest,
) (projectModels.ParticipationInfo, error) {
	var result projectModels.ParticipationInfo
	action := accessConstants.ParticipationCreate
	if id != 0 {
		action = accessConstants.ParticipationUpdate
	}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		action,
		event(workspaceID, "project_participation", id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			input, err := cleanParticipation(request.ParticipationInput)
			if err != nil {
				return err
			}
			if err := validateRelations(tx, workspaceID, input); err != nil {
				return err
			}
			record := projectModels.Participation{
				WorkspaceID: workspaceID,
				Revision:    1,
			}
			if id != 0 {
				before, err := participationDetail(tx, workspaceID, id)
				if err != nil {
					return err
				}
				if request.Revision != before.Revision {
					return stale()
				}
				audit.Before = participationSnapshot(before)
				record.ID = id
				record.Revision = before.Revision + 1
			}
			record.ProjectID = input.ProjectID
			record.StudentID = input.StudentID
			record.Rank = input.Rank
			record.Score = input.Score
			if id == 0 {
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
			} else {
				// 请求明确替换全部子项，先清空旧明细以允许变更所属项目。
				if err := clearDetails(tx, workspaceID, id); err != nil {
					return err
				}
				// 排名和总分明确包含空值；子项在独立表中由同一事务保存。
				if err := tx.Model(&record).Where("workspace_id = ? AND id = ?", workspaceID, id).
					Select("ProjectID", "StudentID", "Rank", "Score", "Revision").
					Updates(&record).Error; err != nil {
					return writeError(err)
				}
			}
			if err := saveDetails(
				tx,
				workspaceID,
				record.ID,
				input.ProjectID,
				input.Details,
			); err != nil {
				return err
			}
			result, err = participationDetail(tx, workspaceID, record.ID)
			if err != nil {
				return err
			}
			audit.ObjectID, audit.After = strconv.FormatUint(
				record.ID,
				10,
			), participationSnapshot(
				result,
			)
			return nil
		},
	)
	return result, err
}

// DeleteParticipation 硬删除参与记录及自有评分子项，同事务保留完整删除快照。
func DeleteParticipation(
	ctx context.Context,
	workspaceID string,
	id, revision uint64,
) error {
	var detailCount int
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.ParticipationDelete,
		event(workspaceID, "project_participation", id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 校验租户及并发版本，删除前保留总分与全部评分子项。
			before, err := participationDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			audit.Before = participationSnapshot(before)
			detailCount = len(before.Details)

			// 评分子项随参与记录级联删除，不删除项目或学生。
			return writeError(
				tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
					Delete(&projectModels.Participation{}).
					Error,
			)
		})

	// 删除与审计提交后记录参与记录及评分子项的变更。
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Project participation deleted; action: %s; workspace: %s; participation: %d; details: %d",
			accessConstants.ParticipationDelete, workspaceID, id, detailCount,
		)
	}
	return err
}

func cleanParticipation(
	input projectModels.ParticipationInput,
) (projectModels.ParticipationInput, error) {
	// 排名独立填写，未填写时保持空值，不由总分推算。
	if input.Rank != nil && *input.Rank < 1 {
		return input, invalid("排名必须为正整数或留空")
	}

	if input.ProjectID == 0 || input.StudentID == 0 || !validScore(input.Score) ||
		len(input.Details) > config.Config.Project.MaxDetails {
		return input, invalid("项目、学生、总分或评分子项数量无效")
	}
	var err error
	input.Details, err = cleanScoreItems(input.Details)
	return input, err
}

func cleanScoreItems(items []projectModels.ScoreItem) ([]projectModels.ScoreItem, error) {
	// 申报、试算及获奖录入共用子项校验，保留空分与零分的区别。
	if len(items) > config.Config.Project.MaxDetails {
		return items, invalid("评分子项数量超过限制")
	}
	if items == nil {
		items = []projectModels.ScoreItem{}
	}
	seen := make(map[string]bool, len(items))
	for index := range items {
		item := &items[index]
		item.Name, item.Description = strings.TrimSpace(
			item.Name,
		), strings.TrimSpace(
			item.Description,
		)
		if item.Name == "" || !validText(item.Name, projectConstants.NameLength, false) ||
			!validText(
				item.Description,
				projectConstants.DescriptionLength,
				true,
			) || !validScore(item.Score) {
			return items, invalid("子项名称、说明或分数无效")
		}
		if seen[item.Name] {
			return items, invalid("同一条记录中的子项名称不能重复")
		}
		seen[item.Name] = true
	}
	return items, nil
}

func validScore(score *float64) bool {
	return score == nil || (!math.IsNaN(*score) && !math.IsInf(*score, 0))
}

func validateRelations(
	tx *gorm.DB,
	workspaceID string,
	input projectModels.ParticipationInput,
) error {
	var projectCount, studentCount int64
	if err := tx.Model(&projectModels.Project{}).
		Where("workspace_id = ? AND id = ?", workspaceID, input.ProjectID).
		Count(&projectCount).
		Error; err != nil {
		return err
	}
	// 仅校验固定学生编号，不读取学生宽表的敏感字段。
	if err := tx.Table("students").
		Where("workspace_id = ? AND id = ?", workspaceID, input.StudentID).
		Count(&studentCount).
		Error; err != nil {
		return err
	}
	if projectCount != 1 || studentCount != 1 {
		return invalid("项目或学生不存在或不可访问")
	}
	return validateDetailNames(tx, workspaceID, input.ProjectID, input.Details)
}

func participationScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return tx.Table("project_participations AS r").
		Joins("JOIN projects AS p ON p.workspace_id = r.workspace_id AND p.id = r.project_id").
		Joins("JOIN students AS s ON s.workspace_id = r.workspace_id AND s.id = r.student_id").
		Where("r.workspace_id = ?", workspaceID)
}
func participationColumns() string {
	return "r.*, p.name AS project_name, p.year, s.student_no, s.name AS student_name, " +
		"s.education_level, s.major, s.grade, " +
		"(SELECT COUNT(*) FROM project_participation_details AS d " +
		"WHERE d.workspace_id = r.workspace_id AND d.participation_id = r.id) AS detail_count"
}

func participationDetail(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
) (projectModels.ParticipationInfo, error) {
	var row participationRow
	if err := participationScope(
		tx,
		workspaceID,
	).Select(participationColumns()).
		Where("r.id = ?", id).
		Take(&row).
		Error; err != nil {
		return projectModels.ParticipationInfo{}, lookupError(err)
	}
	details := []projectModels.ScoreItem{}
	if err := tx.Model(&projectModels.Detail{}).
		Where("workspace_id = ? AND participation_id = ?", workspaceID, id).
		Select("name", "description", "score").
		Order("sort_order ASC, id ASC").
		Scan(&details).
		Error; err != nil {
		return projectModels.ParticipationInfo{}, err
	}
	return projectModels.ParticipationInfo{
		ParticipationSummary: participationInfo(row),
		Details:              details,
	}, nil
}
func participationInfo(row participationRow) projectModels.ParticipationSummary {
	return projectModels.ParticipationSummary{
		ParticipationFields: projectModels.ParticipationFields{
			ProjectID: row.ProjectID,
			StudentID: row.StudentID,
			Rank:      row.Rank,
			Score:     row.Score,
		},
		ID:             row.ID,
		ProjectName:    row.ProjectName,
		Year:           row.Year,
		StudentNo:      row.StudentNo,
		StudentName:    row.StudentName,
		EducationLevel: row.EducationLevel,
		Major:          row.Major,
		Grade:          row.Grade,
		Revision:       row.Revision,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		DetailCount:    row.DetailCount,
	}
}

// ParticipationDetail 按需读取完整子项，列表只返回子项数量。
func ParticipationDetail(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (projectModels.ParticipationInfo, error) {
	var result projectModels.ParticipationInfo
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ParticipationList,
		event(workspaceID, "project_participation", id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			var err error
			result, err = participationDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			audit.ReturnedIDs, audit.ReturnedCount = []string{strconv.FormatUint(id, 10)}, 1
			return nil
		},
	)
	return result, err
}

func clearDetails(tx *gorm.DB, workspaceID string, id uint64) error {
	// 不在删除参与记录时隐式解除外键，只处理显式替换的明细集合。
	return tx.Where("workspace_id = ? AND participation_id = ?", workspaceID, id).
		Delete(&projectModels.Detail{}).Error
}

func saveDetails(
	tx *gorm.DB,
	workspaceID string,
	id, projectID uint64,
	items []projectModels.ScoreItem,
) error {
	// 明细同时关联参与记录与当前项目的配置名称，禁止跨项目引用。
	if len(items) == 0 {
		return nil
	}
	records := make([]projectModels.Detail, len(items))
	for index, item := range items {
		records[index] = projectModels.Detail{
			WorkspaceID:     workspaceID,
			ParticipationID: id,
			ProjectID:       projectID,
			SortOrder:       index,
			Name:            item.Name,
			Description:     item.Description,
			Score:           item.Score,
		}
	}
	return detailWriteError(tx.Create(&records).Error)
}

func participationSnapshot(info projectModels.ParticipationInfo) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":              info.ID,
		"project_id":      info.ProjectID,
		"project_name":    info.ProjectName,
		"year":            info.Year,
		"student_id":      info.StudentID,
		"student_no":      info.StudentNo,
		"student_name":    info.StudentName,
		"education_level": info.EducationLevel,
		"major":           info.Major,
		"grade":           info.Grade,
		"rank":            info.Rank,
		"score":           info.Score,
		"details":         info.Details,
		"revision":        info.Revision,
	}
}
