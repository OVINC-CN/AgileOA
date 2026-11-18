package emailUtils

import (
	"context"
	"net/http"
	"net/mail"
	"strings"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/email/emailModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

// NormalizeAddress 统一邮箱黑名单的比较方式，不改变账号登录的逐字匹配规则。
func NormalizeAddress(value string) (string, bool) {
	// 拒绝带显示名称或控制字符的地址，只规范首尾空白和大小写。
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 ||
		strings.ContainsAny(value, "\r\n\x00") {
		return "", false
	}
	return strings.ToLower(value), true
}

// CheckAllowed 在发信前检查共享黑名单，调用方不得持有数据库事务占用连接。
func CheckAllowed(ctx context.Context, recipient string) error {
	ctx, span := tracing.Tracer.Start(ctx, "emailUtils.CheckAllowed")
	defer span.End()

	// 黑名单读取失败时阻断发信，不能因数据库异常绕过禁发规则。
	email, valid := NormalizeAddress(recipient)
	if !valid {
		return &response.BusinessError{Status: http.StatusBadRequest, Message: "邮箱地址无效"}
	}
	if db.DB() == nil {
		return unavailable()
	}
	var count int64
	if err := db.DB().WithContext(ctx).Model(&emailModels.EmailBlacklist{}).
		Where("email = ?", email).Count(&count).Error; err != nil {
		response.RecordError(ctx, err)
		return unavailable()
	}
	if count != 0 {
		return &response.BusinessError{
			Status: http.StatusForbidden, Message: "该邮箱已被禁止接收邮件",
		}
	}

	return nil
}

func unavailable() error {
	return &response.BusinessError{
		Status: http.StatusServiceUnavailable, Message: "邮件发送暂时不可用",
	}
}
