package database

import (
	"path/filepath"
	"testing"

	"github.com/spiritlhl/goban/internal/models"
)

func TestInitDBMigratesOperationalStateAndDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(tmp, "goban.db"))
	t.Setenv("PASSWORD", "test-password")
	t.Setenv("GOBAN_SECRET_KEY", "test-secret")

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	db := GetDB()
	for _, column := range []string{"next_run_at", "backoff_until", "progress_message"} {
		if !db.Migrator().HasColumn(&models.MonitorTask{}, column) {
			t.Fatalf("monitor_tasks missing column %s", column)
		}
	}
	for _, column := range []string{"digest", "repeat_count", "last_seen_at"} {
		if !db.Migrator().HasColumn(&models.MonitorLog{}, column) {
			t.Fatalf("monitor_logs missing column %s", column)
		}
	}

	for _, key := range []string{
		"cookie_refresh_interval",
		"log_dedupe_window_seconds",
		"risk_backoff_base_seconds",
		"risk_backoff_max_seconds",
	} {
		var setting models.AppSetting
		if err := db.First(&setting, "key = ?", key).Error; err != nil {
			t.Fatalf("missing default setting %s: %v", key, err)
		}
		if setting.Value == "" {
			t.Fatalf("default setting %s has empty value", key)
		}
	}
}
