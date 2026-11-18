package tencentcos

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	cos "github.com/tencentyun/cos-go-sdk-v5"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

func copyTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	originalClient, originalBucket := Client, config.Config.COS.Bucket
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		Client, config.Config.COS.Bucket = originalClient, originalBucket
		server.Close()
	})
	config.Config.COS.Bucket = "private-copy-tests"
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	Client = cos.NewClient(&cos.BaseURL{BucketURL: baseURL}, server.Client())
	Client.DisableURLCheck()
	Client.Conf.RetryOpt.Count = 1
}

func TestCopyEncodesSourceAndCreatesPrivateIndependentObject(t *testing.T) {
	source := "workspaces/fictional/cpc-timeline-templates/7/nodes/node_first/材料 + #?%1.pdf"
	target := "workspaces/fictional/cpc-timeline-templates/8/nodes/node_first/new/材料 + #?%1.pdf"
	calls := 0
	copyTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		encodedSource := r.Header.Get("X-Cos-Copy-Source")
		decoded, err := url.QueryUnescape(encodedSource)
		body, readErr := io.ReadAll(r.Body)
		if err != nil || decoded != Client.BaseURL.BucketURL.Host+"/"+source ||
			strings.ContainsAny(encodedSource, " +#?") || strings.Contains(encodedSource, "材料") ||
			r.Method != http.MethodPut || r.URL.Path != "/"+target ||
			readErr != nil || len(body) != 0 || r.Header.Get("X-Cos-Acl") != "" {
			t.Error("复制请求没有正确编码对象名、使用独立路径或保持私有对象")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(
			w,
			`<CopyObjectResult>`+
				`<LastModified>2026-11-09T00:00:00.000Z</LastModified><ETag>"fictional"</ETag>`+
				`</CopyObjectResult>`,
		)
	})
	if err := Copy(t.Context(), source, target); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("对象复制没有只发送一次服务端复制请求")
	}
}

func TestCopyRejectsErrorEmbeddedInSuccessfulHTTPResponse(t *testing.T) {
	copyTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w,
			`<Error><Code>AccessDenied</Code><Message>fictional copy failure</Message></Error>`)
	})
	if err := Copy(t.Context(), "source.pdf", "target.pdf"); err == nil {
		t.Fatal("HTTP 200 中的复制错误被误判为成功")
	}
}
