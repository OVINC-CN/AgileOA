package monthlyworkSvc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

func headers() []string {
	return []string{"日期", "工作项目", "负责人账号", "详细说明", "外部链接(JSON)", "参与人账号"}
}

// Template 返回固定中文表头的导入模板，文件附件在创建记录后上传。
func Template(
	ctx context.Context,
	workspaceID, category string,
) (monthlyworkModels.FileData, error) {
	var result monthlyworkModels.FileData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkImport),
		event(workspaceID, category, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			content, err := spreadsheet.Encode(
				[][]string{headers()},
				"日期、工作项目及负责人账号必填，日期格式为 YYYY-MM-DD。",
				"参与人账号可选，多人使用英文逗号分隔，例如 fictional.first,fictional.second。",
				"负责人和参与人须为当前空间具有进入权限的启用账号；指派不会授予业务权限。",
				"外部链接填写包含 name 和 url 的 JSON 数组，附件在记录创建后上传。",
			)
			if err != nil {
				return err
			}
			result = monthlyworkModels.FileData{
				Filename: "monthly-work.xlsx", ContentType: spreadsheet.ContentType,
				Content: content,
			}
			audit.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

// Import 仅新增记录，非法日期、链接或审计故障导致整批回滚。
func Import(
	ctx context.Context,
	workspaceID, category string,
	request monthlyworkModels.ImportRequest,
) (monthlyworkModels.ImportData, error) {
	var result monthlyworkModels.ImportData
	var createdIDs []uint64
	var notifications []notificationModels.Notification
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkImport),
		event(workspaceID, category, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 严格检查模板、字节数与行数，禁止导入任意列或外部附件地址。
			rows, err := spreadsheet.Read(request.Content, spreadsheet.Limits{
				MaxBytes:   config.Config.MonthlyWork.MaxFileBytes,
				MaxRows:    config.Config.MonthlyWork.MaxImportRows,
				MaxColumns: len(headers()),
			})
			if err != nil {
				return err
			}
			if len(rows) < 2 {
				return invalid("文件中没有工作记录")
			}
			// 旧模板不含参与人列时继续接受，新增列固定追加在末尾。
			columns := headers()
			if !slices.Equal(rows[0], columns) &&
				!slices.Equal(rows[0], columns[:len(columns)-1]) {
				return invalid("文件表头与模板不一致")
			}
			for _, row := range rows[1:] {
				var owner struct{ ID uint64 }
				if err := accessUtils.WorkspaceUsers(
					tx,
					workspaceID,
				).Select("users.id").
					Where("users.username = ?", strings.TrimSpace(row[2])).
					Take(&owner).
					Error; err != nil {
					return invalid("负责人账号不存在、已停用或不能访问当前工作空间")
				}
				input := monthlyworkModels.Input{
					Date:        row[0],
					WorkItem:    row[1],
					OwnerID:     owner.ID,
					Description: row[3],
				}
				if len(row) == len(columns) {
					input.ParticipantIDs, err = importParticipants(
						tx,
						workspaceID,
						row[len(columns)-1],
					)
					if err != nil {
						return err
					}
				}
				if strings.TrimSpace(row[4]) != "" {
					decoder := json.NewDecoder(strings.NewReader(row[4]))
					decoder.DisallowUnknownFields()
					if decoder.Decode(&input.Links) != nil ||
						!errors.Is(decoder.Decode(new(any)), io.EOF) {
						return invalid("外部链接须为包含 name 和 url 的 JSON 数组")
					}
				}
				input, err = clean(input)
				if err != nil {
					return err
				}
				links, err := json.Marshal(input.Links)
				if err != nil {
					return err
				}
				record := monthlyworkModels.Record{
					WorkspaceID: workspaceID,
					Category:    category,
					Date:        input.Date,
					WorkItem:    input.WorkItem,
					OwnerID:     input.OwnerID,
					Description: input.Description,
					Links:       string(links),
					Revision:    1,
				}
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
				if err := syncParticipants(
					tx,
					workspaceID,
					record.ID,
					input.ParticipantIDs,
					nil,
				); err != nil {
					return err
				}
				createdIDs = append(createdIDs, record.ID)
				// 导入同样属于工作分配；整批回滚时通知一并回滚。
				assigned, err := notifyAssignments(
					tx,
					principal,
					record,
					0,
					input.ParticipantIDs,
					nil,
				)
				if err != nil {
					return err
				}
				notifications = append(notifications, assigned...)
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(record.ID, 10))
				result.Created++
			}
			// 导入快照包含参与人身份，工作硬删除后仍可追溯当时的指派。
			if err := auditImportedRecords(tx, workspaceID, category, createdIDs); err != nil {
				return err
			}
			audit.After = auditModels.Snapshot{"created": result.Created}
			audit.ReturnedCount = int64(result.Created)
			return nil
		},
	)
	if err == nil && len(createdIDs) > 0 {
		notificationUtils.LogCreated(ctx, notifications)
		logger.Logger.WithContext(ctx).Infof(
			"Monthly work imported; action: %s; workspace: %s; category: %s; records: %v",
			actionFor(
				category,
				accessConstants.MonthlyWorkImport,
			), workspaceID, category, createdIDs,
		)
	}
	return result, err
}

func auditImportedRecords(
	tx *gorm.DB,
	workspaceID, category string,
	ids []uint64,
) error {
	// 批量提取固定快照，外部地址仍只保留摘要。
	var records []monthlyworkModels.Record
	if err := tx.Where("workspace_id = ? AND category = ? AND id IN ?", workspaceID, category, ids).
		Order("id ASC").Find(&records).Error; err != nil {
		return err
	}
	infos, err := recordInfos(tx, workspaceID, records, false)
	if err != nil {
		return err
	}
	for index, info := range infos {
		item := event(workspaceID, category, info.ID)
		item.Action = actionFor(category, accessConstants.MonthlyWorkImport)
		item.Result = auditConstants.Success
		item.After = snapshot(info)
		item.Conditions["row"] = index + 2
		if err := auditSvc.Write(tx, item); err != nil {
			return err
		}
	}
	return nil
}
