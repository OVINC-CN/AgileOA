package studentSvc

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// List 只输出基础字段，涉及敏感字段的筛选额外校验处理敏感信息权限。
func List(
	ctx context.Context,
	workspaceID string,
	query studentModels.ListQuery,
) (studentModels.ListData, error) {
	result := studentModels.ListData{
		Items:    []studentModels.Info{},
		Page:     query.Page,
		PageSize: query.PageSize,
	}
	event := queryEvent(workspaceID, query)
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentList, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			scope, err := filteredScope(tx, principal, query)
			if err != nil {
				return err
			}
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			var records []studentModels.BasicRecord
			if err := scope.Select(publicColumns()).Order("s.id DESC").
				Offset((query.Page - 1) * query.PageSize).
				Limit(query.PageSize).Scan(&records).Error; err != nil {
				return err
			}
			for _, record := range records {
				result.Items = append(result.Items, studentInfo(record))
			}
			for _, item := range result.Items {
				event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(item.ID, 10))
			}
			event.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

// Fields 公开固定字段说明，不返回任何学生数据或枚举值。
func Fields(ctx context.Context, workspaceID string) ([]studentModels.Field, error) {
	result := []studentModels.Field{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentList,
		studentEvent(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			for _, field := range studentConstants.Fields {
				result = append(result, field.Field)
				event.ReturnedIDs = append(event.ReturnedIDs, field.Key)
			}
			event.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Options 直接查询当前空间明细中的 distinct 值，不读取独立枚举配置表。
func Options(
	ctx context.Context,
	workspaceID, key string,
	sensitive bool,
) ([]studentModels.Option, error) {
	result := []studentModels.Option{}
	field, ok := studentConstants.Lookup(key)
	category, action := studentConstants.Basic, accessConstants.StudentList
	if sensitive {
		category, action = studentConstants.Sensitive, accessConstants.StudentSensitiveHandle
	}
	if !ok || !field.Enumerable || field.Group != category {
		return result, invalid("不支持此筛选维度")
	}

	event := studentEvent(workspaceID, 0)
	event.Conditions["field"] = key
	err := accessUtils.Workspace(ctx, workspaceID, action, event,
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			values := []string{}
			scope := basicScope(tx, workspaceID).Distinct(field.Column).Order(field.Column + " ASC")
			if field.Type != "enum" {
				scope = scope.Where(field.Column+" <> ?", "")
			}
			if err := scope.Limit(config.Config.Student.MaxOptions+1).
				Pluck(field.Column, &values).
				Error; err != nil {
				return err
			}

			if len(values) > config.Config.Student.MaxOptions {
				return invalid("维度值数量超过限制")
			}
			for _, value := range values {
				value, err := dimensionValue(field, value)
				if err != nil {
					return err
				}
				result = append(
					result,
					studentModels.Option{Value: value, Label: studentConstants.Label(field, value)},
				)
			}

			// 敏感条件值不进入普通审计，审计只保留字段和结果数量。
			event.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Statistics 返回筛选结果的学生、班级、宗教信仰和校外住宿数量。
func Statistics(
	ctx context.Context,
	workspaceID string,
	query studentModels.ListQuery,
) (studentModels.StatisticsData, error) {
	var result studentModels.StatisticsData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentStatistics,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			scope, err := filteredScope(tx, principal, query)
			if err != nil {
				return err
			}
			// 聚合表达式及列固定，枚举比较使用参数，空班级不计入数量。
			if err := scope.Select(
				"COUNT(*) AS total, COUNT(DISTINCT NULLIF(s.class_name, '')) AS class_count, "+
					"COALESCE(SUM(CASE WHEN s.has_religion = ? THEN 1 ELSE 0 END), 0) AS religious_count, "+
					"COALESCE(SUM(CASE WHEN s.accommodation_type = ? THEN 1 ELSE 0 END), 0) AS off_campus_count",
				studentConstants.FlagYes,
				studentConstants.AccommodationOffCampus,
			).Scan(&result).Error; err != nil {
				return err
			}
			event.ReturnedCount = result.Total
			return nil
		},
	)
	return result, err
}

func filteredScope(
	tx *gorm.DB,
	principal accessModels.Principal,
	query studentModels.ListQuery,
) (*gorm.DB, error) {
	if !validText(query.Keyword, 100, false) {
		return nil, invalid("搜索关键词无效")
	}
	workspaceID := principal.Workspace.ID
	scope := basicScope(tx, workspaceID)
	if query.Keyword != "" {
		// 使用 LOCATE 避免输入中的百分号、下划线被当作通配符。
		scope = scope.Where(
			"(LOCATE(?, s.student_no) > 0 OR LOCATE(?, s.name) > 0)",
			query.Keyword,
			query.Keyword,
		)
	}

	// 校验固定筛选字段，并在当前工作空间锁内统一复核敏感筛选权限。
	sensitiveFilter := false
	for key, values := range query.Filters {
		if key == "position_status" {
			continue
		}
		field, ok := studentConstants.Lookup(key)
		if !ok || !field.Enumerable {
			return nil, invalid("筛选字段无效")
		}
		if len(values) > 0 && field.Group == studentConstants.Sensitive {
			sensitiveFilter = true
		}
	}
	if sensitiveFilter {
		if err := accessUtils.Require(
			tx,
			principal,
			accessConstants.StudentSensitiveHandle,
		); err != nil {
			return nil, err
		}
	}

	// 同一维度匹配任一值，不同维度同时满足；枚举先转换为数据库代码。
	for _, field := range studentConstants.Fields {
		values := query.Filters[field.Key]
		if len(values) == 0 {
			continue
		}
		codes := make([]string, 0, len(values))
		for _, value := range values {
			if value == "" || !validText(value, field.MaxLength, false) {
				return nil, invalid("筛选值无效")
			}
			if field.Type == "enum" {
				code, ok := studentConstants.EnumCode(field.Key, value)
				if !ok {
					return nil, invalid(field.Label + "筛选值无效")
				}
				codes = append(codes, code)
			} else {
				codes = append(codes, value)
			}
		}
		scope = scope.Where(field.Column+" IN ?", codes)
	}

	// 任职与未任职同时勾选时覆盖全部学生，仍逐项校验固定代码。
	assigned, unassigned := false, false
	for _, value := range query.Filters["position_status"] {
		switch value {
		case "assigned":
			assigned = true
		case "unassigned":
			unassigned = true
		default:
			return nil, invalid("班内任职筛选无效")
		}
	}
	if assigned && !unassigned {
		scope = scope.Where("s.class_position <> ?", "")
	} else if unassigned && !assigned {
		scope = scope.Where("s.class_position = ?", "")
	}
	return scope, nil
}

func queryEvent(workspaceID string, query studentModels.ListQuery) auditModels.Event {
	event := studentEvent(workspaceID, 0)
	event.Conditions["keyword"] = query.Keyword
	event.Conditions["page"], event.Conditions["page_size"] = query.Page, query.PageSize
	for key, value := range query.Filters {
		if field, ok := studentConstants.Lookup(key); ok && field.Group != studentConstants.Basic {
			event.Conditions[key+"_filtered"] = len(value) > 0
		} else {
			event.Conditions[key] = value
		}
	}
	return event
}

func dimensionValue(field studentConstants.Definition, raw string) (string, error) {
	if field.Type != "enum" {
		return raw, nil
	}
	value := studentConstants.EnumValue(field.Key, raw)
	expected, ok := studentConstants.EnumCode(field.Key, value)
	if !ok || expected != raw {
		return "", errors.New("invalid stored student enum")
	}
	return value, nil
}
