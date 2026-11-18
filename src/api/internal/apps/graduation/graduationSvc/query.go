package graduationSvc

import (
	"context"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/graduation/graduationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// List 查询学生及其唯一毕业去向，包含尚未绑定的学生。
func List(
	ctx context.Context,
	workspaceID string,
	query graduationModels.Query,
) (graduationModels.ListData, error) {
	result := graduationModels.ListData{
		Items: []graduationModels.Info{}, Page: query.Page, PageSize: query.PageSize,
	}
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationList, queryEvent(workspaceID, query),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			scope, err := filteredScope(tx, workspaceID, query)
			if err != nil {
				return err
			}
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			if err := scope.Select(columns()).Order(graduationConstants.ListOrder).
				Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
				Scan(&result.Items).Error; err != nil {
				return err
			}
			for _, item := range result.Items {
				audit.ReturnedIDs = append(
					audit.ReturnedIDs,
					strconv.FormatUint(item.StudentID, 10),
				)
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

// Statistics 按毕业年份及固定维度统计人数，空类型统一计入待就业。
func Statistics(
	ctx context.Context,
	workspaceID string,
	query graduationModels.Query,
	dimension string,
) ([]graduationModels.Statistic, error) {
	result := []graduationModels.Statistic{}
	auditEvent := queryEvent(workspaceID, query)
	auditEvent.Conditions["dimension"] = dimension
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationStatistics, auditEvent,
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			scope, err := filteredScope(tx, workspaceID, query)
			if err != nil {
				return err
			}

			// 分组表达式仅从固定目录选择，不拼接外部字段。
			var grouping string
			switch dimension {
			case graduationConstants.DimensionMajor:
				grouping = "major AS value, major AS label"
				scope = scope.Group("graduation_year, major")
			case graduationConstants.DimensionType:
				grouping = "graduation_type AS value, graduation_type_label AS label"
				scope = scope.Group("graduation_year, graduation_type, graduation_type_label")
			default:
				return invalid("不支持此毕业去向统计维度")
			}
			selection := "graduation_year, " + grouping + ", COUNT(*) AS total, " +
				"SUM(CASE WHEN graduation_type <> '' THEN 1 ELSE 0 END) AS employed, " +
				"SUM(CASE WHEN graduation_type = '' THEN 1 ELSE 0 END) AS pending"
			if err := scope.Select(selection).Order("graduation_year ASC, label ASC, value ASC").
				Limit(config.Config.Student.MaxOptions + 1).Scan(&result).Error; err != nil {
				return err
			}
			if len(result) > config.Config.Student.MaxOptions {
				return invalid("毕业去向统计数量超过限制")
			}
			for index := range result {
				item := &result[index]
				if item.Label == "" {
					item.Label = "未填写专业"
					if dimension == graduationConstants.DimensionType {
						item.Label = graduationConstants.PendingLabel
					}
				}
				if item.Total > 0 {
					item.EmploymentRate = float64(item.Employed) / float64(item.Total)
				}
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Options 按列表或统计权限返回当前空间完整去重筛选选项，包含已保存的类型。
func Options(
	ctx context.Context,
	workspaceID string,
	statistics bool,
) (graduationModels.FilterOptions, error) {
	result := graduationModels.FilterOptions{
		EducationLevels: []graduationModels.Option{}, Majors: []graduationModels.Option{},
		Grades: []graduationModels.Option{}, Types: []graduationModels.Option{},
		GraduationYears: []graduationModels.Option{},
	}
	action := accessConstants.GraduationList
	if statistics {
		action = accessConstants.GraduationStatistics
	}
	err := accessUtils.Workspace(
		ctx, workspaceID, action, event(workspaceID, 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 只查询明确登记的学生基础字段，不接受任意表和列。
			fields := []struct {
				column string
				target *[]graduationModels.Option
			}{
				{column: "education_level", target: &result.EducationLevels},
				{column: "major", target: &result.Majors},
				{column: "grade", target: &result.Grades},
				{column: "graduation_year", target: &result.GraduationYears},
			}
			for _, field := range fields {
				var values []string
				if err := tx.Table("students").Where("workspace_id = ?", workspaceID).
					Where(field.column+" <> ''").Distinct(field.column).Order(field.column+" ASC").
					Limit(config.Config.Student.MaxOptions+1).Pluck(field.column, &values).
					Error; err != nil {
					return err
				}
				if len(values) > config.Config.Student.MaxOptions {
					return invalid("毕业去向筛选选项超过限制")
				}
				for _, value := range values {
					*field.target = append(
						*field.target,
						graduationModels.Option{Value: value, Label: value},
					)
				}
				audit.ReturnedCount += int64(len(values))
			}
			if err := tx.Table("students").
				Where("workspace_id = ? AND graduation_type <> ''", workspaceID).
				Distinct().Select("graduation_type AS value, graduation_type_label AS label").
				Order("label ASC, value ASC").Limit(config.Config.Student.MaxOptions + 1).
				Scan(&result.Types).Error; err != nil {
				return err
			}
			if len(result.Types) > config.Config.Student.MaxOptions {
				return invalid("毕业去向类型超过限制")
			}
			audit.ReturnedCount += int64(len(result.Types))
			return nil
		},
	)
	return result, err
}

// Fields 返回毕业去向固定字段目录及虚构示例。
func Fields(ctx context.Context, workspaceID string) ([]graduationModels.Field, error) {
	result := []graduationModels.Field{}
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.GraduationList, event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			result = append(result, graduationConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

func filteredScope(
	tx *gorm.DB,
	workspaceID string,
	query graduationModels.Query,
) (*gorm.DB, error) {
	// 校验自由文本、年份及固定筛选字段的每个选中值。
	if !validText(query.Keyword, graduationConstants.KeywordLength) {
		return nil, invalid("学生筛选无效")
	}
	for _, year := range query.GraduationYears {
		if !validYear(year) {
			return nil, invalid("毕业年份筛选无效")
		}
	}
	fields := []struct {
		column    string
		values    []string
		maxLength int
	}{
		{"education_level", query.EducationLevels, graduationConstants.EducationLength},
		{"major", query.Majors, graduationConstants.MajorLength},
		{"grade", query.Grades, graduationConstants.GradeLength},
		{"graduation_type", query.Types, graduationConstants.TypeCodeLength},
	}
	for _, field := range fields {
		for _, value := range field.values {
			if !validText(value, field.maxLength) {
				return nil, invalid("毕业去向筛选无效")
			}
		}
	}

	// 所有条件均限定当前工作空间，同一字段内使用集合匹配。
	scope := tx.Table("students").Where("workspace_id = ?", workspaceID)
	if query.Keyword != "" {
		scope = scope.Where(
			"LOCATE(?, student_no) > 0 OR LOCATE(?, name) > 0",
			query.Keyword,
			query.Keyword,
		)
	}
	for _, field := range fields {
		if len(field.values) > 0 {
			scope = scope.Where(field.column+" IN ?", field.values)
		}
	}
	if len(query.GraduationYears) > 0 {
		scope = scope.Where("graduation_year IN ?", query.GraduationYears)
	}
	return scope, nil
}

func queryEvent(workspaceID string, query graduationModels.Query) auditModels.Event {
	result := event(workspaceID, 0)
	result.Conditions = auditModels.Snapshot{
		"workspace_id": workspaceID, "keyword": query.Keyword,
		"education_level": query.EducationLevels, "major": query.Majors, "grade": query.Grades,
		"page": query.Page, "page_size": query.PageSize,
		"graduation_year": query.GraduationYears,
	}
	if len(query.Types) > 0 {
		result.Conditions["type"] = query.Types
	}
	return result
}
