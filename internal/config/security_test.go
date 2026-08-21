package config

import (
	"strings"
	"testing"

	"github.com/LyricTian/gin-admin/v10/pkg/crypto/hash"
)

func secureTestConfig(t *testing.T) *Config {
	t.Helper()
	cfg := new(Config)
	cfg.General.Security.Strict = true
	cfg.General.DisablePrintConfig = true
	cfg.General.Root.Username = "root-admin"
	passwordHash, err := hash.GeneratePassword("strong-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	cfg.General.Root.PasswordHash = passwordHash
	cfg.Middleware.Auth.SigningKey = strings.Repeat("k", 32)
	return cfg
}

func TestValidateSecurityAcceptsProductionConfig(t *testing.T) {
	if err := secureTestConfig(t).ValidateSecurity(); err != nil {
		t.Fatalf("expected secure config, got %v", err)
	}
}

func TestValidateSecurityRejectsWeakValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "debug", mutate: func(c *Config) { c.General.Debug = true }},
		{name: "config output", mutate: func(c *Config) { c.General.DisablePrintConfig = false }},
		{name: "disabled auth", mutate: func(c *Config) { c.Middleware.Auth.Disable = true }},
		{name: "short signing key", mutate: func(c *Config) { c.Middleware.Auth.SigningKey = "short" }},
		{name: "example signing key", mutate: func(c *Config) { c.Middleware.Auth.SigningKey = "ChangeThisJwtSigningKeyWithAtLeast32Chars" }},
		{name: "invalid root hash", mutate: func(c *Config) { c.General.Root.PasswordHash = "plaintext" }},
		{name: "default prometheus", mutate: func(c *Config) {
			c.Util.Prometheus.Enable = true
			c.Util.Prometheus.BasicUsername = "admin"
			c.Util.Prometheus.BasicPassword = "admin"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := secureTestConfig(t)
			tc.mutate(cfg)
			if err := cfg.ValidateSecurity(); err == nil {
				t.Fatal("expected security validation error")
			}
		})
	}
}
