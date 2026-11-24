package config

import (
	"net"
	"time"

	"github.com/redis/go-redis/v9/maintnotifications"
)

type config struct {
	Trace        traceConfig        `yaml:"trace"`
	App          appConfig          `yaml:"app"`
	HTTP         httpConfig         `yaml:"http"`
	MCP          mcpConfig          `yaml:"mcp"`
	MySQL        mysqlConfig        `yaml:"mysql"`
	Redis        redisConfig        `yaml:"redis"`
	Scheduler    schedulerConfig    `yaml:"scheduler"`
	User         userConfig         `yaml:"user"`
	Audit        auditConfig        `yaml:"audit"`
	Student      studentConfig      `yaml:"student"`
	Project      projectConfig      `yaml:"project"`
	MonthlyWork  monthlyworkConfig  `yaml:"monthly_work"`
	PartyBranch  partybranchConfig  `yaml:"party_branch"`
	CPCPerson    cpcpersonConfig    `yaml:"cpc_person"`
	Notification notificationConfig `yaml:"notification"`
	Email        emailConfig        `yaml:"email"`
	CPCTimeline  cpctimelineConfig  `yaml:"cpc_timeline_template"`
	COS          cosConfig          `yaml:"cos"`
	CDN          cdnConfig          `yaml:"cdn"`
}

type emailConfig struct {
	CallbackSign   string   `yaml:"callback_sign"`
	AbnormalEvents []string `yaml:"abnormal_events"`
}

type mcpConfig struct {
	Enabled          bool  `yaml:"enabled"`
	MaxResponseBytes int64 `yaml:"max_response_bytes"`
}

type notificationConfig struct {
	DeliveryTimeout    time.Duration `yaml:"delivery_timeout"`
	DeliveryMaxRetries int           `yaml:"delivery_max_retries"`
}

type cpctimelineConfig struct {
	MaxAttachments     int `yaml:"max_attachments"`
	MaxNodes           int `yaml:"max_nodes"`
	MaxFieldsPerNode   int `yaml:"max_fields_per_node"`
	MaxRulesPerNode    int `yaml:"max_rules_per_node"`
	MaxDefinitionBytes int `yaml:"max_definition_bytes"`
	MaxOffset          int `yaml:"max_offset"`
}

type cpcpersonConfig struct {
	MaxOptions              int           `yaml:"max_options"`
	MaxImportRows           int           `yaml:"max_import_rows"`
	MaxExportRows           int           `yaml:"max_export_rows"`
	MaxFileBytes            int           `yaml:"max_file_bytes"`
	RecalculationTimeout    time.Duration `yaml:"recalculation_timeout"`
	RecalculationMaxRetries int           `yaml:"recalculation_max_retries"`
}

type partybranchConfig struct {
	MaxOptions    int `yaml:"max_options"`
	MaxImportRows int `yaml:"max_import_rows"`
	MaxExportRows int `yaml:"max_export_rows"`
	MaxFileBytes  int `yaml:"max_file_bytes"`
	MaxRuleDepth  int `yaml:"max_rule_depth"`
	MaxRuleNodes  int `yaml:"max_rule_nodes"`
	MaxRuleValues int `yaml:"max_rule_values"`
}

type monthlyworkConfig struct {
	MaxOptions     int `yaml:"max_options"`
	MaxAttachments int `yaml:"max_attachments"`
	MaxLinks       int `yaml:"max_links"`
	MaxImportRows  int `yaml:"max_import_rows"`
	MaxFileBytes   int `yaml:"max_file_bytes"`
}

type projectConfig struct {
	ScoreMaxRules         int           `yaml:"score_max_rules"`
	ScoreRuleMaxDepth     int           `yaml:"score_rule_max_depth"`
	ScoreRuleMaxNodes     int           `yaml:"score_rule_max_nodes"`
	ScoreRuleMaxValues    int           `yaml:"score_rule_max_values"`
	ScoreFormulaMaxLength int           `yaml:"score_formula_max_length"`
	ScoreFormulaMaxNodes  int           `yaml:"score_formula_max_nodes"`
	ScoreFormulaMaxDepth  int           `yaml:"score_formula_max_depth"`
	AutoSubmitTimeout     time.Duration `yaml:"auto_submit_timeout"`
	AutoSubmitMaxRetries  int           `yaml:"auto_submit_max_retries"`
	RankingTimeout        time.Duration `yaml:"ranking_timeout"`
	RankingMaxRetries     int           `yaml:"ranking_max_retries"`
	MaxApprovalTargets    int           `yaml:"max_approval_targets"`
	MaxApprovalSteps      int           `yaml:"max_approval_steps"`
	MaxDetails            int           `yaml:"max_details"`
	MaxAttachments        int           `yaml:"max_attachments"`
	MaxOptions            int           `yaml:"max_options"`
	MaxImportRows         int           `yaml:"max_import_rows"`
	MaxExportRows         int           `yaml:"max_export_rows"`
	MaxFileBytes          int           `yaml:"max_file_bytes"`
}

type cosConfig struct {
	Enabled             bool          `yaml:"enabled"`
	Region              string        `yaml:"region"`
	Bucket              string        `yaml:"bucket"`
	Prefix              string        `yaml:"prefix"`
	SecretID            string        `yaml:"secret_id"`
	SecretKey           string        `yaml:"secret_key"`
	UploadCredentialTTL time.Duration `yaml:"upload_credential_ttl"`
	RequestTimeout      time.Duration `yaml:"request_timeout"`
	MaxRetries          int           `yaml:"max_retries"`
	MaxUploadBytes      int64         `yaml:"max_upload_bytes"`
	AllowedContentTypes []string      `yaml:"allowed_content_types"`
}

type cdnConfig struct {
	Domain          string        `yaml:"domain"`
	PathPrefix      string        `yaml:"path_prefix"`
	SigningKey      string        `yaml:"signing_key"`
	Parameter       string        `yaml:"parameter"`
	SigningCacheTTL time.Duration `yaml:"signing_cache_ttl"`
}

type auditConfig struct {
	QueryDefaultDays int               `yaml:"query_default_days"`
	QueryMaxDays     int               `yaml:"query_max_days"`
	WriteTimeout     time.Duration     `yaml:"write_timeout"`
	MaxPayloadBytes  int               `yaml:"max_payload_bytes"`
	SnapshotKeyID    string            `yaml:"snapshot_key_id"`
	SnapshotKeys     map[string]string `yaml:"snapshot_keys"`
}

type userConfig struct {
	ChartDays            int              `yaml:"chart_days"`
	SessionTTL           time.Duration    `yaml:"session_ttl"`
	PasswordLoginEnabled bool             `yaml:"password_login_enabled"`
	APITokenKey          string           `yaml:"api_token_key"`
	CookieName           string           `yaml:"cookie_name"`
	CookieDomain         string           `yaml:"cookie_domain"`
	CookieSecure         bool             `yaml:"cookie_secure"`
	BcryptCost           int              `yaml:"bcrypt_cost"`
	Captcha              captchaConfig    `yaml:"captcha"`
	EmailLogin           emailLoginConfig `yaml:"email_login"`
	BootstrapUsername    string           `yaml:"bootstrap_username"`
	BootstrapDisplayName string           `yaml:"bootstrap_display_name"`
	BootstrapPassword    string           `yaml:"bootstrap_password"`
}

type emailLoginConfig struct {
	Enabled     bool          `yaml:"enabled"`
	TTL         time.Duration `yaml:"ttl"`
	MinInterval time.Duration `yaml:"min_interval"`
	MaxAttempts int           `yaml:"max_attempts"`
	SES         sesConfig     `yaml:"ses"`
}

type sesConfig struct {
	SecretID       string        `yaml:"secret_id"`
	SecretKey      string        `yaml:"secret_key"`
	Region         string        `yaml:"region"`
	FromAddress    string        `yaml:"from_address"`
	FromName       string        `yaml:"from_name"`
	TemplateID     uint64        `yaml:"template_id"`
	Subject        string        `yaml:"subject"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type captchaConfig struct {
	Enabled        bool          `yaml:"enabled"`
	AppID          uint64        `yaml:"app_id"`
	AppSecretKey   string        `yaml:"app_secret_key"`
	SecretID       string        `yaml:"secret_id"`
	SecretKey      string        `yaml:"secret_key"`
	ChallengeTTL   time.Duration `yaml:"challenge_ttl"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type appConfig struct {
	Debug           bool           `yaml:"debug"`
	Name            string         `yaml:"name"`
	Environment     string         `yaml:"environment"`
	Addr            string         `yaml:"addr"`
	LogLevel        string         `yaml:"log_level"`
	ShutdownTimeout time.Duration  `yaml:"shutdown_timeout"`
	TimeZone        string         `yaml:"time_zone"`
	Location        *time.Location `yaml:"-"`
}

type httpConfig struct {
	DefaultPageSize    int           `yaml:"default_page_size"`
	MaxPageSize        int           `yaml:"max_page_size"`
	CORSAllowOrigins   []string      `yaml:"cors_allow_origins"`
	TrustedProxyCIDRs  []string      `yaml:"trusted_proxy_cidrs"`
	TrustedProxyNets   []*net.IPNet  `yaml:"-"`
	RequestTimeout     time.Duration `yaml:"request_timeout"`
	ReadHeaderTimeout  time.Duration `yaml:"read_header_timeout"`
	ReadTimeout        time.Duration `yaml:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	IdleTimeout        time.Duration `yaml:"idle_timeout"`
	HealthCheckTimeout time.Duration `yaml:"health_check_timeout"`
	BodyLimit          string        `yaml:"body_limit"`
	MaxHeaderBytes     int           `yaml:"max_header_bytes"`
}

type mysqlConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
}

type redisConfig struct {
	Addr                   string                  `yaml:"addr"`
	Password               string                  `yaml:"password"`
	DB                     int                     `yaml:"db"`
	DialTimeout            time.Duration           `yaml:"dial_timeout"`
	ReadTimeout            time.Duration           `yaml:"read_timeout"`
	WriteTimeout           time.Duration           `yaml:"write_timeout"`
	PoolTimeout            time.Duration           `yaml:"pool_timeout"`
	PingTimeout            time.Duration           `yaml:"ping_timeout"`
	PoolSize               int                     `yaml:"pool_size"`
	MaxRetries             int                     `yaml:"max_retries"`
	IdentityEnabled        bool                    `yaml:"identity_enabled"`
	MaintNotificationsMode maintnotifications.Mode `yaml:"maint_notifications_mode"`
}

type schedulerConfig struct {
	Queue             string         `yaml:"queue"`
	Concurrency       int            `yaml:"concurrency"`
	TaskCheckInterval time.Duration  `yaml:"task_check_interval"`
	HeartbeatInterval time.Duration  `yaml:"heartbeat_interval"`
	Timezone          string         `yaml:"timezone"`
	Location          *time.Location `yaml:"-"`
}

type traceConfig struct {
	ReportEnabled      bool              `yaml:"report_enabled"`
	Endpoint           string            `yaml:"endpoint"`
	Insecure           bool              `yaml:"insecure"`
	Headers            map[string]string `yaml:"headers"`
	ExportTimeout      time.Duration     `yaml:"export_timeout"`
	BatchTimeout       time.Duration     `yaml:"batch_timeout"`
	MaxQueueSize       int               `yaml:"max_queue_size"`
	MaxExportBatchSize int               `yaml:"max_export_batch_size"`
}

// studentConfig 集中配置导入导出及选项查询上限。
type studentConfig struct {
	MaxImportRows int `yaml:"max_import_rows"`
	MaxExportRows int `yaml:"max_export_rows"`
	MaxFileBytes  int `yaml:"max_file_bytes"`
	MaxOptions    int `yaml:"max_options"`
}
