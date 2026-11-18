package config

import (
	"testing"
	"time"
)

func TestStorageConfigurationRequiresSafeOriginAndSigningSettings(t *testing.T) {
	cfg := *Config
	cfg.COS.Enabled = true
	cfg.COS.Region = "ap-guangzhou"
	cfg.COS.Bucket = "fixture-project-1250000000"
	cfg.COS.SecretID = "fictional-access"
	cfg.COS.SecretKey = "fictional-secret"
	cfg.CDN.Domain = "https://cdn.example.com"
	cfg.CDN.SigningKey = "fictional-signing-secret"
	for _, change := range []func(*config){
		func(c *config) { c.Project.MaxExportRows = 0 },
		func(c *config) { c.Project.MaxExportRows = -1 },
		func(c *config) { c.COS.Bucket = "fixture/no-account-id" },
		func(c *config) { c.COS.UploadCredentialTTL = 0 },
		func(c *config) { c.COS.UploadCredentialTTL = 3 * time.Hour },
		func(c *config) { c.COS.MaxUploadBytes = 5*1024*1024*1024 + 1 },
		func(c *config) { c.CDN.Domain = "http://cdn.example.com" },
		func(c *config) { c.CDN.Domain = "https://cdn.example.com/path" },
		func(c *config) { c.CDN.PathPrefix = "/files/../other" },
		func(c *config) { c.CDN.Parameter = "eo-sign&extra" },
		func(c *config) { c.CDN.SigningKey = "" },
		func(c *config) { c.CDN.SigningCacheTTL = 0 },
		func(c *config) { c.CDN.SigningCacheTTL = time.Microsecond },
	} {
		invalid := cfg
		change(&invalid)
		if invalid.validateStorage() == nil {
			t.Fatal("无效的存储或鉴权参数被接受")
		}
	}
	if err := cfg.validateStorage(); err != nil {
		t.Fatal(err)
	}
}
