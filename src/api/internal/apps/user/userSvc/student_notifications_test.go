package userSvc_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
)

func checkStudentNotifications(
	t *testing.T,
	server *echo.Echo,
	cookie, admin *http.Cookie,
	student, other studentModels.Student,
) {
	t.Helper()
	t.Run("学生通知隔离及幂等已读", func(t *testing.T) {
		// 同空间另一学生与另一空间学生均不能被当前会话读取或标记。
		connection := db.DB().WithContext(t.Context())
		peer := student
		peer.ID, peer.StudentNo, peer.Email = 0, "00000002", "peer@example.com"
		if err := connection.Create(&peer).Error; err != nil {
			t.Fatal(err)
		}
		var notices []notificationModels.StudentNotification
		for _, recipient := range []studentModels.Student{student, peer, other} {
			err := connection.Transaction(func(tx *gorm.DB) error {
				notice, _, err := notificationUtils.CreateStudent(tx,
					notificationModels.StudentDelivery{
						WorkspaceID: recipient.WorkspaceID, StudentID: recipient.ID,
						SourceID: "fixture-application-notice", Title: "申报待审批通知",
						Content: "虚构评奖申报待审批。",
					})
				notices = append(notices, notice)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		path := "/v1/workspaces/" + student.WorkspaceID + "/student-service/notifications"
		list := data[notificationModels.ListData](t, send(t, server, http.MethodGet,
			path+"?student_id="+strconv.FormatUint(peer.ID, 10), nil, cookie, http.StatusOK))
		if len(list.Items) != 1 || list.Items[0].ID != notices[0].ID || list.UnreadCount != 1 {
			t.Fatal("学生通知未限定当前身份，或读取时错误标记已读")
		}
		send(t, server, http.MethodGet, path, nil, admin, http.StatusForbidden)
		send(t, server, http.MethodGet,
			"/v1/workspaces/"+other.WorkspaceID+"/student-service/notifications",
			nil, cookie, http.StatusForbidden)
		for _, foreign := range notices[1:] {
			send(t, server, http.MethodPost, path+"/read", notificationModels.ReadRequest{
				IDs: []uint64{notices[0].ID, foreign.ID},
			}, cookie, http.StatusNotFound)
		}
		for _, changed := range []int64{1, 0} {
			result := data[notificationModels.ReadData](t, send(t, server, http.MethodPost,
				path+"/read", notificationModels.ReadRequest{IDs: []uint64{notices[0].ID}},
				cookie, http.StatusOK))
			if result.Changed != changed || result.UnreadCount != 0 {
				t.Fatal("批量越权请求产生部分变更，或重复已读未保持幂等")
			}
		}
		all := data[notificationModels.ListData](t, send(t, server, http.MethodGet,
			path+"?status=all", nil, cookie, http.StatusOK))
		if all.Total != 1 || len(all.Items) != 1 || all.Items[0].ReadAt == nil {
			t.Fatal("全部通知未保留历史已读通知")
		}
		unread := data[notificationModels.UnreadData](t, send(t, server, http.MethodGet,
			path+"/unread", nil, cookie, http.StatusOK))
		if unread.UnreadCount != 0 {
			t.Fatal("学生未读总数包含其他接收人的通知")
		}
		checkStudentNotificationAuditRollback(t, server, cookie, notices[0], path)
	})
}

func checkStudentNotificationAuditRollback(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	notice notificationModels.StudentNotification,
	path string,
) {
	t.Helper()
	t.Run("学生通知审计失败回滚", func(t *testing.T) {
		// 只使用独立测试库的虚构记录，故障后已读状态必须仍为空。
		connection := db.DB().WithContext(t.Context())
		if err := connection.Model(&notificationModels.StudentNotification{}).
			Where("workspace_id = ? AND student_id = ? AND id = ?",
				notice.WorkspaceID, notice.StudentID, notice.ID).
			Update("read_at", nil).Error; err != nil {
			t.Fatal(err)
		}
		const callback = "test:student-notification-audit-unavailable"
		if err := db.DB().Callback().Create().Before("gorm:create").
			Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "audit_events" {
					_ = tx.AddError(gorm.ErrInvalidData)
				}
			}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.DB().Callback().Create().Remove(callback); err != nil {
				t.Error(err)
			}
		})
		send(t, server, http.MethodGet, path, nil, cookie, http.StatusServiceUnavailable)
		send(t, server, http.MethodPost, path+"/read",
			notificationModels.ReadRequest{IDs: []uint64{notice.ID}},
			cookie, http.StatusServiceUnavailable)
		var current notificationModels.StudentNotification
		if err := connection.Where("workspace_id = ? AND student_id = ? AND id = ?",
			notice.WorkspaceID, notice.StudentID, notice.ID).
			Take(&current).Error; err != nil || current.ReadAt != nil {
			t.Fatal("学生通知审计失败未回滚已读状态")
		}
	})
}
