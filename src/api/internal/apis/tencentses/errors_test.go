package tencentses

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

func TestDeliveryFailurePreservesDiagnosticsWithoutServerText(t *testing.T) {
	// 云端回复可能包含邮件内容，日志和追踪只保留可诊断的错误代码和请求编号。
	const credential = "fictional-private-credential"
	for _, scenario := range []struct {
		cause     error
		reason    string
		code      string
		requestID string
	}{
		{
			cause: tcerrors.NewTencentCloudSDKError(
				"FailedOperation.InvalidTemplateID", credential, "fictional-request-id",
			),
			reason: "request_failed", code: "FailedOperation.InvalidTemplateID",
			requestID: "fictional-request-id",
		},
		{
			cause: tcerrors.NewTencentCloudSDKErrorWithCause(
				"ClientError.NetworkError", credential, "", context.DeadlineExceeded,
			),
			reason: "timeout", code: "ClientError.NetworkError",
		},
		{
			cause:  fmt.Errorf("%s: %w", credential, context.Canceled),
			reason: "request_canceled",
		},
		{
			cause: errors.New(credential), reason: "request_failed",
		},
	} {
		message := deliveryFailure(scenario.cause).Error()
		if strings.Contains(message, credential) ||
			!strings.Contains(message, "reason: "+scenario.reason) ||
			!strings.Contains(message, "code: "+scenario.code) ||
			!strings.Contains(message, "request id: "+scenario.requestID) {
			t.Fatal("邮件 API 错误丢失诊断信息或泄露原始云端回复")
		}
	}
}
