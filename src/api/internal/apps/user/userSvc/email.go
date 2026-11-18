package userSvc

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apis/tencentses"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/email/emailUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

var emailCodePattern = regexp.MustCompile(userConstants.EmailCodePattern)

// SendLoginEmail 仅在用户名、登记邮箱和启用状态同时满足时发送数字验证码。
func SendLoginEmail(
	ctx context.Context,
	request userModels.SendLoginEmailRequest,
	clientIP string,
) (userModels.EmailLoginChallengeData, error) {
	ctx, span := tracing.Tracer.Start(ctx, "userSvc.SendLoginEmail")
	defer span.End()
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectEmailLogin,
		Conditions: auditModels.Snapshot{"username": request.Username},
	}
	reject := func(err error) (userModels.EmailLoginChallengeData, error) {
		// 知道用户名和邮箱不代表已确认身份，失败时不冒用目标账号写审计。
		return userModels.EmailLoginChallengeData{}, failed(
			ctx, userModels.User{}, accessConstants.UserLoginEmailSend, err, event,
		)
	}
	if db.DB() == nil {
		return reject(unavailable())
	}
	request.Email = strings.TrimSpace(request.Email)
	if !usernamePattern.MatchString(request.Username) || request.Email == "" ||
		!validEmail(request.Email) {
		return reject(badRequest())
	}
	if err := verifyLoginCaptcha(ctx, userModels.LoginRequest{
		CaptchaTicket: request.CaptchaTicket, CaptchaRandstr: request.CaptchaRandstr,
	}, clientIP); err != nil {
		return reject(err)
	}
	if !config.Config.User.EmailLogin.Enabled || db.RDB() == nil {
		return reject(emailLoginUnavailable())
	}
	if err := emailUtils.CheckAllowed(ctx, request.Email); err != nil {
		return reject(err)
	}

	// 账号锁串行化签发、使用、停用和撤销，Redis 独立管理短期凭据。
	var user userModels.User
	code, err := randomEmailCode()
	if err != nil {
		return reject(err)
	}
	challengeID := randomToken()
	hash := tokenHash(challengeID)
	var record userModels.EmailLogin
	reserved := false
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("username = ?", request.Username).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalidEmailCredentials()
		}
		if err != nil {
			return err
		}
		// 在 Go 中逐字比较，避免数据库排序规则放宽用户名或邮箱匹配。
		if !user.IsEnabled || user.Username != request.Username || user.Email != request.Email {
			return invalidEmailCredentials()
		}
		reserved, err = db.RDB().SetNX(
			ctx, emailLoginKeys(user.ID, hash)[2], hash, config.Config.User.EmailLogin.MinInterval,
		).Result()
		if err != nil {
			return err
		}
		if !reserved {
			return &response.BusinessError{
				Status: http.StatusTooManyRequests, Message: response.MessageRateLimited,
			}
		}
		now := time.Now().UTC()
		record = userModels.EmailLogin{
			CodeHash: tokenHash(challengeID + ":" + code),
			UserID:   user.ID, Email: user.Email, SessionVersion: user.SessionVersion,
			ExpiresAt: now.Add(config.Config.User.EmailLogin.TTL), CreatedAt: now,
		}
		event.ObjectID = strconv.FormatUint(user.ID, 10)
		event.After = emailLoginSnapshot(record)
		// 先验证审计可落库，再发邮件；成功后发布新验证码，失败时释放发送额度。
		if err := successful(tx, user, accessConstants.UserLoginEmailSend, event); err != nil {
			return err
		}
		cfg := config.Config.User.EmailLogin.SES
		if err := tencentses.SendTemplate(
			ctx,
			user.Email,
			cfg.TemplateID,
			cfg.Subject,
			loginEmailTemplateData(code),
		); err != nil {
			return emailLoginUnavailable()
		}
		return cacheEmailLogin(ctx, record, hash)
	})
	if err != nil {
		if reserved {
			discardEmailLogin(ctx, user.ID, hash)
		}
		return reject(err)
	}
	// 邮件发送和数据库提交均成功后才记录操作日志，不包含验证码或登录码。
	logger.Logger.WithContext(ctx).Infof(
		"Email login code issued; user: %d", user.ID,
	)
	return emailLoginChallengeData(challengeID, record), nil
}

// LoginByEmail 使用 Redis 原子消费邮件登录码，并沿用密码登录的 Cookie 会话与审计。
func LoginByEmail(
	ctx context.Context,
	request userModels.EmailLoginRequest,
) (userModels.EmailLoginData, string, error) {
	ctx, span := tracing.Tracer.Start(ctx, "userSvc.LoginByEmail")
	defer span.End()
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectEmailLogin,
		Conditions: auditModels.Snapshot{"method": "email"},
	}
	var user userModels.User
	reject := func(err error) (userModels.EmailLoginData, string, error) {
		return userModels.EmailLoginData{}, "", failed(
			ctx, user, accessConstants.UserLogin, err, event,
		)
	}
	if db.DB() == nil {
		return reject(unavailable())
	}
	if !passwordDigestPattern.MatchString(request.ChallengeID) ||
		!emailCodePattern.MatchString(request.Code) ||
		(request.Username != "" && !usernamePattern.MatchString(request.Username)) {
		return reject(badRequest())
	}
	if !config.Config.User.EmailLogin.Enabled || db.RDB() == nil {
		return reject(emailLoginUnavailable())
	}

	// Redis 预读只用于定位账号，在账号锁内复核当前状态并原子消费。
	hash := tokenHash(request.ChallengeID)
	candidate, payload, err := readEmailLogin(ctx, hash)
	if err != nil {
		return reject(err)
	}
	if err := verifyEmailLoginCode(ctx, candidate, hash, payload, request); err != nil {
		return reject(err)
	}
	if candidate.StudentID != 0 {
		return loginStudentByEmail(ctx, candidate, hash, payload)
	}
	var token string
	verified := false
	result := userModels.EmailLoginData{}
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target userModels.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&target, candidate.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return invalidEmailCode()
			}
			return err
		}
		if !target.IsEnabled || target.Email != candidate.Email ||
			target.SessionVersion != candidate.SessionVersion ||
			!time.Now().UTC().Before(candidate.ExpiresAt) {
			return invalidEmailCode()
		}

		// 未核验的邮箱只返回补交用户名的状态，不透露账号、不消费验证码或建立会话。
		if !target.EmailLoginVerified && request.Username == "" {
			valid, err := checkEmailLoginScript.Run(
				ctx, db.RDB(), emailLoginKeys(target.ID, hash)[:2], hash, payload,
			).Int()
			if err != nil {
				return err
			}
			if valid != 1 {
				return invalidEmailCode()
			}
			result.UsernameRequired = true
			return nil
		}
		if request.Username != "" && request.Username != target.Username {
			return &response.BusinessError{
				Status: http.StatusUnauthorized, Message: "用户名不匹配，请重新输入",
			}
		}

		// 用户名必须逐字匹配，核验通过后才原子消费验证码。
		consumed, err := consumeEmailLoginScript.Run(
			ctx, db.RDB(), emailLoginConsumeKeys(candidate, hash), hash, payload,
		).Int()
		if err != nil {
			return err
		}
		if consumed != 1 {
			return invalidEmailCode()
		}
		// 消费后即使数据库提交失败也不恢复登录码，避免并发重放；用户可重新申请。
		user = target
		logger.Logger.WithContext(ctx).Infof(
			"Email login code consumed; user: %d", user.ID,
		)
		if !user.EmailLoginVerified {
			verification := accountEvent(user)
			verification.Conditions = event.Conditions
			verification.Before = accountSnapshot(user)
			user.EmailLoginVerified = true
			if err := tx.Model(&user).Update("email_login_verified", true).Error; err != nil {
				return err
			}
			verification.After = accountSnapshot(user)
			if err := successful(tx, user, accessConstants.UserLogin, verification); err != nil {
				return err
			}
			verified = true
		}
		removed := auditModels.Event{
			ObjectType: auditConstants.ObjectEmailLogin,
			ObjectID:   strconv.FormatUint(user.ID, 10),
			Conditions: event.Conditions,
			Before:     emailLoginSnapshot(candidate),
		}
		if err := successful(tx, user, accessConstants.UserLogin, removed); err != nil {
			return err
		}
		token = randomToken()
		return createLoginSession(tx, user, token, event.Conditions)
	})
	if err != nil {
		return reject(err)
	}

	// 核验标记与会话、审计同事务提交，提交成功后才记录标记变更日志。
	if verified {
		logger.Logger.WithContext(ctx).Infof(
			"Email login username verified; user: %d",
			user.ID,
		)
	}
	if token != "" {
		current := info(user)
		result.User = &current
	}
	return result, token, nil
}

func randomEmailCode() (string, error) {
	// 使用密码学随机数均匀生成六位数字，按字符串保留前导零。
	value, err := rand.Int(rand.Reader, big.NewInt(userConstants.EmailCodeLimit))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value), nil
}

func emailLoginChallengeData(
	challengeID string,
	record userModels.EmailLogin,
) userModels.EmailLoginChallengeData {
	// 仅向发起页面返回校验标识和相对秒数，不返回验证码或摘要。
	seconds := func(until time.Time) int64 {
		return max(0, int64((time.Until(until)+time.Second-1)/time.Second))
	}
	return userModels.EmailLoginChallengeData{
		ChallengeID: challengeID,
		ExpiresIn:   seconds(record.ExpiresAt),
		RetryAfter:  seconds(record.CreatedAt.Add(config.Config.User.EmailLogin.MinInterval)),
	}
}

func loginEmailTemplateData(code string) map[string]string {
	// 模板只接收数字验证码及有效分钟数，不包含校验标识、链接或回跳信息。
	return map[string]string{
		"LOGIN_CODE": code,
		"TTL_MINUTES": strconv.FormatInt(
			int64(config.Config.User.EmailLogin.TTL/time.Minute), 10,
		),
	}
}

func emailLoginSnapshot(record userModels.EmailLogin) auditModels.Snapshot {
	return auditModels.Snapshot{
		"user_id": record.UserID, "email": record.Email,
		"expires_at": record.ExpiresAt, "created_at": record.CreatedAt,
	}
}

func invalidEmailCredentials() error {
	return &response.BusinessError{Status: http.StatusUnauthorized, Message: "用户名或邮箱错误"}
}

func invalidEmailCode() error {
	return &response.BusinessError{
		Status: http.StatusUnauthorized, Message: "验证码错误、已过期或已失效，请重新获取",
	}
}

func emailLoginUnavailable() error {
	return &response.BusinessError{
		Status: http.StatusServiceUnavailable, Message: "邮件登录暂时不可用，请稍后重试",
	}
}
