package tencentcaptcha

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	captcha "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/captcha/v20190722"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

const (
	// EncryptionMode 是 Web 验证码应用鉴权使用的加密模式。
	EncryptionMode   = "gcm"
	captchaWebType   = 9
	captchaPassed    = 1
	captchaMalicious = 100
)

var (
	captchaClient *captcha.Client
	captchaCipher cipher.AEAD
)

func init() {
	cfg := config.Config.User.Captcha
	if !cfg.Enabled {
		return
	}

	// 腾讯云要求较短的应用密钥循环填充至 AES-256 的密钥长度。
	var key [32]byte
	for i := range key {
		key[i] = cfg.AppSecretKey[i%len(cfg.AppSecretKey)]
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		logger.Logger.Fatal("Tencent captcha cipher initialization failed")
	}
	captchaCipher, err = cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		logger.Logger.Fatal("Tencent captcha GCM initialization failed")
	}

	// 验证请求使用 SDK 默认的 HTTPS POST，票据和密钥仅进入请求体。
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	captchaClient, err = captcha.NewClient(credential, "", profile.NewClientProfile())
	if err != nil {
		logger.Logger.Fatal("Tencent captcha client initialization failed")
	}
	captchaClient.WithHttpTransport(
		otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tracing.Provider)),
	)
}

// EncryptAppID 为启用的 Web 验证码生成应用鉴权密文。
func EncryptAppID() (string, string) {
	cfg := config.Config.User.Captcha

	// GCM 自动生成随机 IV，输出按腾讯云要求依次包含 IV、密文和认证标签。
	// 控制台启用一次一密后，腾讯云拒绝同一 IV 被再次用于发起验证。
	appID := strconv.FormatUint(cfg.AppID, 10)
	plaintext := fmt.Sprintf("%s&%d&%d", appID, time.Now().Unix(), cfg.ChallengeTTL/time.Second)
	// #nosec G407 -- IV 由 GCM 自动生成。
	encrypted := captchaCipher.Seal(nil, nil, []byte(plaintext), nil)
	return appID, base64.StdEncoding.EncodeToString(encrypted)
}

// Verify 核查启用的 Web 验证码的票据和风控结果。
func Verify(ctx context.Context, ticket, randstr, clientIP string) (bool, error) {
	cfg := config.Config.User.Captcha

	// 腾讯云核查票据有效期和重放，网络失败后须由用户重新获取验证码。
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()

	request := captcha.NewDescribeCaptchaResultRequest()
	request.CaptchaType = common.Uint64Ptr(captchaWebType)
	request.Ticket = &ticket
	request.Randstr = &randstr
	request.UserIp = &clientIP
	request.CaptchaAppId = &cfg.AppID
	request.AppSecretKey = &cfg.AppSecretKey

	result, err := captchaClient.DescribeCaptchaResultWithContext(ctx, request)
	if err != nil || result == nil || result.Response == nil || result.Response.CaptchaCode == nil {
		// SDK 错误可能包含原始响应，只记录固定的诊断信息。
		return false, errors.New("tencent captcha verification request failed")
	}
	code, evilLevel := *result.Response.CaptchaCode, result.Response.EvilLevel
	return code == captchaPassed && (evilLevel == nil || *evilLevel != captchaMalicious), nil
}
