package projectSvc

import (
	"slices"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func validateApplicationQuery(query projectModels.ApplicationQuery) error {
	// 列表与导出共用固定筛选目录和参数边界。
	if err := validQuery(projectModels.Query{
		Student: query.Student, ProjectNames: query.ProjectNames, Years: query.Years,
	}); err != nil {
		return err
	}
	allowed := []string{
		projectConstants.ApplicationDraft,
		projectConstants.ApplicationPending,
		projectConstants.ApplicationRejected,
		projectConstants.ApplicationApproved,
		projectConstants.ApplicationPendingMine,
	}
	for _, status := range query.Statuses {
		if !slices.Contains(allowed, status) {
			return invalid("审批状态筛选无效")
		}
	}
	return nil
}

func applicationFilterSnapshot(
	query projectModels.ApplicationQuery,
	mode string,
) auditModels.Snapshot {
	return auditModels.Snapshot{
		"student":      query.Student,
		"project_name": query.ProjectNames,
		"year":         query.Years,
		"status":       query.Statuses,
		"view":         mode,
	}
}

func applicationQueryScope(
	tx *gorm.DB,
	workspaceID, mode string,
	actor applicationActor,
	query projectModels.ApplicationQuery,
) *gorm.DB {
	// 先限制当前租户和可见单据，再应用同一组已提交筛选。
	scope := applicationScope(tx, workspaceID, mode, actor)
	if query.Student != "" {
		scope = scope.Where(
			"LOCATE(?, s.student_no) > 0 OR LOCATE(?, s.name) > 0",
			query.Student,
			query.Student,
		)
	}
	if len(query.ProjectNames) > 0 {
		scope = scope.Where("p.name IN ?", query.ProjectNames)
	}
	if len(query.Years) > 0 {
		scope = scope.Where("p.year IN ?", query.Years)
	}
	if len(query.Statuses) > 0 {
		states := []string{}
		for _, status := range query.Statuses {
			if status != projectConstants.ApplicationPendingMine {
				states = append(states, status)
			}
		}
		condition := tx.Where("1 = 0")
		if len(states) > 0 {
			condition = condition.Or("a.status IN ?", states)
		}
		if slices.Contains(query.Statuses, projectConstants.ApplicationPendingMine) {
			nodes := tx.Table("project_application_nodes AS n").Select("1").
				Where("n.workspace_id = ? AND n.application_id = a.id AND n.position = a.current_step", workspaceID).
				Where(applicationNodeActorScope(tx, actor))
			kind, actorID := actorIdentity(actor)
			votes := tx.Model(&projectModels.ApplicationApproval{}).Select("1").
				Where("workspace_id = ? AND application_id = a.id AND position = a.current_step AND actor_type = ? AND actor_id = ?", workspaceID, kind, actorID)
			condition = condition.Or(
				"a.status IN ? AND EXISTS (?) AND NOT EXISTS (?)",
				[]string{
					projectConstants.ApplicationPending,
					projectConstants.ApplicationRejected,
				},
				nodes,
				votes,
			)
		}
		scope = scope.Where(condition)
	}
	return scope
}
