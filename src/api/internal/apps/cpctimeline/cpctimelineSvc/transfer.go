package cpctimelineSvc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// TransferTargets 按新增或编辑用途提供同空间目标模板目录，不推导详情权限。
func TransferTargets(
	ctx context.Context, workspaceID, action, keyword string, id uint64,
) ([]cpctimelineModels.TransferTarget, error) {
	result := []cpctimelineModels.TransferTarget{}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 用途及筛选均限定在固定协议内。
			if action != accessConstants.CPCTimelineCreate &&
				action != accessConstants.CPCTimelineUpdate {
				return invalid("转移模板目录用途无效")
			}
			keyword = strings.TrimSpace(keyword)
			if !validText(keyword, 100, false, false) {
				return invalid("模板搜索条件无效")
			}
			scope := tx.Where("workspace_id = ?", principal.Workspace.ID)
			if id != 0 {
				scope = scope.Where("id = ?", id)
			}
			if keyword != "" {
				scope = scope.Where("LOCATE(?, name) > 0", keyword)
			}
			var templates []cpctimelineModels.Template
			if err := scope.Order("id DESC").Limit(config.Config.CPCPerson.MaxOptions + 1).
				Find(&templates).Error; err != nil {
				return err
			}
			if len(templates) > config.Config.CPCPerson.MaxOptions {
				return invalid("目标模板较多，请输入名称搜索")
			}
			for _, template := range templates {
				nodes, err := cpctimelineUtils.LoadNodes(tx, principal.Workspace.ID, template.ID)
				if err != nil {
					return err
				}
				result = append(result, cpctimelineModels.TransferTarget{
					ID: template.ID, Name: template.Name,
					Definition: cpctimelineModels.Definition{Nodes: nodes},
				})
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(template.ID, 10))
			}
			audit.Conditions["keyword"], audit.Conditions["template_id"] = keyword, id
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

func saveTransfers(
	tx *gorm.DB, workspaceID string, templateID uint64, input cpctimelineModels.Input,
) error {
	// 同时校验来源和目标的最新定义，目标模板变更不能破坏其他模板的已有映射。
	for _, transfer := range input.Transfers {
		if transfer.TargetTemplateID == templateID {
			return invalid("转移目标不能是当前模板")
		}
		target, err := load(tx, workspaceID, transfer.TargetTemplateID)
		if err != nil {
			return err
		}
		if err := cpctimelineUtils.ValidateTransfer(
			input.Definition.Nodes, target.Definition.Nodes, transfer,
		); err != nil {
			return invalid(err.Error())
		}
	}
	var incoming []cpctimelineModels.TemplateTransfer
	if err := tx.Where("workspace_id = ? AND target_template_id = ?", workspaceID, templateID).
		Find(&incoming).Error; err != nil {
		return err
	}
	for _, record := range incoming {
		source, err := cpctimelineUtils.LoadNodes(tx, workspaceID, record.SourceTemplateID)
		if err != nil {
			return err
		}
		transfer := cpctimelineModels.Transfer{TargetTemplateID: templateID}
		if err := json.Unmarshal([]byte(record.NodeMappings), &transfer.NodeMappings); err != nil {
			return err
		}
		if err := cpctimelineUtils.ValidateTransfer(
			source,
			input.Definition.Nodes,
			transfer,
		); err != nil {
			return invalid("节点或字段仍被其他模板的转移配置引用，请先调整相关配置")
		}
	}

	// 显式替换模板自有配置，目标关系由同租户物理外键保护。
	if err := tx.Where("workspace_id = ? AND source_template_id = ?", workspaceID, templateID).
		Delete(&cpctimelineModels.TemplateTransfer{}).Error; err != nil {
		return err
	}
	for index, transfer := range input.Transfers {
		encoded, err := json.Marshal(transfer.NodeMappings)
		if err != nil {
			return err
		}
		if err := tx.Create(&cpctimelineModels.TemplateTransfer{
			WorkspaceID:      workspaceID,
			SourceTemplateID: templateID,
			TargetTemplateID: transfer.TargetTemplateID,
			NodeMappings:     string(encoded),
			Position:         index + 1,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}
