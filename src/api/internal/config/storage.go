package config

import (
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

func (c *config) validateStorage() error {
	// 申报规则和表达式均设置可配置边界，禁止无界嵌套与计算。
	if c.Project.ScoreMaxRules < 1 || c.Project.ScoreRuleMaxDepth < 1 ||
		c.Project.ScoreRuleMaxNodes < 1 || c.Project.ScoreRuleMaxValues < 1 ||
		c.Project.ScoreFormulaMaxLength < 1 || c.Project.ScoreFormulaMaxNodes < 1 ||
		c.Project.ScoreFormulaMaxDepth < 1 {
		return errors.New("project score calculation limits must be positive")
	}
	// 即使不启用存储也校验业务上限，避免无界输入。
	if c.Project.MaxApprovalTargets < 1 || c.Project.MaxApprovalSteps < 1 ||
		c.Project.MaxDetails < 1 || c.Project.MaxAttachments < 1 ||
		c.Project.MaxOptions < 1 || c.Project.MaxImportRows < 1 ||
		c.Project.MaxExportRows < 1 || c.Project.MaxFileBytes < 1 {
		return errors.New("project limits must be positive")
	}
	if c.COS.MaxUploadBytes < 1 || c.COS.RequestTimeout <= 0 || c.COS.MaxRetries < 0 ||
		c.COS.MaxUploadBytes > 5*1024*1024*1024 ||
		c.COS.UploadCredentialTTL < time.Second ||
		c.COS.UploadCredentialTTL > 2*time.Hour ||
		c.COS.UploadCredentialTTL%time.Second != 0 {
		return errors.New("invalid COS limits, credential TTL, or timeout")
	}

	// 前缀用于正式对象键和 STS 资源，统一移除首尾斜杠并拒绝歧义路径。
	c.COS.Prefix = strings.Trim(c.COS.Prefix, "/")
	if prefix := c.COS.Prefix; prefix != "" &&
		(!regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$`).MatchString(prefix) ||
			path.Clean("/"+prefix) != "/"+prefix) {
		return errors.New("cos.prefix must contain plain ASCII path segments")
	}
	// 保留桶根的旧版租户目录，避免新对象进入其他工作空间的旧版头像路径。
	if c.COS.Prefix == "workspaces" || strings.HasPrefix(c.COS.Prefix, "workspaces/") {
		return errors.New("cos.prefix must be outside the reserved workspaces directory")
	}

	if len(c.COS.AllowedContentTypes) == 0 {
		return errors.New("upload content types must not be empty")
	}
	seen := make(map[string]bool)
	for _, contentType := range c.COS.AllowedContentTypes {
		if !regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]*/([a-z0-9][a-z0-9!#$&^_.+-]*|\*)$`).
			MatchString(contentType) || seen[contentType] {
			return errors.New("invalid or duplicate upload content type")
		}
		seen[contentType] = true
	}
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`).MatchString(c.CDN.Parameter) {
		return errors.New("invalid CDN authentication parameter")
	}
	if !regexp.MustCompile(`^(/[A-Za-z0-9_-]+)*$`).MatchString(c.CDN.PathPrefix) {
		return errors.New("CDN path prefix must contain plain ASCII path segments")
	}
	if c.CDN.SigningCacheTTL < time.Millisecond || c.CDN.SigningCacheTTL%time.Millisecond != 0 {
		return errors.New("cdn.signing_cache_ttl must be a positive whole number of milliseconds")
	}
	if !c.COS.Enabled {
		return nil
	}
	// 私有桶和凭据只从 YAML 读取，禁止 SDK 的环境变量或实例身份回退。
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}-[0-9]+$`).MatchString(c.COS.Bucket) ||
		!regexp.MustCompile(`^[a-z][a-z0-9-]+$`).MatchString(c.COS.Region) ||
		strings.TrimSpace(c.COS.SecretID) == "" || strings.TrimSpace(c.COS.SecretKey) == "" {
		return errors.New("COS region, bucket, and static credentials are required")
	}

	domain, err := url.Parse(c.CDN.Domain)
	if err != nil || domain.Scheme != "https" || domain.Host == "" || domain.User != nil ||
		domain.RawQuery != "" || domain.Fragment != "" || domain.Path != "" ||
		c.CDN.SigningKey == "" || strings.ContainsAny(c.CDN.SigningKey, "\r\n") {
		return errors.New("CDN must use an HTTPS origin and a signing key")
	}
	return nil
}
