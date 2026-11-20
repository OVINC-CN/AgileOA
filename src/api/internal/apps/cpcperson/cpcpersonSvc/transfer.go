package cpcpersonSvc

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/spreadsheet"
)

var importHeader = []string{"学号", "发展阶段", "状态", "党内职务", "时间线模板"}

var exportHeader = []string{
	"学号", "姓名", "发展阶段", "状态", "所属党支部", "党支部简称", "党内职务", "性别", "民族", "年级",
	"学生状态", "学历", "班级", "专业", "导师", "联系电话", "来校时间",
	"毕（肄）业学校及单位", "毕（肄）业时间", "时间线模板",
}

// Template 仅生成固定模板，不创建任何人员或预置职务。
func Template(ctx context.Context, workspaceID string) (cpcpersonModels.FileData, error) {
	var result cpcpersonModels.FileData
	err := within(ctx, workspaceID, accessConstants.CPCPersonImport, event(workspaceID, 0),
		func(_ *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(accessConstants.CPCPersonImport); err != nil {
				return err
			}
			notes := []string{
				"# 学号、发展阶段、时间线模板必填，按本空间学号匹配已有学生；只新增党建记录。",
				"# 所属党支部由准入规则自动确定，优先级越大越优先，同值取编号最大者；无匹配时拒绝导入。",
				"# 时间线模板填写本空间已存在的完整模板名称，导入时绑定并生成待处理节点。",
				"# 状态可自定义，最多 8 字符，省略或留空时默认为在编，与学生状态独立。",
				"# 党内职务可以自定义，不参与书记身份判断；基础信息请在学生信息中维护。",
				"# 发展阶段：applicant 入党申请人、activist 入党积极分子、" +
					"development_candidate 发展对象、probationary_member 中共预备党员、" +
					"full_member 中共党员。",
			}
			var err error
			result, err = xlsxFile("cpc-people-template.xlsx", [][]string{importHeader}, notes...)
			audit.ReturnedCount = int64(len(importHeader))
			return err
		})
	return result, err
}

// Import 在同一空间锁和事务内新增全部记录，逐条审计，任一行失败全部回滚。
func Import(
	ctx context.Context,
	workspaceID string,
	request cpcpersonModels.ImportRequest,
) (cpcpersonModels.ImportData, error) {
	var result cpcpersonModels.ImportData
	ids := []string{}
	err := within(ctx, workspaceID, accessConstants.CPCPersonImport, event(workspaceID, 0),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if err := access.require(accessConstants.CPCPersonImport); err != nil {
				return err
			}
			rows, err := parseWorkbook(request.Content)
			if err != nil {
				return err
			}
			for index, row := range rows {
				id, err := importOne(tx, workspaceID, access, row)
				if err != nil {
					return importError(index+2, err)
				}
				info, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonImport)
				if err != nil {
					return err
				}
				item := event(workspaceID, id)
				item.Action, item.Result = accessConstants.CPCPersonImport, auditConstants.Success
				item.After = snapshot(info)
				item.Conditions["row"] = index + 2
				if err := auditSvc.Write(tx, item); err != nil {
					return err
				}
				ids = append(ids, strconv.FormatUint(id, 10))
			}
			result.Created = len(ids)
			audit.ReturnedIDs, audit.ReturnedCount = ids, int64(len(ids))
			return nil
		})
	if err == nil && result.Created > 0 {
		logger.Logger.WithContext(ctx).Infof(
			"CPC people imported; action: %s; workspace: %s; people: %v",
			accessConstants.CPCPersonImport, workspaceID, ids,
		)
	}
	return result, err
}

func importOne(
	tx *gorm.DB,
	workspaceID string,
	access personAccess,
	row map[string]string,
) (uint64, error) {
	var student struct{ ID uint64 }
	if err := tx.Table("students").Select("id").
		Where("workspace_id = ? AND student_no = ?", workspaceID, row["学号"]).
		Take(&student).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, invalid("请先在学生信息中维护对应学号")
		}
		return 0, err
	}
	input := cpcpersonModels.Input{
		StudentID: student.ID, Status: row["状态"], PartyPosition: row["党内职务"],
	}
	input.Stage, _ = cpcpersonConstants.StageValue(row["发展阶段"])
	input, err := clean(input)
	if err != nil {
		return 0, err
	}
	branchID, err := determineBranch(tx, workspaceID, input.StudentID)
	if err != nil {
		return 0, err
	}
	if !access.permits(accessConstants.CPCPersonImport, branchID) {
		return 0, missing()
	}
	var template struct{ ID uint64 }
	if err := tx.Table("cpc_timeline_templates").Select("id").
		Where("workspace_id = ? AND name = ?", workspaceID, row["时间线模板"]).
		Take(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, invalid("请选择当前空间存在的时间线模板名称")
		}
		return 0, err
	}
	templateID := template.ID
	normal := cpcpersonUtils.TimelineNormal
	record := cpcpersonModels.Person{
		WorkspaceID:        workspaceID,
		StudentID:          input.StudentID,
		Stage:              input.Stage,
		Status:             input.Status,
		PartyBranchID:      branchID,
		PartyPosition:      input.PartyPosition,
		TimelineTemplateID: &templateID,
		TimelineStatus:     &normal,
		Revision:           1,
	}
	if err := tx.Create(&record).Error; err != nil {
		return 0, writeError(err)
	}
	err = createTimelineNodes(
		tx, workspaceID, accessConstants.CPCPersonImport, record.ID, templateID,
	)
	return record.ID, err
}

// Export 使用独立导出授权，筛选、记录与数量全部限定在当前读取范围。
func Export(
	ctx context.Context, workspaceID string, query cpcpersonModels.Query,
) (cpcpersonModels.FileData, error) {
	var result cpcpersonModels.FileData
	err := within(
		ctx,
		workspaceID,
		accessConstants.CPCPersonExport,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			format := query.Format
			if format == "" {
				format = "basic"
			}
			if format != "basic" && format != "full" {
				return invalid("导出类型必须为基础数据或完整数据")
			}
			audit.Conditions["format"] = format
			scope, err := filtered(tx, workspaceID, query, access, accessConstants.CPCPersonExport)
			if err != nil {
				return err
			}
			var people []cpcpersonModels.Info
			if err := scope.Order("p.id DESC").
				Limit(config.Config.CPCPerson.MaxExportRows + 1).Find(&people).Error; err != nil {
				return err
			}
			if len(people) > config.Config.CPCPerson.MaxExportRows {
				return invalid("导出记录超过限制，请缩小筛选范围")
			}
			records := make([][]string, 1, len(people)+1)
			records[0] = append([]string{}, exportHeader...)
			timelineRows := map[uint64][]string{}
			filename := "cpc-people.xlsx"
			if format == "full" {
				headers, values, err := timelineExport(tx, workspaceID, people)
				if err != nil {
					return err
				}
				records[0] = append(records[0], headers...)
				timelineRows = values
				filename = "cpc-people-full.xlsx"
				audit.Conditions["timeline_columns"] = len(headers)
			}
			for _, person := range people {
				stage := person.Stage
				for _, option := range cpcpersonConstants.Stages {
					if option.Value == stage {
						stage = option.Label
						break
					}
				}
				row := []string{
					person.StudentNo,
					person.Name,
					stage,
					person.Status,
					person.PartyBranchName,
					person.PartyBranchShortName,
					person.PartyPosition,
					person.Gender,
					person.Ethnicity,
					person.Grade,
					person.StudentStatus,
					person.EducationLevel,
					person.ClassName,
					person.Major,
					person.Advisor,
					person.Phone,
					person.ArrivalMonth,
					person.PreviousSchoolOrEmployer,
					person.PreviousGraduationMonth,
					person.TimelineTemplateName,
				}
				row = append(row, timelineRows[person.ID]...)
				records = append(records, row)
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(person.ID, 10))
			}
			audit.ReturnedCount = int64(len(people))
			result, err = xlsxFile(filename, records)
			if err == nil && len(result.Content) > base64.StdEncoding.EncodedLen(
				config.Config.CPCPerson.MaxFileBytes,
			) {
				return invalid("导出文件超过限制，请缩小筛选范围")
			}
			return err
		},
	)
	return result, err
}

func parseWorkbook(content string) ([]map[string]string, error) {
	records, err := spreadsheet.Read(content, spreadsheet.Limits{
		MaxBytes:   config.Config.CPCPerson.MaxFileBytes,
		MaxRows:    config.Config.CPCPerson.MaxImportRows,
		MaxColumns: len(exportHeader),
	})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, invalid("文件为空或表头无效")
	}
	header := records[0]
	seen := map[string]bool{}
	for index, label := range header {
		label = strings.TrimSpace(label)
		known := false
		for _, allowed := range exportHeader {
			known = known || label == allowed
		}
		if !known || seen[label] {
			return nil, invalid("文件包含未知或重复列")
		}
		header[index], seen[label] = label, true
	}
	if !seen["学号"] || !seen["发展阶段"] || !seen["时间线模板"] {
		return nil, invalid("文件必须包含学号、发展阶段和时间线模板")
	}
	rows := []map[string]string{}
	numbers := map[string]bool{}
	for _, record := range records[1:] {
		row := map[string]string{}
		for index, value := range record {
			value = strings.TrimSpace(value)
			row[header[index]] = value
		}
		if row["学号"] == "" || !validText(row["学号"], 32) || numbers[row["学号"]] {
			return nil, invalid("学号为空、重复或格式无效")
		}
		if _, ok := cpcpersonConstants.StageValue(row["发展阶段"]); !ok {
			return nil, invalid("发展阶段无效")
		}
		if !validText(row["状态"], cpcpersonConstants.StatusLength) {
			return nil, invalid("状态最多 8 字符，且不能包含控制字符")
		}
		if row["时间线模板"] == "" || !validText(row["时间线模板"], 100) {
			return nil, invalid("时间线模板必填且最多 100 字符")
		}
		if !validText(row["所属党支部"], 200) || !validText(row["党支部简称"], 100) ||
			!validText(row["党内职务"], cpcpersonConstants.PositionLength) {
			return nil, invalid("所属党支部或党内职务格式无效")
		}
		numbers[row["学号"]] = true
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, invalid("文件中没有人员记录")
	}
	return rows, nil
}

func xlsxFile(
	name string,
	records [][]string,
	notes ...string,
) (cpcpersonModels.FileData, error) {
	content, err := spreadsheet.Encode(records, notes...)
	return cpcpersonModels.FileData{
		Filename: name, ContentType: spreadsheet.ContentType, Content: content,
	}, err
}

func importError(line int, err error) error {
	var business *response.BusinessError
	if errors.As(err, &business) {
		return &response.BusinessError{
			Status: business.Status, Message: fmt.Sprintf("第 %d 行：%s", line, business.Message),
		}
	}
	return err
}
