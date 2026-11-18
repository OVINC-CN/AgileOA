package projectSvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

type projectExportRow struct {
	projectModels.ProjectInput
	ID              uint64
	AttachmentCount int64
}

type participationExportRow struct {
	ID                 uint64
	ProjectName        string
	Year               int
	ProjectDescription string
	StudentNo          string
	StudentName        string
	EducationLevel     string
	Major              string
	Grade              string
	Rank               *int32
	Score              *float64
}

// Export 按列表筛选导出全部项目或获奖学生明细，独立授权并在审计提交后返回。
func Export(
	ctx context.Context,
	workspaceID string,
	query projectModels.Query,
	participation bool,
	format string,
) (projectModels.FileData, error) {
	// 导出没有分页，审计只保留实际生效的筛选条件。
	action := accessConstants.ProjectExport
	audit := queryEvent(workspaceID, query)
	delete(audit.Conditions, "page")
	delete(audit.Conditions, "page_size")
	if participation {
		action = accessConstants.ParticipationExport
		audit.ObjectType = "project_participation"
	}
	var result projectModels.FileData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		action,
		audit,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 与列表使用相同的校验，禁止通过导出放宽子项搜索范围。
			if err := validQuery(query); err != nil {
				return err
			}
			if !participation && query.DetailKeyword != "" {
				return invalid("子项搜索仅支持获奖学生列表")
			}
			if format == "" {
				format = projectConstants.ExportBasic
			}
			if format != projectConstants.ExportBasic && format != projectConstants.ExportFull {
				return invalid("导出类型必须为获奖学生或获奖明细")
			}
			audit.Conditions["format"] = format

			// 普通学生导出只读取基础列，完整明细才追加子项列。
			var records [][]string
			var err error
			filename := "奖项.xlsx"
			note := "按当前筛选导出全部项目；附件数量只统计已完成上传的附件。"
			if participation {
				records, err = exportParticipations(
					tx,
					principal.Workspace.ID,
					query,
					format,
					audit,
				)
				filename = "获奖学生.xlsx"
				note = "每条获奖记录单独一行，排名与总分独立填写，未填写时保持空单元格。"
				if format == projectConstants.ExportFull {
					filename = "获奖明细.xlsx"
					note += "子项分数列名为“项目名称-子项名称”，说明另列；同名子项按出现顺序编号。"
				}
			} else {
				records, err = exportProjects(tx, principal.Workspace.ID, query, audit)
			}
			if err != nil {
				return err
			}
			result, err = exportSpreadsheet(records, filename, note)
			if err != nil {
				return err
			}

			// 返回数量按业务记录去重，实际工作表行数另存审计条件。
			audit.ReturnedCount = int64(len(audit.ReturnedIDs))
			audit.Conditions["rows"] = len(records) - 1
			return nil
		},
	)
	return result, err
}

func exportSpreadsheet(records [][]string, filename, note string) (projectModels.FileData, error) {
	content, err := spreadsheet.Encode(records, note)
	if err != nil {
		return projectModels.FileData{}, err
	}
	fileSize := base64.RawStdEncoding.DecodedLen(len(strings.TrimRight(content, "=")))
	if fileSize > config.Config.Project.MaxFileBytes {
		return projectModels.FileData{}, invalid("导出文件超过限制，请缩小筛选范围")
	}
	return projectModels.FileData{
		Filename: filename, ContentType: spreadsheet.ContentType, Content: content,
	}, nil
}

func exportProjects(
	tx *gorm.DB,
	workspaceID string,
	query projectModels.Query,
	audit *auditModels.Event,
) ([][]string, error) {
	// 附件计数限定当前项目与工作空间，不读取文件名、存储地址或签名链接。
	var rows []projectExportRow
	err := projectListScope(tx, workspaceID, query).
		Table("projects FORCE INDEX (uk_projects_name_year)").
		Select("projects.id, projects.name, projects.year, projects.description, projects.allow_self_apply, "+
			"projects.self_apply_deadline, "+
			"(SELECT COUNT(*) FROM project_attachments AS a "+
			"WHERE a.workspace_id = projects.workspace_id AND a.project_id = projects.id "+
			"AND a.state = ?) AS attachment_count", attachmentConstants.Ready).
		Order(projectConstants.ProjectOrder).
		Limit(config.Config.Project.MaxExportRows + 1).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) > config.Config.Project.MaxExportRows {
		return nil, invalid("导出行数超过限制，请缩小筛选范围")
	}

	// 批量读取导出项目的子项配置，包含尚未用于评分的名称。
	ids := make([]uint64, len(rows))
	for index, row := range rows {
		ids[index] = row.ID
	}
	options := []projectModels.DetailOption{}
	if len(ids) > 0 {
		if err := tx.Where("workspace_id = ? AND project_id IN ?", workspaceID, ids).
			Order("project_id ASC, name ASC").Find(&options).Error; err != nil {
			return nil, err
		}
	}
	names := make(map[uint64][]string, len(rows))
	for _, option := range options {
		names[option.ProjectID] = append(names[option.ProjectID], option.Name)
	}

	// 所有单元格均由公共 XLSX 编码器按文本写入。
	records := make([][]string, 1, len(rows)+1)
	records[0] = []string{
		"项目名称", "归属年份", "说明", "附件数量", "子项名称(JSON)", "允许自行申报",
		"自行申报截止时间",
	}
	for _, row := range rows {
		configured := names[row.ID]
		if configured == nil {
			configured = []string{}
		}
		encoded, err := json.Marshal(configured)
		if err != nil {
			return nil, err
		}
		deadline := ""
		if row.SelfApplyDeadline != nil {
			deadline = row.SelfApplyDeadline.UTC().Format(time.RFC3339Nano)
		}
		records = append(records, []string{
			row.Name,
			strconv.Itoa(row.Year),
			row.Description,
			strconv.FormatInt(row.AttachmentCount, 10),
			string(encoded),
			strconv.FormatBool(row.AllowSelfApply),
			deadline,
		})
		audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(row.ID, 10))
	}
	return records, nil
}

func exportParticipations(
	tx *gorm.DB,
	workspaceID string,
	query projectModels.Query,
	format string,
	audit *auditModels.Event,
) ([][]string, error) {
	// 基础数据和明细均按参与记录计数，不让子项数量改变导出行数。
	var rows []participationExportRow
	err := participationListScope(tx, workspaceID, query).
		Select("r.id, p.name AS project_name, p.year, p.description AS project_description, " +
			"s.student_no, s.name AS student_name, s.education_level, s.major, s.grade, r.rank, r.score").
		Order(projectConstants.ParticipationOrder).
		Limit(config.Config.Project.MaxExportRows + 1).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) > config.Config.Project.MaxExportRows {
		return nil, invalid("导出行数超过限制，请缩小筛选范围")
	}

	// 只有完整明细读取评分子项，基础导出不包含子项正文或数量。
	records := make([][]string, 1, len(rows)+1)
	records[0] = []string{
		"项目名称", "归属年份", "项目说明", "学号", "姓名", "学历", "专业", "年级", "排名", "总分",
	}
	var detailHeaders []string
	var detailValues map[uint64]map[int]string
	if format == projectConstants.ExportFull {
		detailHeaders, detailValues, err = exportDetails(tx, workspaceID, rows, len(records[0]))
		if err != nil {
			return nil, err
		}
		records[0] = append(records[0], detailHeaders...)
	}
	for _, row := range rows {
		values := []string{
			row.ProjectName, strconv.Itoa(row.Year), row.ProjectDescription,
			row.StudentNo, row.StudentName, row.EducationLevel, row.Major, row.Grade,
			exportRank(row.Rank), exportScore(row.Score),
		}
		if len(detailHeaders) > 0 {
			details := make([]string, len(detailHeaders))
			for index, value := range detailValues[row.ID] {
				details[index] = value
			}
			values = append(values, details...)
		}
		records = append(records, values)
		audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(row.ID, 10))
	}
	return records, nil
}

func exportRank(value *int32) string {
	// 未填写的排名保持空单元格。
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

func exportScore(value *float64) string {
	// 空值保持空单元格，零及双精度小数保留原值。
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
