package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/models"
	"gorm.io/gorm"
)

var db *gorm.DB

func InitDB() error {
	var err error
	cfg := config.GetConfig()
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0750); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err = gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	if err := os.Chmod(cfg.DBPath, 0600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("设置数据库文件权限失败: %w", err)
	}

	// 自动迁移数据库表
	if err := db.AutoMigrate(
		&models.BiliUser{},
		&models.MonitorTask{},
		&models.MonitorTarget{},
		&models.KeywordRule{},
		&models.WhitelistUser{},
		&models.AppSetting{},
		&models.MonitorLog{},
		&models.ReportRecord{},
	); err != nil {
		return err
	}

	return seedDefaultSettings(db)
}

func GetDB() *gorm.DB {
	return db
}

func seedDefaultSettings(db *gorm.DB) error {
	defaults := map[string]string{
		"default_video_count":        "5",
		"default_comment_count":      "50",
		"default_interval":           "300",
		"default_report_delay":       "30",
		"default_daily_report_limit": "100",
		"default_max_retries":        "3",
		"default_retry_interval":     "2",
		"cookie_check_interval":      "3600",
		"cookie_refresh_interval":    "21600",
		"log_dedupe_window_seconds":  "300",
		"risk_backoff_base_seconds":  "1800",
		"risk_backoff_max_seconds":   "86400",
		"webhook_enabled":            "false",
		"webhook_type":               "none",
		"webhook_timeout":            "8",
	}

	for key, value := range defaults {
		setting := models.AppSetting{Key: key, Value: value}
		if err := db.FirstOrCreate(&setting, models.AppSetting{Key: key}).Error; err != nil {
			return err
		}
		if setting.Value == "" {
			if err := db.Model(&setting).Update("value", value).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
