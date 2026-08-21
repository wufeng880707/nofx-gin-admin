package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LyricTian/gin-admin/v10/pkg/encoding/toml"
	"github.com/creasty/defaults"
)

func TestDockerConfigTemplatesParse(t *testing.T) {
	cfg := new(Config)
	if err := defaults.Set(cfg); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"MYSQL_DATABASE": "nofx_admin", "MYSQL_USER": "nofx", "MYSQL_PASSWORD": "mysql_password",
		"REDIS_PASSWORD": "redis_password", "JWT_SIGNING_KEY": "test-only-random-signing-key-2f8f18d0c6e64a86",
		"ADMIN_USERNAME": "admin", "ADMIN_PASSWORD_HASH": "$2a$10$3euPcmQFCiblsZeEu5s7p.9mukpiVAxsDsVLeFwrVIEVMkW6qFOdW",
	}
	for _, name := range []string{"server.toml.template", "middleware.toml.template"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "configs", "docker", name))
		if err != nil {
			t.Fatal(err)
		}
		rendered := os.Expand(string(data), func(key string) string { return values[key] })
		if err := toml.Unmarshal([]byte(rendered), cfg); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
	}
	if cfg.Storage.DB.Type != "mysql" || !strings.Contains(cfg.Storage.DB.DSN, "tcp(mysql:3306)") {
		t.Fatalf("unexpected Docker database config: type=%q dsn=%q", cfg.Storage.DB.Type, cfg.Storage.DB.DSN)
	}
	if cfg.Storage.Cache.Type != "redis" || cfg.Middleware.Auth.Store.Type != "redis" {
		t.Fatalf("Docker Redis config was not applied: %#v %#v", cfg.Storage.Cache, cfg.Middleware.Auth.Store)
	}
	if !cfg.General.Security.Strict || cfg.General.Debug {
		t.Fatalf("Docker strict security config was not applied: %#v", cfg.General)
	}
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("Docker security config is invalid: %v", err)
	}
	if !cfg.Trading.AccountSync.Enabled || cfg.Trading.AccountSync.IntervalSeconds != 300 ||
		cfg.Trading.AccountSync.TimeoutSeconds != 60 || cfg.Trading.AccountSync.SnapshotIntervalHours != 12 ||
		cfg.Trading.AccountSync.SnapshotTimeZone != "Asia/Shanghai" {
		t.Fatalf("Docker trading account sync config was not applied: %#v", cfg.Trading.AccountSync)
	}
}
