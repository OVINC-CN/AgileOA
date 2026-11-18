package userSvc

import (
	"context"
	"net/http"
	"strings"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentcaptcha"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// IssueLoginCaptcha 为单次登录签发短时鉴权密文。
func IssueLoginCaptcha(ctx context.Context) *userModels.LoginCaptcha {
	if !config.Config.User.Captcha.Enabled {
		return nil
	}

	_, span := tracing.Tracer.Start(ctx, "userSvc.IssueLoginCaptcha")
	defer span.End()
	appID, encrypted := tencentcaptcha.EncryptAppID()
	return &userModels.LoginCaptcha{
		AppID:            appID,
		AidEncrypted:     encrypted,
		AidEncryptedType: tencentcaptcha.EncryptionMode,
	}
}

func verifyLoginCaptcha(ctx context.Context, login userModels.LoginRequest, clientIP string) error {
	if !config.Config.User.Captcha.Enabled {
		return nil
	}
	ticket, randstr := login.CaptchaTicket, login.CaptchaRandstr
	if strings.TrimSpace(ticket) == "" || strings.TrimSpace(randstr) == "" ||
		strings.HasPrefix(ticket, "trerror") {
		return invalidCaptcha()
	}

	ctx, span := tracing.Tracer.Start(ctx, "userSvc.VerifyLoginCaptcha")
	defer span.End()
	passed, err := tencentcaptcha.Verify(ctx, ticket, randstr, clientIP)
	if err != nil {
		response.RecordError(ctx, err)
		return &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "验证码服务暂时不可用，请重试",
		}
	}
	if !passed {
		return invalidCaptcha()
	}
	return nil
}

func invalidCaptcha() error {
	return &response.BusinessError{
		Status:  http.StatusBadRequest,
		Message: "验证码无效，请重新验证",
	}
}
