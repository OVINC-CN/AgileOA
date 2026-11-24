package userSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectUtils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/displaytime"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// List 返回用户分页目录；查询结果在成功审计提交后返回。
func List(
	ctx context.Context,
	query userModels.ListQuery,
) (userModels.ListData, error) {
	result := userModels.ListData{
		Items:    []userModels.Row{},
		Page:     query.Page,
		PageSize: query.PageSize,
	}
	conditions := auditModels.Snapshot{
		"page":      query.Page,
		"page_size": query.PageSize,
		"keyword":   query.Keyword,
		"status":    query.Statuses,
	}
	err := authenticated(
		ctx,
		accessConstants.UserList,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			event := auditModels.Event{
				ObjectType: auditConstants.ObjectUser,
				Conditions: conditions,
			}
			scope := tx.Model(&userModels.User{})
			if query.Keyword != "" {
				scope = scope.Where(
					"username LIKE ? OR display_name LIKE ? OR email LIKE ?",
					"%"+query.Keyword+"%",
					"%"+query.Keyword+"%",
					"%"+query.Keyword+"%",
				)
			}
			if len(query.Statuses) > 0 {
				states := make([]bool, 0, len(query.Statuses))
				for _, status := range query.Statuses {
					states = append(states, status == "enabled")
				}
				scope = scope.Where("is_enabled IN ?", states)
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return event, err
			}
			var users []userModels.User
			if err := scope.Select([]string{
				"id",
				"username",
				"display_name",
				"email",
				"is_admin",
				"is_enabled",
				"created_at",
				"updated_at",
				"password_login_enabled",
				"password_change_required",
			}).
				Order("id DESC").
				Offset((query.Page - 1) * query.PageSize).
				Limit(query.PageSize).
				Find(&users).
				Error; err != nil {
				return event, err
			}
			ids := make([]uint64, 0, len(users))
			for _, user := range users {
				ids = append(ids, user.ID)
			}
			// 会话引用和最近登录按当前页编号一次查询，固定只读取全局账号审计。
			type sessionCount struct {
				UserID uint64
				Count  int64
			}

			var counts []sessionCount
			var apiTokens []userModels.APIToken
			var logins []auditModels.LatestLogin
			if len(ids) > 0 {
				if err := tx.Select("user_id").Where("user_id IN ?", ids).
					Find(&apiTokens).Error; err != nil {
					return event, err
				}
				if err := tx.Model(&userModels.Session{}).
					Select("user_id, COUNT(*) AS count").
					Where("user_id IN ?", ids).
					Group("user_id").
					Scan(&counts).
					Error; err != nil {
					return event, err
				}
				rows, err := auditSvc.LatestLogins(tx, ids)
				if err != nil {
					return event, err
				}
				logins = rows
			}
			sessions := map[uint64]int64{}
			hasAPIToken := map[uint64]bool{}
			for _, token := range apiTokens {
				hasAPIToken[token.UserID] = true
			}
			latest := map[uint64]time.Time{}
			for _, count := range counts {
				sessions[count.UserID] = count.Count
			}
			for _, login := range logins {
				latest[login.UserID] = login.Latest
			}
			for _, user := range users {
				row := userModels.Row{
					Info:         info(user),
					IsEnabled:    user.IsEnabled,
					CreatedAt:    user.CreatedAt,
					UpdatedAt:    user.UpdatedAt,
					SessionCount: sessions[user.ID],
					HasAPIToken:  hasAPIToken[user.ID],
				}
				if at, ok := latest[user.ID]; ok {
					row.LastLoginAt = &at
				}
				result.Items = append(result.Items, row)
				event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(user.ID, 10))
			}
			event.ReturnedCount = int64(len(result.Items))
			return event, nil
		},
		auditModels.Event{ObjectType: auditConstants.ObjectUser, Conditions: conditions},
	)
	return result, err
}

// Statistics 从真实账号与登录审计汇总统计，空日期补零。
func Statistics(ctx context.Context) (userModels.StatisticsData, error) {
	var result userModels.StatisticsData
	start, end := displaytime.Window(time.Now(), config.Config.User.ChartDays)
	first := displaytime.Date(start)
	last := displaytime.Date(end.Add(-time.Nanosecond))
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		ObjectID:   "statistics",
		Conditions: auditModels.Snapshot{
			"from": first.Format(time.DateOnly),
			"to":   last.Format(time.DateOnly),
		},
	}
	err := authenticated(
		ctx,
		accessConstants.UserStatistics,
		func(tx *gorm.DB, _ userModels.User, _ userModels.Session) (auditModels.Event, error) {
			if err := tx.Model(&userModels.User{}).
				Select("COUNT(*) AS total, COALESCE(SUM(is_enabled = 1),0) AS enabled, COALESCE(SUM(is_enabled = 0),0) AS disabled, COALESCE(SUM(is_admin = 1),0) AS admins").
				Scan(&result).
				Error; err != nil {
				return event, err
			}
			type count struct {
				Date  string
				Count int64
			}
			var created []count
			dateColumn := displaytime.DateSQL("created_at", start, end)
			if err := tx.Model(&userModels.User{}).
				Select("? AS date, COUNT(*) AS count", dateColumn).
				Where("created_at >= ? AND created_at < ?", start, end).
				Group("date").
				Scan(&created).
				Error; err != nil {
				return event, err
			}
			logins, err := auditSvc.LoginTrend(tx, start, end)
			if err != nil {
				return event, err
			}
			byCreated, byLogin := map[string]int64{}, map[string]int64{}
			for _, item := range created {
				byCreated[item.Date] = item.Count
			}
			for _, item := range logins {
				byLogin[item.Date] = item.Count
				result.Logins += item.Count
			}
			for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
				date := day.Format(time.DateOnly)
				result.Days = append(
					result.Days,
					userModels.Day{Date: date, Created: byCreated[date], Logins: byLogin[date]},
				)
			}
			event.ReturnedIDs = []string{"statistics"}
			event.ReturnedCount = 1
			return event, nil
		},
		event,
	)
	return result, err
}

// Update 更新用户，调整密码登录、重置口令或停用会作废旧会话，但不自动删除会话引用。
func Update(
	ctx context.Context,
	id uint64,
	request userModels.UpdateRequest,
) (userModels.Info, error) {
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		ObjectID:   strconv.FormatUint(id, 10),
		Conditions: auditModels.Snapshot{"id": id},
	}
	if request.IsEnabled == nil || strings.TrimSpace(request.DisplayName) == "" ||
		utf8.RuneCountInString(request.DisplayName) > 100 ||
		(request.Email != nil && !validEmail(strings.TrimSpace(*request.Email))) ||
		(request.PasswordSHA256 != "" && !passwordDigestPattern.MatchString(request.PasswordSHA256)) {
		return userModels.Info{}, failed(
			ctx,
			userModels.User{},
			accessConstants.UserUpdate,
			badRequest(),
			event,
		)
	}
	var target userModels.User
	verificationReset := false
	apiTokenRemoved := false
	err := authenticated(
		ctx,
		accessConstants.UserUpdate,
		func(tx *gorm.DB, operator userModels.User, _ userModels.Session) (auditModels.Event, error) {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&target, id).
				Error; err != nil {
				return event, targetError(err)
			}
			if operator.ID == target.ID && !*request.IsEnabled {
				return event, conflict("不能停用当前登录账号")
			}
			event.Before = accountSnapshot(target)
			disableAccount := target.IsEnabled && !*request.IsEnabled
			target.DisplayName = strings.TrimSpace(request.DisplayName)
			if request.Email != nil && target.Email != strings.TrimSpace(*request.Email) {
				target.Email = strings.TrimSpace(*request.Email)
				verificationReset = target.EmailLoginVerified
				target.EmailLoginVerified = false
			}
			if disableAccount {
				target.SessionVersion++
			}
			target.IsEnabled = *request.IsEnabled

			// 调整密码登录方式时作废旧会话，禁用时清除口令及待改密状态。
			if request.PasswordLoginEnabled != nil &&
				target.PasswordLoginEnabled != *request.PasswordLoginEnabled {
				target.PasswordLoginEnabled = *request.PasswordLoginEnabled
				target.SessionVersion++
			}
			if !target.PasswordLoginEnabled {
				if request.PasswordSHA256 != "" {
					return event, &response.BusinessError{
						Status:  http.StatusBadRequest,
						Message: "未允许密码登录，不能设置临时密码",
					}
				}
				target.PasswordHash = ""
				target.PasswordChangeRequired = false
			} else if target.PasswordHash == "" && request.PasswordSHA256 == "" {
				return event, &response.BusinessError{
					Status:  http.StatusBadRequest,
					Message: "启用密码登录时请设置临时密码",
				}
			}
			if request.PasswordSHA256 != "" {
				hash, err := bcrypt.GenerateFromPassword(
					[]byte(request.PasswordSHA256),
					config.Config.User.BcryptCost,
				)
				if err != nil {
					return event, err
				}
				target.PasswordHash = string(hash)
				target.PasswordChangeRequired = true
				target.SessionVersion++
			}
			if err := tx.Model(&userModels.User{}).
				Where("id = ?", target.ID).
				Updates(map[string]any{
					"display_name":             target.DisplayName,
					"email":                    target.Email,
					"email_login_verified":     target.EmailLoginVerified,
					"is_enabled":               target.IsEnabled,
					"password_login_enabled":   target.PasswordLoginEnabled,
					"password_hash":            target.PasswordHash,
					"password_change_required": target.PasswordChangeRequired,
					"session_version":          target.SessionVersion,
				}).
				Error; err != nil {
				return event, err
			}
			// 仅停用账号时同步撤销 API Token，密码设置只影响网页登录会话。
			if disableAccount {
				var err error
				apiTokenRemoved, err = removeAPIToken(
					tx, operator, accessConstants.UserUpdate, target.ID,
				)
				if err != nil {
					return event, err
				}
			}
			event.After = accountSnapshot(target)
			return event, nil
		},
		event,
	)
	// 邮箱核验状态不向账号响应公开，事务提交成功后记录实际重置。
	if err == nil && verificationReset {
		logger.Logger.WithContext(ctx).Infof(
			"Email login username verification reset; user: %d",
			target.ID,
		)
	}
	logAPITokenRemoval(ctx, id, apiTokenRemoved && err == nil)
	return info(target), err
}

// Delete 硬删除用户，先校验会话、API Token 及审批引用，其余依赖记录按外键级联删除。
func Delete(ctx context.Context, id uint64) error {
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		ObjectID:   strconv.FormatUint(id, 10),
		Conditions: auditModels.Snapshot{"id": id},
	}
	return authenticated(
		ctx,
		accessConstants.UserDelete,
		func(tx *gorm.DB, operator userModels.User, _ userModels.Session) (auditModels.Event, error) {
			var target userModels.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&target, id).
				Error; err != nil {
				return event, targetError(err)
			}
			if target.ID == operator.ID {
				return event, conflict("不能删除当前登录账号")
			}
			var references int64
			if err := tx.Model(&userModels.Session{}).
				Where("user_id = ?", id).
				Count(&references).
				Error; err != nil {
				return event, err
			}
			if references > 0 {
				return event, conflict("该用户存在会话引用，请先单独撤销会话后再删除")
			}
			if err := tx.Model(&userModels.APIToken{}).
				Where("user_id = ?", id).Count(&references).Error; err != nil {
				return event, err
			}
			if references > 0 {
				return event, conflict("该用户存在 API Token，请先单独撤销会话或 Token 后再删除")
			}
			if err := projectUtils.GuardUserDeletion(tx, id); err != nil {
				return event, err
			}
			event.Before = accountSnapshot(target)
			if err := tx.Delete(&target).Error; err != nil {
				return event, conflict("删除用户失败，请检查关联业务后重试")
			}
			return event, nil
		},
		event,
	)
}

// RevokeSessions 显式撤销目标账号全部会话，逐条审计且不自动删除账号。
func RevokeSessions(ctx context.Context, id uint64) error {
	event := auditModels.Event{
		ObjectType: auditConstants.ObjectUser,
		ObjectID:   strconv.FormatUint(id, 10),
		Conditions: auditModels.Snapshot{"id": id},
	}
	var removed bool
	err := authenticated(
		ctx,
		accessConstants.UserRevokeSessions,
		func(tx *gorm.DB, operator userModels.User, _ userModels.Session) (auditModels.Event, error) {
			if operator.ID == id {
				return event, conflict("当前登录账号请通过退出登录结束会话")
			}
			var target userModels.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&target, id).
				Error; err != nil {
				return event, targetError(err)
			}
			var err error
			removed, err = removeAPIToken(tx, operator, accessConstants.UserRevokeSessions, id)
			if err != nil {
				return event, err
			}
			var sessions []userModels.Session
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ?", id).
				Find(&sessions).
				Error; err != nil {
				return event, err
			}
			for _, session := range sessions {
				removed := sessionEvent(session)
				removed.Before = sessionSnapshot(session)
				if err := tx.Delete(&session).Error; err != nil {
					return event, err
				}
				if err := successful(
					tx,
					operator,
					accessConstants.UserRevokeSessions,
					removed,
				); err != nil {
					return event, err
				}
			}
			// 提升账号会话版本，同事务作废 Redis 中尚未使用的邮件验证码。
			event.Before = accountSnapshot(target)
			target.SessionVersion++
			if err := tx.Model(&target).
				Update("session_version", target.SessionVersion).
				Error; err != nil {
				return event, err
			}
			event.After = accountSnapshot(target)
			return event, nil
		},
		event,
	)
	logAPITokenRemoval(ctx, id, removed && err == nil)
	return err
}

func targetError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &response.BusinessError{Status: http.StatusNotFound, Message: "用户不存在"}
	}
	return err
}

func conflict(message string) error {
	return &response.BusinessError{Status: http.StatusConflict, Message: message}
}
