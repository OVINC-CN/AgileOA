package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/gommon/bytes"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

var machineIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (c *config) validate() error {
	// 服务、环境和队列使用英文标识，不接受中文显示名称。
	for name, value := range map[string]string{
		"app.name":        c.App.Name,
		"app.environment": c.App.Environment,
		"scheduler.queue": c.Scheduler.Queue,
	} {
		if !machineIdentifierPattern.MatchString(value) {
			return fmt.Errorf(
				"%s must be an ASCII identifier using letters, digits, '.', '_' or '-'",
				name,
			)
		}
	}

	// 检查监听地址和日志级别。
	_, port, err := net.SplitHostPort(c.App.Addr)
	if err != nil {
		return fmt.Errorf("invalid app.addr: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < minimumTCPPort || portNumber > maximumTCPPort {
		return errors.New("app.addr must use a valid TCP port")
	}
	if _, err := logrus.ParseLevel(c.App.LogLevel); err != nil {
		return fmt.Errorf("invalid app.log_level: %w", err)
	}

	// 检查所有超时和轮询周期，运行参数必须在 YAML 中明确提供。
	for name, duration := range map[string]time.Duration{
		"trace.export_timeout":          c.Trace.ExportTimeout,
		"trace.batch_timeout":           c.Trace.BatchTimeout,
		"app.shutdown_timeout":          c.App.ShutdownTimeout,
		"http.request_timeout":          c.HTTP.RequestTimeout,
		"http.read_header_timeout":      c.HTTP.ReadHeaderTimeout,
		"http.read_timeout":             c.HTTP.ReadTimeout,
		"http.write_timeout":            c.HTTP.WriteTimeout,
		"http.idle_timeout":             c.HTTP.IdleTimeout,
		"http.health_check_timeout":     c.HTTP.HealthCheckTimeout,
		"mysql.conn_max_lifetime":       c.MySQL.ConnMaxLifetime,
		"mysql.connect_timeout":         c.MySQL.ConnectTimeout,
		"mysql.read_timeout":            c.MySQL.ReadTimeout,
		"mysql.write_timeout":           c.MySQL.WriteTimeout,
		"redis.dial_timeout":            c.Redis.DialTimeout,
		"redis.read_timeout":            c.Redis.ReadTimeout,
		"redis.write_timeout":           c.Redis.WriteTimeout,
		"redis.pool_timeout":            c.Redis.PoolTimeout,
		"redis.ping_timeout":            c.Redis.PingTimeout,
		"scheduler.task_check_interval": c.Scheduler.TaskCheckInterval,
		"scheduler.heartbeat_interval":  c.Scheduler.HeartbeatInterval,
		"user.session_ttl":              c.User.SessionTTL,
		"audit.write_timeout":           c.Audit.WriteTimeout,
	} {
		if duration <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if c.HTTP.WriteTimeout <= c.HTTP.RequestTimeout {
		return errors.New("http.write_timeout must exceed http.request_timeout")
	}
	if c.MCP.Enabled && c.MCP.MaxResponseBytes < 1 {
		return errors.New("mcp.max_response_bytes must be positive when MCP is enabled")
	}
	if c.HTTP.ReadHeaderTimeout > c.HTTP.ReadTimeout {
		return errors.New("http.read_header_timeout must not exceed http.read_timeout")
	}

	// 检查请求大小、连接池和重试限制。
	limit, err := bytes.Parse(c.HTTP.BodyLimit)
	if err != nil || limit <= 0 || c.HTTP.MaxHeaderBytes <= 0 {
		return errors.New("http body and header limits must be positive")
	}
	if c.MySQL.MaxOpenConns < 1 || c.MySQL.MaxIdleConns < 0 ||
		c.MySQL.MaxIdleConns > c.MySQL.MaxOpenConns {
		return errors.New("invalid MySQL pool configuration")
	}
	if c.Redis.DB < 0 || c.Redis.PoolSize < 1 || c.Redis.MaxRetries < 0 {
		return errors.New("invalid Redis database, pool, or retry configuration")
	}

	// 旧配置默认关闭维护通知，避免向不支持的服务端发送功能探测命令。
	if c.Redis.MaintNotificationsMode == "" {
		c.Redis.MaintNotificationsMode = maintnotifications.ModeDisabled
	}
	if !c.Redis.MaintNotificationsMode.IsValid() {
		return errors.New("redis.maint_notifications_mode 仅支持 disabled、auto 或 enabled")
	}
	if c.Scheduler.Concurrency < 1 {
		return errors.New("scheduler.concurrency must be positive")
	}
	if c.User.SessionTTL < time.Second || c.User.BcryptCost < bcrypt.DefaultCost ||
		c.User.BcryptCost > 16 {
		return errors.New("invalid user session or password configuration")
	}
	if err := c.User.Captcha.validate(); err != nil {
		return err
	}
	if err := c.User.EmailLogin.validate(); err != nil {
		return err
	}
	if err := c.Email.validate(); err != nil {
		return err
	}
	if c.User.EmailLogin.Enabled && strings.TrimSpace(c.Redis.Addr) == "" {
		return errors.New("user.email_login requires redis.addr")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(c.User.CookieName) {
		return errors.New("user.cookie_name must be a plain cookie name")
	}
	// 空域使用当前主机，非空域必须符合标准 Cookie 域规则，不接受 URL 或端口。
	cookie := http.Cookie{
		Name: c.User.CookieName, Domain: c.User.CookieDomain,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
	if err := cookie.Valid(); err != nil {
		return errors.New(
			"user.cookie_domain must be a valid cookie domain without a scheme, port, or path",
		)
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`).MatchString(c.User.BootstrapUsername) ||
		strings.TrimSpace(c.User.BootstrapDisplayName) == "" ||
		len([]rune(c.User.BootstrapDisplayName)) > 100 {
		return errors.New("invalid user bootstrap identity")
	}
	if c.User.BootstrapPassword != "" &&
		(len(c.User.BootstrapPassword) < 12 || len(c.User.BootstrapPassword) > 72) {
		return errors.New("user.bootstrap_password must contain 12 to 72 bytes")
	}
	for _, origin := range c.HTTP.CORSAllowOrigins {
		if origin == "*" || strings.Contains(origin, "*") {
			return errors.New("credentialed CORS requires explicit origins")
		}
	}
	// 仅信任显式配置的代理网段，客户端不能直接伪造转发地址。
	c.HTTP.TrustedProxyNets = make([]*net.IPNet, 0, len(c.HTTP.TrustedProxyCIDRs))
	for _, cidr := range c.HTTP.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return errors.New("http.trusted_proxy_cidrs must contain valid CIDR ranges")
		}
		c.HTTP.TrustedProxyNets = append(c.HTTP.TrustedProxyNets, network)
	}

	// API Token 使用独立的加密密钥，未配置时不开放生成或再次查看。
	if c.User.APITokenKey != "" {
		key, err := base64.StdEncoding.DecodeString(c.User.APITokenKey)
		if err != nil || len(key) != 32 {
			return errors.New("user.api_token_key must be a base64-encoded 32-byte key")
		}
		for _, auditKey := range c.Audit.SnapshotKeys {
			if auditKey == c.User.APITokenKey {
				return errors.New("user.api_token_key must be independent of audit snapshot keys")
			}
		}
	}

	// 敏感审计快照使用独立的密钥集合，允许保留历史密钥以支持后续轮换。
	if c.HTTP.DefaultPageSize < 1 || c.HTTP.MaxPageSize < c.HTTP.DefaultPageSize ||
		c.HTTP.MaxPageSize > 1000 || c.User.ChartDays < 1 || c.User.ChartDays > 365 ||
		c.Audit.QueryDefaultDays < 1 || c.Audit.QueryMaxDays < c.Audit.QueryDefaultDays ||
		c.Audit.QueryMaxDays > 366 {
		return errors.New("invalid management pagination or date window configuration")
	}
	if c.Audit.MaxPayloadBytes < 1 {
		return errors.New("audit.max_payload_bytes must be positive")
	}
	for keyID, encoded := range c.Audit.SnapshotKeys {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if !machineIdentifierPattern.MatchString(keyID) || err != nil || len(key) != 32 {
			return errors.New("audit snapshot keys must be named base64-encoded 32-byte keys")
		}
	}
	if c.Audit.SnapshotKeyID != "" && c.Audit.SnapshotKeys[c.Audit.SnapshotKeyID] == "" {
		return errors.New("audit.snapshot_key_id must refer to a configured independent key")
	}
	if err := c.validateStudent(); err != nil {
		return err
	}
	for _, limit := range []int{
		c.MonthlyWork.MaxOptions,
		c.MonthlyWork.MaxAttachments,
		c.MonthlyWork.MaxLinks,
		c.MonthlyWork.MaxImportRows,
		c.MonthlyWork.MaxFileBytes,
	} {
		if limit < 1 {
			return errors.New("monthly_work limits must be positive")
		}
	}
	if c.PartyBranch.MaxOptions < 1 || c.PartyBranch.MaxImportRows < 1 ||
		c.PartyBranch.MaxExportRows < 1 ||
		c.PartyBranch.MaxFileBytes < 1 || c.PartyBranch.MaxRuleDepth < 1 ||
		c.PartyBranch.MaxRuleNodes < 1 || c.PartyBranch.MaxRuleValues < 1 {
		return errors.New("party_branch limits must be positive")
	}
	if c.CPCPerson.MaxOptions < 1 || c.CPCPerson.MaxImportRows < 1 ||
		c.CPCPerson.MaxExportRows < 1 || c.CPCPerson.MaxFileBytes < 1 ||
		c.CPCPerson.RecalculationTimeout <= 0 || c.CPCPerson.RecalculationMaxRetries < 0 {
		return errors.New("cpc_person limits must be positive")
	}
	if c.Notification.DeliveryTimeout <= 0 || c.Notification.DeliveryMaxRetries < 0 {
		return errors.New("notification delivery timeout must be positive and retries nonnegative")
	}
	if c.Project.AutoSubmitTimeout <= 0 || c.Project.AutoSubmitMaxRetries < 0 {
		return errors.New("project auto-submit timeout must be positive and retries nonnegative")
	}
	if c.Project.RankingTimeout <= 0 || c.Project.RankingMaxRetries < 0 {
		return errors.New("project ranking timeout must be positive and retries nonnegative")
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	for _, limit := range []int{
		c.CPCTimeline.MaxAttachments,
		c.CPCTimeline.MaxNodes,
		c.CPCTimeline.MaxFieldsPerNode,
		c.CPCTimeline.MaxRulesPerNode,
		c.CPCTimeline.MaxDefinitionBytes,
		c.CPCTimeline.MaxOffset,
	} {
		if limit < 1 {
			return errors.New("cpc_timeline_template limits must be positive")
		}
	}

	// 上报可以关闭，追踪本身没有开关；队列和批次参数保持有效。
	if c.Trace.MaxQueueSize < 1 || c.Trace.MaxExportBatchSize < 1 ||
		c.Trace.MaxExportBatchSize > c.Trace.MaxQueueSize {
		return errors.New("invalid trace queue or batch size")
	}
	if c.Trace.ReportEnabled {
		host, port, err := net.SplitHostPort(c.Trace.Endpoint)
		if err != nil || host == "" || strings.ContainsAny(host, "/\\@?# \t\r\n") {
			return errors.New("trace.endpoint must be a gRPC host:port without a scheme or path")
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < minimumTCPPort || portNumber > maximumTCPPort {
			return errors.New("trace.endpoint must use a valid TCP port")
		}
	}

	// 缓存全局时区，供调度器直接使用。
	if strings.TrimSpace(c.Scheduler.Timezone) == "" {
		return errors.New("scheduler.timezone is required")
	}
	c.Scheduler.Location, err = time.LoadLocation(c.Scheduler.Timezone)
	if err != nil {
		return fmt.Errorf("invalid scheduler.timezone: %w", err)
	}
	return nil
}

func (c *captchaConfig) validate() error {
	if !c.Enabled {
		return nil
	}

	// 腾讯云要求加密有效期使用整数秒，最长为 24 小时。
	const maxChallengeTTL = 24 * time.Hour
	if c.ChallengeTTL < time.Second || c.ChallengeTTL > maxChallengeTTL ||
		c.ChallengeTTL%time.Second != 0 {
		return errors.New("user.captcha.challenge_ttl must be 1 to 86400 whole seconds")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("user.captcha.request_timeout must be positive")
	}
	if c.AppID == 0 || strings.TrimSpace(c.AppSecretKey) == "" || len(c.AppSecretKey) > 32 ||
		strings.TrimSpace(c.SecretID) == "" || strings.TrimSpace(c.SecretKey) == "" {
		return errors.New(
			"user.captcha requires an app ID, an app secret of up to 32 bytes and API credentials",
		)
	}
	return nil
}

func (c *config) validateStudent() error {
	if c.Student.MaxImportRows < 1 || c.Student.MaxExportRows < 1 || c.Student.MaxFileBytes < 1 ||
		c.Student.MaxOptions < 1 {
		return errors.New("student import, export, XLSX, and option limits must be positive")
	}
	return nil
}
