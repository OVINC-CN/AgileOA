package httpd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

const serverErrorBufferSize = 1

// New 使用全局配置创建 HTTP 服务，不透传配置或资源对象。
func New() *echo.Echo {
	// 初始化引擎和统一的错误、参数校验处理。
	engine := echo.New()
	engine.HideBanner = true
	engine.HidePort = true
	engine.HTTPErrorHandler = errorHandler
	engine.Validator = requestValidator{
		validate: validator.New(validator.WithRequiredStructEnabled()),
	}

	// 所有 HTTP 运行参数直接读取全局 YAML 配置。
	engine.Server.Addr = config.Config.App.Addr
	engine.Server.ReadHeaderTimeout = config.Config.HTTP.ReadHeaderTimeout
	engine.Server.ReadTimeout = config.Config.HTTP.ReadTimeout
	engine.Server.WriteTimeout = config.Config.HTTP.WriteTimeout
	engine.Server.IdleTimeout = config.Config.HTTP.IdleTimeout
	engine.Server.MaxHeaderBytes = config.Config.HTTP.MaxHeaderBytes
	trustedProxies := make([]echo.TrustOption, 0, 3+len(config.Config.HTTP.TrustedProxyNets))
	trustedProxies = append(
		trustedProxies,
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	)
	for _, network := range config.Config.HTTP.TrustedProxyNets {
		trustedProxies = append(trustedProxies, echo.TrustIPRange(network))
	}
	engine.IPExtractor = echo.ExtractIPFromXFFHeader(trustedProxies...)

	// 挂载请求追踪、英文日志和恢复中间件。
	engine.Use(traceRequests, auditRequest, requestLogger())
	engine.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     config.Config.HTTP.CORSAllowOrigins,
		AllowCredentials: true,
		AllowHeaders: []string{
			echo.HeaderAuthorization,
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			"X-CSRF-Protection",
			"MCP-Protocol-Version",
			"MCP-Session-Id",
			"MCP-Method",
			"MCP-Name",
		},
	}))
	engine.Use(recoverRequests(), middleware.Secure())
	engine.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 含账号信息或认证错误的响应禁止被浏览器与代理缓存。
			c.Response().Header().Set("Cache-Control", "no-store")
			return next(c)
		}
	})
	engine.Use(mcpOriginProtection, identifyUser, Authorize)
	engine.Use(middleware.BodyLimit(config.Config.HTTP.BodyLimit))
	engine.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: config.Config.HTTP.RequestTimeout,
	}))

	// 统一注册健康检查和业务接口，各 app 不自行注册路由。
	serveHealthz(engine)
	engine.Use(mcpConversion(engine))
	return engine
}

// Serve 启动监听，并在取消时按配置等待请求完成。
func Serve(ctx context.Context) error {
	// 创建服务，通过单个错误槽接收监听结果。
	engine := New()
	errorsCh := make(chan error, serverErrorBufferSize)
	go func() {
		// 监听失败时交回主流程处理。
		errorsCh <- engine.StartServer(engine.Server)
	}()
	logger.Logger.WithContext(ctx).Infof("API started; address: %s", config.Config.App.Addr)

	// 等待监听异常或进程退出信号。
	select {
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("api startup failed: %w", err)
	case <-ctx.Done():
		// 退出时使用独立上下文，避免取消信号中断正在收尾的请求。
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			config.Config.App.ShutdownTimeout,
		)
		defer cancel()
		if err := engine.Shutdown(shutdownContext); err != nil {
			return errors.Join(fmt.Errorf("api shutdown failed: %w", err), engine.Close())
		}
		return nil
	}
}
