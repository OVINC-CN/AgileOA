package studentdashboardSvc

import (
	"context"
	"encoding/base64"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/studentdashboard/studentdashboardModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
)

// Summary 在工作空间锁内复核独立查看权限并保存读取审计。
func Summary(ctx context.Context, workspaceID string) (studentdashboardModels.Data, error) {
	var result studentdashboardModels.Data
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentDashboardList,
		event(),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 专业表只汇总当前空间在籍学生，不读取敏感字段。
			var err error
			result, err = loadSummary(tx, principal.Workspace.ID)
			if err != nil {
				return err
			}
			recordSummary(audit, result)
			return nil
		},
	)
	return result, err
}

// Fields 提供固定字段目录，复用仪表查看权限。
func Fields(
	ctx context.Context,
	workspaceID string,
) ([]studentdashboardModels.Field, error) {
	result := []studentdashboardModels.Field{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentDashboardList,
		event(),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 专业字段按汇总结果展开，不接受客户端传入表名或列名。
			result = append(result, studentdashboardConstants.Fields...)
			result = append(result, studentdashboardConstants.AccommodationFields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Export 使用独立导出权限，复用汇总查询并在返回文件前提交审计。
func Export(
	ctx context.Context,
	workspaceID string,
) (studentdashboardModels.FileData, error) {
	var result studentdashboardModels.FileData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentDashboardExport,
		event(),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 导出使用全部专业列及班级行，页面隐藏字段不影响文件。
			data, err := loadSummary(tx, principal.Workspace.ID)
			if err != nil {
				return err
			}
			if len(data.Items) > config.Config.Student.MaxExportRows {
				return &response.BusinessError{
					Status:  http.StatusBadRequest,
					Message: "导出班级数量超过限制",
				}
			}
			result, err = encodeExport("学生专业表.xlsx", exportRecords(data))
			if err != nil {
				return err
			}
			recordSummary(audit, data)
			return nil
		},
	)
	return result, err
}

func encodeExport(filename string, records [][]string) (studentdashboardModels.FileData, error) {
	// 两张仪表共用文本格式与文件大小限制，超限时整体拒绝导出。
	content, err := spreadsheet.Encode(records)
	if err != nil {
		return studentdashboardModels.FileData{}, err
	}
	fileSize := base64.RawStdEncoding.DecodedLen(len(strings.TrimRight(content, "=")))
	if fileSize > config.Config.Student.MaxFileBytes {
		return studentdashboardModels.FileData{}, &response.BusinessError{
			Status:  http.StatusBadRequest,
			Message: "导出文件超过限制",
		}
	}
	return studentdashboardModels.FileData{
		Filename:    filename,
		ContentType: spreadsheet.ContentType,
		Content:     content,
	}, nil
}

type majorCount struct {
	ClassName string
	Major     string
	Count     int64
}

type monitorName struct {
	ClassName     string
	ClassPosition string
	Name          string
}

func loadSummary(tx *gorm.DB, workspaceID string) (studentdashboardModels.Data, error) {
	// 仅在籍学生参与固定字段人数聚合，避免加载完整学生档案。
	var counts []majorCount
	if err := studentScope(tx, workspaceID).
		Select("class_name, major, COUNT(*) AS count").
		Group("class_name, major").Scan(&counts).Error; err != nil {
		return studentdashboardModels.Data{}, err
	}

	// 班长同样限定在籍状态，独立职务识别在应用内完成，排除副班长。
	var monitors []monitorName
	if err := studentScope(tx, workspaceID).
		Select("class_name, class_position, name").
		Where("class_position LIKE ?", "%班长%").
		Group("class_name, class_position, name").Scan(&monitors).Error; err != nil {
		return studentdashboardModels.Data{}, err
	}
	return summarize(counts, monitors), nil
}

func studentScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	// 人数、专业列与班长共用租户及在籍范围，导出沿用同一查询。
	return tx.Table("students").Where(
		"workspace_id = ? AND status = ?",
		workspaceID,
		studentConstants.StatusEnrolled,
	)
}

func summarize(counts []majorCount, monitors []monitorName) studentdashboardModels.Data {
	// 按清理后的名称合并历史数据，空班级与空专业均保留人数。
	result := studentdashboardModels.Data{
		Majors: []string{},
		Items:  []studentdashboardModels.ClassSummary{},
		Totals: studentdashboardModels.Counts{MajorCounts: map[string]int64{}},
	}
	classes := map[string]*studentdashboardModels.ClassSummary{}
	classNames := []string{}
	for _, count := range counts {
		className, major := strings.TrimSpace(count.ClassName), strings.TrimSpace(count.Major)
		item, ok := classes[className]
		if !ok {
			item = &studentdashboardModels.ClassSummary{
				ClassName: className,
				Counts:    studentdashboardModels.Counts{MajorCounts: map[string]int64{}},
			}
			classes[className] = item
			classNames = append(classNames, className)
		}
		if _, exists := result.Totals.MajorCounts[major]; !exists {
			result.Majors = append(result.Majors, major)
		}
		item.MajorCounts[major] += count.Count
		item.Total += count.Count
		result.Totals.MajorCounts[major] += count.Count
		result.Totals.Total += count.Count
	}

	// 姓名只关联已汇总的班级，按独立职务匹配，不从职务推导权限。
	names := map[string][]string{}
	for _, monitor := range monitors {
		className := strings.TrimSpace(monitor.ClassName)
		if _, ok := classes[className]; !ok || !isMonitor(monitor.ClassPosition) {
			continue
		}
		if name := strings.TrimSpace(monitor.Name); name != "" {
			names[className] = append(names[className], name)
		}
	}
	slices.Sort(result.Majors)
	slices.Sort(classNames)
	for _, className := range classNames {
		values := names[className]
		slices.Sort(values)
		item := classes[className]
		item.Monitors = strings.Join(slices.Compact(values), ",")
		result.Items = append(result.Items, *item)
	}
	return result
}

func isMonitor(position string) bool {
	// 支持常见多职务分隔符，只接受独立的班长职务。
	positions := strings.FieldsFunc(position, func(value rune) bool {
		return unicode.IsSpace(value) || strings.ContainsRune(",，、;；/兼", value)
	})
	return slices.Contains(positions, studentdashboardConstants.MonitorPosition)
}

func event() auditModels.Event {
	return auditModels.Event{
		ObjectType: "student_dashboard",
		Conditions: auditModels.Snapshot{
			"status":   studentConstants.StatusEnrolled,
			"group_by": []string{"class_name", "major"},
		},
	}
}

func recordSummary(audit *auditModels.Event, data studentdashboardModels.Data) {
	// 审计记录聚合数量，不保存学生姓名或完整档案。
	audit.ReturnedCount = int64(len(data.Items))
	audit.Conditions["matched_students"] = data.Totals.Total
	audit.Conditions["major_count"] = len(data.Majors)
}

func exportRecords(data studentdashboardModels.Data) [][]string {
	// 第一列为班级，末两列固定为总数与班长，导出包含人数合计。
	header := append([]string{"班级"}, data.Majors...)
	header = append(header, "总数", "班长")
	records := make([][]string, 1, len(data.Items)+2)
	records[0] = header
	for _, item := range data.Items {
		row := countCells(item.ClassName, item.Counts, data.Majors)
		records = append(records, append(row, item.Monitors))
	}
	footer := countCells("合计", data.Totals, data.Majors)
	return append(records, append(footer, ""))
}

func countCells(label string, counts studentdashboardModels.Counts, majors []string) []string {
	// 无人数的专业单元格写入文本零，空姓名由调用方保留空单元格。
	row := make([]string, 1, len(majors)+3)
	row[0] = label
	for _, major := range majors {
		row = append(row, strconv.FormatInt(counts.MajorCounts[major], 10))
	}
	return append(row, strconv.FormatInt(counts.Total, 10))
}
