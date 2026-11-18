package userUtils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func authenticateStudent(ctx context.Context, token string) (userModels.Identity, error) {
	// 校验格式后只用摘要查询，不把原始 Cookie 留在身份上下文。
	deny := &response.BusinessError{Status: http.StatusUnauthorized, Message: "请先登录"}
	value := strings.TrimPrefix(token, userConstants.StudentSessionPrefix)
	if len(value) != userConstants.SessionBytes*2 {
		return userModels.Identity{}, deny
	}
	if _, err := hex.DecodeString(value); err != nil {
		return userModels.Identity{}, deny
	}
	if db.DB() == nil {
		return userModels.Identity{}, &response.BusinessError{
			Status: http.StatusServiceUnavailable, Message: "账号服务暂时不可用",
		}
	}
	digest := sha256.Sum256([]byte(token))
	var student userModels.StudentIdentity
	err := db.DB().WithContext(ctx).Table("student_sessions").Select([]string{
		"students.id AS student_id",
		"students.student_no",
		"students.name",
		"students.email",
		"students.status",
		"students.workspace_id",
		"workspaces.name AS workspace_name",
		"workspaces.is_enabled AS workspace_enabled",
		"student_sessions.id AS session_id",
		"student_sessions.workspace_id AS session_workspace_id",
		"student_sessions.student_id AS session_student_id",
		"student_sessions.student_no AS session_student_no",
		"student_sessions.email AS session_email",
		"student_sessions.expires_at AS session_expires_at",
		"student_sessions.created_at AS session_created_at",
		"student_sessions.updated_at AS session_updated_at",
	}).Joins("JOIN students ON students.workspace_id = student_sessions.workspace_id "+
		"AND students.id = student_sessions.student_id").
		Joins("JOIN workspaces ON workspaces.id = students.workspace_id").
		Where("student_sessions.token_hash = ? AND student_sessions.expires_at > ?",
			hex.EncodeToString(digest[:]), time.Now().UTC()).Take(&student).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return userModels.Identity{}, deny
	}
	if err != nil {
		return userModels.Identity{}, err
	}
	if !student.WorkspaceEnabled || student.Status != studentConstants.StatusEnrolled ||
		student.StudentNo != student.Session.StudentNo || student.Email != student.Session.Email {
		return userModels.Identity{}, deny
	}

	// 学生审计编号与全局账号编号分开，避免编号碰撞冒用管理账号。
	auditUtils.SetActor(ctx, StudentActor(student))
	return userModels.Identity{Student: &student}, nil
}

// StudentActor 构造学生专用的可信身份快照，不使用全局账号编号。
func StudentActor(student userModels.StudentIdentity) auditModels.Actor {
	return auditModels.Actor{
		StudentID: student.StudentID, IdentityType: userConstants.IdentityStudent,
		WorkspaceID: student.WorkspaceID, Username: student.StudentNo, DisplayName: student.Name,
	}
}

// StudentInfo 返回学生会话的固定公开字段，不包含全局管理权限。
func StudentInfo(student userModels.StudentIdentity) userModels.Info {
	return userModels.Info{
		ID: student.StudentID, Username: student.StudentNo, DisplayName: student.Name,
		Email: student.Email, IdentityType: userConstants.IdentityStudent,
		WorkspaceID: student.WorkspaceID, WorkspaceName: student.WorkspaceName,
	}
}
