package accessUtils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func authorizationDatabase(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	connection, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{
		Conn: connection, SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	return database.WithContext(t.Context()), mock
}

func TestWorkspacePrecheckUsesOneNonlockingStatement(t *testing.T) {
	for _, test := range []struct {
		name      string
		enabled   bool
		permitted bool
		missing   bool
		status    int
	}{
		{name: "显式授权", enabled: true, permitted: true},
		{name: "已撤权", enabled: true, status: http.StatusForbidden},
		{name: "已停用", permitted: true, status: http.StatusForbidden},
		{name: "不存在", missing: true, status: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock := authorizationDatabase(t)
			rows := sqlmock.NewRows([]string{"is_enabled", "permitted"})
			if !test.missing {
				rows.AddRow(test.enabled, test.permitted)
			}
			// 精确匹配排除 BEGIN、COMMIT、锁定读取及额外权限查询。
			mock.ExpectQuery(
				"SELECT is_enabled, EXISTS (SELECT 1 FROM `access_permissions` "+
					"WHERE user_id = ? AND action = ? AND workspace_id = ?) AS permitted "+
					"FROM `workspaces` WHERE id = ? LIMIT ?",
			).WithArgs(1, accessConstants.StudentStatistics, "fixture", "fixture", 1).
				WillReturnRows(rows)
			err := checkWorkspace(database, 1, "fixture", accessConstants.StudentStatistics)
			var business *response.BusinessError
			if test.status == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &business) || business.Status != test.status {
				t.Fatalf("预检结果错误：%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkspaceBoundaryLocksKeepReadsSharedAndWritesExclusive(t *testing.T) {
	for _, test := range []struct {
		name     string
		strength string
		lock     func(*gorm.DB, string) (workspaceModels.Workspace, error)
	}{
		{name: "并行查询", strength: "SHARE", lock: ReadWorkspace},
		{name: "业务写入与撤权", strength: "UPDATE", lock: LockWorkspace},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock := authorizationDatabase(t)
			mock.ExpectBegin()
			mock.ExpectQuery(
				"SELECT * FROM `workspaces` WHERE id = ? LIMIT ? FOR "+test.strength,
			).WithArgs("fixture", 1).WillReturnRows(
				sqlmock.NewRows([]string{"id", "is_enabled"}).AddRow("fixture", true),
			)
			mock.ExpectCommit()
			err := database.Transaction(func(tx *gorm.DB) error {
				_, err := test.lock(tx, "fixture")
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
