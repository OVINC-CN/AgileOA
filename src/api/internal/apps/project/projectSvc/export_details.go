package projectSvc

import (
	"database/sql"
	"strconv"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type detailExportRow struct {
	RecordID    uint64
	ProjectName string
	Name        string
	Description string
	Score       *float64
}

type detailExportKey struct {
	ProjectName string
	Name        string
	Occurrence  int
}

// exportDetails 仅将本次导出记录的评分子项转为列，不改变参与记录的行数。
func exportDetails(
	tx *gorm.DB,
	workspaceID string,
	participations []participationExportRow,
	baseColumns int,
) ([]string, map[uint64]map[int]string, error) {
	// 空结果保留基础表头，不读取其他参与记录的子项。
	headers := []string{}
	values := make(map[uint64]map[int]string, len(participations))
	if len(participations) == 0 {
		return headers, values, nil
	}
	ids := make([]uint64, len(participations))
	for index, record := range participations {
		ids[index] = record.ID
	}

	// 通过工作空间和参与编号限定范围，按项目、年份及子项顺序固定列的首次出现顺序。
	rows, err := participationScope(tx, workspaceID).
		Joins("JOIN project_participation_details AS d "+
			"ON d.workspace_id = r.workspace_id AND d.participation_id = r.id").
		Where("r.id IN ?", ids).
		Select("r.id AS record_id, p.name AS project_name, d.name, d.description, d.score").
		Order("p.name ASC, p.year ASC, r.id ASC, d.sort_order ASC, d.id ASC").Rows()
	if err != nil {
		return nil, nil, err
	}
	return detailExportColumns(tx, rows, baseColumns)
}

func detailExportColumns(tx *gorm.DB, rows *sql.Rows, baseColumns int) (
	[]string, map[uint64]map[int]string, error,
) {
	headers := []string{}
	values := map[uint64]map[int]string{}
	// 及时关闭数据库游标，不保留全部子项的中间副本。
	defer func() { _ = rows.Close() }()
	columns := map[detailExportKey]int{}
	usedHeaders := map[string]bool{}
	occurrences := map[string]int{}
	var previousID uint64
	for rows.Next() {
		var detail detailExportRow
		if err := tx.ScanRows(rows, &detail); err != nil {
			return nil, nil, err
		}
		if detail.RecordID != previousID {
			occurrences = map[string]int{}
			previousID = detail.RecordID
		}
		occurrences[detail.Name]++
		key := detailExportKey{detail.ProjectName, detail.Name, occurrences[detail.Name]}
		index, exists := columns[key]
		if !exists {
			if baseColumns+len(headers)+2 > excelize.MaxColumns {
				return nil, nil, invalid("明细列数超过 XLSX 上限，请缩小筛选范围")
			}
			index = len(headers)
			columns[key] = index
			header := uniqueExportHeader(detail.ProjectName+"-"+detail.Name, usedHeaders)
			headers = append(headers, header, uniqueExportHeader(header+"-说明", usedHeaders))
		}
		if values[detail.RecordID] == nil {
			values[detail.RecordID] = map[int]string{}
		}
		values[detail.RecordID][index] = exportScore(detail.Score)
		values[detail.RecordID][index+1] = detail.Description
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return headers, values, nil
}

func uniqueExportHeader(name string, used map[string]bool) string {
	// 同名子项及名称拼接冲突分别编号，避免覆盖已有列。
	header := name
	for occurrence := 2; used[header]; occurrence++ {
		header = name + "（" + strconv.Itoa(occurrence) + "）"
	}
	used[header] = true
	return header
}
