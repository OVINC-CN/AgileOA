package httpd

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
)

func recoverRequests() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			// HTTP 与 MCP 复用异常恢复，工具执行所在的协程也必须受到保护。
			logger.Logger.WithContext(c.Request().Context()).Errorf(
				"Panic recovered: %v\nstack: %s", err, string(stack),
			)
			return err
		},
	})
}

func requestLogger() echo.MiddlewareFunc {
	// 请求日志复用全局 logger，由日志钩子自动补充 Trace ID 和 Span ID。
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:       true,
		LogLatency:      true,
		LogMethod:       true,
		LogRemoteIP:     true,
		LogUserAgent:    true,
		LogReferer:      true,
		LogResponseSize: true,
		HandleError:     true,
		LogValuesFunc: func(c echo.Context, values middleware.RequestLoggerValues) error {
			// 探针跳过请求日志，统一错误处理仍记录异常。
			if values.Method == http.MethodGet && (c.Path() == "/health" || c.Path() == "/ready") {
				return nil
			}

			// IP 沿用可信代理配置，UA、Referer 分行记录并转义控制字符，不记录 URL 参数或凭据。
			logger.Logger.WithContext(c.Request().Context()).Infof(
				"%s %s %d %s; ip: %s; bytes: %d\nua: %q\nreferer: %q",
				values.Method,
				c.Path(),
				values.Status,
				values.Latency,
				values.RemoteIP,
				values.ResponseSize,
				values.UserAgent,
				requestLogReferer(values.Referer),
			)
			return nil
		},
	})
}

func requestLogReferer(value string) string {
	// 校验来源 URL，仅接受 HTTP(S) 站点，缺失或格式异常时使用统一占位。
	referer, err := url.Parse(value)
	if err != nil || referer.Host == "" ||
		(referer.Scheme != "http" && referer.Scheme != "https") {
		return "-"
	}

	// 仅记录来源站点和路径，避免 URL 凭据、登录码、签名及片段进入日志。
	return (&url.URL{
		Scheme:  referer.Scheme,
		Host:    referer.Host,
		Path:    referer.Path,
		RawPath: referer.RawPath,
	}).String()
}
