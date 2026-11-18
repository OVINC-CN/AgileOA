package userUtils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestRequireLocksCurrentAccountAndRechecksIdentity(t *testing.T) {
	for _, test := range []struct {
		name     string
		action   string
		strength string
		version  uint64
		enabled  bool
		admin    bool
		status   int
	}{
		{
			name: "统计共享账号锁", action: accessConstants.StudentStatistics,
			strength: "SHARE", version: 1, enabled: true,
		},
		{
			name: "写入保持排他锁", action: accessConstants.StudentUpdate,
			strength: "UPDATE", version: 1, enabled: true,
		},
		{
			name: "混合操作保持排他锁", action: accessConstants.StudentSensitiveHandle,
			strength: "UPDATE", version: 1, enabled: true,
		},
		{
			name: "会话版本已变更", action: accessConstants.StudentStatistics,
			strength: "SHARE", version: 2, enabled: true, status: http.StatusUnauthorized,
		},
		{
			name: "账号已停用", action: accessConstants.StudentStatistics,
			strength: "SHARE", version: 1, status: http.StatusUnauthorized,
		},
		{
			name: "管理员身份已撤销", action: accessConstants.UserList,
			strength: "SHARE", version: 1, enabled: true, status: http.StatusForbidden,
		},
		{
			name: "管理员查询共享锁", action: accessConstants.UserAPITokenRead,
			strength: "SHARE", version: 1, enabled: true, admin: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			connection, mock, err := sqlmock.New(
				sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual),
			)
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

			// 中间件身份仍有效时，事务必须按最新账号状态决定是否放行。
			ctx := WithIdentity(t.Context(), userModels.Identity{
				User: userModels.User{ID: 1, SessionVersion: 1, IsEnabled: true, IsAdmin: true},
			})
			mock.ExpectBegin()
			mock.ExpectQuery(
				"SELECT * FROM `users` WHERE `users`.`id` = ? ORDER BY `users`.`id` "+
					"LIMIT ? FOR "+test.strength,
			).WithArgs(1, 1).WillReturnRows(sqlmock.NewRows(
				[]string{"id", "session_version", "is_enabled", "is_admin"},
			).AddRow(1, test.version, test.enabled, test.admin))
			if test.status == 0 {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			err = database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				_, err := Require(tx, test.action)
				return err
			})
			var business *response.BusinessError
			if test.status == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.As(err, &business) || business.Status != test.status {
				t.Fatalf("账号状态复核结果错误：%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
