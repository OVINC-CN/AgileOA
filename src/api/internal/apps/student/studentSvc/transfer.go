package studentSvc

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
)

// Template 生成基础模板，按开关一次包含全部敏感字段，不按字段划分权限。
func Template(
	ctx context.Context,
	workspaceID string,
	includeSensitive bool,
) (studentModels.FileData, error) {
	var result studentModels.FileData
	fields := []studentConstants.Definition{}
	required := []string{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Basic || includeSensitive || field.Required {
			fields = append(fields, field)
		}
		if field.Required {
			required = append(required, field.Label)
		}
	}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentImport,
		studentEvent(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			header := make([]string, 0, len(fields))
			for _, field := range fields {
				if field.Key == "birth_date" {
					continue
				}
				header = append(header, field.Label)
				event.ReturnedIDs = append(event.ReturnedIDs, field.Key)
			}
			avatarExample := attachmentUtils.WithObjectPrefix(avatarPrefix(workspaceID) + "照片.png")
			notes := []string{
				"# 导入说明：所有业务字段按文本读取和存储；模板已将所有列设置为文本，保留学号、证件和电话的前导零。",
				"# 必填字段：" + strings.Join(required, "、") + "；其他列可以留空。",
				"# 只新增学生，不覆盖已有学号；任何一行失败时整批不保存。",
				"# 头像可留空，或填写原始 COS 对象键，例如 " + avatarExample + "。",
				"# 在第一张工作表填写数据，请保留列名行，不要重复列名；填写说明不参与导入。",
				"# 自由文本维度可填写已有名称或新名称；出生日期由身份证号码生成。",
				"# 来校时间和毕（肄）业时间使用 YYYY-MM，例如 2026-09；年份为 1900 至 9999，可留空。",
				"# 宗教信仰起始年份为文本，例如 2020年；不作为筛选条件。",
			}
			for _, field := range fields {
				if len(field.Options) == 0 {
					continue
				}
				options := make([]string, 0, len(field.Options))
				for _, option := range field.Options {
					if field.Required && option.Value == "unknown" {
						continue
					}
					options = append(options, option.Label+"（"+option.Value+"）")
				}
				blankNote := "；留空表示未填写。"
				if field.Required {
					blankNote = "；必填，不接受空值或未知。"
				}
				notes = append(
					notes,
					"# 枚举字段："+field.Label+"，仅填写 "+strings.Join(options, "、")+blankNote,
				)
			}
			notes = append(notes, "# 身份证号码为必填敏感字段，导入需要处理敏感信息权限。")
			var err error
			result, err = xlsxFile("student-template.xlsx", [][]string{header}, notes...)
			event.ReturnedCount = int64(len(header))
			return err
		},
	)
	return result, err
}

// Import 新增学生并逐条审计；任一行失败整批回滚，敏感字段统一鉴权。
func Import(
	ctx context.Context,
	workspaceID string,
	request studentModels.ImportRequest,
) (studentModels.ImportData, error) {
	var result studentModels.ImportData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.StudentImport,
		studentEvent(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			rows, err := parseWorkbook(request.Content)
			if err != nil {
				return err
			}
			for _, row := range rows {
				if len(row.Sensitive) > 0 {
					if err := accessUtils.Require(
						tx,
						principal,
						accessConstants.StudentSensitiveHandle,
					); err != nil {
						return err
					}
					break
				}
			}
			for index, row := range rows {
				student, err := create(tx, workspaceID, row.Input, row.Avatar)
				if err != nil {
					return importRowError(index+2, err)
				}
				student.BirthDate, err = saveSensitive(tx, workspaceID, student.ID, row.Sensitive)
				if err != nil {
					return importRowError(index+2, err)
				}
				student.BasicRecord, err = basicDetail(tx, workspaceID, student.ID)
				if err != nil {
					return importRowError(index+2, err)
				}
				item := studentEvent(workspaceID, student.ID)
				item.Action, item.Result = accessConstants.StudentImport, auditConstants.Success
				item.Conditions["row"] = index + 2
				item.After = basicSnapshot(student)
				if len(row.Sensitive) > 0 {
					item.SensitiveAfter = auditModels.Snapshot{}
					for key, value := range row.Sensitive {
						item.SensitiveAfter[key] = value
					}
				}
				if err := auditSvc.Write(tx, item); err != nil {
					return err
				}
				event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(student.ID, 10))
			}
			result.Created = len(rows)
			event.ReturnedCount = int64(result.Created)
			event.Conditions["rows"] = result.Created
			return nil
		},
	)
	return result, err
}

// Export 按筛选导出学生档案、XJ 明细或校外住宿明细，读取敏感字段前复核权限。
func Export(
	ctx context.Context,
	workspaceID string,
	query studentModels.ListQuery,
	includeSensitive bool,
	format string,
) (studentModels.FileData, error) {
	var result studentModels.FileData
	religionReport := format == studentConstants.ExportReligion
	offCampusReport := format == studentConstants.ExportOffCampus
	includeSensitive = includeSensitive || religionReport || offCampusReport
	event := queryEvent(workspaceID, query)
	event.Conditions["include_sensitive"] = includeSensitive
	if format != "" {
		event.Conditions["format"] = format
	}
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentExport, event,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			// 明细表固定包含敏感字段，调用方不能通过开关取消鉴权。
			if format != "" && !religionReport && !offCampusReport {
				return invalid("不支持此导出格式")
			}
			if includeSensitive {
				if err := accessUtils.Require(
					tx,
					principal,
					accessConstants.StudentSensitiveHandle,
				); err != nil {
					return err
				}
			}
			scope, err := filteredScope(tx, principal, query)
			if err != nil {
				return err
			}
			if religionReport {
				scope = scope.Where("s.has_religion = ?", studentConstants.FlagYes)
			}
			if offCampusReport {
				scope = scope.Where(
					"s.accommodation_type = ?",
					studentConstants.AccommodationOffCampus,
				)
			}
			var total int64
			if err := scope.Session(&gorm.Session{}).Count(&total).Error; err != nil {
				return err
			}
			if total > int64(config.Config.Student.MaxExportRows) {
				return invalid("导出数量超过限制，请缩小筛选范围")
			}
			// 各报表只读取自身使用的固定列，审计提交后才发送文件。
			scope = scope.Order("s.id DESC").
				Limit(config.Config.Student.MaxExportRows)
			if offCampusReport {
				result, err = exportOffCampusReport(scope, event)
			} else {
				result, err = exportStudentFile(scope, event, religionReport, includeSensitive)
			}
			if err != nil {
				return err
			}
			if len(
				result.Content,
			) > base64.StdEncoding.EncodedLen(
				config.Config.Student.MaxFileBytes,
			) {
				return invalid("导出文件超过限制，请缩小筛选范围")
			}
			event.ReturnedCount = int64(len(event.ReturnedIDs))
			return nil
		})
	return result, err
}

func exportStudentFile(
	scope *gorm.DB,
	event *auditModels.Event,
	religionReport, includeSensitive bool,
) (studentModels.FileData, error) {
	fields := transferFields(includeSensitive)
	columns := make([]string, 1, len(fields)+1)
	columns[0] = "s.id"
	if religionReport {
		columns = append(columns, religionReportColumns...)
	} else {
		for _, field := range fields {
			columns = append(columns, field.Column)
		}
	}
	var students []studentModels.Student
	if err := scope.Select(columns).Scan(&students).Error; err != nil {
		return studentModels.FileData{}, err
	}
	for _, student := range students {
		event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(student.ID, 10))
	}
	if religionReport {
		return xlsxFile("XJ 明细表.xlsx", religionReportRecords(students))
	}
	return xlsxFile("students.xlsx", studentExportRecords(students, fields, includeSensitive))
}

func studentExportRecords(
	students []studentModels.Student,
	fields []studentConstants.Definition,
	includeSensitive bool,
) [][]string {
	header := make([]string, 0, len(fields))
	for _, field := range fields {
		header = append(header, field.Label)
	}
	records := make([][]string, 1, len(students)+1)
	records[0] = header
	for _, student := range students {
		values := inputValues(studentInfo(student.BasicRecord).Input)
		values["avatar"] = student.Avatar
		values["birth_date"] = student.BirthDate
		if includeSensitive {
			for key, value := range sensitiveValues(student) {
				values[key] = value
			}
		}
		record := make([]string, 0, len(fields))
		for _, field := range fields {
			value := values[field.Key]
			if value != "" {
				value = studentConstants.Label(field, value)
			}
			record = append(record, value)
		}
		records = append(records, record)
	}
	return records
}

type importRow struct {
	Input     studentModels.Input
	Avatar    string
	Sensitive map[string]string
}

func parseWorkbook(content string) ([]importRow, error) {
	records, err := spreadsheet.Read(content, spreadsheet.Limits{
		MaxBytes:   config.Config.Student.MaxFileBytes,
		MaxRows:    config.Config.Student.MaxImportRows,
		MaxColumns: len(studentConstants.Fields) + 1,
	})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, invalid("文件为空或表头无效")
	}
	header := records[0]
	keys := make([]string, len(header))
	seen := map[string]bool{}
	for index, label := range header {
		key := headerKey(strings.TrimSpace(label))
		if key == "" || seen[key] {
			return nil, invalid("模板包含未知列或重复列")
		}
		seen[key] = true
		keys[index] = key
	}
	for _, field := range studentConstants.Fields {
		// 原始模板可从住宿园区推断住宿类型，仍在逐行校验中要求有效住宿类型。
		inferredAccommodation := field.Key == "accommodation_type" && seen["campus"]
		if field.Required && !seen[field.Key] && !inferredAccommodation {
			return nil, invalid("模板必须包含" + field.Label + "列")
		}
	}
	result := []importRow{}
	numbers := map[string]bool{}
	for index, record := range records[1:] {
		line := index + 2
		values := map[string]string{}
		for index, value := range record {
			value = strings.TrimSpace(value)
			values[keys[index]] = value
		}
		row, err := parseRow(values)
		if err != nil {
			return nil, importRowError(line, err)
		}
		if numbers[row.Input.StudentNo] {
			return nil, invalid(fmt.Sprintf("第 %d 行学号在文件内重复", line))
		}
		numbers[row.Input.StudentNo] = true
		result = append(result, row)
	}
	if len(result) == 0 {
		return nil, invalid("模板中没有学生记录")
	}
	return result, nil
}

func parseRow(values map[string]string) (importRow, error) {
	// 兼容没有住宿类型列的原始示例；显式留空由必填校验拒绝。
	if _, hasAccommodationType := values["accommodation_type"]; !hasAccommodationType {
		switch values["campus"] {
		case "校外住宿":
			values["accommodation_type"], values["campus"] = "off_campus", ""
		case "校内住宿":
			values["accommodation_type"], values["campus"] = "on_campus", ""
		default:
			if values["campus"] != "" {
				values["accommodation_type"] = "on_campus"
			}
		}
	}
	// 中文名称只在导入边界转换，业务判断始终使用英文代码。
	for _, field := range studentConstants.Fields {
		if field.Type == "enum" {
			switch values[field.Key] {
			case "未知", "未填写", "未填写／未知":
				values[field.Key] = "unknown"
			}
		}
		for _, option := range field.Options {
			if option.Label == values[field.Key] {
				values[field.Key] = option.Value
				break
			}
		}
	}
	row := importRow{Avatar: values["avatar"], Sensitive: map[string]string{}}
	var err error
	row.Input, err = cleanInput(inputFromValues(values))
	if err != nil {
		return row, err
	}
	private := []studentModels.FieldValue{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Sensitive &&
			(field.Required || values[field.Key] != "") {
			private = append(
				private,
				studentModels.FieldValue{Field: field.Key, Value: values[field.Key]},
			)
		}
	}
	if len(private) > 0 {
		row.Sensitive, err = validateSensitive(private)
	}
	return row, err
}

func headerKey(label string) string {
	for _, field := range studentConstants.Fields {
		if label == field.Label || label == field.Key {
			return field.Key
		}
	}
	aliases := map[string]string{
		"楼号": "building",
		"序号": "sequence", "班级职务": "class_position", "qq号码": "qq",
		"现家庭详细居住地（精确到门牌号）": "home_address", "与本人关系": "guardian1_relationship",
		"联系方式1": "guardian1_phone", "联系方式2": "guardian2_phone", "是否有宗教信仰": "has_religion",
		"其他要说明的本人或家庭特殊情况": "special_circumstances",
	}
	return aliases[label]
}

func transferFields(includeSensitive bool) []studentConstants.Definition {
	fields := []studentConstants.Definition{}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Basic || includeSensitive {
			fields = append(fields, field)
		}
	}
	return fields
}

func xlsxFile(
	filename string,
	records [][]string,
	notes ...string,
) (studentModels.FileData, error) {
	content, err := spreadsheet.Encode(records, notes...)
	if err != nil {
		return studentModels.FileData{}, err
	}
	return studentModels.FileData{
		Filename:    filename,
		ContentType: spreadsheet.ContentType,
		Content:     content,
	}, nil
}

func importRowError(line int, cause error) error {
	// 仅公开既有的业务校验消息，组件异常和密钥错误保持原始服务状态。
	var public *response.BusinessError
	if errors.As(cause, &public) {
		return &response.BusinessError{Status: public.Status,
			Message: fmt.Sprintf("第 %d 行：%s", line, public.Message)}
	}
	return cause
}
