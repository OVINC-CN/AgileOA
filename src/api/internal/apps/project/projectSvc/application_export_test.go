package projectSvc

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func mockDetailExportRows(t *testing.T, data *sqlmock.Rows) (*gorm.DB, *sql.Rows) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = connection.Close()
	})
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn: connection, SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT export_details").WillReturnRows(data).RowsWillBeClosed()
	rows, err := connection.QueryContext(t.Context(), "SELECT export_details")
	if err != nil {
		t.Fatal(err)
	}
	return database.WithContext(t.Context()), rows
}

func TestSharedDetailExportColumns(t *testing.T) {
	// 同名跨年度复用列，同一申报内的重名及说明名称冲突分别保留，空值和零不混淆。
	data := sqlmock.NewRows([]string{"record_id", "project_name", "name", "description", "score"}).
		AddRow(7, "优秀学生", "志愿服务", "首次说明", 0.0).
		AddRow(7, "优秀学生", "志愿服务", "再次说明", 5.123456).
		AddRow(7, "优秀学生", "志愿服务-说明", "名称冲突", 2.0).
		AddRow(8, "优秀学生", "志愿服务", "另一年度", nil)
	tx, rows := mockDetailExportRows(t, data)
	headers, values, err := detailExportColumns(tx, rows, len(applicationExportHeaders()))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"优秀学生-志愿服务", "优秀学生-志愿服务-说明",
		"优秀学生-志愿服务（2）", "优秀学生-志愿服务（2）-说明",
		"优秀学生-志愿服务-说明（2）", "优秀学生-志愿服务-说明（2）-说明",
	}
	if !reflect.DeepEqual(headers, want) || values[7][0] != "0" || values[7][2] != "5.123456" ||
		values[7][4] != "2" || values[8][0] != "" || values[8][1] != "另一年度" {
		t.Fatalf("共享明细列规则不符合预期：%v %v", headers, values)
	}
}

func TestDetailExportColumnLimit(t *testing.T) {
	data := sqlmock.NewRows([]string{"record_id", "project_name", "name", "description", "score"}).
		AddRow(7, "优秀学生", "志愿服务", "说明", 1.0)
	tx, rows := mockDetailExportRows(t, data)
	if _, _, err := detailExportColumns(tx, rows, excelize.MaxColumns-1); err == nil {
		t.Fatal("超过 XLSX 列数上限时应整体拒绝导出")
	}
}

func TestApplicationExportRecords(t *testing.T) {
	rank, score := int32(1), 0.0
	rows := []applicationExportRow{
		{
			ApplicationSummary: projectModels.ApplicationSummary{
				ID: 7, ProjectName: "优秀学生", Year: 2026, StudentNo: "0007", StudentName: "虚构学生",
				ClassName: "一班", Status: projectConstants.ApplicationApproved,
			},
			Statement: "申报说明", Rank: &rank, Score: &score,
		},
		{
			ApplicationSummary: projectModels.ApplicationSummary{
				ID:     8,
				Status: projectConstants.ApplicationRejected,
			},
		},
	}
	columns := []string{"优秀学生-志愿服务", "优秀学生-志愿服务-说明"}
	records := applicationExportRecords(rows, applicationExportHeaders(), columns,
		map[uint64]map[int]string{7: {0: "0", 1: "志愿服务说明"}})
	if len(records) != 3 || len(records[0]) != 17 || records[1][4] != "0007" ||
		records[1][10] != "申报说明" || records[1][11] != "已通过" || records[1][13] != "1" ||
		records[1][14] != "0" || records[1][15] != "0" || records[1][16] != "志愿服务说明" ||
		records[2][11] != "已驳回" || records[2][13] != "" || records[2][15] != "" {
		t.Fatalf("填报导出未保留申报状态、评分和空值语义：%v", records)
	}
	empty := applicationExportRecords(nil, applicationExportHeaders(), nil, nil)
	if len(empty) != 1 || len(empty[0]) != 15 {
		t.Fatal("空结果应保留全部基础表头")
	}
}
