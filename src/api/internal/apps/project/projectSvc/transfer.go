package projectSvc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

func transferAction(participation bool) string {
	if participation {
		return accessConstants.ParticipationImport
	}
	return accessConstants.ProjectImport
}
func headers(participation bool) []string {
	if participation {
		return []string{"项目名称", "归属年份", "学号", "排名", "总分", "评分子项(JSON)"}
	}
	return []string{"项目名称", "归属年份", "说明", "子项名称(JSON)"}
}

// Template 返回文本格式 XLSX 模板，评分子项列填写带 name、description、score 的 JSON 数组。
func Template(
	ctx context.Context,
	workspaceID string,
	participation bool,
) (projectModels.FileData, error) {
	var result projectModels.FileData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		transferAction(participation),
		event(workspaceID, "project", 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			note := "子项名称(JSON)填写名称数组，例如 [\"志愿服务\",\"社会实践\"]，留空表示尚未配置。" +
				"仅导入项目基础资料，审批设置和分数计算配置在项目编辑中单独维护。"
			if participation {
				note = "评分子项(JSON)填写包含 name、description、score 的数组；" +
					"name 只能使用所属项目已配置的子项名称，单条记录内不能重复，不允许新增名称。"
			}
			content, err := spreadsheet.Encode([][]string{headers(participation)}, note)
			if err != nil {
				return err
			}
			name := "projects.xlsx"
			if participation {
				name = "project-participations.xlsx"
			}
			result = projectModels.FileData{
				Filename:    name,
				ContentType: spreadsheet.ContentType,
				Content:     content,
			}
			audit.ReturnedCount = 1
			return nil
		},
	)
	return result, err
}

// Import 整批新增项目或参与记录，任一校验、外键或审计失败全部回滚。
func Import(
	ctx context.Context,
	workspaceID string,
	request projectModels.ImportRequest,
	participation bool,
) (projectModels.ImportData, error) {
	var result projectModels.ImportData
	ids := []uint64{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		transferAction(participation),
		event(workspaceID, "project", 0),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			records, err := spreadsheet.Read(request.Content, spreadsheet.Limits{
				MaxBytes:   config.Config.Project.MaxFileBytes,
				MaxRows:    config.Config.Project.MaxImportRows,
				MaxColumns: len(headers(participation)),
			})
			if err != nil {
				return err
			}
			if len(records) < 2 {
				return invalid("文件中没有导入记录")
			}
			validHeader := strings.Join(
				records[0],
				"\x00",
			) == strings.Join(
				headers(participation),
				"\x00",
			)
			if !validHeader {
				return invalid("文件表头与模板不一致")
			}
			for _, row := range records[1:] {
				var id uint64
				if participation {
					id, err = importParticipation(tx, workspaceID, row)
				} else {
					id, err = importProject(tx, workspaceID, row)
				}
				if err != nil {
					return err
				}
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(id, 10))
				ids = append(ids, id)
				result.Created++
			}
			audit.After, audit.ReturnedCount = auditModels.Snapshot{
				"created": result.Created,
			}, int64(
				result.Created,
			)
			return nil
		},
	)
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Project records imported; workspace: %s; participations: %t; records: %v",
			workspaceID,
			participation,
			ids,
		)
	}
	return result, err
}

func importProject(tx *gorm.DB, workspaceID string, row []string) (uint64, error) {
	year, err := strconv.Atoi(row[1])
	if err != nil {
		return 0, invalid("项目归属年份无效")
	}
	input := projectModels.ProjectInput{
		Name: row[0], Year: year, Description: row[2],
	}
	if row[3] != "" && json.Unmarshal([]byte(row[3]), &input.DetailNames) != nil {
		return 0, invalid("子项名称必须是有效的 JSON 字符串数组")
	}
	input, err = cleanProject(input)
	if err != nil {
		return 0, err
	}
	record := projectModels.Project{WorkspaceID: workspaceID, Name: input.Name, Year: input.Year,
		Description: input.Description, Revision: 1}
	err = writeError(tx.Create(&record).Error)
	if err == nil {
		err = saveDetailOptions(tx, workspaceID, record.ID, input.DetailNames)
	}
	return record.ID, err
}

func importParticipation(tx *gorm.DB, workspaceID string, row []string) (uint64, error) {
	year, err := strconv.Atoi(row[1])
	if err != nil {
		return 0, invalid("项目归属年份无效")
	}
	var project projectModels.Project
	if err := tx.Where(
		"workspace_id = ? AND name = ? AND year = ?",
		workspaceID,
		strings.TrimSpace(row[0]),
		year,
	).
		Take(&project).
		Error; err != nil {
		return 0, lookupError(err)
	}
	var student struct{ ID uint64 }
	if err := tx.Table("students").
		Select("id").
		Where("workspace_id = ? AND student_no = ?", workspaceID, strings.TrimSpace(row[2])).
		Take(&student).
		Error; err != nil {
		return 0, lookupError(err)
	}
	input := projectModels.ParticipationInput{
		ParticipationFields: projectModels.ParticipationFields{
			ProjectID: project.ID,
			StudentID: student.ID,
		},
	}
	if row[3] != "" {
		rank, err := strconv.ParseInt(row[3], 10, 32)
		if err != nil || rank < 1 {
			return 0, invalid("排名必须为 1 至 2147483647 的正整数或留空")
		}
		value := int32(rank)
		input.Rank = &value
	}
	if row[4] != "" {
		score, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return 0, invalid("总分必须是有效的双精度数字")
		}
		input.Score = &score
	}
	if row[5] != "" {
		decoder := json.NewDecoder(strings.NewReader(row[5]))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input.Details) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) {
			return 0, invalid("评分子项必须是有效的 JSON 数组")
		}
	}
	input, err = cleanParticipation(input)
	if err != nil {
		return 0, err
	}
	if err := validateDetailNames(tx, workspaceID, input.ProjectID, input.Details); err != nil {
		return 0, err
	}
	record := projectModels.Participation{
		WorkspaceID: workspaceID,
		ProjectID:   input.ProjectID,
		StudentID:   input.StudentID,
		Rank:        input.Rank,
		Score:       input.Score,
		Revision:    1,
	}
	err = writeError(tx.Create(&record).Error)
	if err == nil {
		err = saveDetails(tx, workspaceID, record.ID, input.ProjectID, input.Details)
	}
	return record.ID, err
}
