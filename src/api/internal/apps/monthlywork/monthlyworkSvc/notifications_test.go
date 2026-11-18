package monthlyworkSvc

import (
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
)

func TestAssignmentNotificationLinks(t *testing.T) {
	for _, test := range []struct{ category, path string }{
		{"student", "/stu/monthly-work"},
		{"cpc", "/cpc/monthly-work"},
	} {
		t.Run(test.category, func(t *testing.T) {
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
			link := fmt.Sprintf(
				"%s?month=03&work_id=7&workspace=w7m2k9p4s6n8c3x5&year=2025", test.path,
			)
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `notifications`")).
				WithArgs(8, "monthly-work:w7m2k9p4s6n8c3x5:7:1", "工作分配通知",
					sqlmock.AnyArg(), link, nil).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()
			err = database.Transaction(func(tx *gorm.DB) error {
				notifications, err := notifyAssignments(tx,
					accessModels.Principal{User: userModels.User{ID: 99}},
					monthlyworkModels.Record{
						WorkspaceID: "w7m2k9p4s6n8c3x5", Category: test.category,
						ID: 7, Revision: 1, Date: "2025-03-01", OwnerID: 8, WorkItem: "虚构工作",
					}, 0, nil, nil)
				if err == nil && (len(notifications) != 1 || notifications[0].URL != link) {
					t.Fatal("工作分配通知缺少正确的空间、类别或记录定位链接")
				}
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

func TestAssignmentRecipients(t *testing.T) {
	for _, test := range []struct {
		name                    string
		owner, previousOwner    uint64
		current, previous, want []uint64
	}{
		{"新增工作通知负责人及参与人", 1, 0, []uint64{1, 2, 3}, nil, []uint64{1, 2, 3}},
		{"普通编辑不重复通知", 1, 1, []uint64{2, 3}, []uint64{2, 3}, nil},
		{"新增参与人", 1, 1, []uint64{2, 3}, []uint64{2}, []uint64{3}},
		{"重新分配负责人", 2, 1, []uint64{3}, []uint64{3}, []uint64{2}},
		{"负责人同时参与只通知一次", 2, 1, []uint64{2, 3}, nil, []uint64{2, 3}},
		{"移除参与人不通知", 1, 1, nil, []uint64{2, 3}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual := assignmentRecipients(
				99,
				test.owner,
				test.previousOwner,
				test.current,
				test.previous,
			)
			if !slices.Equal(actual, test.want) {
				t.Fatalf("工作分配通知接收人错误：%v", actual)
			}
		})
	}
	if got := assignmentRecipients(1, 1, 0, []uint64{1, 2}, nil); !slices.Equal(got, []uint64{2}) {
		t.Fatal("分配工作不应向当前操作人发送通知")
	}
	if got := assignmentRecipients(
		1,
		2,
		0,
		[]uint64{1, 2, 3},
		nil,
	); !slices.Equal(
		got,
		[]uint64{2, 3},
	) {
		t.Fatal("负责人不是当前用户时必须收到一条通知，不能被参与人去重遗漏")
	}
}
