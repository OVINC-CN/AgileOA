package config

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/email/emailConstants"
)

func (c *emailConfig) validate() error {
	// 异常事件允许按部署调整，拒绝拼写错误和重复配置；空列表表示不新增黑名单。
	if c.AbnormalEvents == nil {
		return errors.New("email.abnormal_events must be configured; use [] to disable new blocks")
	}
	seen := make(map[string]bool, len(c.AbnormalEvents))
	for _, event := range c.AbnormalEvents {
		if _, ok := emailConstants.Events[event]; !ok || seen[event] {
			return errors.New("email.abnormal_events contains an invalid or duplicate event")
		}
		seen[event] = true
	}
	return nil
}

func (c *emailLoginConfig) validate() error {
	// 未启用时兼容已有部署，启用后必须明确配置云端模板与邮件运行参数。
	if !c.Enabled {
		return nil
	}
	if c.TTL < time.Minute || c.TTL%time.Minute != 0 ||
		c.MinInterval < time.Millisecond || c.MinInterval >= c.TTL {
		return errors.New("invalid user.email_login TTL or minimum interval")
	}
	if c.MaxAttempts < 1 || c.MaxAttempts > 10 {
		return errors.New("user.email_login.max_attempts must be between 1 and 10")
	}

	// 邮件推送独立使用 YAML 凭据，不回退到验证码配置或环境凭据。
	ses := c.SES
	if strings.TrimSpace(ses.SecretID) == "" || strings.TrimSpace(ses.SecretKey) == "" {
		return errors.New("user.email_login.ses requires secret_id and secret_key")
	}
	if ses.Region != "ap-guangzhou" && ses.Region != "ap-hongkong" {
		return errors.New("user.email_login.ses.region must be ap-guangzhou or ap-hongkong")
	}
	if ses.TemplateID == 0 || strings.TrimSpace(ses.Subject) == "" ||
		strings.ContainsAny(ses.Subject, "\r\n") || ses.RequestTimeout <= 0 {
		return errors.New("invalid user.email_login.ses template, subject or request timeout")
	}
	address, err := mail.ParseAddress(ses.FromAddress)
	if err != nil || address.Address != ses.FromAddress ||
		strings.ContainsAny(ses.FromName, ":<>\r\n") {
		return errors.New("invalid user.email_login.ses sender")
	}
	return nil
}
