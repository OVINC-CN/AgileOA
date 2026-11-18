package storage

import (
	"crypto/md5" // #nosec G501 -- 测试 EdgeOne 指定的 MD5 协议。
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

func TestSignedCDNURLUsesIssueTimeAndCustomParameter(t *testing.T) {
	if db.RDB() != nil {
		t.Skip("此用例验证未启用 Redis 时的按需签发")
	}
	original := config.Config.CDN
	t.Cleanup(func() { config.Config.CDN = original })
	config.Config.CDN.Domain = "https://cdn.example.com"
	config.Config.CDN.PathPrefix = "/files"
	config.Config.CDN.Parameter = "eo-sign"
	config.Config.CDN.SigningKey = "fictional-signing-key"
	const object = "workspaces/w7m2k9p4s6n8c3x5/projects/1/项目说明 #1?.pdf"
	before := time.Now().Unix()
	signedURL := SignedCDNURL(t.Context(), object)
	target, err := url.Parse(signedURL)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(target.Query().Get("eo-sign"), "-")
	if len(parts) != 4 || parts[2] != "0" {
		t.Fatal("签名格式或用户占位不符合方式 A")
	}
	issuedAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || issuedAt < before || issuedAt > time.Now().Unix() {
		t.Fatal("签名未使用签发时间")
	}
	expected := md5.Sum(
		[]byte(target.EscapedPath() + "-" + parts[0] + "-" + parts[1] + "-0-fictional-signing-key"),
	) // #nosec G401 -- EdgeOne 方式 A。
	if parts[3] != hex.EncodeToString(expected[:]) || target.Host != "cdn.example.com" ||
		target.Path != "/files/"+object {
		t.Fatal("路径前缀或 MD5 签名不正确")
	}
	anotherURL := SignedCDNURL(t.Context(), object)
	if anotherURL == signedURL {
		t.Fatal("按需签名复用了随机串")
	}
	config.Config.CDN.Parameter = "custom-auth"
	customURL := SignedCDNURL(t.Context(), object)
	custom, err := url.Parse(customURL)
	if err != nil || custom.Query().Get("custom-auth") == "" ||
		custom.Query().Get("eo-sign") != "" {
		t.Fatal("自定义签名参数未生效")
	}
}

func TestSignedCDNURLReusesRedisSignatureUntilExpiration(t *testing.T) {
	client := db.RDB()
	if client == nil {
		t.Skip("签名缓存测试需要配置独立测试 Redis")
	}
	original := config.Config.CDN
	t.Cleanup(func() { config.Config.CDN = original })
	config.Config.CDN.Domain = "https://cdn.example.com"
	config.Config.CDN.PathPrefix = "/files"
	config.Config.CDN.Parameter = "eo-sign"
	config.Config.CDN.SigningKey = rand.Text()
	const cacheTTL = 2 * time.Minute
	config.Config.CDN.SigningCacheTTL = cacheTTL
	const object = "workspaces/w7m2k9p4s6n8c3x5/projects/1/fixture.pdf"
	ctx := t.Context()
	key := cdnSignatureCacheKey("/files/" + object)
	defer func() { _ = client.Del(ctx, key).Err() }()

	first := SignedCDNURL(ctx, object)
	if ttl, err := client.PTTL(ctx, key).Result(); err != nil ||
		ttl <= time.Minute || ttl > cacheTTL {
		t.Fatal("签名未按配置写入带有效期的缓存")
	}

	// 缩短剩余时间以验证命中缓存不会重置有效期。
	if ok, err := client.Expire(ctx, key, time.Minute).Result(); err != nil || !ok {
		t.Fatal("无法设置缓存剩余时间")
	}
	if SignedCDNURL(ctx, object) != first {
		t.Fatal("缓存命中时没有复用签名")
	}
	if ttl, err := client.PTTL(ctx, key).Result(); err != nil || ttl <= 0 || ttl > time.Minute {
		t.Fatal("缓存命中后有效期发生变化")
	}
	if ok, err := client.Expire(ctx, key, 0).Result(); err != nil || !ok {
		t.Fatal("无法使签名缓存过期")
	}
	if SignedCDNURL(ctx, object) == first {
		t.Fatal("缓存过期后没有重新签发")
	}
}
