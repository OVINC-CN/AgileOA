package tencentcos

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func TestSTSOnlyGrantsExactObjectPutWithRequiredSizeAndType(t *testing.T) {
	original, originalConfig := STSClient, config.Config.COS
	t.Cleanup(func() { STSClient, config.Config.COS = original, originalConfig })
	config.Config.COS.Bucket = "fixture-upload-1250000000"
	config.Config.COS.Region = "ap-guangzhou"
	config.Config.COS.UploadCredentialTTL = 30 * time.Minute
	const key = "workspaces/fixture/projects/1/012345abcd/材料.pdf"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-TC-Action") != "GetFederationToken" {
			t.Error("STS 请求未使用腾讯云 SDK 的签名 POST")
		}
		var request struct {
			Policy          string
			DurationSeconds uint64
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("无法解析 STS 策略")
		}
		decodedPolicy, decodeErr := url.QueryUnescape(request.Policy)
		var policy uploadPolicy
		if decodeErr != nil || json.Unmarshal([]byte(decodedPolicy), &policy) != nil ||
			len(policy.Statement) != 1 {
			t.Error("STS 策略不是单个对象的最小授权")
			return
		}
		statement := policy.Statement[0]
		if request.DurationSeconds != 1800 || len(statement.Action) != 1 ||
			statement.Action[0] != "name/cos:PutObject" || len(statement.Resource) != 1 ||
			statement.Resource[0] != "qcs::cos:ap-guangzhou:uid/1250000000:fixture-upload-1250000000/"+key ||
			statement.Condition["numeric_equal"]["cos:content-length"] != float64(1024) ||
			statement.Condition["string_equal"]["cos:content-type"] != "application/pdf" ||
			strings.Contains(
				decodedPolicy,
				"_if_exist",
			) || strings.Contains(decodedPolicy, "forbid-overwrite") {
			t.Error("STS 未限制准确字节数、声明类型和单个对象，或增加了未约定的防覆盖限制")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"Response": map[string]any{
			"Credentials": map[string]string{
				"TmpSecretId":  "fixture-id",
				"TmpSecretKey": "fixture-key",
				"Token":        "fixture-token",
			},
			"ExpiredTime": time.Now().Add(time.Hour).Unix(),
		}})
	}))
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint, clientProfile.HttpProfile.Scheme = endpoint.Host, "http"
	STSClient, err = sts.NewClient(common.NewCredential("fixture-id", "fixture-key"),
		"ap-guangzhou", clientProfile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Credentials(t.Context(), key, "application/pdf", 1024)
	if err != nil || result.Key != key || result.TmpSecretKey != "fixture-key" ||
		result.ExpiredTime <= result.StartTime {
		t.Fatal("COS 直传凭据没有保留单对象范围或有效期")
	}
	if _, err := Credentials(
		t.Context(),
		"workspaces/fixture/*.pdf",
		"application/pdf",
		1024,
	); err == nil {
		t.Fatal("策略资源中的对象通配符被接受")
	}
}
