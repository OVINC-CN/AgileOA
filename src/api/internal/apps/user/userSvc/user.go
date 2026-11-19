package userSvc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

var (
	usernamePattern       = regexp.MustCompile(userConstants.UsernamePattern)
	passwordDigestPattern = regexp.MustCompile(userConstants.DigestPattern)
	emailPattern          = regexp.MustCompile(userConstants.EmailPattern)
	dummyPasswordHash     []byte
)

func init() {
	// 未知账号执行同成本校验，避免通过响应时间枚举账号。
	var err error
	dummyPasswordHash, err = bcrypt.GenerateFromPassword(
		[]byte(randomToken()),
		config.Config.User.BcryptCost,
	)
	if err != nil {
		logger.Logger.Fatalf("Password verifier initialization failed: %v", err)
	}
}

// Bootstrap 在首次 API 启动时创建管理员，之后不会重置或恢复账号。
func Bootstrap(ctx context.Context) error {
	// 未配置数据库时保留探针入口，账号服务拒绝提供业务数据。
	if db.DB() == nil {
		return nil
	}
	ctx, span := tracing.Tracer.Start(ctx, "userSvc.Bootstrap")
	defer span.End()
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 唯一初始化行锁保证多实例启动只创建一次。
		var state userModels.Bootstrap
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&state, 1).
			Error; err != nil {
			return fmt.Errorf("bootstrap state unavailable; execute the manual SQL first: %w", err)
		}
		if state.Initialized {
			return nil
		}
		var count int64
		if err := tx.Model(&userModels.User{}).
			Where("is_admin = ?", true).
			Count(&count).
			Error; err != nil {
			return fmt.Errorf("administrator lookup failed: %w", err)
		}
		if count == 0 {
			// 私有 YAML 中的初始密码按相同的用户名摘要与 bcrypt 规则存储。
			cfg := config.Config.User
			if cfg.BootstrapPassword == "" {
				return errors.New("user.bootstrap_password is required for the first API startup")
			}
			digest := sha256.Sum256([]byte(cfg.BootstrapUsername + ":" + cfg.BootstrapPassword))
			hash, err := bcrypt.GenerateFromPassword(
				[]byte(hex.EncodeToString(digest[:])),
				cfg.BcryptCost,
			)
			if err != nil {
				return fmt.Errorf("administrator password hashing failed: %w", err)
			}
			admin := userModels.User{
				Username:               cfg.BootstrapUsername,
				DisplayName:            cfg.BootstrapDisplayName,
				PasswordHash:           string(hash),
				PasswordLoginEnabled:   true,
				PasswordChangeRequired: true,
				IsAdmin:                true,
				IsEnabled:              true,
			}
			if err := tx.Create(&admin).Error; err != nil {
				return fmt.Errorf("administrator creation failed: %w", err)
			}
			// 启动初始化没有已登录操作者，不生成审计记录。
		}
		// 初始化状态与账号共用事务，不创建没有操作者身份的审计。
		if err := tx.Model(&state).Update("initialized", true).Error; err != nil {
			return fmt.Errorf("bootstrap state persistence failed: %w", err)
		}
		return nil
	})
	response.RecordError(ctx, err)
	return err
}

// Login 校验验证码和启用账号后创建会话，原始令牌仅返回给 Cookie 写入逻辑。
func Login(
	ctx context.Context,
	request userModels.LoginRequest,
	clientIP string,
) (userModels.Info, string, error) {
	// 审计条件仅包含用户名，口令摘要不进入审计参数。
	ctx, span := tracing.Tracer.Start(ctx, "userSvc.Login")
	defer span.End()
	var user userModels.User
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		Conditions: auditModels.Snapshot{"username": request.Username},
	}

	// 全局关闭时直接拒绝，不校验验证码、读取账号或创建密码登录会话。
	if !config.Config.User.PasswordLoginEnabled {
		return userModels.Info{}, "", failed(
			ctx,
			user,
			accessConstants.UserLogin,
			invalidCredentials(),
			event,
		)
	}

	// 检查账号服务及登录参数。
	if db.DB() == nil {
		return userModels.Info{}, "", unavailable()
	}
	if !usernamePattern.MatchString(request.Username) ||
		!passwordDigestPattern.MatchString(request.PasswordSHA256) {
		return userModels.Info{}, "", failed(
			ctx,
			user,
			accessConstants.UserLogin,
			badRequest(),
			event,
		)
	}

	// 先核查本次验证码票据，再查询账号或计算 bcrypt。
	if err := verifyLoginCaptcha(ctx, request, clientIP); err != nil {
		return userModels.Info{}, "", failed(ctx, user, accessConstants.UserLogin, err, event)
	}

	// 慢速口令比较在锁外执行，事务中再次复核账号状态和散列。
	err := db.DB().WithContext(ctx).Where("username = ?", request.Username).First(&user).Error
	hash := dummyPasswordHash
	if err == nil && user.PasswordLoginEnabled {
		hash = []byte(user.PasswordHash)
	}
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(request.PasswordSHA256))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return userModels.Info{}, "", failed(
			ctx,
			user,
			accessConstants.UserLogin,
			err,
			event,
		)
	}
	if err != nil || passwordErr != nil || !user.IsEnabled || !user.PasswordLoginEnabled {
		return userModels.Info{}, "", failed(
			ctx,
			userModels.User{},
			accessConstants.UserLogin,
			invalidCredentials(),
			event,
		)
	}
	token := randomToken()
	verifiedHash := user.PasswordHash
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&user, user.ID).
			Error; err != nil {
			return err
		}
		if !user.IsEnabled || !user.PasswordLoginEnabled || user.PasswordHash != verifiedHash {
			return invalidCredentials()
		}
		return createLoginSession(tx, user, token, event.Conditions)
	})
	if err != nil {
		return userModels.Info{}, "", failed(ctx, user, accessConstants.UserLogin, err, event)
	}
	return info(user), token, nil
}

func createLoginSession(
	tx *gorm.DB,
	user userModels.User,
	token string,
	conditions auditModels.Snapshot,
) error {
	// 旋转已有会话时逐条记录删除与创建，共享同一个请求 ID。
	identity, _ := userUtils.Identity(tx.Statement.Context)
	if identity.Session.ID != 0 {
		var previous userModels.Session
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", identity.Session.ID).
			First(&previous).
			Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if err := tx.Delete(&previous).Error; err != nil {
				return err
			}
			removed := sessionEvent(previous)
			removed.Before = sessionSnapshot(previous)
			if err := successful(tx, user, accessConstants.UserLogin, removed); err != nil {
				return err
			}
		}
	}
	now := time.Now().UTC()
	session := userModels.Session{
		TokenHash:      tokenHash(token),
		SessionVersion: user.SessionVersion,
		UserID:         user.ID,
		ExpiresAt:      now.Add(config.Config.User.SessionTTL),
	}
	if err := tx.Create(&session).Error; err != nil {
		return err
	}
	if err := tx.Where("id = ?", session.ID).Take(&session).Error; err != nil {
		return err
	}
	created := sessionEvent(session)
	created.Conditions = conditions
	created.After = sessionSnapshot(session)
	return successful(tx, user, accessConstants.UserLogin, created)
}

// Info 根据中间件确认的身份返回当前账号，成功和失败均不纳入审计。
func Info(ctx context.Context) (userModels.Info, error) {
	identity, err := userUtils.Identity(ctx)
	if err != nil {
		return userModels.Info{}, err
	}
	if identity.Student != nil {
		return userUtils.StudentInfo(*identity.Student), nil
	}
	return info(identity.User), nil
}

// Logout 仅撤销当前会话，快照不包含凭据。
func Logout(ctx context.Context) error {
	if identity, err := userUtils.Identity(ctx); err == nil && identity.Student != nil {
		return logoutStudent(ctx, *identity.Student)
	}
	return authenticated(
		ctx,
		accessConstants.UserLogout,
		func(tx *gorm.DB, user userModels.User, session userModels.Session) (auditModels.Event, error) {
			event := accountEvent(user)
			if session.ID != 0 {
				event = sessionEvent(session)
				event.Before = sessionSnapshot(session)
				if err := tx.Delete(&session).Error; err != nil {
					return event, err
				}
			}
			return event, nil
		},
	)
}

// Create 仅允许启用的系统管理员添加普通账号，不自动授予工作空间业务 action。
func Create(
	ctx context.Context,
	request userModels.CreateRequest,
) (userModels.Info, error) {
	// 固定字段目录与前端相同，密码摘要只用于 bcrypt，不进入事件。
	email := strings.TrimSpace(request.Email)
	passwordLoginEnabled := request.PasswordLoginEnabled == nil || *request.PasswordLoginEnabled
	conditions := auditModels.Snapshot{
		"username":               request.Username,
		"display_name":           request.DisplayName,
		"email":                  email,
		"password_login_enabled": passwordLoginEnabled,
	}
	if !usernamePattern.MatchString(request.Username) ||
		strings.TrimSpace(request.DisplayName) == "" ||
		utf8.RuneCountInString(request.DisplayName) > 100 ||
		!validEmail(email) ||
		(passwordLoginEnabled && !passwordDigestPattern.MatchString(request.PasswordSHA256)) ||
		(!passwordLoginEnabled && request.PasswordSHA256 != "") {
		return userModels.Info{}, failed(
			ctx,
			userModels.User{},
			accessConstants.UserCreate,
			badRequest(),
			auditModels.Event{ObjectType: auditConstants.ObjectUser, Conditions: conditions},
		)
	}
	created := userModels.User{
		Username:               request.Username,
		DisplayName:            strings.TrimSpace(request.DisplayName),
		Email:                  email,
		PasswordLoginEnabled:   passwordLoginEnabled,
		PasswordChangeRequired: passwordLoginEnabled,
		IsEnabled:              true,
	}
	err := authenticated(
		ctx,
		accessConstants.UserCreate,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			event := auditModels.Event{
				ObjectType: auditConstants.ObjectUser,
				Conditions: conditions,
			}
			// 先复核管理员权限，仅为允许密码登录的账号保存口令。
			if created.PasswordLoginEnabled {
				hash, err := bcrypt.GenerateFromPassword(
					[]byte(request.PasswordSHA256),
					config.Config.User.BcryptCost,
				)
				if err != nil {
					return event, err
				}
				created.PasswordHash = string(hash)
			}
			if err := tx.Create(&created).Error; err != nil {
				var mysqlError *mysql.MySQLError
				if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
					return event, &response.BusinessError{
						Status:  http.StatusConflict,
						Message: "用户名已存在",
					}
				}
				return event, err
			}
			if err := tx.Where("id = ?", created.ID).Take(&created).Error; err != nil {
				return event, err
			}
			event.ObjectID = strconv.FormatUint(created.ID, 10)
			event.After = accountSnapshot(created)
			return event, nil
		},
		auditModels.Event{ObjectType: auditConstants.ObjectUser, Conditions: conditions},
	)
	return info(created), err
}

func authenticated(
	ctx context.Context,
	action string,
	operation func(*gorm.DB, userModels.User, userModels.Session) (auditModels.Event, error),
	details ...auditModels.Event,
) error {
	// 只使用中间件身份，在账号行锁内复核业务权限。
	ctx, span := tracing.Tracer.Start(ctx, action)
	defer span.End()
	var user userModels.User
	if db.DB() == nil {
		return unavailable()
	}
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 跨账号操作先锁定固定初始化行，查看共享锁，修改排他锁。
		if action == accessConstants.UserCreate || action == accessConstants.UserUpdate ||
			action == accessConstants.UserDelete || action == accessConstants.UserRevokeSessions ||
			action == accessConstants.UserAPITokenWrite || action == accessConstants.UserAPITokenDelete ||
			action == accessConstants.UserAPITokenRead {
			var lock userModels.Bootstrap
			strength := "UPDATE"
			if accessConstants.ReadOnly(action) {
				strength = "SHARE"
			}
			if err := tx.Clauses(clause.Locking{Strength: strength}).
				First(&lock, 1).
				Error; err != nil {
				return err
			}
		}
		identity, identityErr := userUtils.Identity(ctx)
		if identityErr != nil {
			return identityErr
		}
		var authErr error
		user, authErr = userUtils.Require(tx, action)
		if authErr != nil {
			return authErr
		}
		event, err := operation(tx, user, identity.Session)
		if err != nil {
			return err
		}
		return successful(tx, user, action, event)
	})
	if err != nil {
		return failed(ctx, user, action, err, details...)
	}
	return nil
}

func successful(tx *gorm.DB, user userModels.User, action string, event auditModels.Event) error {
	// 这里只提供账号业务语义，审计生成、快照差异、加密与落库均由 audit app 负责。
	event.Actor = actor(user)
	event.Action = action
	event.Result = auditConstants.Success
	return auditSvc.Write(tx, event)
}

func failed(
	ctx context.Context,
	user userModels.User,
	action string,
	cause error,
	details ...auditModels.Event,
) error {
	// 公开错误不携带组件详情，转换前先记录原始异常和关联日志。
	response.RecordError(ctx, cause)
	// 只使用固定的公开错误说明，禁止把底层 SQL 异常或参数交给审计。
	failure := &response.BusinessError{Status: http.StatusServiceUnavailable, Message: "账号服务暂时不可用"}
	var provided *response.BusinessError
	if errors.As(cause, &provided) {
		failure = provided
	}
	event := accountEvent(user)
	if len(details) > 0 {
		event = details[0]
	}
	event.Actor = actor(user)
	event.Action = action
	event.ErrorCode = "http_" + strconv.Itoa(failure.Status)
	event.ErrorMessage = failure.Message
	if err := auditSvc.Failure(ctx, event); err != nil {
		response.RecordError(ctx, err)
		return unavailable()
	}
	return failure
}

func actor(user userModels.User) auditModels.Actor {
	return auditModels.Actor{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
	}
}

func accountEvent(user userModels.User) auditModels.Event {
	return auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		ObjectID:   strconv.FormatUint(user.ID, 10),
	}
}

func accountSnapshot(user userModels.User) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":                       user.ID,
		"username":                 user.Username,
		"display_name":             user.DisplayName,
		"email":                    user.Email,
		"email_login_verified":     user.EmailLoginVerified,
		"password_login_enabled":   user.PasswordLoginEnabled,
		"password_change_required": user.PasswordChangeRequired,
		"is_admin":                 user.IsAdmin,
		"is_enabled":               user.IsEnabled,
		"created_at":               user.CreatedAt,
		"revision":                 user.SessionVersion,
	}
}

func sessionEvent(session userModels.Session) auditModels.Event {
	return auditModels.Event{
		ObjectType: auditConstants.ObjectSession,
		ObjectID:   strconv.FormatUint(session.ID, 10),
		Conditions: auditModels.Snapshot{"user_id": session.UserID},
	}
}

func sessionSnapshot(session userModels.Session) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id":         session.ID,
		"user_id":    session.UserID,
		"expires_at": session.ExpiresAt,
		"created_at": session.CreatedAt,
	}
}

func validEmail(email string) bool {
	// 邮箱可留空，只接受纯地址，不接受带显示名称的格式。
	return email == "" ||
		(len(email) <= userConstants.MaxEmailLength && emailPattern.MatchString(email))
}

func info(user userModels.User) userModels.Info {
	return userModels.Info{
		IdentityType:           userConstants.IdentityUser,
		ID:                     user.ID,
		Username:               user.Username,
		DisplayName:            user.DisplayName,
		Email:                  user.Email,
		IsAdmin:                user.IsAdmin,
		PasswordLoginEnabled:   user.PasswordLoginEnabled,
		PasswordChangeRequired: user.PasswordLoginEnabled && user.PasswordChangeRequired,
	}
}

func randomToken() string {
	// 熵源失败时 crypto/rand 会终止进程，不回退到可预测随机值。
	value := make([]byte, userConstants.SessionBytes)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}

func tokenHash(token string) string {
	value := sha256.Sum256([]byte(token))
	return hex.EncodeToString(value[:])
}

func unavailable() error {
	return &response.BusinessError{Status: http.StatusServiceUnavailable, Message: "账号服务暂时不可用"}
}
func badRequest() error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: "账号参数无效"}
}
func invalidCredentials() error {
	return &response.BusinessError{Status: http.StatusUnauthorized, Message: "用户名或密码错误"}
}

// Reject 对参数错误及跨站保护拒绝调用独立审计服务，审计不可用时阻断响应。
func Reject(ctx context.Context, action string, status int) error {
	message := response.MessageBadRequest
	if status == http.StatusForbidden {
		message = response.MessageForbidden
	}
	return failed(
		ctx,
		userModels.User{},
		action,
		&response.BusinessError{Status: status, Message: message},
	)
}
