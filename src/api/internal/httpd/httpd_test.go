package httpd_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/bytes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func checkResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) response.Envelope[json.RawMessage] {
	t.Helper()

	// 所有业务响应必须恰好包含三个约定字段。
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["trace"] == nil || fields["error_msg"] == nil ||
		fields["data"] == nil {
		t.Fatalf("响应字段不符合约定：%s", recorder.Body.String())
	}

	// 使用 OpenTelemetry 校验真实 Trace ID，并检查 data 固定为 null。
	var result response.Envelope[json.RawMessage]
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	traceID, err := trace.TraceIDFromHex(result.Trace)
	if err != nil || !traceID.IsValid() || string(result.Data) != "null" {
		t.Fatalf("追踪或数据字段无效：%s", recorder.Body.String())
	}
	return result
}

func TestProbesOnlyExposeHTTPStatus(t *testing.T) {
	t.Parallel()

	// 默认关闭上报时，探针仍必须保留有效的 OpenTelemetry 追踪。
	server := httpd.New()
	for _, route := range []string{"/health", "/ready"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, route, nil)
		request.Header.Set("Accept-Language", "en-US")
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		result := checkResponse(t, recorder)
		if recorder.Code != http.StatusOK || result.ErrorMsg != "" {
			t.Fatalf("探针响应错误：%s", recorder.Body.String())
		}
	}
}

func TestTracePropagationCannotDisableTracing(t *testing.T) {
	t.Parallel()

	// 上游未采样的标记不能关闭本服务追踪，响应沿用其 Trace ID。
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	request.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-00")
	recorder := httptest.NewRecorder()
	httpd.New().ServeHTTP(recorder, request)
	result := checkResponse(t, recorder)
	if result.Trace != traceID || !strings.HasSuffix(recorder.Header().Get("traceparent"), "-01") {
		t.Fatalf("追踪未正确传播或被关闭：%s", recorder.Body.String())
	}
}

func TestAuditClientIPUsesTrustedProxies(t *testing.T) {
	// 非并发设置可信代理网段，结束后恢复全局配置。
	_, network, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	original := config.Config.HTTP.TrustedProxyNets
	t.Cleanup(func() { config.Config.HTTP.TrustedProxyNets = original })
	config.Config.HTTP.TrustedProxyNets = []*net.IPNet{network}

	// 通过 HTTP 路由读取审计上下文，验证代理信任和直连回退。
	server := httpd.New()
	const path = "/test/audit-client-ip"
	server.GET(path, func(c echo.Context) error {
		return c.String(http.StatusOK, auditUtils.Request(c.Request().Context()).ClientIP)
	})
	for _, test := range []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{"可信代理链", "192.0.2.1:12345", "203.0.113.10, 198.51.100.10, 192.0.2.2", "198.51.100.10"},
		{"非可信来源", "198.51.100.10:12345", "203.0.113.10", "198.51.100.10"},
		{"无转发头", "192.0.2.1:12345", "", "192.0.2.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
			request.RemoteAddr = test.remoteAddr
			if test.forwardedFor != "" {
				request.Header.Set(echo.HeaderXForwardedFor, test.forwardedFor)
			}
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK || recorder.Body.String() != test.want {
				t.Fatalf("状态码为 %d，审计地址为 %q，期望 %q",
					recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestErrorsHideInternalDetails(t *testing.T) {
	t.Parallel()

	// 测试路由也集中在 httpd 范围内，模拟内部异常。
	server := httpd.New()
	server.GET("/test/failure", func(_ echo.Context) error {
		return errors.New("sensitive database details")
	})
	server.GET("/test/not-ready", func(_ echo.Context) error {
		return echo.NewHTTPError(http.StatusServiceUnavailable).
			SetInternal(errors.New("internal dependency address"))
	})
	for _, test := range []struct {
		route   string
		status  int
		message string
	}{
		{"/api/v1/system", http.StatusNotFound, response.MessageNotFound},
		{"/test/failure", http.StatusInternalServerError, response.MessageInternalError},
		{"/test/not-ready", http.StatusServiceUnavailable, response.MessageNotReady},
	} {
		// 失败仅用 HTTP 状态和通用错误说明表达，数据仍为空。
		recorder := httptest.NewRecorder()
		server.ServeHTTP(
			recorder,
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.route, nil),
		)
		result := checkResponse(t, recorder)
		if recorder.Code != test.status || result.ErrorMsg != test.message ||
			strings.Contains(recorder.Body.String(), "sensitive database details") ||
			strings.Contains(recorder.Body.String(), "internal dependency address") {
			t.Fatalf("错误响应泄露信息或状态不正确：%s", recorder.Body.String())
		}
	}
}

func TestHTTPSettingsComeFromYAML(t *testing.T) {
	t.Parallel()

	// 独立的监听超时和请求头限制必须来自配置。
	server := httpd.New()
	if server.Server.ReadHeaderTimeout != config.Config.HTTP.ReadHeaderTimeout ||
		server.Server.ReadTimeout != config.Config.HTTP.ReadTimeout ||
		server.Server.WriteTimeout != config.Config.HTTP.WriteTimeout ||
		server.Server.IdleTimeout != config.Config.HTTP.IdleTimeout ||
		server.Server.MaxHeaderBytes != config.Config.HTTP.MaxHeaderBytes {
		t.Fatal("HTTP 参数未完整使用 YAML 配置")
	}

	// 请求体超限同样保留有效追踪并返回空数据。
	limit, err := bytes.Parse(config.Config.HTTP.BodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test/body", nil)
	request.ContentLength = limit + 1
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	result := checkResponse(t, recorder)
	if recorder.Code != http.StatusRequestEntityTooLarge ||
		result.ErrorMsg != response.MessageRequestTooLarge {
		t.Fatalf("请求体限制未生效：%s", recorder.Body.String())
	}
}

func TestSwaggerDocumentIsServed(t *testing.T) {
	// 非并发检查配置开关，结束后恢复全局配置。
	original := config.Config.App.Debug
	t.Cleanup(func() { config.Config.App.Debug = original })
	config.Config.App.Debug = false
	hidden := httptest.NewRecorder()
	httpd.New().
		ServeHTTP(hidden, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/swagger/doc.json", nil))
	if hidden.Code != http.StatusNotFound {
		t.Fatal("非 DEBUG 模式暴露了 Swagger")
	}
	config.Config.App.Debug = true

	// 验证实际 HTTP 文档入口，避免仅生成文件但无法访问。
	recorder := httptest.NewRecorder()
	httpd.New().
		ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/swagger/doc.json", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("Swagger 文档无法访问：%s", recorder.Body.String())
	}
	var document struct {
		Paths map[string]map[string]struct {
			Responses map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}

	// 所有接口提供成功响应文档，附件下载统一描述 JSON 地址响应。
	expected := map[string]bool{}
	for _, route := range httpd.New().Routes() {
		if route.Path != "/v1/swagger/*" {
			expected[route.Path] = true
		}
	}
	if len(document.Paths) != len(expected) {
		t.Fatal("文档未覆盖全部接口")
	}
	for path, methods := range document.Paths {
		for methodName, method := range methods {
			statuses := []string{"200"}
			if methodName == "get" && strings.HasSuffix(path, "/download") {
				statuses = append(statuses, "400")
			}
			for _, status := range statuses {
				if method.Responses[status] == nil {
					t.Fatalf("接口文档缺少状态码：%s %s", path, status)
				}
			}
		}
	}
}

func TestEveryAPIRouteHasAnAction(t *testing.T) {
	t.Parallel()
	for _, route := range httpd.New().Routes() {
		if route.Path == "/v1/swagger/*" {
			continue
		}
		if !strings.HasPrefix(route.Path, "/api/") && !strings.HasPrefix(route.Path, "/v1/") {
			continue
		}
		if _, ok := accessConstants.ForEndpoint(route.Method, route.Path); !ok {
			t.Fatalf("接口未登记操作权限：%s %s", route.Method, route.Path)
		}
	}
}

func TestUnregisteredRouteCannotRun(t *testing.T) {
	t.Parallel()
	server := httpd.New()
	called := false
	server.GET("/v1/workspaces/:workspace_id/unregistered", func(c echo.Context) error {
		called = true
		return response.Success(c, true)
	})
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/v1/workspaces/w7m2k9p4s6n8c3x5/unregistered", nil))
	checkResponse(t, recorder)
	if called || recorder.Code != http.StatusForbidden {
		t.Fatal("未登记的工作空间接口绕过了通用鉴权中间件")
	}
}
