package userSvc

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestStudentLoginEmailLocksOnlyStudentRow(t *testing.T) {
	link := userModels.EmailLogin{
		StudentID: 17, WorkspaceID: "0123456789abcdef", WorkspaceName: "学生登录验收",
		StudentNo: "000001", Email: "student@example.com",
	}
	for _, test := range []struct {
		name   string
		change func(*userModels.StudentIdentity, *workspaceModels.Workspace)
	}{
		{name: "匹配的在籍学生"},
		{name: "空间停用", change: func(_ *userModels.StudentIdentity, w *workspaceModels.Workspace) {
			w.IsEnabled = false
		}},
		{name: "空间改名", change: func(_ *userModels.StudentIdentity, w *workspaceModels.Workspace) {
			w.Name += "改名"
		}},
		{name: "学生离籍", change: func(s *userModels.StudentIdentity, _ *workspaceModels.Workspace) {
			s.Status = "离籍"
		}},
		{name: "学号变更", change: func(s *userModels.StudentIdentity, _ *workspaceModels.Workspace) {
			s.StudentNo += "2"
		}},
		{name: "邮箱变更", change: func(s *userModels.StudentIdentity, _ *workspaceModels.Workspace) {
			s.Email = "changed@example.com"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// SQL 契约限定主键与租户条件，只允许学生查询出现 FOR UPDATE。
			sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Error(err)
				}
				_ = sqlDB.Close()
			})
			connection, err := gorm.Open(
				gormmysql.New(gormmysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
				&gorm.Config{DisableAutomaticPing: true, Logger: gormlogger.Discard},
			)
			if err != nil {
				t.Fatal(err)
			}
			student := userModels.StudentIdentity{
				StudentID: link.StudentID, StudentNo: link.StudentNo,
				Name: "虚构学生", Email: link.Email, Status: studentConstants.StatusEnrolled,
			}
			workspace := workspaceModels.Workspace{
				ID: link.WorkspaceID, Name: link.WorkspaceName, IsEnabled: true,
			}
			if test.change != nil {
				test.change(&student, &workspace)
			}
			mock.ExpectBegin()
			mock.ExpectQuery(
				"SELECT id AS student_id, student_no, name, email, status FROM `students` "+
					"WHERE id = ? AND workspace_id = ? LIMIT ? FOR UPDATE",
			).WithArgs(link.StudentID, link.WorkspaceID, 1).WillReturnRows(
				sqlmock.NewRows([]string{"student_id", "student_no", "name", "email", "status"}).
					AddRow(student.StudentID, student.StudentNo, student.Name, student.Email, student.Status),
			)
			mock.ExpectQuery(
				"SELECT id, name, is_enabled FROM `workspaces` WHERE id = ? LIMIT ?",
			).WithArgs(link.WorkspaceID, 1).WillReturnRows(
				sqlmock.NewRows([]string{"id", "name", "is_enabled"}).
					AddRow(workspace.ID, workspace.Name, workspace.IsEnabled),
			)
			if test.change == nil {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			var identity userModels.StudentIdentity
			err = connection.WithContext(t.Context()).Transaction(func(tx *gorm.DB) error {
				var err error
				identity, err = lockStudentLoginEmail(tx, link)
				return err
			})
			if test.change != nil {
				var business *response.BusinessError
				if !errors.As(err, &business) || business.Status != http.StatusUnauthorized {
					t.Fatalf("资料变更后应拒绝签发：%v", err)
				}
			} else if err != nil || identity.StudentID != link.StudentID ||
				identity.WorkspaceID != link.WorkspaceID || identity.WorkspaceName != link.WorkspaceName {
				t.Fatalf("单行锁定后未返回正确学生身份：%v", err)
			}
		})
	}
}

func TestStudentLoginEmailConcurrentIssuance(t *testing.T) {
	if db.RDB() == nil {
		t.Skip("配置独立 Redis 后运行学生邮件并发签发验收")
	}
	ctx := t.Context()
	link := userModels.EmailLogin{
		StudentID: 17, WorkspaceID: randomToken()[:16], WorkspaceName: "学生邮件并发验收",
		CodeHash:  tokenHash(randomToken()),
		StudentNo: "000001", Email: "student@example.com",
		CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	first := tokenHash(randomToken())
	second := tokenHash(randomToken())
	previous := tokenHash(randomToken())
	otherHash := tokenHash(randomToken())
	other := link
	other.StudentID++
	t.Cleanup(func() {
		// 每个测试使用随机空间前缀，清理只针对自己的签发记录。
		for _, hash := range []string{first, second, previous} {
			discardEmailLoginChallenge(ctx, link, hash)
		}
		discardEmailLoginChallenge(ctx, other, otherHash)
	})

	// 已成功签发的链接应一直保留到新邮件发送成功。
	if err := reserveStudentLoginEmail(ctx, link, previous); err != nil {
		t.Fatal(err)
	}
	if err := cacheEmailLogin(ctx, link, previous); err != nil {
		t.Fatal(err)
	}
	cooldown := emailLoginChallengeKeys(link, first)[2]
	if err := db.RDB().Del(ctx, cooldown).Err(); err != nil {
		t.Fatal(err)
	}
	if err := reserveStudentLoginEmail(ctx, link, first); err != nil {
		t.Fatal(err)
	}
	var business *response.BusinessError
	if err := reserveStudentLoginEmail(ctx, link, second); !errors.As(err, &business) ||
		business.Status != http.StatusTooManyRequests {
		t.Fatal("同一学生冷却期内必须拒绝重复发信")
	}
	if err := reserveStudentLoginEmail(ctx, other, otherHash); err != nil {
		t.Fatal("同空间其他学生不应共享发送额度")
	}
	if _, _, err := readEmailLogin(ctx, previous); err != nil {
		t.Fatal("新邮件尚在发送时不应废弃已发送的链接")
	}

	// 模拟冷却期已结束而旧邮件仍在等待，最新请求优先，无需持有空间锁。
	if err := db.RDB().Del(ctx, cooldown).Err(); err != nil {
		t.Fatal(err)
	}
	if err := reserveStudentLoginEmail(ctx, link, second); err != nil {
		t.Fatal(err)
	}
	if err := cacheEmailLogin(ctx, link, first); err == nil {
		t.Fatal("较慢的旧请求不得发布登录链接")
	}
	discardEmailLoginChallenge(ctx, link, first)
	if err := cacheEmailLogin(ctx, link, second); err != nil {
		t.Fatal("清理旧请求不得删除新请求的签发标记")
	}
	if _, _, err := readEmailLogin(ctx, previous); err == nil {
		t.Fatal("新邮件签发成功后应使旧链接失效")
	}
	if _, _, err := readEmailLogin(ctx, first); err == nil {
		t.Fatal("旧请求不应留下有效链接")
	}
	if _, _, err := readEmailLogin(ctx, second); err != nil {
		t.Fatal("新请求签发的链接应有效")
	}
	if current, err := db.RDB().Get(ctx, cooldown).Result(); err != nil || current != second {
		t.Fatal("清理旧请求不得释放新请求的发送额度")
	}

	// 发送失败或提交失败只清除自己的状态，之后可以立即重试。
	discardEmailLoginChallenge(ctx, link, second)
	if _, _, err := readEmailLogin(ctx, second); err == nil {
		t.Fatal("失败签发不得留下有效链接")
	}
	if err := reserveStudentLoginEmail(ctx, link, first); err != nil {
		t.Fatal("失败签发清理后应允许立即重试")
	}
}
