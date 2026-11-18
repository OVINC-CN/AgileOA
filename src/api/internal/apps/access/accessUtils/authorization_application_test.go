package accessUtils

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestWorkspaceActionsRequireExplicitPermission(t *testing.T) {
	for _, action := range []string{
		accessConstants.AccessWorkspace,
		accessConstants.ProjectApplicationList,
		accessConstants.ProjectApplicationDelete,
	} {
		for _, actor := range []struct {
			name    string
			admin   bool
			granted bool
		}{
			{name: "未授权账号"},
			{name: "未授权管理员", admin: true},
			{name: "显式授权账号", granted: true},
		} {
			t.Run(action+"/"+actor.name, func(t *testing.T) {
				checkExplicitPermission(t, action, actor.admin, actor.granted)
			})
		}
	}
}

func checkExplicitPermission(t *testing.T, action string, admin, granted bool) {
	t.Helper()
	// 只允许查询显式权限，缺少授权时不得查询审批节点作为放行依据。
	connection, mock, err := sqlmock.New()
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
	rows := sqlmock.NewRows([]string{"user_id", "workspace_id", "action"})
	if granted {
		rows.AddRow(1, "fixture", action)
	}
	mock.ExpectQuery("SELECT .*access_permissions.*FOR SHARE$").WillReturnRows(rows)
	principal := accessModels.Principal{
		User:      userModels.User{ID: 1, IsAdmin: admin},
		Workspace: workspaceModels.Workspace{ID: "fixture", IsEnabled: true},
	}
	err = Require(
		database.WithContext(t.Context()),
		principal,
		action,
	)
	var denied *response.BusinessError
	if granted {
		if err != nil {
			t.Fatalf("显式授权应正常通过：%v", err)
		}
	} else if !errors.As(err, &denied) || denied.Status != http.StatusForbidden {
		t.Fatalf("缺少显式权限时必须拒绝，不得通过节点指派或管理员身份绕过：%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
