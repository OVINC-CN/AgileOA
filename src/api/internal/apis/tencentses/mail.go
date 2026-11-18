package tencentses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ses "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ses/v20201002"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

var client *ses.Client

func init() {
	// 共享客户端只使用 YAML 凭据，保留 SDK 默认的 HTTPS POST 和签名。
	if !config.Config.User.EmailLogin.Enabled {
		return
	}
	cfg := config.Config.User.EmailLogin.SES
	clientProfile := profile.NewClientProfile()
	// 统一由请求上下文执行 YAML 中的精确超时，禁止失败重试造成重复邮件。
	clientProfile.HttpProfile.ReqTimeout = 0
	clientProfile.NetworkFailureMaxRetries = 0
	clientProfile.RateLimitExceededMaxRetries = 0
	clientProfile.UnsafeRetryOnConnectionFailure = false
	var err error
	client, err = ses.NewClient(
		common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, clientProfile,
	)
	if err != nil {
		logger.Logger.Fatalf("Tencent SES initialization failed; error type: %T", err)
	}
	client.WithHttpTransport(
		otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(tracing.Provider)),
	)
}

// SendTemplate 使用已审核的腾讯云模板发送单人触发类邮件，沿用请求上下文和全局追踪。
func SendTemplate(
	ctx context.Context,
	recipient string,
	templateID uint64,
	subject string,
	templateData map[string]string,
) error {
	ctx, span := tracing.Tracer.Start(ctx, "tencentses.SendTemplate",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()
	if client == nil {
		err := errors.New("tencent SES client unavailable")
		response.RecordError(ctx, err)
		return err
	}
	cfg := config.Config.User.EmailLogin.SES
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()

	// 模板参数按协议序列化为 JSON 字符串，不上传本地 HTML 或记录邮件内容。
	data, err := json.Marshal(templateData)
	if err != nil {
		failure := errors.New("tencent SES template data encoding failed")
		response.RecordError(ctx, failure)
		return failure
	}
	from := cfg.FromAddress
	if cfg.FromName != "" {
		from = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromAddress)
	}
	request := ses.NewSendEmailRequest()
	request.FromEmailAddress = &from
	request.Destination = []*string{&recipient}
	request.Subject = &subject
	request.Template = &ses.Template{
		TemplateID:   &templateID,
		TemplateData: common.StringPtr(string(data)),
	}
	request.TriggerType = common.Uint64Ptr(1)
	request.Unsubscribe = common.StringPtr("0")
	result, err := client.SendEmailWithContext(ctx, request)
	if err != nil {
		failure := deliveryFailure(err)
		response.RecordError(ctx, failure)
		return failure
	}

	// 腾讯云返回消息标识才表示已接受发送，空响应不可发布可用的登录验证码。
	if result == nil || result.Response == nil || result.Response.MessageId == nil ||
		*result.Response.MessageId == "" {
		err := errors.New("tencent SES delivery failed: missing message id")
		response.RecordError(ctx, err)
		return err
	}
	return nil
}
