package partybranchSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Save 在工作空间锁内复核授权，保存党支部及同事务审计。
func Save(
	ctx context.Context,
	workspaceID string,
	id uint64,
	request partybranchModels.Request,
) (partybranchModels.Info, error) {
	var result partybranchModels.Info
	action := accessConstants.CPCPartyBranchCreate
	if id != 0 {
		action = accessConstants.CPCPartyBranchUpdate
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验固定字段及当前版本，原书记停用后仍允许保留或显式清空。
			input, err := clean(request.Input)
			if err != nil {
				return err
			}
			record := newRecord(principal.Workspace.ID, input)
			var previousSecretary *uint64
			if id != 0 {
				before, err := detail(tx, principal.Workspace.ID, id)
				if err != nil {
					return err
				}
				if request.Revision != before.Revision {
					return stale()
				}
				audit.Before = snapshot(before)
				previousSecretary = before.SecretaryID
				record.Priority = before.Priority
				record.ID, record.Revision = id, before.Revision+1
			}
			if request.Priority != nil {
				record.Priority = *request.Priority
			}
			var rule *partybranchModels.AdmissionRuleNode
			if request.AdmissionRule != nil {
				// 基础信息和规则先统一校验，再在同一事务中保存。
				rule, record.AdmissionRule, err = prepareAdmissionRule(request.AdmissionRule.Rule)
				if err != nil {
					return err
				}
				if id != 0 {
					before, err := admissionRuleDetail(tx, principal.Workspace.ID, id)
					if err != nil {
						return err
					}
					audit.Before["admission_rule"] = before.Rule
				}
			}
			if input.SecretaryID != nil &&
				(previousSecretary == nil || *input.SecretaryID != *previousSecretary) {
				if err := validateSecretary(
					tx,
					principal.Workspace.ID,
					*input.SecretaryID,
				); err != nil {
					return err
				}
			}

			// 创建及更新均限制当前租户；外键保护书记账号引用。
			if id == 0 {
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
			} else {
				updates := map[string]any{
					"full_name":    record.FullName,
					"short_name":   record.ShortName,
					"priority":     record.Priority,
					"secretary_id": record.SecretaryID,
					"revision":     record.Revision,
				}
				if request.AdmissionRule != nil {
					updates["admission_rule"] = record.AdmissionRule
				}
				if err := tx.Model(&partybranchModels.Branch{}).
					Where("workspace_id = ? AND id = ?", principal.Workspace.ID, id).
					Updates(updates).Error; err != nil {
					return writeError(err)
				}
			}

			// 返回已保存记录，审计保存字段及书记身份快照。
			result, err = detail(tx, principal.Workspace.ID, record.ID)
			if err != nil {
				return err
			}
			audit.ObjectID = strconv.FormatUint(record.ID, 10)
			audit.After = snapshot(result)
			if request.AdmissionRule != nil {
				audit.After["admission_rule"] = rule
			}
			return nil
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Party branch saved; action: %s; workspace: %s; branch: %d",
			action, workspaceID, result.ID,
		)
	}
	return result, err
}

// Delete 显式硬删除当前空间党支部，保留历史审计和书记身份快照。
func Delete(ctx context.Context, workspaceID string, id, revision uint64) error {
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPartyBranchDelete,
		event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验目标所属空间及并发版本。
			before, err := detail(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}

			// 所属准入规则、党建人员及相关时间线记录随支部级联删除。
			audit.Before = snapshot(before)
			rule, err := admissionRuleDetail(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			audit.Before["admission_rule"] = rule.Rule
			return writeError(tx.Where("workspace_id = ? AND id = ?", principal.Workspace.ID, id).
				Delete(&partybranchModels.Branch{}).Error)
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Party branch deleted; action: %s; workspace: %s; branch: %d",
			accessConstants.CPCPartyBranchDelete, workspaceID, id,
		)
	}
	return err
}

func clean(input partybranchModels.Input) (partybranchModels.Input, error) {
	// 固定文本字段去除首尾空格，不接受空值、控制字符及超长文本。
	input.FullName = strings.TrimSpace(input.FullName)
	input.ShortName = strings.TrimSpace(input.ShortName)
	for _, field := range []struct {
		value, label string
		limit        int
	}{
		{input.FullName, "党支部全称", partybranchConstants.FullNameLength},
		{input.ShortName, "党支部简称", partybranchConstants.ShortNameLength},
	} {
		if field.value == "" || !validText(field.value, field.limit) {
			return input, invalid(field.label + "不能为空、包含控制字符或超过长度限制")
		}
	}
	if input.SecretaryID != nil && *input.SecretaryID == 0 {
		return input, invalid("请选择有效的党支部书记")
	}
	return input, nil
}

func newRecord(workspaceID string, input partybranchModels.Input) partybranchModels.Branch {
	return partybranchModels.Branch{
		WorkspaceID: workspaceID,
		FullName:    input.FullName,
		ShortName:   input.ShortName,
		SecretaryID: input.SecretaryID,
		Revision:    1,
	}
}

func detail(tx *gorm.DB, workspaceID string, id uint64) (partybranchModels.Info, error) {
	var result partybranchModels.Info
	if err := infoScope(tx, workspaceID).Where("b.id = ?", id).Take(&result).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return result, &response.BusinessError{
				Status: http.StatusNotFound, Message: "党支部不存在或不可访问",
			}
		}
		return result, err
	}
	return result, nil
}

func snapshot(info partybranchModels.Info) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":                 info.ID,
		"full_name":          info.FullName,
		"short_name":         info.ShortName,
		"priority":           info.Priority,
		"secretary_id":       info.SecretaryID,
		"secretary_name":     info.SecretaryName,
		"secretary_username": info.SecretaryUsername,
		"revision":           info.Revision,
	}
}

func event(workspaceID string, id uint64) auditModels.Event {
	result := auditModels.Event{
		TenantID:   &workspaceID,
		ObjectType: "cpc_party_branch",
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID},
	}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}

func validText(value string, limit int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "党支部已被修改，请刷新后重试"}
}

func writeError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		switch database.Number {
		case 1062:
			return &response.BusinessError{
				Status: http.StatusConflict, Message: "当前空间已存在相同全称或简称的党支部",
			}
		case 1451:
			return &response.BusinessError{
				Status: http.StatusConflict, Message: "删除党支部失败，请检查关联业务后重试",
			}
		case 1452:
			return invalid("关联记录不存在或不可访问")
		}
	}
	return err
}
