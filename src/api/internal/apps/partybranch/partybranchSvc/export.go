package partybranchSvc

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
)

// Export 按列表筛选及排序导出党支部，在空间锁内复核独立权限并保存审计。
func Export(
	ctx context.Context,
	workspaceID string,
	query partybranchModels.Query,
) (partybranchModels.FileData, error) {
	// 导出使用已提交的关键词，不受列表分页影响。
	query.Keyword = strings.TrimSpace(query.Keyword)
	auditEvent := event(workspaceID, 0)
	auditEvent.Conditions["keyword"] = query.Keyword
	var result partybranchModels.FileData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.CPCPartyBranchExport, auditEvent,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 查询限定当前租户，超过行数上限时拒绝整个文件。
			scope, err := filteredScope(tx, principal.Workspace.ID, query.Keyword)
			if err != nil {
				return err
			}
			var items []partybranchModels.Info
			if err := scope.Order(partybranchConstants.ListOrder).
				Limit(config.Config.PartyBranch.MaxExportRows + 1).Find(&items).Error; err != nil {
				return err
			}
			result, err = exportFile(items)
			if err != nil {
				return err
			}

			// 读取结果与审计共用事务，审计提交后才返回文件。
			for _, item := range items {
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.ID, 10))
			}
			audit.ReturnedCount = int64(len(items))
			return nil
		},
	)
	return result, err
}

func exportFile(items []partybranchModels.Info) (partybranchModels.FileData, error) {
	// 表头和数据按文本保存，零优先级及空管理员均保留原值。
	if len(items) > config.Config.PartyBranch.MaxExportRows {
		return partybranchModels.FileData{}, invalid("导出行数超过限制，请缩小筛选范围")
	}
	records := make([][]string, 1, len(items)+1)
	records[0] = []string{"优先级", "党支部简称", "党支部全称", "管理员姓名", "管理员账号"}
	for _, item := range items {
		records = append(records, []string{
			strconv.FormatInt(int64(item.Priority), 10), item.ShortName, item.FullName,
			item.SecretaryName, item.SecretaryUsername,
		})
	}
	content, err := spreadsheet.Encode(records)
	if err != nil {
		return partybranchModels.FileData{}, err
	}
	fileSize := base64.RawStdEncoding.DecodedLen(len(strings.TrimRight(content, "=")))
	if fileSize > config.Config.PartyBranch.MaxFileBytes {
		return partybranchModels.FileData{}, invalid("导出文件超过限制，请缩小筛选范围")
	}

	// 文件沿用通用的 Base64 XLSX 协议。
	return partybranchModels.FileData{
		Filename: "party-branches.xlsx", ContentType: spreadsheet.ContentType, Content: content,
	}, nil
}
