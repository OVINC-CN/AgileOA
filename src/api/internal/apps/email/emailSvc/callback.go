package emailSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/email/emailUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// ReceiveCallback 独立接收回调，异常事件与邮箱拉黑在同一事务内提交。
func ReceiveCallback(ctx context.Context, request emailModels.CallbackRequest) error {
	ctx, span := tracing.Tracer.Start(ctx, "emailSvc.ReceiveCallback")
	defer span.End()

	// 只保存固定字段，不关联既有账号、发信记录或 IP。
	callback, err := prepareCallback(request)
	if err != nil {
		return err
	}
	if db.DB() == nil {
		return unavailable()
	}
	created, blocked := false, false
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		created, blocked, err = saveCallback(tx, &callback)
		return err
	})
	if err != nil {
		response.RecordError(ctx, err)
		return unavailable()
	}

	// 自动回调不冒用用户身份写审计，实际变更提交后记录英文操作日志。
	if created {
		logger.Logger.WithContext(ctx).Infof(
			"Email callback created; callback: %d; event: %s", callback.ID, callback.Event,
		)
	}
	if blocked {
		logger.Logger.WithContext(ctx).Infof(
			"Email address blocked; callback: %d; email: %q", callback.ID, callback.Email,
		)
	}
	return nil
}

func saveCallback(tx *gorm.DB, callback *emailModels.EmailCallback) (bool, bool, error) {
	// 内容摘要约束重复投递，重试不重复写入，也不修改首次拉黑原因。
	if err := tx.Create(callback).Error; err != nil {
		if duplicate(err) {
			return false, false, nil
		}
		return false, false, err
	}
	if !slices.Contains(config.Config.Email.AbnormalEvents, callback.Event) {
		return true, false, nil
	}
	blacklist := emailModels.EmailBlacklist{
		Email: callback.Email, Event: callback.Event, Reason: callback.Reason,
	}
	if err := tx.Create(&blacklist).Error; err != nil {
		if duplicate(err) {
			return true, false, nil
		}
		return false, false, err
	}
	return true, true, nil
}

func prepareCallback(request emailModels.CallbackRequest) (emailModels.EmailCallback, error) {
	// 固定事件目录校验拼写及编号，异常事件的业务判断只读取 YAML。
	email, valid := emailUtils.NormalizeAddress(request.Email)
	event, known := emailConstants.Events[request.Event]
	if !valid || !known || request.Timestamp <= 0 || request.BulkID == "" ||
		(request.EventID != 0 && request.EventID != event.ID) {
		return emailModels.EmailCallback{}, badRequest()
	}
	request.Email = email
	request.EventID = event.ID
	if request.Link != "" {
		link, err := url.Parse(request.Link)
		if err != nil || link.Host == "" || (link.Scheme != "http" && link.Scheme != "https") {
			return emailModels.EmailCallback{}, badRequest()
		}
		// 登录码和下载签名可能出现在点击地址中，数据库只保留站点和路径。
		request.Link = (&url.URL{Scheme: link.Scheme, Host: link.Host, Path: link.Path}).String()
	}

	// 固定结构序列化保证字段顺序稳定，摘要不包含来源 IP 或原始请求体。
	payload, err := json.Marshal(request)
	if err != nil {
		return emailModels.EmailCallback{}, err
	}
	digest := sha256.Sum256(payload)
	return emailModels.EmailCallback{
		EventKey: hex.EncodeToString(digest[:]), CallbackRequest: request,
	}, nil
}

func duplicate(err error) bool {
	var failure *mysql.MySQLError
	return errors.As(err, &failure) && failure.Number == 1062
}

func badRequest() error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: "邮件回调参数无效"}
}

func unavailable() error {
	return &response.BusinessError{
		Status: http.StatusServiceUnavailable, Message: "邮件回调暂时不可用",
	}
}
