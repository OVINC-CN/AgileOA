package storage

import (
	"context"
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- EdgeOne 鉴权方式 A 的协议规定使用 MD5。
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

const cdnSignatureCachePrefix = "storage:cdn:signature:"

// SignedCDNURL 为已获授权的私有对象生成短时链接，优先复用 Redis 中的签名。
func SignedCDNURL(ctx context.Context, object string) string {
	cfg := config.Config.CDN
	// 对原始文件名进行路径编码，签名覆盖实际发送的转义路径，避免中文和特殊字符改变请求目标。
	rawPath := cfg.PathPrefix + "/" + object
	path := (&url.URL{Path: rawPath}).EscapedPath()
	value := cachedCDNSignature(ctx, path)
	return cfg.Domain + path + "?" + url.Values{cfg.Parameter: {value}}.Encode()
}

func cachedCDNSignature(ctx context.Context, path string) string {
	client := db.RDB()
	if client == nil {
		return newCDNSignature(path)
	}

	// 复用 Redis 读取超时限制整个缓存操作，缓存不可用时继续签发。
	ctx, cancel := context.WithTimeout(ctx, config.Config.Redis.ReadTimeout)
	defer cancel()
	key := cdnSignatureCacheKey(path)
	cached, err := client.Get(ctx, key).Result()
	if err == nil && cached != "" {
		return cached
	}
	signature := newCDNSignature(path)
	if !errors.Is(err, redis.Nil) {
		return signature
	}

	// 并发未命中时只保留第一份签名及其过期时间，其他请求优先复用已写入的签名。
	ttl := config.Config.CDN.SigningCacheTTL
	written, err := client.SetNX(ctx, key, signature, ttl).Result()
	if err != nil || written {
		return signature
	}
	if cached, err := client.Get(ctx, key).Result(); err == nil && cached != "" {
		return cached
	}
	return signature
}

func cdnSignatureCacheKey(path string) string {
	cfg := config.Config.CDN
	// 使用密钥派生缓存键，切换 CDN 配置时避开旧缓存，键中不保存原始密钥或文件路径。
	scope := strings.Join(
		[]string{cfg.Domain, path, cfg.Parameter, cfg.SigningCacheTTL.String()},
		"\x00",
	)
	hash := hmac.New(sha256.New, []byte(cfg.SigningKey))
	_, _ = hash.Write([]byte(scope))
	return cdnSignatureCachePrefix + hex.EncodeToString(hash.Sum(nil))
}

func newCDNSignature(path string) string {
	// timestamp 是签发时间，边缘规则以签发时间加有效时长判断过期。
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	// 方式 A 用连字符分隔字段，随机值使用无连字符的十六进制字符串。
	var random [24]byte
	_, _ = rand.Read(random[:])
	nonce := hex.EncodeToString(random[:])
	input := fmt.Sprintf("%s-%s-%s-0-%s", path, stamp, nonce, config.Config.CDN.SigningKey)
	hash := md5.Sum([]byte(input)) // #nosec G401 -- 必须遵守 EdgeOne 方式 A 的签名协议。
	return stamp + "-" + nonce + "-0-" + hex.EncodeToString(hash[:])
}
