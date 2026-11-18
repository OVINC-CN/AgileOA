package studentSvc_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

func checkStudentDeleteCascade(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	path, workspaceID string,
) {
	t.Helper()
	student := addStudent(t, server, cookie, path, "CASCADE-DELETE", "虚构级联学生", "女")
	studentPath := fmt.Sprintf("%s/%d", path, student.ID)
	info := decode[studentModels.DetailData](
		t, request(t, server, "GET", studentPath, nil, cookie, 200),
	)
	details := studentModels.OffCampusValues{
		OffCampusFields: studentModels.OffCampusFields{
			Reason: "虚构删除原因", Address: "虚构级联删除地址", LandlordPhone: "00000123",
		},
		StartDate: "2026-09-01", EndDate: "2027-06-30",
	}
	info.AccommodationType = "off_campus"
	student = decode[studentModels.MutationData](t, request(
		t, server, "PUT", studentPath, studentModels.UpdateRequest{
			Input: info.Input, Revision: student.Revision, OffCampusDetails: &details,
		}, cookie, 200,
	))
	deletePath := fmt.Sprintf("%s?revision=%d", studentPath, student.Revision)

	// 审计故障必须回滚父记录及住宿明细的级联删除。
	const callback = "test:student-cascade-audit-unavailable"
	if err := db.DB().Callback().Create().Before("gorm:create").
		Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Table == "audit_events" {
				_ = tx.AddError(gorm.ErrInvalidData)
			}
		}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.DB().Callback().Create().Remove(callback) })
	request(t, server, "DELETE", deletePath, nil, cookie, 503)
	if err := db.DB().Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	unchanged := decode[studentModels.DetailData](
		t, request(t, server, "GET", studentPath, nil, cookie, 200),
	)
	if unchanged.Revision != student.Revision || unchanged.OffCampusDetails == nil ||
		*unchanged.OffCampusDetails != details {
		t.Fatal("审计故障没有完整回滚学生及校外住宿明细")
	}

	// 一次删除清理学生及自有明细，成功审计仍保存加密的完整住宿信息。
	request(t, server, "DELETE", deletePath, nil, cookie, 200)
	connection := db.DB().WithContext(t.Context())
	var count int64
	if err := connection.Model(&studentModels.StudentOffCampusDetail{}).
		Where("workspace_id = ? AND student_id = ?", workspaceID, student.ID).
		Count(&count).Error; err != nil || count != 0 {
		t.Fatal("删除学生未级联清理校外住宿明细")
	}
	if err := connection.Model(&studentModels.Student{}).
		Where("workspace_id = ? AND id = ?", workspaceID, student.ID).
		Count(&count).Error; err != nil || count != 0 {
		t.Fatal("学生未被硬删除")
	}
	var deletion auditModels.Record
	if err := connection.Where(
		"workspace_id = ? AND instance_id = ? AND action_id = ? AND result_code = ?",
		workspaceID, fmt.Sprint(student.ID), accessConstants.StudentDelete, 0,
	).Take(&deletion).Error; err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(deletion.InstanceOriginData, []byte(details.Address)) ||
		bytes.Contains(deletion.SensitiveSnapshot, []byte(details.Address)) {
		t.Fatal("住宿明细泄露到普通审计或未加密保存")
	}
	key, err := base64.StdEncoding.DecodeString(
		config.Config.Audit.SnapshotKeys[deletion.SensitiveKeyID],
	)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		t.Fatal(err)
	}
	associated, err := auditSvc.SensitiveAssociatedData(deletion.SensitiveContext)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := aead.Open(nil, nil, deletion.SensitiveSnapshot, associated)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct {
		Before struct {
			OffCampusDetails studentModels.OffCampusValues `json:"off_campus_details"`
		} `json:"before"`
	}
	if err := json.Unmarshal(plaintext, &snapshot); err != nil ||
		snapshot.Before.OffCampusDetails != details {
		t.Fatal("级联删除未保留完整的住宿明细加密快照")
	}
}
