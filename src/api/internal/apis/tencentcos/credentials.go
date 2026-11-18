package tencentcos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

// UploadParameters 仅在申请响应中传递直传凭据，不进入列表、缓存、日志或审计。
type UploadParameters struct {
	// Bucket 是包含 APPID 的私有桶名称。
	Bucket string `json:"bucket" example:"examplebucket-1250000000"`
	// Region 是 COS 地域。
	Region string `json:"region" example:"ap-guangzhou"`
	// Key 是服务端生成的本次唯一对象键。
	Key string `json:"key" example:"workspaces/example/projects/1/012345abcd/材料.pdf"`
	// ContentType 是本次 STS 强制匹配的声明媒体类型。
	ContentType string `json:"content_type" example:"application/pdf"`
	// TmpSecretID 是临时访问密钥编号。
	TmpSecretID string `json:"tmp_secret_id" example:"temporary-secret-id"`
	// TmpSecretKey 是临时访问密钥。
	TmpSecretKey string `json:"tmp_secret_key" example:"temporary-secret-key"`
	// SecurityToken 是临时安全令牌。
	SecurityToken string `json:"security_token" example:"temporary-security-token"`
	// StartTime 是签发起始 Unix 秒时间。
	StartTime int64 `json:"start_time" example:"1794873600"`
	// ExpiredTime 是 STS 失效 Unix 秒时间，不控制 pending 的存续时间。
	ExpiredTime int64 `json:"expired_time" example:"1794875400"`
}

type uploadPolicy struct {
	Version   string            `json:"version"`
	Statement []uploadStatement `json:"statement"`
}

type uploadStatement struct {
	Effect    string                    `json:"effect"`
	Action    []string                  `json:"action"`
	Resource  []string                  `json:"resource"`
	Condition map[string]map[string]any `json:"condition"`
}

func policyForUpload(object, contentType string, size int64) uploadPolicy {
	cfg := config.Config.COS
	appID := cfg.Bucket[strings.LastIndexByte(cfg.Bucket, '-')+1:]
	return uploadPolicy{
		Version: "2.0",
		Statement: []uploadStatement{{
			Effect: "allow",
			Action: []string{"name/cos:PutObject"},
			Resource: []string{fmt.Sprintf(
				"qcs::cos:%s:uid/%s:%s/%s", cfg.Region, appID, cfg.Bucket, object,
			)},
			Condition: map[string]map[string]any{
				"numeric_equal": {"cos:content-length": size},
				"string_equal":  {"cos:content-type": contentType},
				"bool_equal":    {"cos:secure-transport": true},
			},
		}},
	}
}

// Credentials 申请只允许简单上传单个已登记对象的短时 STS。
func Credentials(
	ctx context.Context, object, contentType string, size int64,
) (UploadParameters, error) {
	var result UploadParameters
	if STSClient == nil {
		return result, errors.New("COS STS client unavailable")
	}
	cfg := config.Config.COS
	if object == "" || strings.Contains(object, "*") || size < 1 || size > cfg.MaxUploadBytes {
		return result, errors.New("invalid COS upload parameters")
	}
	policy, err := json.Marshal(policyForUpload(object, contentType, size))
	if err != nil {
		return result, errors.New("COS upload policy encoding failed")
	}

	// 临时凭据只在审计已提交后的响应中返回，不记录 SDK 的原始响应或错误。
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	name, encoded := "cosUpload", url.QueryEscape(string(policy))
	lifetime := cfg.UploadCredentialTTL / time.Second
	if lifetime < 1 || lifetime > 7200 {
		return result, errors.New("invalid COS upload credential duration")
	}
	duration := uint64(lifetime)
	request := sts.NewGetFederationTokenRequest()
	request.Name, request.Policy, request.DurationSeconds = &name, &encoded, &duration
	startTime := time.Now().Unix()
	response, err := STSClient.GetFederationTokenWithContext(ctx, request)
	if err != nil || response == nil || response.Response == nil {
		return result, errors.New("COS upload credential request failed")
	}
	data := response.Response
	credentials := data.Credentials
	if credentials == nil || credentials.TmpSecretId == nil || credentials.TmpSecretKey == nil ||
		credentials.Token == nil || data.ExpiredTime == nil ||
		*credentials.TmpSecretId == "" || *credentials.TmpSecretKey == "" ||
		*credentials.Token == "" || *data.ExpiredTime > 1<<63-1 {
		return result, errors.New("COS upload credential response invalid")
	}
	expiredTime := int64(*data.ExpiredTime)
	if expiredTime <= startTime {
		return result, errors.New("COS upload credential response expired")
	}
	return UploadParameters{
		Bucket:        cfg.Bucket,
		Region:        cfg.Region,
		Key:           object,
		ContentType:   contentType,
		TmpSecretID:   *credentials.TmpSecretId,
		TmpSecretKey:  *credentials.TmpSecretKey,
		SecurityToken: *credentials.Token,
		StartTime:     startTime,
		ExpiredTime:   expiredTime,
	}, nil
}
