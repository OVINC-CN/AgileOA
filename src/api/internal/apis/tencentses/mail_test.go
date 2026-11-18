package tencentses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ses "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ses/v20201002"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

type mailTransport func(*http.Request) (*http.Response, error)

// RoundTrip 在内存中检查真实 SDK 生成的协议请求，不连接外部邮件服务。
func (transport mailTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestSendTemplateProtocolAndResponse(t *testing.T) {
	// 临时替换共享客户端，覆盖协议、云端拒绝和缺少消息标识的响应。
	originalClient := client
	original := config.Config.User.EmailLogin
	t.Cleanup(func() {
		client = originalClient
		config.Config.User.EmailLogin = original
	})
	cfg := &config.Config.User.EmailLogin.SES
	cfg.FromAddress = "login@example.com"
	cfg.RequestTimeout = 3 * time.Second
	var err error
	client, err = ses.NewClient(
		common.NewCredential("fictional-secret-id", "fictional-secret-key"),
		"ap-guangzhou",
		profile.NewClientProfile(),
	)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]string{
		"LOGIN_CODE": "012345", "TTL_MINUTES": "10",
	}
	for _, scenario := range []struct {
		name     string
		fromName string
		body     string
		failure  string
	}{
		{
			name: "accepted", fromName: "敏捷办公 Agile OA",
			body: `{"Response":{"MessageId":"fictional-message-id","RequestId":"request-id"}}`,
		},
		{
			name: "without-alias",
			body: `{"Response":{"MessageId":"fictional-message-id","RequestId":"request-id"}}`,
		},
		{
			name: "rejected",
			body: `{"Response":{"Error":{"Code":"FailedOperation.InvalidTemplateID",` +
				`"Message":"fictional-private-content"},"RequestId":"request-id"}}`,
			failure: "FailedOperation.InvalidTemplateID",
		},
		{name: "missing-response", body: `{}`, failure: "missing message id"},
		{name: "missing-message", body: `{"Response":{}}`, failure: "missing message id"},
		{
			name: "empty-message", body: `{"Response":{"MessageId":""}}`,
			failure: "missing message id",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cfg.FromName = scenario.fromName
			calls := 0
			client.WithHttpTransport(
				mailTransport(func(request *http.Request) (*http.Response, error) {
					defer func() {
						if err := request.Body.Close(); err != nil {
							t.Error(err)
						}
					}()
					calls++
					deadline, bounded := request.Context().Deadline()
					if request.Method != http.MethodPost || request.URL.Scheme != "https" ||
						request.URL.Host != "ses.tencentcloudapi.com" || request.URL.RawQuery != "" ||
						request.Header.Get("X-TC-Action") != "SendEmail" ||
						request.Header.Get("X-TC-Version") != "2020-10-02" ||
						request.Header.Get("X-TC-Region") != "ap-guangzhou" ||
						!strings.HasPrefix(
							request.Header.Get("Authorization"),
							"TC3-HMAC-SHA256 ",
						) ||
						!bounded || time.Until(deadline) > cfg.RequestTimeout {
						t.Fatal("邮件请求未使用签名 HTTPS API、指定地域或有界上下文")
					}

					payload := ses.NewSendEmailRequest()
					if err := json.NewDecoder(request.Body).Decode(payload); err != nil {
						t.Fatal(err)
					}
					from := cfg.FromAddress
					if cfg.FromName != "" {
						from = cfg.FromName + " <" + from + ">"
					}
					if payload.FromEmailAddress == nil || *payload.FromEmailAddress != from ||
						payload.Subject == nil || *payload.Subject != "登录敏捷办公" ||
						len(payload.Destination) != 1 || payload.Destination[0] == nil ||
						*payload.Destination[0] != "recipient@example.com" ||
						payload.Simple != nil || payload.Template == nil ||
						payload.Template.TemplateID == nil || *payload.Template.TemplateID != 100091 ||
						payload.Template.TemplateData == nil ||
						payload.TriggerType == nil || *payload.TriggerType != 1 ||
						payload.Unsubscribe == nil || *payload.Unsubscribe != "0" {
						t.Fatal("邮件请求未使用指定发件人、单个收件人或触发类模板")
					}
					var actual map[string]string
					if err := json.Unmarshal(
						[]byte(*payload.Template.TemplateData),
						&actual,
					); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(actual, data) {
						t.Fatal("模板变量未以 JSON 字符串完整传递")
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": {"application/json"}},
						Body:       io.NopCloser(strings.NewReader(scenario.body)),
						Request:    request,
					}, nil
				}),
			)
			err := SendTemplate(t.Context(), "recipient@example.com", 100091, "登录敏捷办公", data)
			if calls != 1 || (scenario.failure == "" && err != nil) ||
				(scenario.failure != "" && (err == nil ||
					!strings.Contains(err.Error(), scenario.failure))) ||
				(err != nil && strings.Contains(err.Error(), "fictional-private-content")) {
				t.Fatal("邮件 API 未正确处理成功、拒绝或不完整响应")
			}
		})
	}

	// 上游取消后立即失败，不发送邮件或恢复请求上下文。
	client.WithHttpTransport(mailTransport(func(request *http.Request) (*http.Response, error) {
		defer func() {
			if err := request.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		if request.Context().Err() == nil {
			t.Fatal("邮件 API 丢失上游取消状态")
		}
		return nil, request.Context().Err()
	}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := SendTemplate(ctx, "recipient@example.com", 100091, "登录敏捷办公", data); err == nil ||
		!strings.Contains(err.Error(), "request_canceled") {
		t.Fatal("邮件 API 未沿用上游取消状态")
	}
}
