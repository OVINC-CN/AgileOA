package userSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentses"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// SendStudentLoginEmail 只为启用空间中三项信息匹配的在籍学生发送数字验证码。
func SendStudentLoginEmail(
	ctx context.Context,
	request userModels.SendStudentLoginEmailRequest,
	clientIP string,
) (userModels.EmailLoginChallengeData, error) {
	ctx, span := tracing.Tracer.Start(ctx, "userSvc.SendStudentLoginEmail")
	defer span.End()
	request.WorkspaceName = strings.TrimSpace(request.WorkspaceName)
	request.StudentNo = strings.TrimSpace(request.StudentNo)
	request.Email = strings.TrimSpace(request.Email)
	if request.WorkspaceName == "" || utf8.RuneCountInString(request.WorkspaceName) > 100 ||
		request.StudentNo == "" || utf8.RuneCountInString(request.StudentNo) > 32 ||
		request.Email == "" || !validEmail(request.Email) {
		return userModels.EmailLoginChallengeData{}, badRequest()
	}
	if err := verifyLoginCaptcha(ctx, userModels.LoginRequest{
		CaptchaTicket: request.CaptchaTicket, CaptchaRandstr: request.CaptchaRandstr,
	}, clientIP); err != nil {
		return userModels.EmailLoginChallengeData{}, err
	}
	if db.DB() == nil || db.RDB() == nil || !config.Config.User.EmailLogin.Enabled {
		return userModels.EmailLoginChallengeData{}, studentEmailUnavailable()
	}

	// 普通查询只定位目标，短事务内仅锁定该学生行，不锁工作空间。
	code, err := randomEmailCode()
	if err != nil {
		return userModels.EmailLoginChallengeData{}, err
	}
	challengeID := randomToken()
	hash := tokenHash(challengeID)
	record := userModels.EmailLogin{
		CodeHash:      tokenHash(challengeID + ":" + code),
		WorkspaceName: request.WorkspaceName, StudentNo: request.StudentNo, Email: request.Email,
	}
	err = db.DB().WithContext(ctx).Table("students").
		Select("students.id AS student_id, students.workspace_id").
		Joins("JOIN workspaces ON workspaces.id = students.workspace_id").
		Where("workspaces.name = ? AND students.student_no = ?",
			request.WorkspaceName, request.StudentNo).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = studentNotFound()
	}
	if err != nil {
		return userModels.EmailLoginChallengeData{}, failed(
			ctx, userModels.User{}, accessConstants.StudentLoginEmailSend, err,
		)
	}
	reject := func(err error) (userModels.EmailLoginChallengeData, error) {
		discardEmailLoginChallenge(ctx, record, hash)
		return userModels.EmailLoginChallengeData{}, failed(
			ctx, userModels.User{}, accessConstants.StudentLoginEmailSend, err,
		)
	}
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockStudentLoginEmail(tx, record); err != nil {
			return err
		}
		now := time.Now().UTC()
		record.CreatedAt = now
		record.ExpiresAt = now.Add(config.Config.User.EmailLogin.TTL)
		return reserveStudentLoginEmail(ctx, record, hash)
	})
	if err != nil {
		return reject(err)
	}

	// 邮件服务调用在事务外执行，等待期间不占用任何数据库行锁。
	if err := emailUtils.CheckAllowed(ctx, record.Email); err != nil {
		return reject(err)
	}
	cfg := config.Config.User.EmailLogin.SES
	if err := tencentses.SendTemplate(
		ctx, record.Email, cfg.TemplateID, cfg.Subject, loginEmailTemplateData(code),
	); err != nil {
		return reject(studentEmailUnavailable())
	}

	// 发信后重新读取当前资料，仅最新签发请求可以发布验证码，审计失败时使验证码失效。
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		student, err := lockStudentLoginEmail(tx, record)
		if err != nil {
			return err
		}
		event := studentEvent(student, auditConstants.ObjectEmailLogin)
		event.After = auditModels.Snapshot{
			"student_id": student.StudentID, "expires_at": record.ExpiresAt,
		}
		if err := writeStudentAudit(
			tx,
			student,
			accessConstants.StudentLoginEmailSend,
			event,
		); err != nil {
			return err
		}
		return cacheEmailLogin(ctx, record, hash)
	})
	if err != nil {
		return reject(err)
	}

	// 邮件及审计提交后记录操作结果，不包含邮箱验证码或凭据。
	logger.Logger.WithContext(ctx).Infof(
		"Student login email issued; workspace: %s, student: %d", record.WorkspaceID, record.StudentID,
	)
	return emailLoginChallengeData(challengeID, record), nil
}

func lockStudentLoginEmail(
	tx *gorm.DB,
	record userModels.EmailLogin,
) (userModels.StudentIdentity, error) {
	// 主键与租户条件精确定位单个学生，禁止在关联查询中扩大加锁范围。
	var student userModels.StudentIdentity
	err := tx.Table("students").Select("id AS student_id, student_no, name, email, status").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND workspace_id = ?", record.StudentID, record.WorkspaceID).
		Take(&student).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return student, studentNotFound()
	}
	if err != nil {
		return student, err
	}

	// 学生锁取得后再用普通查询读取空间，避免等待学生行锁前建立旧快照。
	var workspace workspaceModels.Workspace
	err = tx.Select("id, name, is_enabled").Where("id = ?", record.WorkspaceID).
		Take(&workspace).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return student, studentNotFound()
	}
	if err != nil {
		return student, err
	}
	if !workspace.IsEnabled || workspace.Name != record.WorkspaceName ||
		student.Status != studentConstants.StatusEnrolled || student.StudentNo != record.StudentNo ||
		student.Email != record.Email {
		return student, studentNotFound()
	}
	student.WorkspaceID = workspace.ID
	student.WorkspaceName = workspace.Name
	student.WorkspaceEnabled = workspace.IsEnabled
	return student, nil
}

func loginStudentByEmail(
	ctx context.Context,
	record userModels.EmailLogin,
	hash, payload string,
) (userModels.EmailLoginData, string, error) {
	var student userModels.StudentIdentity
	var session userModels.StudentSession
	token := userConstants.StudentSessionPrefix + randomToken()
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		student, err = lockStudentByID(tx, record.WorkspaceID, record.StudentID)
		if err != nil {
			return err
		}
		if student.Email != record.Email || student.StudentNo != record.StudentNo ||
			student.WorkspaceName != record.WorkspaceName || !time.Now().UTC().Before(record.ExpiresAt) {
			return invalidEmailCode()
		}
		consumed, err := consumeEmailLoginScript.Run(
			ctx, db.RDB(), emailLoginConsumeKeys(record, hash), hash, payload,
		).Int()
		if err != nil {
			return err
		}
		if consumed != 1 {
			return invalidEmailCode()
		}
		logger.Logger.WithContext(ctx).Infof(
			"Student email login code consumed; workspace: %s, student: %d",
			student.WorkspaceID, student.StudentID,
		)

		// 验证码只消费一次；会话和审计同事务提交，提交失败不恢复登录码。
		identity, _ := userUtils.Identity(ctx)
		if identity.Student != nil {
			previous := identity.Student.Session
			result := tx.Where(
				"id = ? AND workspace_id = ? AND student_id = ?",
				previous.ID,
				previous.WorkspaceID,
				previous.StudentID,
			).Delete(&userModels.StudentSession{})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > 0 {
				removed := studentEvent(*identity.Student, auditConstants.ObjectStudentSession)
				removed.Before = studentSessionSnapshot(previous)
				if err := writeStudentAudit(
					tx,
					*identity.Student,
					accessConstants.UserLogin,
					removed,
				); err != nil {
					return err
				}
			}
		}
		session = userModels.StudentSession{
			TokenHash: tokenHash(token), WorkspaceID: student.WorkspaceID,
			StudentID: student.StudentID, StudentNo: student.StudentNo, Email: student.Email,
			ExpiresAt: time.Now().UTC().Add(config.Config.User.SessionTTL),
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND workspace_id = ?", session.ID, student.WorkspaceID).
			Take(&session).Error; err != nil {
			return err
		}
		event := studentEvent(student, auditConstants.ObjectStudentSession)
		event.ObjectID = strconv.FormatUint(session.ID, 10)
		event.After = studentSessionSnapshot(session)
		return writeStudentAudit(tx, student, accessConstants.UserLogin, event)
	})
	if err != nil {
		var business *response.BusinessError
		if errors.As(err, &business) && business.Status == http.StatusUnauthorized {
			err = invalidEmailCode()
		}
		return userModels.EmailLoginData{}, "", failed(
			ctx, userModels.User{}, accessConstants.UserLogin, err,
		)
	}
	logger.Logger.WithContext(ctx).Infof(
		"Student session created; workspace: %s, student: %d, session: %d",
		student.WorkspaceID, student.StudentID, session.ID,
	)
	info := userUtils.StudentInfo(student)
	return userModels.EmailLoginData{User: &info}, token, nil
}

func logoutStudent(ctx context.Context, student userModels.StudentIdentity) error {
	// 按可信身份限定会话所属学生和空间，删除与审计同时提交。
	removed := false
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := lockStudentByID(tx, student.WorkspaceID, student.StudentID)
		if err != nil {
			return err
		}
		if current.StudentNo != student.StudentNo || current.Email != student.Email {
			return studentNotFound()
		}
		result := tx.Where("id = ? AND workspace_id = ? AND student_id = ?",
			student.Session.ID, student.WorkspaceID, student.StudentID).
			Delete(&userModels.StudentSession{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		removed = true
		event := studentEvent(student, auditConstants.ObjectStudentSession)
		event.ObjectID = strconv.FormatUint(student.Session.ID, 10)
		event.Before = studentSessionSnapshot(student.Session)
		return writeStudentAudit(tx, student, accessConstants.UserLogout, event)
	})
	if err != nil {
		return failed(ctx, userModels.User{}, accessConstants.UserLogout, err,
			studentEvent(student, auditConstants.ObjectStudentSession))
	}
	if removed {
		logger.Logger.WithContext(ctx).Infof(
			"Student session deleted; workspace: %s, student: %d, session: %d",
			student.WorkspaceID, student.StudentID, student.Session.ID,
		)
	}
	return nil
}

func lockStudentByID(
	tx *gorm.DB,
	workspaceID string,
	studentID uint64,
) (userModels.StudentIdentity, error) {
	var workspace workspaceModels.Workspace
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", workspaceID).Take(&workspace).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return userModels.StudentIdentity{}, studentNotFound()
	}
	if err != nil {
		return userModels.StudentIdentity{}, err
	}
	return lockStudent(tx, workspace, "id = ?", studentID)
}

func lockStudent(
	tx *gorm.DB,
	workspace workspaceModels.Workspace,
	condition string,
	value any,
) (userModels.StudentIdentity, error) {
	// 查询条件由内部固定调用提供，始终携带工作空间条件且只读取登录字段。
	var student userModels.StudentIdentity
	err := tx.Table("students").Select("id AS student_id, student_no, name, email, status").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ?", workspace.ID).Where(condition, value).Take(&student).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return student, studentNotFound()
	}
	if err != nil {
		return student, err
	}
	if !workspace.IsEnabled || student.Status != studentConstants.StatusEnrolled {
		return student, studentNotFound()
	}
	student.WorkspaceID = workspace.ID
	student.WorkspaceName = workspace.Name
	student.WorkspaceEnabled = workspace.IsEnabled
	return student, nil
}

func studentEvent(student userModels.StudentIdentity, objectType string) auditModels.Event {
	return auditModels.Event{
		TenantID: &student.WorkspaceID, ObjectType: objectType,
		ObjectID:   strconv.FormatUint(student.StudentID, 10),
		Conditions: auditModels.Snapshot{"identity_type": userConstants.IdentityStudent},
	}
}

func writeStudentAudit(
	tx *gorm.DB,
	student userModels.StudentIdentity,
	action string,
	event auditModels.Event,
) error {
	event.Actor = userUtils.StudentActor(student)
	event.Action = action
	event.Result = auditConstants.Success
	return auditSvc.Write(tx, event)
}

func studentSessionSnapshot(session userModels.StudentSession) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":           session.ID,
		"workspace_id": session.WorkspaceID,
		"student_id":   session.StudentID,
		"expires_at":   session.ExpiresAt,
		"created_at":   session.CreatedAt,
		"updated_at":   session.UpdatedAt,
	}
}

func studentNotFound() error {
	return &response.BusinessError{Status: http.StatusUnauthorized, Message: "用户不存在"}
}

func studentEmailUnavailable() error {
	return &response.BusinessError{
		Status: http.StatusServiceUnavailable, Message: "学生邮件登录暂时不可用，请稍后重试",
	}
}
