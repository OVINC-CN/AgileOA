package attachmentUtils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	cos "github.com/tencentyun/cos-go-sdk-v5"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apis/tencentcos"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestUploadRequestEnforcesExactMetadataAndAllowedTypes(t *testing.T) {
	original := config.Config.COS
	t.Cleanup(func() { config.Config.COS = original })
	config.Config.COS.MaxUploadBytes = 5 * 1024 * 1024
	config.Config.COS.AllowedContentTypes = []string{"image/*", "application/pdf"}
	request := attachmentModels.UploadRequest{
		Filename: "虚构材料.pdf", Size: 1024, ContentType: "application/pdf",
	}
	if err := ValidateUploadRequest(request); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*attachmentModels.UploadRequest){
		func(r *attachmentModels.UploadRequest) { r.Size = 0 },
		func(r *attachmentModels.UploadRequest) { r.Size = config.Config.COS.MaxUploadBytes + 1 },
		func(r *attachmentModels.UploadRequest) { r.ContentType = "" },
		func(r *attachmentModels.UploadRequest) { r.ContentType = "application/zip" },
		func(r *attachmentModels.UploadRequest) { r.ContentType = "application/pdf; charset=utf-8" },
		func(r *attachmentModels.UploadRequest) { r.Filename = "../材料.pdf" },
		func(r *attachmentModels.UploadRequest) { r.Filename = "*.pdf" },
	} {
		invalid := request
		change(&invalid)
		if ValidateUploadRequest(invalid) == nil {
			t.Fatal("非法直传元数据被接受")
		}
	}
}

func TestUploadCompletionOnlyUsesHeadAndRequiresMatchingMetadata(t *testing.T) {
	original := tencentcos.Client
	t.Cleanup(func() { tencentcos.Client = original })
	status, size, contentType := http.StatusOK, int64(7), "application/pdf"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/workspaces/fixture/012345abcd/材料.pdf" {
			t.Error("完成上传读取了文件正文或使用了客户端对象路径")
		}
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	tencentcos.Client = cos.NewClient(&cos.BaseURL{BucketURL: endpoint}, server.Client())
	tencentcos.Client.DisableURLCheck()
	tencentcos.Client.Conf.RetryOpt.Count = 1
	record := attachmentModels.Record{
		ObjectKey: "workspaces/fixture/012345abcd/材料.pdf", Size: size, ContentType: contentType,
	}
	if err := VerifyUploadedObject(t.Context(), record); err != nil {
		t.Fatal(err)
	}
	size++
	if VerifyUploadedObject(t.Context(), record) == nil {
		t.Fatal("对象大小不一致仍可完成上传")
	}
	size = record.Size
	contentType = "image/png"
	if VerifyUploadedObject(t.Context(), record) == nil {
		t.Fatal("对象声明类型不一致仍可完成上传")
	}
	status = http.StatusNotFound
	var failure *response.BusinessError
	if err := VerifyUploadedObject(t.Context(), record); !errors.As(err, &failure) ||
		failure.Status != http.StatusConflict {
		t.Fatal("尚未上传的对象没有返回可重试的冲突状态")
	}
}
