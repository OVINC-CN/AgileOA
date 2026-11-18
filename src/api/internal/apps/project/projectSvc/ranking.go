package projectSvc

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"slices"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

type applicationRankingRow struct {
	ID, Revision            uint64
	Rank                    *int32
	Score                   *float64
	Grade, Major, ClassName string
}

type applicationRankingChange struct {
	ID, Revision  uint64
	Before, After *int32
}

// RankApplications 在后台按最新配置原子重排同奖项全部已通过申报。
func RankApplications(ctx context.Context, workspaceID string, projectID uint64) error {
	if db.DB() == nil {
		return errors.New("database is unavailable")
	}
	var changes []applicationRankingChange
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 与审批、项目编辑和学生资料变更共用空间锁，重复任务幂等执行。
		workspace, err := accessUtils.LockWorkspace(tx, workspaceID)
		if rankingTargetMissing(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !workspace.IsEnabled {
			return nil
		}
		var project projectModels.Project
		if err := tx.Select("score_calculation").
			Where("workspace_id = ? AND id = ?", workspaceID, projectID).
			Take(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		calculation, err := decodeScoreCalculation(project.ScoreCalculation)
		if err != nil {
			return err
		}
		if calculation == nil || !calculation.Enabled {
			return nil
		}

		// 只读取排名所需的基础维度及已保存总分，不读取敏感档案或未完成申报。
		var rows []applicationRankingRow
		if err := tx.Table("project_applications AS a").
			Select("a.id, a.revision, a.rank, a.score, s.grade, s.major, s.class_name").
			Joins("JOIN students AS s ON s.workspace_id = a.workspace_id AND s.id = a.student_id").
			Where("a.workspace_id = ? AND a.project_id = ? AND a.status = ?",
				workspaceID, projectID, projectConstants.ApplicationApproved).
			Order("a.id ASC").Scan(&rows).Error; err != nil {
			return err
		}
		changes, err = rankApplicationRows(rows, calculation.RankingDimensions)
		if err != nil {
			return err
		}
		for _, change := range changes {
			written := tx.Model(&projectModels.Application{}).
				Where("workspace_id = ? AND project_id = ? AND id = ? AND revision = ? AND status = ?",
					workspaceID, projectID, change.ID, change.Revision,
					projectConstants.ApplicationApproved).
				Updates(map[string]any{"rank": change.After, "revision": change.Revision + 1})
			if written.Error != nil {
				return written.Error
			}
			if written.RowsAffected != 1 {
				return errors.New("application revision changed during ranking")
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	// 排名全部提交后才记录实际变更，不冒用触发人的用户审计身份。
	for _, change := range changes {
		logger.Logger.WithContext(ctx).Infof(
			"Application ranking updated; workspace: %s; project: %d; application: %d; "+
				"before: %v; after: %v",
			workspaceID, projectID, change.ID, rankingLogValue(change.Before),
			rankingLogValue(change.After),
		)
	}
	return nil
}

func rankingTargetMissing(err error) bool {
	var business *response.BusinessError
	return errors.Is(err, gorm.ErrRecordNotFound) ||
		errors.As(err, &business) && business.Status == http.StatusNotFound
}

func rankApplicationRows(
	rows []applicationRankingRow,
	dimensions []string,
) ([]applicationRankingChange, error) {
	// 空总分按 0 分参与排序，负分仍有效；同分使用竞赛排名，例：1、1、3。
	if len(rows) > math.MaxInt32 {
		return nil, errors.New("application ranking exceeds supported range")
	}
	groups := make(map[[3]string][]applicationRankingRow)
	for _, row := range rows {
		score := applicationRankingScore(row)
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return nil, errors.New("application score is not finite")
		}
		var group [3]string
		for _, field := range dimensions {
			switch field {
			case "grade":
				group[0] = row.Grade
			case "major":
				group[1] = row.Major
			case "class_name":
				group[2] = row.ClassName
			}
		}
		groups[group] = append(groups[group], row)
	}
	ranks := make(map[uint64]int32, len(rows))
	for _, group := range groups {
		slices.SortFunc(group, func(left, right applicationRankingRow) int {
			leftScore, rightScore := applicationRankingScore(left), applicationRankingScore(right)
			switch {
			case leftScore > rightScore:
				return -1
			case leftScore < rightScore:
				return 1
			default:
				return 0
			}
		})
		var rank int32
		for index, row := range group {
			if index == 0 ||
				applicationRankingScore(row) != applicationRankingScore(group[index-1]) {
				rank = int32(index + 1) // #nosec G115 -- 已校验总记录数不超过 MaxInt32。
			}
			ranks[row.ID] = rank
		}
	}

	// 按原编号顺序写入，仅更新实际变化的排名，不改写原始总分。
	changes := make([]applicationRankingChange, 0, len(rows))
	for _, row := range rows {
		after := ranks[row.ID]
		if row.Rank != nil && *row.Rank == after {
			continue
		}
		changes = append(changes, applicationRankingChange{
			ID: row.ID, Revision: row.Revision, Before: row.Rank, After: &after,
		})
	}
	return changes, nil
}

func applicationRankingScore(row applicationRankingRow) float64 {
	if row.Score == nil {
		return 0
	}
	return *row.Score
}

func rankingLogValue(rank *int32) any {
	if rank == nil {
		return nil
	}
	return *rank
}
