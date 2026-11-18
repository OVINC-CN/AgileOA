package tencentses

import (
	"context"
	"errors"
	"fmt"
	"net"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
)

func deliveryFailure(cause error) error {
	// 云端错误说明可能回显凭据或模板变量，只保留错误代码、请求编号和错误类型。
	reason, code, requestID := "request_failed", "", ""
	var sdkError *tcerrors.TencentCloudSDKError
	if errors.As(cause, &sdkError) {
		code, requestID = sdkError.GetCode(), sdkError.GetRequestId()
	}
	var network net.Error
	switch {
	case errors.Is(cause, context.Canceled):
		reason = "request_canceled"
	case errors.Is(cause, context.DeadlineExceeded):
		reason = "timeout"
	case errors.As(cause, &network) && network.Timeout():
		reason = "timeout"
	}
	return fmt.Errorf(
		"tencent SES delivery failed; reason: %s; code: %s; request id: %s; error type: %T",
		reason, code, requestID, cause,
	)
}
