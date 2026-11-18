package cpcpersonSvc

import (
	"context"
	"slices"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
)

func within(
	ctx context.Context, workspaceID, action string, audit auditModels.Event,
	operation func(*gorm.DB, personAccess, *auditModels.Event) error,
) error {
	return accessUtils.Workspace(ctx, workspaceID, action, audit,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			access, err := loadAccess(tx, principal)
			if err != nil {
				return err
			}
			// 细粒度权限已在空间事务内复核，读取时由查询条件将空范围转换为空结果。
			return operation(tx, access, audit)
		})
}

type personAccess struct {
	actions   []string
	branchIDs []uint64
}

func loadAccess(tx *gorm.DB, principal accessModels.Principal) (personAccess, error) {
	// 调用方已持有空间锁，书记及授权修改也使用同一把锁。
	var result personAccess
	var err error
	result.actions, err = accessUtils.Actions(tx, principal.User.ID, principal.Workspace.ID)
	if err != nil {
		return result, err
	}
	err = tx.Table("cpc_party_branches").
		Where("workspace_id = ? AND secretary_id = ?", principal.Workspace.ID, principal.User.ID).
		Order("id ASC").Pluck("id", &result.branchIDs).Error
	return result, err
}

// scopeAction 仅扩大对应操作的数据范围，不替代人员细粒度权限。
func scopeAction(action string) string {
	switch action {
	case accessConstants.CPCPersonCreate, accessConstants.CPCPersonUpdate,
		accessConstants.CPCPersonImport, accessConstants.CPCPersonTimelineTransferReview:
		return accessConstants.CPCPartyBranchUpdate
	case accessConstants.CPCPersonDelete:
		return accessConstants.CPCPartyBranchDelete
	default:
		return accessConstants.CPCPartyBranchList
	}
}

func (a personAccess) all(action string) bool {
	return slices.Contains(a.actions, scopeAction(action))
}

func (a personAccess) hasScope(action string) bool {
	return a.all(action) || len(a.branchIDs) > 0
}

func (a personAccess) require(action string) error {
	if !slices.Contains(a.actions, action) || !a.hasScope(action) {
		return accessUtils.Denied()
	}
	return nil
}

func (a personAccess) permits(action string, branchID *uint64) bool {
	if branchID == nil || *branchID == 0 {
		return false
	}
	return slices.Contains(a.actions, action) && (a.all(action) ||
		slices.Contains(a.branchIDs, *branchID))
}

func (a personAccess) limit(query *gorm.DB, action, column string) *gorm.DB {
	if a.all(action) {
		return query
	}
	if len(a.branchIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(column+" IN ?", a.branchIDs)
}
