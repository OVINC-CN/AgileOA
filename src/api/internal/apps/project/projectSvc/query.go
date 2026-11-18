package projectSvc

import (
	"context"
	"sort"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// ListProjects 查询当前空间项目，按名称、年份升序及编号倒序排列。
func ListProjects(
	ctx context.Context,
	workspaceID string,
	query projectModels.Query,
) (projectModels.ProjectList, error) {
	result := projectModels.ProjectList{
		Items:    []projectModels.ProjectInfo{},
		Page:     query.Page,
		PageSize: query.PageSize,
	}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ProjectList,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			if err := validQuery(query); err != nil {
				return err
			}
			if query.DetailKeyword != "" {
				return invalid("子项搜索仅支持获奖学生列表")
			}
			scope := projectListScope(tx, workspaceID, query)
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			var ids []uint64
			if err := scope.Order(projectConstants.ProjectOrder).
				Offset((query.Page-1)*query.PageSize).
				Limit(query.PageSize).
				Pluck("id", &ids).
				Error; err != nil {
				return err
			}
			for _, id := range ids {
				info, err := projectDetail(tx, workspaceID, id)
				if err != nil {
					return err
				}
				result.Items = append(result.Items, info)
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(id, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

// ListParticipations 支持项目和学生筛选，按项目、年份、排名升序及编号倒序分页。
func ListParticipations(
	ctx context.Context,
	workspaceID string,
	query projectModels.Query,
) (projectModels.ParticipationList, error) {
	result := projectModels.ParticipationList{
		Items:    []projectModels.ParticipationSummary{},
		Page:     query.Page,
		PageSize: query.PageSize,
	}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ParticipationList,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			if err := validQuery(query); err != nil {
				return err
			}
			scope := participationListScope(tx, workspaceID, query)
			if err := scope.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
				return err
			}
			var rows []participationRow
			if err := scope.Select(participationColumns()).
				Order(projectConstants.ParticipationOrder).
				Offset((query.Page - 1) * query.PageSize).
				Limit(query.PageSize).
				Scan(&rows).
				Error; err != nil {
				return err
			}
			for _, row := range rows {
				result.Items = append(result.Items, participationInfo(row))
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(row.ID, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

func projectListScope(tx *gorm.DB, workspaceID string, query projectModels.Query) *gorm.DB {
	// 列表和导出复用同一租户边界及筛选条件。
	scope := tx.Model(&projectModels.Project{}).Where("workspace_id = ?", workspaceID)
	if query.Keyword != "" {
		scope = scope.Where("LOCATE(?, name) > 0", query.Keyword)
	}
	if len(query.ProjectNames) > 0 {
		scope = scope.Where("name IN ?", query.ProjectNames)
	}
	if len(query.Years) > 0 {
		scope = scope.Where("year IN ?", query.Years)
	}
	return scope
}

func participationListScope(tx *gorm.DB, workspaceID string, query projectModels.Query) *gorm.DB {
	// 子项关键词只选择参与记录，完整评分明细仍由调用方按需读取。
	scope := detailFilteredScope(tx, workspaceID, query)
	if query.Keyword != "" {
		scope = scope.Where("LOCATE(?, p.name) > 0", query.Keyword)
	}
	if len(query.ProjectNames) > 0 {
		scope = scope.Where("p.name IN ?", query.ProjectNames)
	}
	if query.ProjectID != 0 {
		scope = scope.Where("r.project_id = ?", query.ProjectID)
	}
	if len(query.Years) > 0 {
		scope = scope.Where("p.year IN ?", query.Years)
	}
	if query.Student != "" {
		scope = scope.Where(
			"LOCATE(?, s.student_no) > 0 OR LOCATE(?, s.name) > 0",
			query.Student,
			query.Student,
		)
	}
	if len(query.EducationLevels) > 0 {
		scope = scope.Where("s.education_level IN ?", query.EducationLevels)
	}
	if len(query.Majors) > 0 {
		scope = scope.Where("s.major IN ?", query.Majors)
	}
	if len(query.Grades) > 0 {
		scope = scope.Where("s.grade IN ?", query.Grades)
	}
	return scope
}

// Statistics 按项目归属年份统计去重参加人数，同名项目形成同一条年度折线。
func Statistics(
	ctx context.Context,
	workspaceID string,
	query projectModels.Query,
) ([]projectModels.Statistic, error) {
	result := []projectModels.Statistic{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ProjectStatistics,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 统计复用已提交的项目和学生筛选，未匹配学生的项目仍保留零人数。
			if err := validQuery(query); err != nil {
				return err
			}
			projects := projectListScope(tx, workspaceID, query).Select("id")
			if query.ProjectID != 0 {
				projects = projects.Where("id = ?", query.ProjectID)
			}
			participations := participationListScope(tx, workspaceID, query).
				Select("r.workspace_id, r.project_id, r.student_id")
			err := tx.Table("projects AS p").
				Joins(
					"LEFT JOIN (?) AS r ON r.workspace_id = p.workspace_id AND r.project_id = p.id",
					participations,
				).
				Where("p.workspace_id = ?", workspaceID).
				Where("p.id IN (?)", projects).
				Select("p.name, p.year, COUNT(DISTINCT r.student_id) AS count").
				Group("p.name, p.year").
				Order("p.year ASC, p.name ASC").
				Limit(config.Config.Project.MaxOptions + 1).Scan(&result).Error
			if err != nil {
				return err
			}
			if len(result) > config.Config.Project.MaxOptions {
				return invalid("项目统计数量超过限制")
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Options 仅查询固定的项目或学生投影，不暴露学生敏感字段。
func Options(
	ctx context.Context,
	workspaceID, keyword string,
	students bool,
) ([]projectModels.Option, error) {
	result := []projectModels.Option{}
	action := accessConstants.ProjectList
	if students {
		action = accessConstants.ParticipationList
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, "project", 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			if !validText(keyword, projectConstants.NameLength, false) {
				return invalid("搜索关键词无效")
			}
			var scope *gorm.DB
			if students {
				scope = tx.Table("students").Where("workspace_id = ?", workspaceID).
					Where("LOCATE(?, student_no) > 0 OR LOCATE(?, name) > 0", keyword, keyword).
					Select("id AS value, CONCAT(name, '(', student_no, ')') AS label").
					Order("student_no ASC, id ASC")
			} else {
				scope = tx.Model(&projectModels.Project{}).Where("workspace_id = ?", workspaceID).
					Where("LOCATE(?, name) > 0", keyword).
					Select("id AS value, CONCAT(year, ' · ', name) AS label").
					Order("name ASC, year ASC, id ASC")
			}
			if err := scope.Limit(config.Config.Project.MaxOptions + 1).
				Scan(&result).
				Error; err != nil {
				return err
			}
			if len(result) > config.Config.Project.MaxOptions {
				return invalid("选项过多，请输入关键词搜索")
			}
			for _, item := range result {
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.Value, 10))
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Fields 返回固定字段目录，不接受任意数据库表名和列名。
func Fields(ctx context.Context, workspaceID string) ([]projectModels.Field, error) {
	result := []projectModels.Field{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ProjectList,
		event(workspaceID, "project", 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			result = append(result, projectConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}
func validQuery(query projectModels.Query) error {
	// 校验每个选中名称和年份，禁止用无效值放宽子项搜索范围。
	if !validText(query.Keyword, projectConstants.NameLength, false) ||
		!validText(query.Student, projectConstants.NameLength, false) ||
		!validText(query.DetailKeyword, projectConstants.NameLength, false) {
		return invalid("项目或学生筛选无效")
	}
	for _, name := range query.ProjectNames {
		if !validText(name, projectConstants.NameLength, true) {
			return invalid("项目名称筛选无效")
		}
	}
	for _, year := range query.Years {
		if year < projectConstants.MinYear || year > projectConstants.MaxYear {
			return invalid("年份筛选无效")
		}
	}
	// 学生筛选沿用学生固定字段目录的长度约束。
	fields := []struct {
		key    string
		values []string
	}{
		{key: "education_level", values: query.EducationLevels},
		{key: "major", values: query.Majors},
		{key: "grade", values: query.Grades},
	}
	for _, item := range fields {
		field, _ := studentConstants.Lookup(item.key)
		for _, value := range item.values {
			if !validText(value, field.MaxLength, true) {
				return invalid("学生筛选无效")
			}
		}
	}
	if query.DetailKeyword != "" &&
		((len(query.ProjectNames) == 0 && query.ProjectID == 0) || len(query.Years) == 0) {
		return invalid("子项搜索必须先选择项目和归属年份")
	}
	return nil
}
func queryEvent(workspaceID string, query projectModels.Query) auditModels.Event {
	result := event(workspaceID, "project", 0)
	result.Conditions = auditModels.Snapshot{
		"project":         query.Keyword,
		"project_id":      query.ProjectID,
		"project_name":    query.ProjectNames,
		"detail_keyword":  query.DetailKeyword,
		"student":         query.Student,
		"education_level": query.EducationLevels,
		"major":           query.Majors,
		"grade":           query.Grades,
		"year":            query.Years,
		"page":            query.Page,
		"page_size":       query.PageSize,
	}
	return result
}

// DetailNames 仅枚举当前项目配置的子项名称，参与评分时不可新增名称。
func DetailNames(
	ctx context.Context,
	workspaceID string,
	projectID uint64,
	keyword string,
) ([]projectModels.NameOption, error) {
	result := []projectModels.NameOption{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.ParticipationList,
		event(workspaceID, "project_detail", 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			audit.Conditions["project_id"] = projectID
			if projectID == 0 || !validText(keyword, projectConstants.NameLength, false) {
				return invalid("项目编号或子项名称关键词无效")
			}
			var project projectModels.Project
			if err := tx.Select("id").
				Where("workspace_id = ? AND id = ?", workspaceID, projectID).
				Take(&project).Error; err != nil {
				return lookupError(err)
			}
			names := []string{}
			if err := tx.Model(&projectModels.DetailOption{}).
				Where(
					"workspace_id = ? AND project_id = ? AND LOCATE(?, name) > 0",
					workspaceID,
					projectID,
					keyword,
				).
				Order("name ASC").
				Limit(config.Config.Project.MaxOptions+1).
				Pluck("name", &names).
				Error; err != nil {
				return err
			}
			if len(names) > config.Config.Project.MaxOptions {
				return invalid("子项名称过多，请输入关键词搜索")
			}
			for _, name := range names {
				result = append(result, projectModels.NameOption{Value: name, Label: name})
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// FilterOptions 按当前列表权限读取完整的项目及学生筛选选项，供前端本地搜索。
func FilterOptions(
	ctx context.Context,
	workspaceID string,
	participation bool,
) (projectModels.FilterOptionsData, error) {
	result := projectModels.FilterOptionsData{
		Names:           []projectModels.FilterOption{},
		Years:           []projectModels.FilterOption{},
		EducationLevels: []projectModels.FilterOption{},
		Majors:          []projectModels.FilterOption{},
		Grades:          []projectModels.FilterOption{},
	}
	action := accessConstants.ProjectList
	if participation {
		action = accessConstants.ParticipationList
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, "project", 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			if err := loadProjectFilterOptions(tx, workspaceID, &result, audit); err != nil {
				return err
			}
			if participation {
				return studentFilterOptions(tx, workspaceID, &result, audit)
			}
			return nil
		})
	return result, err
}

func loadProjectFilterOptions(tx *gorm.DB, workspaceID string,
	result *projectModels.FilterOptionsData, audit *auditModels.Event,
) error {
	// 固定查询名称与年份，超限明确拒绝，不能返回不完整枚举。
	var rows []struct {
		Name string
		Year int
	}
	if err := tx.Model(&projectModels.Project{}).Where("workspace_id = ?", workspaceID).
		Select("name", "year").
		Order("name ASC, year ASC").
		Limit(config.Config.Project.MaxOptions + 1).Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) > config.Config.Project.MaxOptions {
		return invalid("项目筛选选项超过限制")
	}
	names, yearSet := map[string]bool{}, map[int]bool{}
	years := []int{}
	for _, row := range rows {
		if !names[row.Name] {
			result.Names = append(
				result.Names,
				projectModels.FilterOption{Value: row.Name, Label: row.Name},
			)
			names[row.Name] = true
		}
		if !yearSet[row.Year] {
			years = append(years, row.Year)
			yearSet[row.Year] = true
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))
	for _, year := range years {
		value := strconv.Itoa(year)
		result.Years = append(
			result.Years,
			projectModels.FilterOption{Value: value, Label: value + " 年"},
		)
	}
	audit.ReturnedCount = int64(len(result.Names) + len(result.Years))
	return nil
}

func studentFilterOptions(
	tx *gorm.DB,
	workspaceID string,
	result *projectModels.FilterOptionsData,
	audit *auditModels.Event,
) error {
	// 仅枚举固定学生基础字段，与毕业去向使用相同的工作空间边界和选项上限。
	fields := []struct {
		column string
		target *[]projectModels.FilterOption
	}{
		{column: "education_level", target: &result.EducationLevels},
		{column: "major", target: &result.Majors},
		{column: "grade", target: &result.Grades},
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
			return invalid("学生筛选选项超过限制")
		}
		for _, value := range values {
			*field.target = append(
				*field.target,
				projectModels.FilterOption{Value: value, Label: value},
			)
		}
		audit.ReturnedCount += int64(len(values))
	}
	return nil
}

func detailFilteredScope(
	tx *gorm.DB,
	workspaceID string,
	query projectModels.Query,
) *gorm.DB {
	scope := participationScope(tx, workspaceID)
	if query.DetailKeyword == "" {
		return scope
	}

	// 按工作空间、项目名称及年份收敛项目集合，不用模糊关键词放宽范围。
	projectScope := tx.Model(&projectModels.Project{}).
		Where("workspace_id = ? AND year IN ?", workspaceID, query.Years)
	if len(query.ProjectNames) > 0 {
		projectScope = projectScope.Where("name IN ?", query.ProjectNames)
	}
	if query.ProjectID != 0 {
		projectScope = projectScope.Where("id = ?", query.ProjectID)
	}
	// 主查询按租户和项目索引收敛，子查询复用租户与参与编号索引；EXISTS 不会重复参与记录。
	details := tx.Table("project_participation_details AS d").Select("1").
		Where("d.workspace_id = ? AND d.participation_id = r.id", workspaceID).
		Where("(LOCATE(?, d.name) > 0 OR LOCATE(?, d.description) > 0)", query.DetailKeyword, query.DetailKeyword)
	return scope.Where("r.project_id IN (?)", projectScope.Select("id")).
		Where("EXISTS (?)", details)
}
