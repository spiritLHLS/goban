package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigGeneratesFileBackedSecrets(t *testing.T) {
	globalConfig = nil
	t.Cleanup(func() { globalConfig = nil })

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "goban.db")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("PASSWORD", "")
	t.Setenv("GOBAN_SECRET_KEY", "")
	t.Setenv("GOBAN_PASSWORD_FILE", "")
	t.Setenv("GOBAN_SECRET_KEY_FILE", "")

	cfg := LoadConfig()
	if cfg.Password == "" {
		t.Fatal("expected generated password")
	}
	if cfg.SecretKey == "" {
		t.Fatal("expected generated secret key")
	}
	if cfg.Password == cfg.SecretKey {
		t.Fatal("password and encryption key should be separate generated values")
	}

	for _, path := range []string{
		filepath.Join(tmp, ".goban_admin_password"),
		filepath.Join(tmp, ".goban_secret_key"),
	} {
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated secret file %s: %v", path, err)
		} else if info.Mode().Perm() != 0600 {
			t.Fatalf("expected %s to have 0600 permissions, got %v", path, info.Mode().Perm())
		}
	}
}

func TestLoadConfigParsesAllowedOrigins(t *testing.T) {
	globalConfig = nil
	t.Cleanup(func() { globalConfig = nil })

	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")
	t.Setenv("ALLOWED_ORIGINS", "https://one.example, https://two.example,https://one.example")

	cfg := LoadConfig()
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("expected duplicate origins to be removed, got %#v", cfg.AllowedOrigins)
	}
	if cfg.AllowedOrigins[0] != "https://one.example" || cfg.AllowedOrigins[1] != "https://two.example" {
		t.Fatalf("unexpected origins: %#v", cfg.AllowedOrigins)
	}
}

func TestLoadConfigPrefersGobanUsername(t *testing.T) {
	globalConfig = nil
	t.Cleanup(func() { globalConfig = nil })

	t.Setenv("GOBAN_USERNAME", "goban-admin")
	t.Setenv("USERNAME", "shell-user")
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")

	cfg := LoadConfig()
	if cfg.Username != "goban-admin" {
		t.Fatalf("expected GOBAN_USERNAME to win, got %q", cfg.Username)
	}
}

func TestLoadConfigKeepsUsernameCompatibility(t *testing.T) {
	globalConfig = nil
	t.Cleanup(func() { globalConfig = nil })

	t.Setenv("GOBAN_USERNAME", "")
	t.Setenv("USERNAME", "legacy-admin")
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")

	cfg := LoadConfig()
	if cfg.Username != "legacy-admin" {
		t.Fatalf("expected USERNAME compatibility, got %q", cfg.Username)
	}
}
