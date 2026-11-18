package config

import (
	"testing"
	"time"
)

func TestEmailAbnormalEventsConfiguration(t *testing.T) {
	t.Parallel()

	// 异常规则完全取自 YAML，空列表显式关闭新增拉黑，缺失、未知或重复配置报错。
	for _, test := range []struct {
		name   string
		events []string
		valid  bool
	}{
		{name: "默认异常事件", events: []string{"dropped", "bounce", "spamreport", "unsubscribe"}, valid: true},
		{name: "自定义正常事件", events: []string{"delivered", "open", "click", "deferred"}, valid: true},
		{name: "显式关闭", events: []string{}, valid: true},
		{name: "缺失配置"},
		{name: "拼写错误", events: []string{"bounced"}},
		{name: "大小写错误", events: []string{"Bounce"}},
		{name: "重复事件", events: []string{"bounce", "bounce"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := emailConfig{AbnormalEvents: test.events}
			if (cfg.validate() == nil) != test.valid {
				t.Fatal("邮件异常事件配置校验结果不符合预期")
			}
		})
	}
}

func TestEmailLoginConfiguration(t *testing.T) {
	// 邮件登录必须同时提供 Redis、云端模板和独立的 API 凭据。
	cfg := *Config
	cfg.Redis.Addr = "redis.example.com:6379"
	cfg.User.EmailLogin = emailLoginConfig{
		Enabled: true,
		TTL:     10 * time.Minute, MinInterval: time.Minute, MaxAttempts: 5,
		SES: sesConfig{
			SecretID: "fictional-secret-id", SecretKey: "fictional-secret-key",
			Region: "ap-guangzhou", TemplateID: 100091, Subject: "登录敏捷办公",
			FromAddress: "login@example.com", FromName: "敏捷办公",
			RequestTimeout: 10 * time.Second,
		},
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	hongkong := cfg
	hongkong.User.EmailLogin.SES.Region = "ap-hongkong"
	hongkong.User.EmailLogin.SES.FromName = ""
	if err := hongkong.validate(); err != nil {
		t.Fatal("中国香港地域或无别名发信配置被拒绝")
	}
	for _, change := range []func(*config){
		func(c *config) { c.Redis.Addr = "" },
		func(c *config) { c.User.EmailLogin.TTL = 10 * time.Second },
		func(c *config) { c.User.EmailLogin.TTL = 90 * time.Second },
		func(c *config) { c.User.EmailLogin.MinInterval = 0 },
		func(c *config) { c.User.EmailLogin.MinInterval = c.User.EmailLogin.TTL },
		func(c *config) { c.User.EmailLogin.MaxAttempts = 0 },
		func(c *config) { c.User.EmailLogin.MaxAttempts = 11 },
		func(c *config) { c.User.EmailLogin.SES.SecretID = "" },
		func(c *config) { c.User.EmailLogin.SES.SecretKey = " " },
		func(c *config) { c.User.EmailLogin.SES.Region = "ap-beijing" },
		func(c *config) { c.User.EmailLogin.SES.TemplateID = 0 },
		func(c *config) { c.User.EmailLogin.SES.Subject = " " },
		func(c *config) { c.User.EmailLogin.SES.Subject = "登录\r\n敏捷办公" },
		func(c *config) { c.User.EmailLogin.SES.RequestTimeout = 0 },
		func(c *config) { c.User.EmailLogin.SES.FromAddress = "invalid" },
		func(c *config) { c.User.EmailLogin.SES.FromAddress = "别名 <login@example.com>" },
		func(c *config) { c.User.EmailLogin.SES.FromName = "敏捷办公:登录" },
		func(c *config) { c.User.EmailLogin.SES.FromName = "敏捷办公\r\nBcc: other@example.com" },
	} {
		invalid := cfg
		change(&invalid)
		if invalid.validate() == nil {
			t.Fatal("邮件登录接受了无效配置")
		}
	}
}
