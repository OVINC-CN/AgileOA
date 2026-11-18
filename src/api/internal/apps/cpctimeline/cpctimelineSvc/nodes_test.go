package cpctimelineSvc

import (
	"errors"
	"net/http"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

const fixtureWorkspaceID = "w7m2k9p4s6n8c3x5"

func nodeTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn: connection, SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	return database.WithContext(t.Context()), mock
}

func TestNodeDefinitionsAndPositionRoundTripFromChildTable(t *testing.T) {
	input := fixtureInput()
	input.Definition.Nodes[0], input.Definition.Nodes[1] = input.Definition.Nodes[1], input.Definition.Nodes[0]
	input.Definition.Nodes[1].Name = "重命名后的节点"
	input.Definition.Nodes[1].GroupName = "申请阶段"
	input.Definition.Nodes[1].IsBaseline = true
	input.Definition.Nodes[1].Fields[0].StudentField = "adulthood_date"
	records, err := nodeRecords(fixtureWorkspaceID, 7, input.Definition.Nodes)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].Position != 1 || records[1].Position != 2 || records[1].NodeID != "node_first" {
		t.Fatal("子表没有保存拖拽后的节点顺序或稳定标识")
	}
	database, mock := nodeTestDB(t)
	rows := sqlmock.NewRows([]string{
		"workspace_id",
		"timeline_template_id",
		"node_id",
		"name",
		"description",
		"group_name",
		"is_baseline",
		"time_field_id",
		"field_definitions",
		"validation_rules",
		"position",
	})
	for _, record := range records {
		rows.AddRow(
			record.WorkspaceID,
			int64(7),
			record.NodeID,
			record.Name,
			record.Description,
			record.GroupName,
			record.IsBaseline,
			record.TimeFieldID,
			record.FieldDefinitions,
			record.ValidationRules,
			record.Position,
		)
	}
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM `cpc_timeline_template_nodes` WHERE workspace_id = ? AND "+
			"timeline_template_id = ? ORDER BY position ASC, node_id ASC",
	)).WithArgs(fixtureWorkspaceID, 7).WillReturnRows(rows)
	nodes, err := cpctimelineUtils.LoadNodes(database, fixtureWorkspaceID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nodes, input.Definition.Nodes) {
		t.Fatal("节点定义、分组、基准标记、字段、规则、顺序或引用未从子表完整恢复")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeDefinitionUpdateRollsBackWhenDeletedNodeHasAttachments(t *testing.T) {
	database, mock := nodeTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `cpc_timeline_template_nodes` WHERE workspace_id = \\? AND timeline_template_id = \\?").
		WithArgs(fixtureWorkspaceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"node_id"}).AddRow("node_first").AddRow("node_second"))
	mock.ExpectExec("UPDATE `cpc_timeline_template_nodes` SET .* WHERE workspace_id = \\? AND timeline_template_id = \\? AND node_id = \\?").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), "", false, sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), fixtureWorkspaceID, 7, "node_first",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT `id` FROM `cpc_people` WHERE workspace_id = \\? AND timeline_template_id = \\? ORDER BY id ASC").
		WithArgs(fixtureWorkspaceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes` WHERE workspace_id = \\? AND timeline_template_id = \\? ORDER BY person_id ASC, node_id ASC").
		WithArgs(fixtureWorkspaceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"person_id", "node_id"}))
	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM `cpc_timeline_template_nodes` WHERE workspace_id = ? AND "+
			"timeline_template_id = ? AND node_id = ?",
	)).WithArgs(fixtureWorkspaceID, 7, "node_second").WillReturnError(&mysqlDriver.MySQLError{Number: 1451})
	mock.ExpectRollback()
	node := fixtureInput().Definition.Nodes[0]
	node.Rules = []cpctimelineModels.Rule{}
	err := database.Transaction(func(tx *gorm.DB) error {
		return syncNodes(tx, fixtureWorkspaceID, 7, []cpctimelineModels.Node{node})
	})
	var business *response.BusinessError
	if !errors.As(err, &business) || business.Status != http.StatusConflict {
		t.Fatalf("有附件的节点未阻止整体保存：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeReadRejectsMalformedStoredFields(t *testing.T) {
	database, mock := nodeTestDB(t)
	mock.ExpectQuery("SELECT .* FROM `cpc_timeline_template_nodes` WHERE workspace_id = \\? AND timeline_template_id = \\? ORDER BY position ASC, node_id ASC").
		WithArgs(fixtureWorkspaceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{
			"node_id", "name", "time_field_id", "field_definitions", "validation_rules", "position",
		}).AddRow("node_first", "虚构节点", "occurred_at", `[{"required":"invalid"}]`, "[]", 1))
	if _, err := cpctimelineUtils.LoadNodes(database, fixtureWorkspaceID, 7); err == nil {
		t.Fatal("无法解析的子表字段被静默忽略")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPersonNodeRemovalWithAttachmentsRollsBack(t *testing.T) {
	database, mock := nodeTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `cpc_people`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
		WillReturnRows(sqlmock.NewRows([]string{
			"workspace_id",
			"person_id",
			"timeline_template_id",
			"node_id",
			"field_values",
			"status",
			"revision",
		}).AddRow(fixtureWorkspaceID, 5, 7, "node_removed", "{}", "pending", 1))
	mock.ExpectExec("DELETE FROM `cpc_person_timeline_nodes`").
		WithArgs(fixtureWorkspaceID, 5, "node_removed").
		WillReturnError(&mysqlDriver.MySQLError{Number: 1451})
	mock.ExpectRollback()
	err := database.Transaction(func(tx *gorm.DB) error {
		return syncPeople(tx, fixtureWorkspaceID, 7, fixtureInput().Definition.Nodes)
	})
	var business *response.BusinessError
	if !errors.As(err, &business) || business.Status != http.StatusConflict {
		t.Fatalf("人员附件未阻止节点删除：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNewTemplateNodeCreatesPendingPersonNode(t *testing.T) {
	database, mock := nodeTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `cpc_people`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
		WillReturnRows(sqlmock.NewRows([]string{"person_id", "node_id"}))
	mock.ExpectExec("INSERT INTO `cpc_person_timeline_nodes`").
		WithArgs(fixtureWorkspaceID, 5, 7, "node_first", "{}", "pending", 1, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `cpc_people`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "timeline_status", "revision"}).AddRow(5, "normal", 1))
	mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
		WillReturnRows(sqlmock.NewRows([]string{
			"person_id", "timeline_template_id", "node_id", "field_values", "status", "revision",
		}).AddRow(5, 7, "node_first", "{}", "pending", 1))
	mock.ExpectCommit()
	node := fixtureInput().Definition.Nodes[0]
	node.Rules = []cpctimelineModels.Rule{}
	if err := database.Transaction(func(tx *gorm.DB) error {
		return syncPeople(tx, fixtureWorkspaceID, 7, []cpctimelineModels.Node{node})
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateReferenceChangesClearStoredValuesForEveryStatus(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     string
		source     string
		stored     string
		wantStatus string
	}{
		{"pending_reference", "pending", "adulthood_date",
			`{"occurred_at":"1990-01-01","note":"保留备注"}`, "pending"},
		{"completed_reference", "completed", "adulthood_date",
			`{"occurred_at":"1990-01-01","note":"保留备注"}`, "completed"},
		{"skipped_reference", "skipped", "adulthood_date",
			`{"occurred_at":"1990-01-01","note":"保留备注"}`, "skipped"},
		{"manual_required", "completed", "", `{"note":"保留备注"}`, "pending"},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock := nodeTestDB(t)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `cpc_people`").
				WithArgs(fixtureWorkspaceID, 7).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
				WithArgs(fixtureWorkspaceID, 7).
				WillReturnRows(sqlmock.NewRows([]string{
					"workspace_id", "person_id", "timeline_template_id", "node_id",
					"field_values", "status", "revision",
				}).AddRow(fixtureWorkspaceID, 5, 7, "node_first", test.stored, test.status, 1))
			mock.ExpectExec("UPDATE `cpc_person_timeline_nodes`").
				WithArgs(`{"note":"保留备注"}`, 2, test.wantStatus, sqlmock.AnyArg(),
					fixtureWorkspaceID, 5, "node_first").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT .* FROM `cpc_people`").
				WithArgs(fixtureWorkspaceID, 5, 7, 1).
				WillReturnRows(sqlmock.NewRows([]string{"id", "timeline_status", "revision"}).
					AddRow(5, "normal", 1))
			mock.ExpectQuery("SELECT .* FROM `cpc_person_timeline_nodes`").
				WithArgs(fixtureWorkspaceID, 5, 7).
				WillReturnRows(sqlmock.NewRows([]string{"node_id", "field_values"}).
					AddRow("node_first", `{"note":"保留备注"}`))
			if test.source != "" {
				mock.ExpectQuery("SELECT p.id, s.birth_date FROM cpc_people AS p").
					WithArgs(fixtureWorkspaceID, 5).
					WillReturnRows(sqlmock.NewRows([]string{"id", "birth_date"}).
						AddRow(5, "2008-02-29"))
			}
			mock.ExpectCommit()

			node := fixtureInput().Definition.Nodes[0]
			node.Fields[0].StudentField = test.source
			node.Rules = []cpctimelineModels.Rule{}
			if err := database.Transaction(func(tx *gorm.DB) error {
				return syncPeople(tx, fixtureWorkspaceID, 7, []cpctimelineModels.Node{node})
			}); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
