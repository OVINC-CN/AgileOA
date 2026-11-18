package attachmentUtils

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func TestDownloadResponse(t *testing.T) {
	link := attachmentModels.URLData{
		Filename: "申报材料.pdf",
		URL:      "https://cdn.example.com/fixture.pdf?eo-sign=fixture",
	}
	recorder := httptest.NewRecorder()
	c := echo.New().NewContext(
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download", nil), recorder,
	)
	if err := RespondDownload(c, link); err != nil {
		t.Fatal(err)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" ||
		recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("下载地址响应缺少缓存或来源保护")
	}
	var result response.Envelope[map[string]string]
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || recorder.Header().Get("Location") != "" ||
		result.ErrorMsg != "" ||
		!maps.Equal(result.Data, map[string]string{"filename": link.Filename, "url": link.URL}) {
		t.Fatal("下载没有返回完整的 JSON 文件信息")
	}
}
