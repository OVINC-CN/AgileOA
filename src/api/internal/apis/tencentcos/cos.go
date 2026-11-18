package tencentcos

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

// Client 是在 init 中初始化的共享 COS 客户端。
var Client *cos.Client

// STSClient 是在 init 中初始化的共享腾讯云 STS 客户端。
var STSClient *sts.Client
var transport *http.Transport

// ErrObjectNotFound 表示客户端尚未完成对应对象的上传。
var ErrObjectNotFound = errors.New("COS object not found")

func init() {
	cfg := config.Config.COS
	if !cfg.Enabled {
		return
	}

	// COS 和 STS 共用全局追踪及连接池，凭据仅来自 YAML。
	transport = http.DefaultTransport.(*http.Transport).Clone()
	traced := otelhttp.NewTransport(transport, otelhttp.WithTracerProvider(tracing.Provider))
	bucketURL, err := url.Parse(fmt.Sprintf(
		"https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region,
	))
	if err != nil {
		logger.Logger.Fatal("COS endpoint initialization failed")
	}
	Client = cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &cos.AuthorizationTransport{
			SecretID: cfg.SecretID, SecretKey: cfg.SecretKey, Transport: traced,
		},
	})
	Client.Conf.RetryOpt.Count = cfg.MaxRetries + 1
	Client.Conf.RetryOpt.AutoSwitchHost = false

	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.ReqTimeout = 0
	clientProfile.NetworkFailureMaxRetries = 0
	clientProfile.RateLimitExceededMaxRetries = 0
	clientProfile.UnsafeRetryOnConnectionFailure = false
	STSClient, err = sts.NewClient(
		common.NewCredential(cfg.SecretID, cfg.SecretKey), cfg.Region, clientProfile,
	)
	if err != nil {
		logger.Logger.Fatal("COS STS client initialization failed")
	}
	STSClient.WithHttpTransport(traced)
}

// Enabled 表示已初始化私有 COS 和 STS 客户端。
func Enabled() bool { return Client != nil && STSClient != nil }

// Metadata 是 COS HEAD 返回的实际对象元数据。
type Metadata struct {
	Size        int64
	ContentType string
}

// Head 只读取对象元数据，不下载文件内容。
func Head(ctx context.Context, object string) (Metadata, error) {
	if Client == nil {
		return Metadata{}, errors.New("COS client unavailable")
	}
	result, err := Client.Object.Head(ctx, object, nil)
	if err != nil {
		if result != nil && result.StatusCode == http.StatusNotFound {
			return Metadata{}, ErrObjectNotFound
		}
		return Metadata{}, errors.New("COS object metadata request failed")
	}
	return Metadata{
		Size: result.ContentLength, ContentType: result.Header.Get("Content-Type"),
	}, nil
}

// Copy 在私有桶中复制模板附件，来源由业务保存的对象键确定。
func Copy(ctx context.Context, source, destination string) error {
	if Client == nil {
		return errors.New("COS client unavailable")
	}
	// SDK 负责对象路径编码，保留中文、空格及特殊字符。
	sourceURL := Client.BaseURL.BucketURL.Host + "/" + source
	result, _, err := Client.Object.Copy(ctx, destination, sourceURL, nil)
	if err != nil || result == nil || strings.TrimSpace(result.ETag) == "" {
		return errors.New("COS object copy failed")
	}
	return nil
}

// Delete 幂等删除对象，供业务显式删除及副本补偿使用。
func Delete(ctx context.Context, object string) error {
	if Client == nil {
		return errors.New("COS client unavailable")
	}
	_, err := Client.Object.Delete(ctx, object)
	if err != nil {
		return errors.New("COS object deletion failed")
	}
	return nil
}

// Close 在共享资源退出时关闭空闲连接。
func Close() {
	if transport != nil {
		transport.CloseIdleConnections()
	}
}
