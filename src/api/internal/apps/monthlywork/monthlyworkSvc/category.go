package monthlyworkSvc

import (
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkConstants"
)

func actionFor(category, action string) string {
	// 仅允许固定类别的独立操作，未知类别交由授权层默认拒绝。
	if category == monthlyworkConstants.CategoryStudent {
		return action
	}
	if category == monthlyworkConstants.CategoryCPC {
		switch action {
		case accessConstants.MonthlyWorkList:
			return accessConstants.CPCMonthlyWorkList
		case accessConstants.MonthlyWorkCreate:
			return accessConstants.CPCMonthlyWorkCreate
		case accessConstants.MonthlyWorkUpdate:
			return accessConstants.CPCMonthlyWorkUpdate
		case accessConstants.MonthlyWorkDelete:
			return accessConstants.CPCMonthlyWorkDelete
		case accessConstants.MonthlyWorkImport:
			return accessConstants.CPCMonthlyWorkImport
		case accessConstants.MonthlyWorkAttachmentRead:
			return accessConstants.CPCMonthlyWorkAttachmentRead
		}
	}
	return ""
}
