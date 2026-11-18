package userSvc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

var publishEmailLoginScript = redis.NewScript(`
if #KEYS > 2 and redis.call('GET', KEYS[3]) ~= ARGV[3] then
    return 0
end
local previous = redis.call('GET', KEYS[2])
if previous then
    redis.call('DEL', ARGV[4] .. previous, ARGV[5] .. previous)
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[2], ARGV[3], 'PX', ARGV[2])
if #KEYS > 2 then
    redis.call('DEL', KEYS[3])
end
return 1
`)

var reserveStudentEmailLoginScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
    return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[3])
return 1
`)

var consumeEmailLoginScript = redis.NewScript(`
if redis.call('GET', KEYS[2]) ~= ARGV[1] or redis.call('GET', KEYS[1]) ~= ARGV[2] then
    return 0
end
redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
return 1
`)

var verifyEmailLoginCodeScript = redis.NewScript(`
if redis.call('GET', KEYS[2]) ~= ARGV[1] or redis.call('GET', KEYS[1]) ~= ARGV[2] then
    return 0
end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then
    return 0
end
local attempts = tonumber(redis.call('GET', KEYS[3]) or '0')
if attempts >= tonumber(ARGV[4]) then
    redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
    return 0
end
if cjson.decode(ARGV[2]).code_hash ~= ARGV[3] then
    attempts = attempts + 1
    if attempts >= tonumber(ARGV[4]) then
        redis.call('DEL', KEYS[1], KEYS[2], KEYS[3])
    else
        redis.call('SET', KEYS[3], attempts, 'PX', ttl)
    end
    return 0
end
return 1
`)

var checkEmailLoginScript = redis.NewScript(`
if redis.call('GET', KEYS[2]) ~= ARGV[1] or redis.call('GET', KEYS[1]) ~= ARGV[2] then
    return 0
end
return 1
`)

var discardEmailLoginScript = redis.NewScript(`
local removed = redis.call('DEL', KEYS[1], ARGV[2])
if redis.call('GET', KEYS[2]) == ARGV[1] then
    removed = removed + redis.call('DEL', KEYS[2])
end
if redis.call('GET', KEYS[3]) == ARGV[1] then
    removed = removed + redis.call('DEL', KEYS[3])
end
if #KEYS > 3 and redis.call('GET', KEYS[4]) == ARGV[1] then
    removed = removed + redis.call('DEL', KEYS[4])
end
return removed
`)

func emailLoginKeys(userID uint64, hash string) []string {
	id := strconv.FormatUint(userID, 10)
	return []string{
		userConstants.EmailLoginChallengePrefix + hash,
		userConstants.EmailLoginUserPrefix + id,
		userConstants.EmailLoginCooldownPrefix + id,
	}
}

func emailLoginChallengeKeys(record userModels.EmailLogin, hash string) []string {
	if record.StudentID == 0 {
		return emailLoginKeys(record.UserID, hash)
	}
	// 学生验证码与管理账号共用随机码命名空间，发送额度及最新验证码索引相互隔离。
	id := record.WorkspaceID + ":" + strconv.FormatUint(record.StudentID, 10)
	return []string{
		userConstants.EmailLoginChallengePrefix + hash,
		userConstants.StudentEmailLoginPrefix + id,
		userConstants.StudentEmailCooldownPrefix + id,
		userConstants.StudentEmailPendingPrefix + id,
	}
}

func emailLoginConsumeKeys(record userModels.EmailLogin, hash string) []string {
	keys := emailLoginChallengeKeys(record, hash)
	return []string{keys[0], keys[1], userConstants.EmailLoginAttemptsPrefix + hash}
}

func verifyEmailLoginCode(
	ctx context.Context,
	record userModels.EmailLogin,
	hash, payload string,
	request userModels.EmailLoginRequest,
) error {
	// 原子比较本次验证码摘要并累计错误次数，失败不能延长验证码的有效期。
	valid, err := verifyEmailLoginCodeScript.Run(
		ctx,
		db.RDB(),
		emailLoginConsumeKeys(record, hash),
		hash,
		payload,
		tokenHash(request.ChallengeID+":"+request.Code),
		config.Config.User.EmailLogin.MaxAttempts,
	).Int()
	if err != nil {
		return err
	}
	if valid != 1 {
		return invalidEmailCode()
	}
	return nil
}

func reserveStudentLoginEmail(
	ctx context.Context,
	record userModels.EmailLogin,
	hash string,
) error {
	// 冷却期独立于在途签发；冷却期结束后重发时，旧请求不能覆盖新请求的验证码。
	ttl := time.Until(record.ExpiresAt)
	if ttl < time.Millisecond {
		return studentEmailUnavailable()
	}
	reserved, err := reserveStudentEmailLoginScript.Run(
		ctx, db.RDB(), emailLoginChallengeKeys(record, hash)[2:],
		hash, config.Config.User.EmailLogin.MinInterval.Milliseconds(), ttl.Milliseconds(),
	).Int()
	if err != nil {
		return err
	}
	if reserved != 1 {
		return &response.BusinessError{
			Status: http.StatusTooManyRequests, Message: response.MessageRateLimited,
		}
	}
	return nil
}

func cacheEmailLogin(ctx context.Context, record userModels.EmailLogin, hash string) error {
	// 仅以摘要作为键，登录信息与账号索引使用相同 TTL 原子发布，并作废旧验证码。
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ttl := time.Until(record.ExpiresAt)
	if ttl < time.Millisecond {
		return emailLoginUnavailable()
	}
	keys := emailLoginChallengeKeys(record, hash)
	if record.StudentID != 0 {
		keys = []string{keys[0], keys[1], keys[3]}
	} else {
		keys = keys[:2]
	}
	published, err := publishEmailLoginScript.Run(
		ctx, db.RDB(), keys,
		string(payload), ttl.Milliseconds(), hash,
		userConstants.EmailLoginChallengePrefix, userConstants.EmailLoginAttemptsPrefix,
	).Int()
	if err != nil {
		return err
	}
	if published != 1 {
		return studentEmailUnavailable()
	}
	return nil
}

func readEmailLogin(ctx context.Context, hash string) (userModels.EmailLogin, string, error) {
	// 不把 Redis 返回内容交给日志或追踪，缺失与已过期验证码使用相同公开错误。
	var record userModels.EmailLogin
	payload, err := db.RDB().Get(ctx, userConstants.EmailLoginChallengePrefix+hash).Result()
	if errors.Is(err, redis.Nil) {
		return record, "", invalidEmailCode()
	}
	if err != nil {
		return record, "", err
	}
	if err := json.Unmarshal([]byte(payload), &record); err != nil ||
		(record.UserID == 0 && record.StudentID == 0) ||
		(record.UserID != 0 && record.StudentID != 0) ||
		!passwordDigestPattern.MatchString(record.CodeHash) {
		return record, "", errors.New("invalid email login cache data")
	}
	return record, payload, nil
}

func discardEmailLogin(ctx context.Context, userID uint64, hash string) {
	discardEmailLoginChallenge(ctx, userModels.EmailLogin{UserID: userID}, hash)
}

func discardEmailLoginChallenge(ctx context.Context, record userModels.EmailLogin, hash string) {
	// 请求取消后仍有界清理本次失败的签发，不覆盖并发的新验证码或发送额度。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.Config.Redis.WriteTimeout)
	defer cancel()
	removed, err := discardEmailLoginScript.Run(
		ctx, db.RDB(), emailLoginChallengeKeys(record, hash),
		hash, userConstants.EmailLoginAttemptsPrefix+hash,
	).Int()
	if err != nil {
		response.RecordError(ctx, err)
		return
	}
	if removed > 0 {
		logger.Logger.WithContext(ctx).Infof(
			"Email login issuance discarded; user: %d, student: %d", record.UserID, record.StudentID,
		)
	}
}
