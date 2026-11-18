package projectSvc

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func detailOptionNames(tx *gorm.DB, workspaceID string, projectID uint64) ([]string, error) {
	// 配置按当前项目读取，不复用其他项目或年份的历史名称。
	names := []string{}
	err := tx.Model(&projectModels.DetailOption{}).
		Where("workspace_id = ? AND project_id = ?", workspaceID, projectID).
		Order("name ASC").
		Pluck("name", &names).Error
	return names, err
}

func saveDetailOptions(
	tx *gorm.DB,
	workspaceID string,
	projectID uint64,
	names []string,
) error {
	// 仅删除本次明确移除的配置，已引用的名称在删除前显式校验。
	current, err := detailOptionNames(tx, workspaceID, projectID)
	if err != nil {
		return err
	}
	// 级联外键不承担业务保护，显式阻止删除已被获奖或申报引用的名称。
	for _, table := range []string{"project_participation_details", "project_application_details"} {
		var count int64
		scope := tx.Table(table).
			Where("workspace_id = ? AND project_id = ?", workspaceID, projectID)
		if len(names) > 0 {
			scope = scope.Where("name NOT IN ?", names)
		}
		if err := scope.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return &response.BusinessError{
				Status: http.StatusConflict, Message: "子项名称仍被获奖或申报记录引用，不能移除",
			}
		}
	}
	removed := tx.Where("workspace_id = ? AND project_id = ?", workspaceID, projectID)
	if len(names) > 0 {
		removed = removed.Where("name NOT IN ?", names)
	}
	if err := removed.Delete(&projectModels.DetailOption{}).Error; err != nil {
		var database *mysql.MySQLError
		if errors.As(err, &database) && database.Number == 1451 {
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "子项名称仍有评分记录引用，请先处理评分明细",
			}
		}
		return err
	}

	// 保留未变化配置的数据库时间，只插入新配置的名称。
	existing := make(map[string]bool, len(current))
	for _, name := range current {
		existing[name] = true
	}
	for _, name := range names {
		if existing[name] {
			continue
		}
		record := projectModels.DetailOption{
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			Name:        name,
		}
		if err := tx.Create(&record).Error; err != nil {
			var database *mysql.MySQLError
			if errors.As(err, &database) && database.Number == 1062 {
				return invalid("子项名称重复，请检查配置")
			}
			return writeError(err)
		}
	}
	return nil
}

func validateDetailNames(
	tx *gorm.DB, workspaceID string, projectID uint64, items []projectModels.ScoreItem,
) error {
	names, err := canonicalDetailNames(tx, workspaceID, projectID, items)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return invalid("同一条记录中的子项名称不能重复")
		}
		seen[name] = true
	}
	return nil
}

func canonicalDetailNames(
	tx *gorm.DB, workspaceID string, projectID uint64, items []projectModels.ScoreItem,
) ([]string, error) {
	// 新增、编辑和导入共用项目配置校验，不修改已有内容的名称写法。
	if len(items) == 0 {
		return []string{}, nil
	}
	names, err := detailOptionNames(tx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]string, len(names))
	for _, name := range names {
		allowed[name] = name
	}
	result := make([]string, len(items))
	for index, item := range items {
		name, found := allowed[item.Name]
		if !found {
			// 沿用数据库排序规则，防止大小写或重音差异绕过唯一限制。
			var option projectModels.DetailOption
			err := tx.Select("name").Where(
				"workspace_id = ? AND project_id = ? AND name = ?", workspaceID, projectID, item.Name,
			).
				Take(&option).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, invalid("子项名称未在当前项目中配置，请从已配置的子项中选择")
			}
			if err != nil {
				return nil, err
			}
			name = option.Name
			allowed[item.Name] = name
		}
		result[index] = name
	}
	return result, nil
}

func detailWriteError(err error) error {
	// 数据库唯一约束失败时仍返回子项级提示，不混淆为项目或学生重复。
	var database *mysql.MySQLError
	if errors.As(err, &database) && database.Number == 1062 &&
		(strings.Contains(database.Message, "uk_participation_detail_name") ||
			strings.Contains(database.Message, "uk_application_detail_name")) {
		return invalid("同一条记录中的子项名称不能重复")
	}
	return writeError(err)
}
