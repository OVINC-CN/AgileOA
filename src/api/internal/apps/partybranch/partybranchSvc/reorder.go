package partybranchSvc

import (
	"context"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonTask"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// ReorderPeople 在空间锁内复核党支部编辑权限，仅提交后台任务，不在请求中修改归属。
func ReorderPeople(
	ctx context.Context,
	workspaceID string,
) (partybranchModels.ReorderData, error) {
	var result partybranchModels.ReorderData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.CPCPartyBranchUpdate,
		event(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 入队时仍持有空间锁，后台取得同一把锁后读取最新已提交规则。
			audit.Conditions["operation"] = "reorder_people"
			var err error
			result.Queued, err = cpcpersonTask.Enqueue(
				tx, workspaceID, principal.User.ID, principal.Workspace.Name,
			)
			audit.Conditions["queued"] = result.Queued
			return err
		},
	)

	// 审计提交后记录成功入队；实际归属变更由后台事务记录。
	if err == nil && result.Queued {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person reorder task queued; action: %s; workspace: %s",
			accessConstants.CPCPartyBranchUpdate,
			workspaceID,
		)
	}
	return result, err
}
