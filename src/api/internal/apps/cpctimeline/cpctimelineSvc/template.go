package cpctimelineSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Save 在空间锁内整体保存模板，写入与审计共用事务。
func Save(
	ctx context.Context,
	workspaceID string,
	id uint64,
	request cpctimelineModels.Request,
) (cpctimelineModels.Info, error) {
	action := accessConstants.CPCTimelineCreate
	if id != 0 {
		action = accessConstants.CPCTimelineUpdate
	}
	return save(ctx, workspaceID, action, id, request)
}

// Import 仅按独立导入权限新增一个模板，不覆盖现有记录。
func Import(
	ctx context.Context,
	workspaceID string,
	input cpctimelineModels.Input,
) (cpctimelineModels.Info, error) {
	return save(ctx, workspaceID, accessConstants.CPCTimelineImport, 0,
		cpctimelineModels.Request{Input: input})
}

func save(
	ctx context.Context,
	workspaceID, action string,
	id uint64,
	request cpctimelineModels.Request,
) (cpctimelineModels.Info, error) {
	var result cpctimelineModels.Info
	var boundPersonIDs []uint64
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验协议结构及并发版本，不校验任何人员发生时间。
			input, err := prepare(request.Input)
			if err != nil {
				return err
			}
			record := cpctimelineModels.Template{
				WorkspaceID: principal.Workspace.ID,
				Name:        input.Name, Description: input.Description,
				Revision: 1,
			}
			if id == 0 {
				if request.Revision != 0 {
					return invalid("新增模板参数无效")
				}
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
			} else {
				before, err := load(tx, principal.Workspace.ID, id)
				if err != nil {
					return err
				}
				if request.Revision != before.Revision {
					return stale()
				}
				// 避免模板同步在待审核期间改写已锁定的人员节点或转移映射。
				var transferring int64
				if err := tx.Model(&cpcpersonModels.Person{}).
					Where("workspace_id = ? AND timeline_status = ?",
						workspaceID, cpcpersonUtils.TimelineTransferring).
					Where("timeline_template_id = ? OR id IN (?)", id,
						tx.Model(&cpcpersonModels.TimelineTransferReview{}).Select("person_id").
							Where("workspace_id = ? AND target_template_id = ?", workspaceID, id)).
					Count(&transferring).Error; err != nil {
					return err
				}
				if transferring > 0 {
					return invalid("此模板关联的人员时间线正在转移中，请先完成或取消转移")
				}
				// 节点删除将影响所有已绑定人员，接口也必须显式确认。
				if !request.ConfirmNodeDeletion &&
					hasRemovedNodes(before.Definition.Nodes, input.Definition.Nodes) {
					return invalid("删除节点将同步删除所有绑定人员的对应节点及填写内容，请明确确认后重试")
				}
				audit.Before = snapshot(before)
				record.ID, record.Revision = id, before.Revision+1

				// 更新限定空间及版本，外键和唯一约束提供额外写入保护。
				written := tx.Model(&cpctimelineModels.Template{}).
					Where("workspace_id = ? AND id = ? AND revision = ?",
						principal.Workspace.ID, id, request.Revision).
					Updates(map[string]any{
						"name": record.Name, "description": record.Description,
						"revision": record.Revision,
					})
				if written.Error != nil {
					return writeError(written.Error)
				}
				if written.RowsAffected != 1 {
					return stale()
				}
			}

			// 节点定义及顺序与模板资料统一提交，有附件的节点由复合外键保护。
			if err := saveTransfers(tx, principal.Workspace.ID, record.ID, input); err != nil {
				return err
			}
			if err := syncNodes(
				tx,
				principal.Workspace.ID,
				record.ID,
				input.Definition.Nodes,
			); err != nil {
				return err
			}

			if err := tx.Table("cpc_people").Where("workspace_id = ? AND timeline_template_id = ?",
				principal.Workspace.ID, record.ID).
				Order("id ASC").Pluck("id", &boundPersonIDs).Error; err != nil {
				return err
			}
			// 返回保存后的完整定义，审计保留稳定标识及节点顺序。
			result, err = load(tx, principal.Workspace.ID, record.ID)
			if err != nil {
				return err
			}
			audit.ObjectID = strconv.FormatUint(record.ID, 10)
			audit.After = snapshot(result)
			return nil
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"CPC timeline template saved; action: %s; workspace: %s; template: %d; bound people: %v",
			action, workspaceID, result.ID, boundPersonIDs,
		)
	}
	return result, err
}

// Delete 已绑定人员时拒绝删除模板，其余节点、附件和转移配置按外键级联删除。
func Delete(ctx context.Context, workspaceID string, id, revision uint64) error {
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCTimelineDelete,
		event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验租户和版本，删除前保留节点及自有转移配置的完整快照。
			before, err := load(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			audit.Before = snapshot(before)
			var bound int64
			if err := tx.Table("cpc_people").Where("workspace_id = ? AND timeline_template_id = ?",
				principal.Workspace.ID, id).Count(&bound).Error; err != nil {
				return err
			}
			if bound > 0 {
				return invalid("模板已绑定人员，不能删除")
			}

			// 节点、节点附件记录和来源或目标转移配置随模板级联删除。
			return writeError(tx.Where("workspace_id = ? AND id = ?", principal.Workspace.ID, id).
				Delete(&cpctimelineModels.Template{}).Error)
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"CPC timeline template deleted; action: %s; workspace: %s; template: %d",
			accessConstants.CPCTimelineDelete, workspaceID, id,
		)
	}
	return err
}

func load(tx *gorm.DB, workspaceID string, id uint64) (cpctimelineModels.Info, error) {
	var record cpctimelineModels.Template
	if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
		Take(&record).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cpctimelineModels.Info{}, &response.BusinessError{
				Status: http.StatusNotFound, Message: "时间线模板不存在或不可访问",
			}
		}
		return cpctimelineModels.Info{}, err
	}
	nodes, err := cpctimelineUtils.LoadNodes(tx, workspaceID, id)
	if err != nil {
		return cpctimelineModels.Info{}, err
	}
	transfers, err := cpctimelineUtils.LoadTransfers(tx, workspaceID, id)
	if err != nil {
		return cpctimelineModels.Info{}, err
	}
	result := info(record, cpctimelineModels.Definition{Nodes: nodes})
	result.Transfers = transfers
	return result, nil
}

func info(
	record cpctimelineModels.Template,
	definition cpctimelineModels.Definition,
) cpctimelineModels.Info {
	return cpctimelineModels.Info{
		Summary: cpctimelineModels.Summary{
			ID: record.ID, Name: record.Name, Description: record.Description,
			NodeCount: len(definition.Nodes), Revision: record.Revision,
			CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		},
		Definition: definition,
	}
}

func snapshot(value cpctimelineModels.Info) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id": value.ID, "name": value.Name, "description": value.Description,
		"definition": value.Definition, "transfers": value.Transfers, "revision": value.Revision,
	}
}

func event(workspaceID string, id uint64) auditModels.Event {
	result := auditModels.Event{
		TenantID: &workspaceID, ObjectType: "cpc_timeline_template",
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID},
	}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "时间线模板已被修改，请刷新后重试"}
}

func writeError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		switch database.Number {
		case 1062:
			return &response.BusinessError{Status: http.StatusConflict, Message: "当前空间已存在同名时间线模板"}
		case 1451:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "删除模板失败，请检查关联业务后重试",
			}
		case 1452:
			return invalid("关联记录不存在或不可访问")
		}
	}
	return err
}

func hasRemovedNodes(before, after []cpctimelineModels.Node) bool {
	kept := make(map[string]bool, len(after))
	for _, node := range after {
		kept[node.ID] = true
	}
	for _, node := range before {
		if !kept[node.ID] {
			return true
		}
	}
	return false
}
