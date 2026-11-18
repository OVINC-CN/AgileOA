package projectSvc

import (
	"errors"
	"net/url"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
)

func notificationApplication(
	status string,
	step int,
	reviewers ...projectModels.ApplicationReviewer,
) projectModels.ApplicationInfo {
	return projectModels.ApplicationInfo{
		ApplicationSummary: projectModels.ApplicationSummary{
			ID: 7, StudentID: 9, StudentName: "虚构学生", StudentNo: "0009",
			ProjectName: "优秀学生", Year: 2026, Revision: 3, Status: status, CurrentStep: step,
		},
		Steps: []projectModels.ApplicationStepInfo{{Position: step, Reviewers: reviewers}},
	}
}

func TestApplicationNotificationRecipients(t *testing.T) {
	student := projectModels.ApplicationReviewer{IdentityType: "student", ID: 8}
	user := projectModels.ApplicationReviewer{IdentityType: "user", ID: 8}
	signed := student
	signed.Approved = true
	pending := notificationApplication("pending", 1, student, user)
	partial := notificationApplication("pending", 1, signed, user)
	for _, item := range []struct {
		name          string
		before, after projectModels.ApplicationInfo
		want          []string
	}{
		{"草稿不通知", projectModels.ApplicationInfo{}, notificationApplication("draft", 0), nil},
		{"首次提交通知全部身份", projectModels.ApplicationInfo{}, pending,
			[]string{"student:8", "user:8"}},
		{"草稿提交", notificationApplication("draft", 0), pending,
			[]string{"student:8", "user:8"}},
		{"驳回重提重新通知", notificationApplication("rejected", 0), pending,
			[]string{"student:8", "user:8"}},
		{"普通保存不重复通知", pending, pending, nil},
		{"并签单人通过不重复通知", pending, partial, nil},
		{"修改后重新通知失效签署", partial, pending, []string{"student:8"}},
		{"下一级仅通知新节点", pending, notificationApplication("pending", 2, user),
			[]string{"user:8"}},
		{"驳回通知申报人", pending, notificationApplication("rejected", 0),
			[]string{"student:9"}},
		{"驳回后保存不通知", notificationApplication("rejected", 0),
			notificationApplication("rejected", 0), nil},
		{"完全通过通知申报人", partial, notificationApplication("approved", 1),
			[]string{"student:9"}},
		{"多任职匹配去重", projectModels.ApplicationInfo{},
			notificationApplication("pending", 1, student, student), []string{"student:8"}},
		{"申报人审批节点仍须提醒", projectModels.ApplicationInfo{},
			notificationApplication("pending", 1,
				projectModels.ApplicationReviewer{IdentityType: "student", ID: 9}),
			[]string{"student:9"}},
	} {
		t.Run(item.name, func(t *testing.T) {
			recipients := applicationNotificationRecipients(item.before, item.after)
			keys := make([]string, 0, len(recipients))
			for _, recipient := range recipients {
				keys = append(keys, identityKey(recipient.IdentityType, recipient.ID))
			}
			if !slices.Equal(keys, item.want) {
				t.Fatalf("流转通知接收人错误：%v，预期 %v", keys, item.want)
			}
		})
	}
}

func TestApplicationNotificationDeliveryIsAtomic(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "正常提交", true: "审计失败回滚"}[rollback], func(t *testing.T) {
			// 两种身份使用相同编号，必须写入不同表并共享调用方事务。
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
			workspaceID := "w7m2k9p4s6n8c3x5"
			info := notificationApplication("pending", 1,
				projectModels.ApplicationReviewer{IdentityType: "student", ID: 8},
				projectModels.ApplicationReviewer{IdentityType: "user", ID: 8})
			source := "project-application:" + workspaceID + ":7:3"
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO `student_notifications`").WithArgs(
				workspaceID, 8, source, "申报待审批通知", sqlmock.AnyArg(),
				applicationNotificationURL(workspaceID, info, "student"), nil,
			).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("INSERT INTO `notifications`").WithArgs(
				8, source, "申报待审批通知", sqlmock.AnyArg(),
				applicationNotificationURL(workspaceID, info, "user"), nil,
			).WillReturnResult(sqlmock.NewResult(2, 1))
			if rollback {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			failure := errors.New("audit persistence unavailable")
			err = database.WithContext(t.Context()).Transaction(func(tx *gorm.DB) error {
				notices, err := notifyApplication(
					tx,
					workspaceID,
					projectModels.ApplicationInfo{},
					info,
				)
				if err != nil {
					return err
				}
				if len(notices.students) != 1 || len(notices.users) != 1 {
					t.Fatal("学生和组织审批人未分别收到通知")
				}
				if rollback {
					return failure
				}
				return nil
			})
			if rollback && !errors.Is(err, failure) || !rollback && err != nil {
				t.Fatalf("通知与业务事务的提交结果不一致：%v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApplicationNotificationLinks(t *testing.T) {
	for _, item := range []struct{ identity, status, path, tab string }{
		{"student", "pending", "/student-service/projects", "reviews"},
		{"student", "rejected", "/student-service/projects", "mine"},
		{"student", "approved", "/student-service/projects", "mine"},
		{"user", "pending", "/stu/projects", "applications"},
	} {
		info := notificationApplication(item.status, 1)
		link, err := url.Parse(applicationNotificationURL("w7m2k9p4s6n8c3x5", info, item.identity))
		if err != nil || link.Path != item.path || link.Query().Get("tab") != item.tab ||
			link.Query().Get("application_id") != "7" ||
			link.Query().Get("workspace") != "w7m2k9p4s6n8c3x5" {
			t.Fatalf("申报通知链接缺少身份入口或定位参数：%v %v", link, err)
		}
	}
}
