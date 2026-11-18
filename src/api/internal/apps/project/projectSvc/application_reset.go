package projectSvc

import (
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func resetProjectApplications(
	tx *gorm.DB,
	workspaceID string,
	project projectModels.ProjectInfo,
	actor applicationActor,
) ([]uint64, applicationNotifications, error) {
	var notifications applicationNotifications
	ids := []uint64{}
	// 已通过申报保留最终流程和签署，草稿只更新预览及并发版本。
	var applications []projectModels.Application
	if err := tx.Where("workspace_id = ? AND project_id = ? AND status IN ?",
		workspaceID, project.ID, []string{
			projectConstants.ApplicationDraft,
			projectConstants.ApplicationPending,
			projectConstants.ApplicationRejected,
		}).Order("id ASC").Find(&applications).Error; err != nil {
		return nil, notifications, err
	}
	for _, application := range applications {
		if application.Status != projectConstants.ApplicationDraft &&
			len(project.ApprovalSteps) == 0 {
			return nil, notifications, invalid("项目仍有未完成申报，审批流至少保留一级")
		}
	}

	// 共用项目编辑的空间排他锁，逐单重建流程并保留变更前后的有效签署快照。
	for _, application := range applications {
		before, err := applicationWorkflowSnapshot(tx, application)
		if err != nil {
			return nil, notifications, err
		}
		if err := resetApplicationWorkflow(
			tx,
			application,
			project.ApprovalSteps,
			actor,
		); err != nil {
			return nil, notifications, err
		}
		var updated projectModels.Application
		if err := tx.Where("workspace_id = ? AND project_id = ? AND id = ?",
			workspaceID, project.ID, application.ID).Take(&updated).Error; err != nil {
			return nil, notifications, err
		}
		after, err := applicationWorkflowSnapshot(tx, updated)
		if err != nil {
			return nil, notifications, err
		}
		item := event(workspaceID, "project_application", application.ID)
		item.Action, item.Result = accessConstants.ProjectUpdate, auditConstants.Success
		item.Before, item.After = before, after
		if err := auditSvc.Write(tx, item); err != nil {
			return nil, notifications, err
		}
		created, err := notifyApplicationReset(tx, project, updated)
		if err != nil {
			return nil, notifications, err
		}
		notifications.users = append(notifications.users, created.users...)
		notifications.students = append(notifications.students, created.students...)
		ids = append(ids, application.ID)
	}
	return ids, notifications, nil
}

func resetApplicationWorkflow(
	tx *gorm.DB,
	application projectModels.Application,
	steps []projectModels.ApprovalStepInput,
	actor applicationActor,
) error {
	// 清除全部当前签署但不删历史、内容或附件；草稿不提前复制审批节点。
	if err := clearNodeApprovals(tx, application.WorkspaceID, application.ID, 0, true); err != nil {
		return err
	}
	if err := tx.Where("workspace_id = ? AND application_id = ?",
		application.WorkspaceID, application.ID).
		Delete(&projectModels.ApplicationNode{}).Error; err != nil {
		return err
	}
	status, current, configured := projectConstants.ApplicationPending, 1, steps
	if application.Status == projectConstants.ApplicationDraft {
		status, current, configured = projectConstants.ApplicationDraft, 0, nil
	}
	if err := initializeApplicationNodes(
		tx, application.WorkspaceID, application.ID, configured,
	); err != nil {
		return err
	}
	written := tx.Model(&projectModels.Application{}).
		Where("workspace_id = ? AND project_id = ? AND id = ? AND revision = ?",
			application.WorkspaceID, application.ProjectID, application.ID, application.Revision).
		Updates(map[string]any{
			"status": status, "current_step": current, "revision": application.Revision + 1,
		})
	if written.Error != nil {
		return written.Error
	}
	if written.RowsAffected != 1 {
		return stale()
	}
	return applicationHistory(
		tx, application.WorkspaceID, application.ID, application.CurrentStep, actor,
		projectConstants.ApplicationReset, "奖项审批流已更新，未完成申报的审批状态已重置",
	)
}

func applicationWorkflowSnapshot(
	tx *gorm.DB,
	application projectModels.Application,
) (auditModels.Snapshot, error) {
	// 只读取本次实际变更的流程和签署，业务正文、评分及附件保持原值。
	nodes := []projectModels.ApplicationNode{}
	if err := tx.Where("workspace_id = ? AND application_id = ?",
		application.WorkspaceID, application.ID).
		Order("position ASC, target_order ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	approvals := []projectModels.ApplicationApproval{}
	if err := tx.Where("workspace_id = ? AND application_id = ?",
		application.WorkspaceID, application.ID).
		Order("position ASC, actor_type ASC, actor_id ASC").Find(&approvals).Error; err != nil {
		return nil, err
	}
	return auditModels.Snapshot{
		"id": application.ID, "project_id": application.ProjectID,
		"student_id": application.StudentID, "status": application.Status,
		"current_step": application.CurrentStep, "revision": application.Revision,
		"approval_nodes": nodes, "approvals": approvals,
		"created_at": application.CreatedAt, "updated_at": application.UpdatedAt,
	}, nil
}

func notifyApplicationReset(
	tx *gorm.DB,
	project projectModels.ProjectInfo,
	application projectModels.Application,
) (applicationNotifications, error) {
	var empty applicationNotifications
	if application.Status == projectConstants.ApplicationDraft {
		return empty, nil
	}
	var student struct{ ClassName, StudentNo, Name string }
	if err := tx.Table("students").Select("class_name, student_no, name").
		Where("workspace_id = ? AND id = ?", application.WorkspaceID, application.StudentID).
		Take(&student).Error; err != nil {
		return empty, err
	}
	info := projectModels.ApplicationInfo{
		ApplicationSummary: projectModels.ApplicationSummary{
			ID:          application.ID,
			ProjectID:   project.ID,
			ProjectName: project.Name,
			Year:        project.Year,
			StudentID:   application.StudentID,
			StudentName: student.Name,
			StudentNo:   student.StudentNo,
			ClassName:   student.ClassName,
			Status:      application.Status,
			CurrentStep: application.CurrentStep,
			Revision:    application.Revision,
		},
	}
	nodes, err := applicationStepNodes(tx, application.WorkspaceID, application.ID, 1)
	if err != nil {
		return empty, err
	}
	reviewers, ready, err := nodeReviewers(
		tx,
		application.WorkspaceID,
		info.ApplicationSummary,
		nodes,
	)
	if err != nil {
		return empty, err
	}
	info.Steps = []projectModels.ApplicationStepInfo{
		{Position: 1, Reviewers: reviewers, Ready: ready},
	}
	// 重置开启新的审批轮次，即使首级处理人未变也重新提醒，按新版本幂等。
	return notifyApplication(tx, application.WorkspaceID, projectModels.ApplicationInfo{}, info)
}
