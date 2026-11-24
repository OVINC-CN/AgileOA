package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, change func(string) string) string {
	t.Helper()

	// 使用完整的项目配置，避免测试依赖额外的代码默认值。
	content, err := os.ReadFile(FilePath)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), defaultConfigFilename)
	if err := os.WriteFile(filename, []byte(change(string(content))), 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func TestLoadYAMLIgnoresEnvironment(t *testing.T) {
	// 冲突的环境变量不应覆盖文件中的配置。
	t.Setenv("APP_ADDR", "invalid-environment-address")
	t.Setenv("APP_NAME", "environment-service")
	t.Setenv("REDIS_ADDR", "invalid-environment-address")
	filename := writeConfig(t, func(content string) string {
		// 只修改目标字段，其他运行参数仍来自配置文件。
		content = strings.Replace(content, "name: agile-workspace", "name: test-workspace.v2_1", 1)
		content = strings.Replace(content, "environment: development", "environment: staging", 1)
		content = strings.Replace(content, "addr: 127.0.0.1:8080", "addr: 127.0.0.1:9080", 1)
		content = strings.Replace(content, `  prefix: ""`, "  prefix: /apps/agile/", 1)
		return strings.Replace(content, "request_timeout: 30s", "request_timeout: 12s", 1)
	})

	// 校验文件内容和独立超时字段均被正确读取。
	cfg, err := load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Name != "test-workspace.v2_1" || cfg.App.Environment != "staging" ||
		cfg.App.Addr != "127.0.0.1:9080" ||
		cfg.HTTP.RequestTimeout != 12*time.Second {
		t.Fatalf("未正确读取 YAML 配置：%+v", cfg.App)
	}
	if cfg.Redis.Addr != "" || cfg.HTTP.ReadTimeout != 30*time.Second {
		t.Fatal("环境变量覆盖了 YAML 配置，或独立超时字段相互影响")
	}
	if cfg.COS.Prefix != "apps/agile" {
		t.Fatalf("COS 前缀解析错误：%q", cfg.COS.Prefix)
	}
}

func TestLoadPasswordLoginEnabled(t *testing.T) {
	t.Parallel()

	// 开关从 YAML 读取，未配置时保持关闭。
	for _, test := range []struct {
		name    string
		setting string
		want    bool
	}{
		{name: "enabled", setting: "  password_login_enabled: true\n", want: true},
		{name: "disabled", setting: "  password_login_enabled: false\n", want: false},
		{name: "omitted", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			filename := writeConfig(t, func(content string) string {
				return strings.Replace(content, "  password_login_enabled: true\n", test.setting, 1)
			})
			cfg, err := load(filename)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.User.PasswordLoginEnabled != test.want {
				t.Fatalf("密码登录开关解析错误：%t", cfg.User.PasswordLoginEnabled)
			}
		})
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	// 覆盖严格解析、边界值及各类运行参数错误。
	for _, test := range []struct{ name, old, replacement string }{
		{"unknown-field", "name: agile-workspace", "unknown: test"},
		{"duplicate-key", "name: agile-workspace", "name: first\n  name: second"},
		{"invalid-duration", "request_timeout: 30s", "request_timeout: forever"},
		{"zero-timeout", "health_check_timeout: 2s", "health_check_timeout: 0s"},
		{"zero-upload-size", "max_upload_bytes: 5242880", "max_upload_bytes: 0"},
		{"empty-upload-types", "[application/pdf, \"image/*\"]", "[]"},
		{"invalid-upload-types", "[application/pdf, \"image/*\"]", "[pdf]"},
		{"duplicate-upload-types", "[application/pdf, \"image/*\"]", "[image/png, image/png]"},
		{"zero-notification-timeout", "delivery_timeout: 30s", "delivery_timeout: 0s"},
		{"zero-auto-submit-timeout", "auto_submit_timeout: 5m", "auto_submit_timeout: 0s"},
		{"negative-auto-submit-retries", "auto_submit_max_retries: 3", "auto_submit_max_retries: -1"},
		{"zero-ranking-timeout", "ranking_timeout: 5m", "ranking_timeout: 0s"},
		{"negative-ranking-retries", "ranking_max_retries: 3", "ranking_max_retries: -1"},
		{"zero-mcp-response-limit", "max_response_bytes: 33554432", "max_response_bytes: 0"},
		{"negative-notification-retries", "delivery_max_retries: 5", "delivery_max_retries: -1"},
		{"missing-timeout", "  idle_timeout: 60s\n", ""},
		{"invalid-port", "addr: 127.0.0.1:8080", "addr: 127.0.0.1:70000"},
		{"invalid-pool", "max_open_conns: 10", "max_open_conns: 2"},
		{"empty-name", "name: agile-workspace", "name: ''"},
		// 中文仅作为无效输入，验证显示名称不能进入内部标识。
		{"chinese-service-name", "name: agile-workspace", "name: 敏捷办公"},
		{"chinese-environment", "environment: development", "environment: 开发环境"},
		{"chinese-queue", "queue: agile-workspace", "queue: 后台任务"},
		{"mixed-environment", "environment: development", "environment: development环境"},
		{"identifier-with-spaces", "name: agile-workspace", "name: agile workspace"},
		{"identifier-with-path", "queue: agile-workspace", "queue: agile/workspace"},
		{"empty-environment", "environment: development", "environment: ''"},
		{"empty-queue", "queue: agile-workspace", "queue: ''"},
		{"invalid-body-limit", "body_limit: 16M", "body_limit: invalid"},
		{"short-write-timeout", "write_timeout: 31s", "write_timeout: 30s"},
		{"invalid-timezone", "timezone: Asia/Shanghai", "timezone: invalid"},
		{"invalid-display-timezone", "time_zone: Asia/Shanghai", "time_zone: invalid"},
		{"local-display-timezone", "time_zone: Asia/Shanghai", "time_zone: Local"},
		{
			"removed-student-account-sync",
			"timezone: Asia/Shanghai",
			"timezone: Asia/Shanghai\n  student_account_sync:\n    cron: \"0 * * * *\"\n    timeout: 50m",
		},
		{"invalid-retries", "max_retries: 3", "max_retries: -2"},
		{
			"invalid-redis-maintenance-mode",
			"maint_notifications_mode: disabled",
			"maint_notifications_mode: unsupported",
		},
		{"cookie-domain-url", "cookie_domain: \"\"", "cookie_domain: https://example.com"},
		{"cookie-domain-port", "cookie_domain: \"\"", "cookie_domain: example.com:8080"},
		{"cookie-domain-path", "cookie_domain: \"\"", "cookie_domain: example.com/v1"},
		{"invalid-api-token-key", "api_token_key: \"\"", "api_token_key: invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// 无效配置必须在资源初始化前被拒绝。
			filename := writeConfig(t, func(content string) string {
				return strings.Replace(content, test.old, test.replacement, 1)
			})
			if _, err := load(filename); err == nil {
				t.Fatal("无效配置未被拒绝")
			}
		})
	}
}

func TestLoadRedisMaintenanceMode(t *testing.T) {
	t.Parallel()

	// 维护通知支持三种显式模式，旧配置省略字段时保持关闭。
	for _, mode := range []string{"disabled", "auto", "enabled", ""} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Parallel()
			filename := writeConfig(t, func(content string) string {
				replacement := ""
				if mode != "" {
					replacement = "  maint_notifications_mode: " + mode + "\n"
				}
				return strings.Replace(
					content,
					"  maint_notifications_mode: disabled\n",
					replacement,
					1,
				)
			})
			cfg, err := load(filename)
			if err != nil {
				t.Fatal(err)
			}
			want := mode
			if want == "" {
				want = "disabled"
			}
			if cfg.Redis.MaintNotificationsMode.String() != want {
				t.Fatalf("Redis 维护通知模式不正确：%s", cfg.Redis.MaintNotificationsMode)
			}
		})
	}
}

func TestPartyBranchExportLimitMustBePositive(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, -1, 1} {
		cfg := *Config
		cfg.PartyBranch.MaxExportRows = limit
		if err := cfg.validate(); (err == nil) != (limit > 0) {
			t.Fatalf("党支部导出行数上限校验错误：%d，%v", limit, err)
		}
	}
}

func TestLoadRequiresOneDocumentAndExistingFile(t *testing.T) {
	t.Parallel()

	// 多文档不能绕过配置校验。
	filename := writeConfig(t, func(content string) string {
		return content + "\n---\napp: {}\n"
	})
	if _, err := load(filename); err == nil {
		t.Fatal("多文档配置未被拒绝")
	}

	// 文件缺失时不允许静默回退到代码默认值。
	if _, err := load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("缺失的配置文件未被拒绝")
	}
}

func TestConfigFilenameBeforeResourceInit(t *testing.T) {
	t.Parallel()

	// init 阶段需要识别独立参数和等号参数两种写法。
	directory := t.TempDir()
	for _, arguments := range [][]string{
		{"api", "--config", "custom.yaml"},
		{"--config=custom.yaml", "worker"},
	} {
		filename, err := configFilename(arguments, directory)
		if err != nil || filename != filepath.Join(directory, "custom.yaml") {
			t.Fatalf("配置参数解析错误：%v，%s，%v", arguments, filename, err)
		}
	}

	// 无效参数不能被忽略后改用其他配置。
	for _, arguments := range [][]string{{"--config"}, {"--config="}, {"--config", ""}, {"--config", "--help"}} {
		if _, err := configFilename(arguments, directory); err == nil {
			t.Fatalf("无效配置参数未被拒绝：%v", arguments)
		}
	}
}

func TestGRPCEndpointValidation(t *testing.T) {
	t.Parallel()

	// gRPC 端点只接受有效的主机和端口，不能继续沿用 HTTP 上报路径。
	for _, test := range []struct {
		endpoint string
		valid    bool
	}{
		{"127.0.0.1:4317", true},
		{"collector.example.com:443", true},
		{"[::1]:4317", true},
		{"", false},
		{":4317", false},
		{"http://127.0.0.1:4317", false},
		{"https://collector.example.com:443", false},
		{"127.0.0.1:4318/v1/traces", false},
		{"collector/path:4317", false},
		{"collector:0", false},
		{"collector:70000", false},
		{"collector:grpc", false},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			t.Parallel()

			// 使用配置副本，避免并发校验修改全局运行参数。
			cfg := *Config
			cfg.Trace.ReportEnabled = true
			cfg.Trace.Endpoint = test.endpoint
			if err := cfg.validate(); (err == nil) != test.valid {
				t.Fatalf("gRPC 端点校验结果错误：%q，%v", test.endpoint, err)
			}
		})
	}
}
