package partybranchSvc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
)

func headers() []string {
	return []string{"党支部全称", "党支部简称", "党支部书记账号", "优先级"}
}

// Template 提供固定中文 XLSX 表头，编号由数据库自动生成。
func Template(ctx context.Context, workspaceID string) (partybranchModels.FileData, error) {
	var result partybranchModels.FileData
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchImport,
		event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			content, err := spreadsheet.Encode([][]string{headers()})
			if err != nil {
				return err
			}
			result = partybranchModels.FileData{
				Filename:    "party-branches.xlsx",
				ContentType: spreadsheet.ContentType,
				Content:     content,
			}
			audit.ReturnedCount = 1
			return nil
		})
	return result, err
}

// Import 整批新增党支部，按账号匹配书记，任一行或审计失败均回滚。
func Import(
	ctx context.Context,
	workspaceID string,
	request partybranchModels.ImportRequest,
) (partybranchModels.ImportData, error) {
	var result partybranchModels.ImportData
	ids := []string{}
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchImport,
		event(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验固定表头及配置中的导入边界，拒绝任意列或预设编号。
			rows, err := workbookRows(request.Content)
			if err != nil {
				return err
			}
			for index, row := range rows {
				input := partybranchModels.Input{
					FullName: row[0], ShortName: row[1],
				}
				username := strings.TrimSpace(row[2])
				if username != "" {
					var user struct{ ID uint64 }
					err := accessUtils.WorkspaceUsers(
						tx,
						principal.Workspace.ID,
					).Select("users.id").
						Where("users.username = ?", username).Take(&user).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return invalid(fmt.Sprintf("第 %d 行：书记账号不存在、已停用或不能访问当前空间", index+2))
					}
					if err != nil {
						return err
					}
					input.SecretaryID = &user.ID
				}
				input, err = clean(input)
				if err != nil {
					return err
				}
				priority := int64(0)
				if value := strings.TrimSpace(row[3]); value != "" {
					priority, err = strconv.ParseInt(value, 10, 32)
					if err != nil {
						return invalid(fmt.Sprintf("第 %d 行：优先级必须为 32 位整数", index+2))
					}
				}

				// 逐条保存业务快照，所有事件复用同一请求和事务。
				record := newRecord(principal.Workspace.ID, input)
				record.Priority = int32(priority)
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
				info, err := detail(tx, principal.Workspace.ID, record.ID)
				if err != nil {
					return err
				}
				rowAudit := event(principal.Workspace.ID, record.ID)
				rowAudit.Action = accessConstants.CPCPartyBranchImport
				rowAudit.Result = auditConstants.Success
				rowAudit.After = snapshot(info)
				if err := auditSvc.Write(tx, rowAudit); err != nil {
					return err
				}
				ids = append(ids, strconv.FormatUint(record.ID, 10))
				result.Created++
			}
			audit.ReturnedIDs, audit.ReturnedCount = ids, int64(result.Created)
			audit.After = auditModels.Snapshot{"created": result.Created}
			return nil
		})
	if err != nil {
		return partybranchModels.ImportData{}, err
	}

	// 只在业务与全部审计提交后记录实际新增对象。
	logger.Logger.WithContext(ctx).Infof(
		"Party branches imported; action: %s; workspace: %s; branches: %s",
		accessConstants.CPCPartyBranchImport, workspaceID, strings.Join(ids, ","),
	)
	return result, nil
}

func workbookRows(content string) ([][]string, error) {
	rows, err := spreadsheet.Read(content, spreadsheet.Limits{
		MaxBytes:   config.Config.PartyBranch.MaxFileBytes,
		MaxRows:    config.Config.PartyBranch.MaxImportRows,
		MaxColumns: len(headers()),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, invalid("文件中没有党支部记录")
	}
	if !slices.Equal(rows[0], headers()) && !slices.Equal(rows[0], headers()[:3]) {
		return nil, invalid("文件表头与模板不一致")
	}
	for index := 1; index < len(rows); index++ {
		if len(rows[index]) != len(rows[0]) {
			return nil, invalid("文件列数与表头不一致")
		}
		if len(rows[index]) == 3 {
			// 兼容既有三列模板，未提供优先级时沿用默认零值。
			rows[index] = append(rows[index], "")
		}
	}
	return rows[1:], nil
}
