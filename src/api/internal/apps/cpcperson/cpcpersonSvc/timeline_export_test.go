package cpcpersonSvc

import (
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
)

func timelineTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(
		mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		&gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	return database.WithContext(t.Context()), mock
}

func TestFullExportUsesNodeOrderWithoutSharingPersonValues(t *testing.T) {
	database, mock := timelineTestDB(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	for _, templateID := range []int{1, 2} {
		dateID := "date"
		if templateID == 2 {
			dateID = "other_date"
		}
		fields := `[{"id":"` + dateID + `","name":"发生时间","type":"date","required":true},` +
			`{"id":"confirmed","name":"已确认","type":"boolean","required":false}]`
		nodeRows := sqlmock.NewRows([]string{
			"id",
			"node_id",
			"name",
			"time_field_id",
			"field_definitions",
			"validation_rules",
			"position",
		}).AddRow(templateID+6, "shared", "虚构节点", dateID, fields, "[]", 1)
		if templateID == 1 {
			nodeRows.AddRow(3, "next", "后续节点", dateID, fields, "[]", 2)
		}
		mock.ExpectQuery("SELECT .* FROM `cpc_timeline_template_nodes`").
			WithArgs(workspaceID, templateID).
			WillReturnRows(nodeRows)
	}
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
		WithArgs(workspaceID, 11, 22, 33, 1, 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"person_id", "timeline_template_id", "node_id", "field_values",
		}).AddRow(11, 1, "shared", `{"date":"2026-11-06","confirmed":false}`).
			AddRow(11, 1, "next", `{"date":"2026-11-07","confirmed":true}`).
			AddRow(22, 2, "shared", `{"other_date":"2025-11-06","confirmed":true}`))
	one, two := uint64(1), uint64(2)
	headers, rows, err := timelineExport(
		database.WithContext(t.Context()),
		workspaceID,
		[]cpcpersonModels.Info{
			{ID: 11, Input: cpcpersonModels.Input{TimelineTemplateID: &one}},
			{ID: 22, Input: cpcpersonModels.Input{TimelineTemplateID: &two}},
			{ID: 33},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headers, []string{"1-发生时间", "1-已确认", "2-发生时间", "2-已确认"}) ||
		!reflect.DeepEqual(rows[11], []string{"2026-11-06", "否", "2026-11-07", "是"}) ||
		!reflect.DeepEqual(rows[22], []string{"2025-11-06", "是", "", ""}) ||
		!reflect.DeepEqual(rows[33], []string{"", "", "", ""}) {
		t.Fatal("导出表头没有使用模板内顺序号、模板字段串值或未绑定人员没有保持空值")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFullExportResolvesCurrentStudentDatesAndIgnoresHistoricalValues(t *testing.T) {
	database, mock := timelineTestDB(t)
	const workspaceID = "w7m2k9p4s6n8c3x5"
	fields := `[{"id":"birth","name":"出生日期","type":"date","student_field":"birth_date"},` +
		`{"id":"adult","name":"成年日期","type":"date","student_field":"adulthood_date"}]`
	mock.ExpectQuery("SELECT .* FROM `cpc_timeline_template_nodes`").
		WithArgs(workspaceID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"node_id", "field_definitions", "validation_rules",
		}).AddRow("reference", fields, "[]"))
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
		WithArgs(workspaceID, 10, 20, 30, 40, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"person_id", "timeline_template_id", "node_id", "field_values",
		}).AddRow(10, 1, "reference", `{"birth":"1990-01-01","adult":"2008-01-01"}`).
			AddRow(20, 1, "reference", `{"birth":"1990-01-01","adult":"2008-01-01"}`).
			AddRow(30, 1, "reference", "{}").
			AddRow(40, 1, "reference", "{}"))
	mock.ExpectQuery("SELECT p.id, s.birth_date FROM cpc_people AS p").
		WithArgs(workspaceID, 10, 20, 30, 40).
		WillReturnRows(sqlmock.NewRows([]string{"id", "birth_date"}).
			AddRow(10, "2008-02-29").AddRow(20, "").AddRow(30, "2008-02-30").
			AddRow(40, "9999-12-31"))
	templateID := uint64(1)
	people := make([]cpcpersonModels.Info, 0, 4)
	for _, id := range []uint64{10, 20, 30, 40} {
		people = append(people, cpcpersonModels.Info{
			ID: id, Input: cpcpersonModels.Input{TimelineTemplateID: &templateID},
		})
	}
	headers, rows, err := timelineExport(database, workspaceID, people)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headers, []string{"1-出生日期", "1-成年日期"}) ||
		!reflect.DeepEqual(rows[10], []string{"2008-02-29", "2026-02-28"}) ||
		!reflect.DeepEqual(rows[20], []string{"", ""}) ||
		!reflect.DeepEqual(rows[30], []string{"", ""}) ||
		!reflect.DeepEqual(rows[40], []string{"9999-12-31", ""}) {
		t.Fatalf("导出未按当前学生资料解析引用、空值或年份溢出：%v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
