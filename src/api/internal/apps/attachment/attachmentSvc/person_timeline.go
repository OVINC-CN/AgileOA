package attachmentSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// PersonTimelineDownload 只允许下载可操作人员的当前节点文件，不扩大模板附件管理权限。
func PersonTimelineDownload(
	ctx context.Context, workspaceID string, personID uint64, nodeID string,
	fileID uint64, templateFile, editing bool,
) (attachmentModels.URLData, error) {
	var result attachmentModels.URLData
	action, category := accessConstants.CPCPersonDetail, attachmentConstants.CategoryCPCPersonTimelineNode
	if editing {
		action = accessConstants.CPCPersonUpdate
	}
	if templateFile {
		category = attachmentConstants.CategoryCPCTimelineNode
	}
	err := accessUtils.Workspace(ctx, workspaceID, action,
		attachmentEvent(workspaceID, category, fileID),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 实时复核支部范围和绑定关系，不能凭文件编号读取其他人员或模板的附件。
			templateID, err := cpcpersonSvc.RequireTimelineNode(
				tx,
				principal,
				personID,
				nodeID,
				action,
			)
			if err != nil {
				return err
			}
			business, err := policyFor(category)
			if err != nil {
				return err
			}
			ownerID := personID
			if templateFile {
				ownerID = templateID
			}
			var record attachmentModels.OwnedRecord
			err = business.scope(tx, principal, action).Select(business.ownerSelection()).
				Where("id = ? AND "+business.ownerColumn+" = ? AND node_id = ?", fileID, ownerID, nodeID).
				Take(&record).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return failure(http.StatusNotFound, "附件不存在或不可访问")
			}
			if err != nil {
				return err
			}
			if record.State != attachmentConstants.Ready {
				return failure(http.StatusConflict, "文件尚未完成或正在删除")
			}
			if !storage.Enabled() {
				return unavailable()
			}
			result.Filename = record.Filename
			result.URL = storage.SignedCDNURL(tx.Statement.Context, record.ObjectKey)
			audit.Conditions["person_id"], audit.Conditions["node_id"] = personID, nodeID
			audit.ReturnedIDs = []string{strconv.FormatUint(fileID, 10)}
			audit.ReturnedCount = 1
			return nil
		})
	return result, err
}
