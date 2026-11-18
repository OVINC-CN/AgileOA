package projectSvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// AutoSubmitDraftApplications 提交项目绑定的草稿，不携带或生成用户操作审计。
func AutoSubmitDraftApplications(ctx context.Context, value projectModels.AutoSubmitPayload) error {
	if db.DB() == nil {
		return errors.New("database is unavailable")
	}
	var ids []uint64
	if err := db.DB().WithContext(ctx).Table("project_applications AS a").
		Joins("JOIN projects AS p ON p.workspace_id = a.workspace_id AND p.id = a.project_id").
		Where("a.workspace_id = ? AND a.project_id = ? AND a.status = ?",
			value.WorkspaceID, value.ProjectID, projectConstants.ApplicationDraft).
		Where("p.self_apply_deadline = ? AND p.self_apply_deadline < ?",
			value.Deadline, time.Now().UTC()).
		Order("a.id ASC").Pluck("a.id", &ids).Error; err != nil {
		return err
	}

	// 每张草稿独立提交，失败不阻塞其他草稿；重试时只读取仍未提交的记录。
	var failure error
	failed := 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		changed := false
		var notifications applicationNotifications
		err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var err error
			changed, notifications, err = autoSubmitDraft(tx, value, id)
			return err
		}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			failed++
			if failure == nil {
				failure = fmt.Errorf("application %d: %w", id, err)
			}
			logger.Logger.WithContext(ctx).Errorf(
				"Application auto-submit failed; workspace: %s; project: %d; application: %d; error: %v",
				value.WorkspaceID, value.ProjectID, id, err,
			)
			continue
		}

		// 只在事务真正提交后记录变更，重复任务不会产生成功日志或流转记录。
		if changed {
			notifications.logCreated(ctx)
			logger.Logger.WithContext(ctx).Infof(
				"Application submitted; operation: auto_submit; workspace: %s; project: %d; application: %d",
				value.WorkspaceID, value.ProjectID, id,
			)
		}
	}
	if failure != nil {
		return fmt.Errorf("failed to auto-submit %d applications: %w", failed, failure)
	}
	return nil
}

func autoSubmitDraft(
	tx *gorm.DB,
	value projectModels.AutoSubmitPayload,
	id uint64,
) (bool, applicationNotifications, error) {
	var notifications applicationNotifications
	// 与项目编辑、手动提交和权限修改共用空间锁，锁内重新读取当前截止时间。
	workspace, err := accessUtils.LockWorkspace(tx, value.WorkspaceID)
	if err != nil {
		var business *response.BusinessError
		if errors.As(err, &business) && business.Status == http.StatusNotFound {
			return false, notifications, nil
		}
		return false, notifications, err
	}
	if !workspace.IsEnabled {
		return false, notifications, errors.New("workspace is disabled")
	}
	var project projectModels.Project
	if err := tx.Where("workspace_id = ? AND id = ?", value.WorkspaceID, value.ProjectID).
		Take(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, notifications, nil
		}
		return false, notifications, err
	}
	if !draftAutoSubmitDue(project.SelfApplyDeadline, value.Deadline, time.Now().UTC()) {
		return false, notifications, nil
	}
	calculation, err := decodeScoreCalculation(project.ScoreCalculation)
	if err != nil {
		return false, notifications, err
	}
	var application projectModels.Application
	if err := tx.Where("workspace_id = ? AND project_id = ? AND id = ? AND status = ?",
		value.WorkspaceID, value.ProjectID, id, projectConstants.ApplicationDraft).
		Take(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, notifications, nil
		}
		return false, notifications, err
	}

	// 首次提交沿用最新审批配置、班委覆盖及附件完整性校验，不改写草稿内容。
	steps, err := approvalSteps(tx, value.WorkspaceID, value.ProjectID)
	if err != nil {
		return false, notifications, err
	}
	var student struct{ ClassName, StudentNo, Name string }
	if err := tx.Table("students").Select("class_name", "student_no", "name").
		Where("workspace_id = ? AND id = ?", value.WorkspaceID, application.StudentID).
		Take(&student).Error; err != nil {
		return false, notifications, err
	}
	if err := requireApplicationFilesReady(tx, value.WorkspaceID, id); err != nil {
		return false, notifications, err
	}
	// 自动提交同样使用当前分数规则，失败时保留整张草稿。
	if calculation != nil && calculation.Enabled {
		var details []projectModels.ApplicationDetail
		if err := tx.Table("project_application_details FORCE INDEX (idx_project_application_detail_order)").
			Select("name", "score").
			Where("workspace_id = ? AND application_id = ?", value.WorkspaceID, id).
			Order("sort_order ASC, id ASC").
			Find(&details).
			Error; err != nil {
			return false, notifications, err
		}
		content := projectModels.ApplicationContent{
			ApplicationValues: projectModels.ApplicationValues{Score: application.Score},
		}
		for _, detail := range details {
			content.Details = append(content.Details, projectModels.ApplicationDetailInput{
				ScoreItem: projectModels.ScoreItem{Name: detail.Name, Score: detail.Score},
			})
		}
		if err := calculateApplicationScore(
			tx, value.WorkspaceID, project.ID, application.StudentID, calculation, &content,
		); err != nil {
			return false, notifications, err
		}
		application.Score = content.Score
	}
	if _, err := prepareDraftSubmission(
		tx, value.WorkspaceID, id,
		projectModels.ProjectInput{ApprovalSteps: steps}, student.ClassName,
	); err != nil {
		return false, notifications, err
	}
	result := tx.Model(&projectModels.Application{}).
		Where("workspace_id = ? AND project_id = ? AND id = ? AND status = ? AND revision = ?",
			value.WorkspaceID, value.ProjectID, id,
			projectConstants.ApplicationDraft, application.Revision).
		Updates(map[string]any{
			"score":        application.Score,
			"status":       projectConstants.ApplicationPending,
			"current_step": 1,
			"revision":     application.Revision + 1,
		})
	if result.Error != nil {
		return false, notifications, result.Error
	}
	if result.RowsAffected != 1 {
		return false, notifications, errors.New("application changed during auto-submit")
	}
	err = tx.Create(&projectModels.ApplicationHistory{
		WorkspaceID:   value.WorkspaceID,
		ApplicationID: id,
		Operation:     projectConstants.ApplicationSubmit,
		ActorType:     "system",
		ActorName:     "系统",
		Comment:       "截止时间已过，系统自动提交草稿申报",
	}).Error
	if err != nil {
		return false, notifications, err
	}

	// 自动提交同样提醒首级处理人，投递失败时回滚状态和系统流转记录。
	info := projectModels.ApplicationInfo{
		ApplicationSummary: projectModels.ApplicationSummary{
			ID: id, ProjectID: project.ID, ProjectName: project.Name, Year: project.Year,
			StudentID: application.StudentID, StudentName: student.Name,
			StudentNo: student.StudentNo, ClassName: student.ClassName,
			Status: projectConstants.ApplicationPending, CurrentStep: 1,
			Revision: application.Revision + 1,
		},
	}
	nodes, err := applicationStepNodes(tx, value.WorkspaceID, id, 1)
	if err != nil {
		return false, notifications, err
	}
	reviewers, ready, err := nodeReviewers(tx, value.WorkspaceID, info.ApplicationSummary, nodes)
	if err != nil {
		return false, notifications, err
	}
	info.Steps = []projectModels.ApplicationStepInfo{
		{Position: 1, Reviewers: reviewers, Ready: ready},
	}
	notifications, err = notifyApplication(
		tx,
		value.WorkspaceID,
		projectModels.ApplicationInfo{},
		info,
	)
	return err == nil, notifications, err
}

func draftAutoSubmitDue(current *time.Time, scheduled, now time.Time) bool {
	// 截止时间变更后旧任务失效；相等时仍不提交，必须严格晚于当前截止时间。
	return current != nil && current.Equal(scheduled) && now.After(*current)
}
