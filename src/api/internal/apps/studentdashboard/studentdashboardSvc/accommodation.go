package studentdashboardSvc

import (
	"context"
	"net/http"
	"strconv"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/studentdashboard/studentdashboardModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Accommodation 只统计当前空间的在籍学生，读取审计提交后返回住宿信息表。
func Accommodation(
	ctx context.Context,
	workspaceID string,
	query studentdashboardModels.AccommodationQuery,
) (studentdashboardModels.AccommodationData, error) {
	var result studentdashboardModels.AccommodationData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.StudentDashboardList, accommodationEvent(query),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 查询和导出共用固定筛选、住宿归并及人数统计。
			var err error
			result, err = loadAccommodation(tx, principal.Workspace.ID, query)
			if err != nil {
				return err
			}
			recordAccommodation(audit, result)
			return nil
		},
	)
	return result, err
}

// ExportAccommodation 按当前筛选导出完整住宿分组，不受页面显示字段影响。
func ExportAccommodation(
	ctx context.Context,
	workspaceID string,
	query studentdashboardModels.AccommodationQuery,
) (studentdashboardModels.FileData, error) {
	var result studentdashboardModels.FileData
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.StudentDashboardExport, accommodationEvent(query),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 独立复核导出权限，沿用在籍范围、位置归并及已提交筛选。
			data, err := loadAccommodation(tx, principal.Workspace.ID, query)
			if err != nil {
				return err
			}
			if len(data.Items) > config.Config.Student.MaxExportRows {
				return accommodationInvalid("导出住宿分组数量超过限制")
			}
			result, err = encodeExport("住宿信息表.xlsx", accommodationRecords(data))
			if err != nil {
				return err
			}
			recordAccommodation(audit, data)
			return nil
		},
	)
	return result, err
}

// AccommodationOptions 按仪表查看权限提供当前空间在籍学生的完整住宿筛选选项。
func AccommodationOptions(
	ctx context.Context,
	workspaceID string,
) (studentdashboardModels.AccommodationOptions, error) {
	result := studentdashboardModels.AccommodationOptions{
		AccommodationTypes: []studentdashboardModels.Option{},
		Campuses:           []studentdashboardModels.Option{},
		Buildings:          []studentdashboardModels.Option{},
		Rooms:              []studentdashboardModels.Option{},
	}
	err := accessUtils.Workspace(
		ctx, workspaceID, accessConstants.StudentDashboardList,
		accommodationEvent(studentdashboardModels.AccommodationQuery{}),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 位置选项只读取校内住宿，不将非校内记录的旧位置作为筛选值。
			fields := []struct {
				column string
				target *[]studentdashboardModels.Option
			}{
				{column: "accommodation_type", target: &result.AccommodationTypes},
				{column: "campus", target: &result.Campuses},
				{column: "building", target: &result.Buildings},
				{column: "room", target: &result.Rooms},
			}
			for _, field := range fields {
				scope := accommodationScope(tx, principal.Workspace.ID)
				if field.column != "accommodation_type" {
					scope = scope.Where(
						"a.accommodation_type = ?",
						studentConstants.AccommodationOnCampus,
					)
				}
				var values []string
				if err := scope.Distinct(field.column).Order(field.column+" ASC").
					Limit(config.Config.Student.MaxOptions+1).Pluck(field.column, &values).
					Error; err != nil {
					return err
				}
				if len(values) > config.Config.Student.MaxOptions {
					return accommodationInvalid("住宿筛选选项数量超过限制")
				}
				for _, value := range values {
					option := studentdashboardModels.Option{Value: value, Label: value}
					if value == "" {
						option.Label = "-"
					}
					if field.column == "accommodation_type" {
						option = accommodationType(value)
					}
					*field.target = append(*field.target, option)
				}
				audit.ReturnedCount += int64(len(values))
			}
			return nil
		},
	)
	return result, err
}

type accommodationCount struct {
	AccommodationType string
	Campus            string
	Building          string
	Room              string
	Count             int64
}

func loadAccommodation(
	tx *gorm.DB,
	workspaceID string,
	query studentdashboardModels.AccommodationQuery,
) (studentdashboardModels.AccommodationData, error) {
	// 仅允许固定目录的住宿类型和有长度限制的位置筛选。
	if err := validateAccommodation(query); err != nil {
		return studentdashboardModels.AccommodationData{}, err
	}
	scope := accommodationScope(tx, workspaceID)
	if len(query.AccommodationTypes) > 0 {
		codes := make([]string, len(query.AccommodationTypes))
		for index, value := range query.AccommodationTypes {
			codes[index], _ = studentConstants.EnumCode("accommodation_type", value)
		}
		scope = scope.Where("a.accommodation_type IN ?", codes)
	}
	for _, filter := range []struct {
		column string
		values []string
	}{
		{column: "a.campus", values: query.Campuses},
		{column: "a.building", values: query.Buildings},
		{column: "a.room", values: query.Rooms},
	} {
		if len(filter.values) > 0 {
			scope = scope.Where(filter.column+" IN ?", filter.values)
		}
	}

	// 先将非校内位置归为空值，再按归并后的位置筛选和聚合。
	var counts []accommodationCount
	if err := scope.Select("accommodation_type, campus, building, room, COUNT(*) AS count").
		Group("accommodation_type, campus, building, room").
		Order("accommodation_type ASC, campus ASC, building ASC, room ASC").
		Scan(&counts).Error; err != nil {
		return studentdashboardModels.AccommodationData{}, err
	}
	result := studentdashboardModels.AccommodationData{
		Items: make([]studentdashboardModels.AccommodationSummary, 0, len(counts)),
	}
	for _, count := range counts {
		result.Items = append(result.Items, studentdashboardModels.AccommodationSummary{
			AccommodationType: accommodationType(count.AccommodationType),
			Campus:            count.Campus,
			Building:          count.Building,
			Room:              count.Room,
			Count:             count.Count,
		})
		result.Total += count.Count
	}
	return result, nil
}

func accommodationScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	// 固定投影先规范住宿类型，非校内的位置不参与分组或匹配历史残留值。
	students := studentScope(tx, workspaceID).Select(
		"CASE WHEN accommodation_type IN ? THEN accommodation_type ELSE ? END AS accommodation_type, "+
			"CASE WHEN accommodation_type = ? THEN TRIM(campus) ELSE '' END AS campus, "+
			"CASE WHEN accommodation_type = ? THEN TRIM(building) ELSE '' END AS building, "+
			"CASE WHEN accommodation_type = ? THEN TRIM(room) ELSE '' END AS room",
		[]string{
			studentConstants.AccommodationOnCampus,
			studentConstants.AccommodationOffCampus,
			studentConstants.AccommodationJointAbroad,
		},
		studentConstants.EnumUnknown,
		studentConstants.AccommodationOnCampus,
		studentConstants.AccommodationOnCampus,
		studentConstants.AccommodationOnCampus,
	)
	return tx.Table("(?) AS a", students)
}

func validateAccommodation(query studentdashboardModels.AccommodationQuery) error {
	// 值数量复用学生选项上限，不接收任意数据库字段或住宿类型。
	for _, value := range query.AccommodationTypes {
		if value != "unknown" && value != "on_campus" && value != "off_campus" &&
			value != "joint_abroad" {
			return accommodationInvalid("住宿类型无效")
		}
	}
	for _, filter := range []struct {
		values []string
		length int
	}{
		{values: query.AccommodationTypes, length: 16},
		{values: query.Campuses, length: 64},
		{values: query.Buildings, length: 32},
		{values: query.Rooms, length: 32},
	} {
		if len(filter.values) > config.Config.Student.MaxOptions {
			return accommodationInvalid("住宿筛选值数量超过限制")
		}
		for _, value := range filter.values {
			if !utf8.ValidString(value) || utf8.RuneCountInString(value) > filter.length {
				return accommodationInvalid("住宿筛选值长度或格式无效")
			}
		}
	}
	return nil
}

func accommodationType(code string) studentdashboardModels.Option {
	// 住宿类型复用学生固定目录的英文代码与中文名称。
	value := studentConstants.EnumValue("accommodation_type", code)
	field, _ := studentConstants.Lookup("accommodation_type")
	return studentdashboardModels.Option{Value: value, Label: studentConstants.Label(field, value)}
}

func accommodationEvent(query studentdashboardModels.AccommodationQuery) auditModels.Event {
	return auditModels.Event{
		ObjectType: "student_dashboard",
		Conditions: auditModels.Snapshot{
			"table":              "accommodation",
			"status":             studentConstants.StatusEnrolled,
			"group_by":           []string{"accommodation_type", "campus", "building", "room"},
			"accommodation_type": query.AccommodationTypes,
			"campus":             query.Campuses,
			"building":           query.Buildings,
			"room":               query.Rooms,
		},
	}
}

func recordAccommodation(audit *auditModels.Event, data studentdashboardModels.AccommodationData) {
	// 审计只记录筛选及汇总数量，不保存学生身份或完整档案。
	audit.ReturnedCount = int64(len(data.Items))
	audit.Conditions["matched_students"] = data.Total
}

func accommodationRecords(data studentdashboardModels.AccommodationData) [][]string {
	// 固定表头顺序与页面一致，姓名和其他学生明细不进入汇总文件。
	header := make([]string, len(studentdashboardConstants.AccommodationFields))
	for index, field := range studentdashboardConstants.AccommodationFields {
		header[index] = field.Label
	}
	records := make([][]string, 1, len(data.Items)+2)
	records[0] = header
	for _, item := range data.Items {
		records = append(records, []string{
			item.AccommodationType.Label,
			item.Campus,
			item.Building,
			item.Room,
			strconv.FormatInt(item.Count, 10),
		})
	}
	return append(records, []string{"合计", "", "", "", strconv.FormatInt(data.Total, 10)})
}

func accommodationInvalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}
