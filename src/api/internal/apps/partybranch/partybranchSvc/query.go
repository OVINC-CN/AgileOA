package partybranchSvc

import (
	"context"
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
)

// List 分页查询当前空间的党支部，审计持久化后返回。
func List(
	ctx context.Context,
	workspaceID string,
	query partybranchModels.Query,
) (partybranchModels.ListData, error) {
	result := partybranchModels.ListData{
		Items: []partybranchModels.Info{}, Page: query.Page, PageSize: query.PageSize,
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	auditEvent := event(workspaceID, 0)
	auditEvent.Conditions["keyword"] = query.Keyword
	auditEvent.Conditions["page"] = query.Page
	auditEvent.Conditions["page_size"] = query.PageSize
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchList,
		auditEvent,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 列表与导出共用固定搜索字段及校验。
			scope, err := filteredScope(tx, principal.Workspace.ID, query.Keyword)
			if err != nil {
				return err
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			if err := scope.Order(partybranchConstants.ListOrder).Limit(query.PageSize).
				Offset((query.Page - 1) * query.PageSize).Find(&result.Items).Error; err != nil {
				return err
			}

			// 保存当前筛选及实际返回对象，读取不绕过审计。
			for _, item := range result.Items {
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.ID, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

func filteredScope(tx *gorm.DB, workspaceID, keyword string) (*gorm.DB, error) {
	// 搜索仅覆盖固定目录中的名称和关联书记账号。
	keyword = strings.TrimSpace(keyword)
	if !validText(keyword, partybranchConstants.KeywordLength) {
		return nil, invalid("搜索关键词最多 200 字符，且不能包含控制字符")
	}
	scope := infoScope(tx, workspaceID)
	if keyword != "" {
		scope = scope.Where(
			"(LOCATE(?, b.full_name) > 0 OR LOCATE(?, b.short_name) > 0 OR "+
				"LOCATE(?, u.display_name) > 0 OR LOCATE(?, u.username) > 0)",
			keyword, keyword, keyword, keyword,
		)
	}
	return scope, nil
}

func infoScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return tx.Table("cpc_party_branches AS b").
		Select("b.id, b.full_name, b.short_name, b.priority, b.secretary_id, b.revision, "+
			"b.created_at, b.updated_at, COALESCE(u.display_name, '') AS secretary_name, "+
			"COALESCE(u.username, '') AS secretary_username").
		Joins("LEFT JOIN users AS u ON u.id = b.secretary_id").
		Where("b.workspace_id = ?", workspaceID)
}

// Secretaries 枚举当前工作空间可用的用户，仅返回编号及显示名称。
func Secretaries(
	ctx context.Context,
	workspaceID string,
) ([]partybranchModels.SecretaryOption, error) {
	result := []partybranchModels.SecretaryOption{}
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchList,
		event(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 用户表为全局账号，候选项限定为当前空间具备进入权限的启用账号。
			var users []struct {
				ID                    uint64
				DisplayName, Username string
			}
			if err := accessUtils.WorkspaceUsers(tx, principal.Workspace.ID).
				Select("users.id", "users.display_name", "users.username").
				Order("users.display_name ASC, users.id ASC").
				Limit(config.Config.PartyBranch.MaxOptions + 1).Find(&users).Error; err != nil {
				return err
			}
			if len(users) > config.Config.PartyBranch.MaxOptions {
				return invalid("书记用户选项超过限制，请联系管理员")
			}
			for _, user := range users {
				result = append(result, partybranchModels.SecretaryOption{
					Value: user.ID, Label: user.DisplayName + "(" + user.Username + ")",
				})
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(user.ID, 10))
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

func validateSecretary(tx *gorm.DB, workspaceID string, id uint64) error {
	var count int64
	if err := accessUtils.WorkspaceUsers(
		tx,
		workspaceID,
	).Where("users.id = ?", id).
		Count(&count).
		Error; err != nil {
		return err
	}
	if count != 1 {
		return invalid("党支部书记须为当前工作空间可用的用户")
	}
	return nil
}

// Fields 提供党支部固定字段目录及中文说明，复用列表权限。
func Fields(ctx context.Context, workspaceID string) ([]partybranchModels.Field, error) {
	var result []partybranchModels.Field
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchList,
		event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			result = append([]partybranchModels.Field{}, partybranchConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}
