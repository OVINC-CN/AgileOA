package monthlyworkSvc

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

// List 一次读取筛选范围内当前账号负责或参与的工作，不分页。
func List(
	ctx context.Context,
	workspaceID, category string,
	query monthlyworkModels.Query,
) (monthlyworkModels.ListData, error) {
	// 保存实际提交的筛选条件，读取仍须在审计提交后返回。
	result := monthlyworkModels.ListData{Items: []monthlyworkModels.Info{}}
	auditEvent := event(workspaceID, category, 0)
	auditEvent.Conditions = auditModels.Snapshot{
		"category": category,
		"year":     query.Years,
		"month":    query.Months,
		"keyword":  query.Keyword,
	}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkList),
		auditEvent,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 校验年份、月份和关键词，未选中的维度不限制。
			query.Keyword = strings.TrimSpace(query.Keyword)
			for _, year := range query.Years {
				if year < monthlyworkConstants.MinYear || year > monthlyworkConstants.MaxYear {
					return invalid("请选择 1900 至 9999 年")
				}
			}
			for _, month := range query.Months {
				if month < 1 || month > 12 {
					return invalid("月份须为 1 至 12")
				}
			}
			if !validText(query.Keyword, monthlyworkConstants.NameLength, false) {
				return invalid("搜索关键词最多 100 字符，且不能包含控制字符")
			}

			// 先限定本人负责或参与的工作，多个年份使用日期范围的并集。
			scope := monthlyworkUtils.VisibleRecords(tx, workspaceID, category, principal.User.ID)
			if len(query.Years) > 0 {
				years := tx.Where("1 = 0")
				for _, year := range query.Years {
					years = years.Or("date BETWEEN ? AND ?",
						fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year))
				}
				scope = scope.Where(years)
			}
			if len(query.Months) > 0 {
				months := make([]string, 0, len(query.Months))
				for _, month := range query.Months {
					months = append(months, fmt.Sprintf("%02d", month))
				}
				scope = scope.Where("SUBSTRING(date, 6, 2) IN ?", months)
			}
			if query.Keyword != "" {
				owners := tx.Table("users").Select("id").
					Where("LOCATE(?, display_name) > 0 OR LOCATE(?, username) > 0", query.Keyword, query.Keyword)
				participants := tx.Model(&monthlyworkModels.Participant{}).
					Select("monthly_work_id").
					Where("workspace_id = ? AND user_id IN (?)", workspaceID, owners)
				scope = scope.Where(tx.Where("LOCATE(?, work_item) > 0", query.Keyword).
					Or("LOCATE(?, description) > 0", query.Keyword).
					Or("owner_id IN (?)", owners).
					Or("id IN (?)", participants))
			}
			var records []monthlyworkModels.Record
			if err := scope.Order("date DESC, id DESC").Find(&records).Error; err != nil {
				return err
			}

			// 附件元数据单独检查读取权限，批量读取关联，避免逐条额外请求。
			actions, err := accessUtils.Actions(tx, principal.User.ID, workspaceID)
			if err != nil {
				return err
			}
			canReadFiles := slices.Contains(
				actions,
				actionFor(category, accessConstants.MonthlyWorkAttachmentRead),
			)
			result.Items, err = recordInfos(tx, workspaceID, records, canReadFiles)
			if err != nil {
				return err
			}
			result.Total = int64(len(result.Items))
			for _, info := range result.Items {
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(info.ID, 10))
			}
			audit.ReturnedCount = result.Total
			return nil
		},
	)
	return result, err
}

// FilterOptions 提供当前账号可见工作记录的年份，月份使用固定的十二个月份。
func FilterOptions(
	ctx context.Context,
	workspaceID, category string,
) (monthlyworkModels.FilterOptionsData, error) {
	result := monthlyworkModels.FilterOptionsData{Years: []monthlyworkModels.FilterOption{}}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkList),
		event(workspaceID, category, 0),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			var years []string
			if err := monthlyworkUtils.VisibleRecords(tx, workspaceID, category, principal.User.ID).
				Distinct().
				Order("LEFT(date, 4) DESC").
				Pluck("LEFT(date, 4)", &years).
				Error; err != nil {
				return err
			}
			for _, year := range years {
				result.Years = append(
					result.Years,
					monthlyworkModels.FilterOption{Value: year, Label: year + " 年"},
				)
			}
			audit.ReturnedCount = int64(len(result.Years))
			return nil
		},
	)
	return result, err
}

// Fields 复用列表权限返回固定字段目录，审计持久化后才能返回。
func Fields(
	ctx context.Context,
	workspaceID, category string,
) ([]monthlyworkModels.Field, error) {
	var result []monthlyworkModels.Field
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkList),
		event(workspaceID, category, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			result = append([]monthlyworkModels.Field{}, monthlyworkConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Owners 返回当前空间内具有进入权限的启用账号，供负责人和参与人共用。
func Owners(
	ctx context.Context,
	workspaceID, category string,
) ([]monthlyworkModels.OwnerOption, error) {
	result := []monthlyworkModels.OwnerOption{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkList),
		event(workspaceID, category, 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			var users []struct {
				ID                    uint64
				DisplayName, Username string
			}
			if err := accessUtils.WorkspaceUsers(
				tx,
				workspaceID,
			).Select("users.id", "users.display_name", "users.username").
				Order("users.display_name ASC, users.id ASC").
				Limit(config.Config.MonthlyWork.MaxOptions + 1).
				Find(&users).
				Error; err != nil {
				return err
			}
			if len(users) > config.Config.MonthlyWork.MaxOptions {
				return invalid("负责人及参与人选项超过限制，请联系管理员")
			}
			for _, user := range users {
				result = append(
					result,
					monthlyworkModels.OwnerOption{
						Value: user.ID,
						Label: user.DisplayName + "(" + user.Username + ")",
					},
				)
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(user.ID, 10))
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

func validateOwner(tx *gorm.DB, workspaceID string, id uint64) error {
	var count int64
	if err := accessUtils.WorkspaceUsers(tx, workspaceID).
		Where("users.id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return invalid("负责人须为当前工作空间可用的用户")
	}
	return nil
}
