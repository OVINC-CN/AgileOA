package projectSvc

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

type applicationExportRow struct {
	projectModels.ApplicationSummary
	ProjectDescription string
	EducationLevel     string
	Major              string
	Grade              string
	Statement          string
	Rank               *int32
	Score              *float64
}

// ExportApplications 按填报管理可见范围和筛选导出全部申报，子项列复用获奖明细规则。
func ExportApplications(
	ctx context.Context,
	workspaceID string,
	query projectModels.ApplicationQuery,
) (
	projectModels.FileData, error,
) {
	var result projectModels.FileData
	err := applicationAccess(
		ctx, workspaceID, accessConstants.ProjectApplicationExport,
		event(workspaceID, "project_application", 0),
		func(tx *gorm.DB, actor applicationActor, audit *auditModels.Event) error {
			// 独立授权后仍按原列表收窄单据范围，不通过导出扩大审批人的读取权限。
			if err := validateApplicationQuery(query); err != nil {
				return err
			}
			audit.Conditions = applicationFilterSnapshot(query, projectConstants.ApplicationManage)
			audit.Conditions["format"] = projectConstants.ExportFull
			var rows []applicationExportRow
			if err := applicationQueryScope(
				tx,
				workspaceID,
				projectConstants.ApplicationManage,
				actor,
				query,
			).
				Select("a.id, p.name AS project_name, p.year, p.description AS project_description, " +
					"s.student_no, s.name AS student_name, s.education_level, s.major, s.grade, s.class_name, " +
					"a.statement, a.status, a.current_step, a.rank, a.score").
				Order("a.id DESC").
				Limit(config.Config.Project.MaxExportRows + 1).
				Scan(&rows).
				Error; err != nil {
				return err
			}
			if len(rows) > config.Config.Project.MaxExportRows {
				return invalid("导出行数超过限制，请缩小筛选范围")
			}
			ids := make([]uint64, len(rows))
			for index, row := range rows {
				ids[index] = row.ID
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(row.ID, 10))
			}
			reviewers, err := applicationExportReviewers(tx, workspaceID, ids)
			if err != nil {
				return err
			}
			for index := range rows {
				if rows[index].Status != projectConstants.ApplicationApproved {
					rows[index].CurrentReviewer = reviewers[rows[index].ID]
				}
			}
			headers := applicationExportHeaders()
			detailHeaders, detailValues, err := applicationExportDetails(
				tx,
				workspaceID,
				ids,
				len(headers),
			)
			if err != nil {
				return err
			}
			records := applicationExportRecords(rows, headers, detailHeaders, detailValues)
			result, err = exportSpreadsheet(
				records,
				"填报明细.xlsx",
				"按当前填报管理筛选导出全部可见申报，每张申报单独一行；"+
					"子项按“项目名称-子项名称”展开分数和说明，同名子项按出现顺序编号。",
			)
			if err != nil {
				return err
			}
			audit.ReturnedCount = int64(len(rows))
			audit.Conditions["rows"] = len(rows)
			return nil
		},
	)
	return result, err
}

func applicationExportHeaders() []string {
	return []string{
		"申报编号", "项目名称", "归属年份", "项目说明", "学号", "姓名", "学历", "专业", "年级",
		"班级", "申报说明", "审批状态", "当前处理人", "排名", "总分",
	}
}

func applicationExportRecords(rows []applicationExportRow, headers, detailHeaders []string,
	detailValues map[uint64]map[int]string,
) [][]string {
	records := make([][]string, 1, len(rows)+1)
	records[0] = append(headers, detailHeaders...)
	for _, row := range rows {
		values := make([]string, len(headers)+len(detailHeaders))
		copy(values, []string{
			strconv.FormatUint(
				row.ID,
				10,
			),
			row.ProjectName,
			strconv.Itoa(row.Year),
			row.ProjectDescription,
			row.StudentNo,
			row.StudentName,
			row.EducationLevel,
			row.Major,
			row.Grade,
			row.ClassName,
			row.Statement,
			applicationStatusLabel(row.Status),
			row.CurrentReviewer,
			exportRank(row.Rank),
			exportScore(row.Score),
		})
		for index, value := range detailValues[row.ID] {
			values[len(headers)+index] = value
		}
		records = append(records, values)
	}
	return records
}

func applicationStatusLabel(status string) string {
	switch status {
	case projectConstants.ApplicationDraft:
		return "草稿"
	case projectConstants.ApplicationPending:
		return "审批中"
	case projectConstants.ApplicationRejected:
		return "已驳回"
	case projectConstants.ApplicationApproved:
		return "已通过"
	default:
		return status
	}
}

func applicationExportReviewers(
	tx *gorm.DB,
	workspaceID string,
	ids []uint64,
) (map[uint64]string, error) {
	result := map[uint64]string{}
	if len(ids) == 0 {
		return result, nil
	}
	var nodes []projectModels.ApplicationNode
	if err := tx.Table("project_application_nodes AS n").
		Joins("JOIN project_applications AS a ON a.workspace_id = n.workspace_id "+
			"AND a.id = n.application_id AND a.current_step = n.position").
		Where("n.workspace_id = ? AND a.id IN ?", workspaceID, ids).
		Select("n.*").
		Order("n.application_id ASC, n.target_order ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	labels := map[uint64][]string{}
	for _, node := range nodes {
		labels[node.ApplicationID] = append(labels[node.ApplicationID], node.ReviewerLabel)
	}
	for id, names := range labels {
		result[id] = strings.Join(names, "、")
	}
	return result, nil
}

func applicationExportDetails(tx *gorm.DB, workspaceID string, ids []uint64, baseColumns int) (
	[]string, map[uint64]map[int]string, error,
) {
	if len(ids) == 0 {
		return []string{}, map[uint64]map[int]string{}, nil
	}
	// 仅按本次已授权的申报编号读取子项，附件及私有对象地址不进入导出。
	rows, err := tx.Table("project_application_details AS d").
		Joins("JOIN project_applications AS a ON a.workspace_id = d.workspace_id AND a.id = d.application_id").
		Joins("JOIN projects AS p ON p.workspace_id = a.workspace_id AND p.id = a.project_id").
		Where("d.workspace_id = ? AND a.id IN ?", workspaceID, ids).
		Select("a.id AS record_id, p.name AS project_name, d.name, d.description, d.score").
		Order("p.name ASC, p.year ASC, a.id ASC, d.sort_order ASC, d.id ASC").Rows()
	if err != nil {
		return nil, nil, err
	}
	return detailExportColumns(tx, rows, baseColumns)
}
