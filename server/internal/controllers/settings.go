package controllers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/settings"
)

func GetSettings(c *gin.Context) {
	values, err := settings.All()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取配置失败")
		return
	}
	cfg := config.GetConfig()
	respondOK(c, gin.H{
		"settings": values,
		"runtime": gin.H{
			"port":                 cfg.Port,
			"db_path":              cfg.DBPath,
			"allowed_origins":      cfg.AllowedOrigins,
			"max_concurrent_tasks": cfg.MaxConcurrentTasks,
			"db_max_open_conns":    cfg.DBMaxOpenConns,
			"db_max_idle_conns":    cfg.DBMaxIdleConns,
			"db_conn_max_lifetime": cfg.DBConnMaxLifetime.String(),
		},
	})
}

func UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := validateSettingsInput(req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := settings.Save(req); err != nil {
		respondError(c, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	values, _ := settings.All()
	respondCreated(c, "保存成功", gin.H{"message": "保存成功", "settings": values})
}

type settingRange struct {
	min int
	max int
}

var numericSettingRanges = map[string]settingRange{
	"default_video_count":        {min: 1, max: 50},
	"default_comment_count":      {min: 1, max: 500},
	"default_interval":           {min: 60, max: 86400},
	"default_report_delay":       {min: 30, max: 3600},
	"default_daily_report_limit": {min: 1, max: 5000},
	"default_max_retries":        {min: 0, max: 10},
	"default_retry_interval":     {min: 1, max: 300},
	"cookie_check_interval":      {min: 60, max: 86400},
	"cookie_refresh_interval":    {min: 300, max: 604800},
	"log_dedupe_window_seconds":  {min: 0, max: 86400},
	"risk_backoff_base_seconds":  {min: 60, max: 604800},
	"risk_backoff_max_seconds":   {min: 60, max: 1209600},
	"webhook_timeout":            {min: 1, max: 60},
}

var textSettingLimits = map[string]int{
	"telegram_bot_token":   500,
	"telegram_chat_id":     200,
	"feishu_webhook_url":   1000,
	"dingtalk_webhook_url": 1000,
}

func validateSettingsInput(values map[string]string) error {
	for key, value := range values {
		value = strings.TrimSpace(value)
		if bounds, ok := numericSettingRanges[key]; ok {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < bounds.min || parsed > bounds.max {
				return fmt.Errorf("%s 必须在 %d-%d 之间", key, bounds.min, bounds.max)
			}
			continue
		}
		if limit, ok := textSettingLimits[key]; ok {
			if runeLen(value) > limit {
				return fmt.Errorf("%s 不能超过 %d 个字符", key, limit)
			}
			if strings.HasSuffix(key, "_webhook_url") {
				if err := validateHTTPWebhookURL(value); err != nil {
					return fmt.Errorf("%s %w", key, err)
				}
			}
			continue
		}
		switch key {
		case "webhook_enabled":
			if _, err := strconv.ParseBool(value); err != nil {
				return fmt.Errorf("webhook_enabled 必须是 true 或 false")
			}
		case "webhook_type":
			switch value {
			case "none", "telegram", "feishu", "dingtalk":
			default:
				return fmt.Errorf("webhook_type 不支持")
			}
		default:
			return fmt.Errorf("未知配置项: %s", key)
		}
	}
	return nil
}

func validateHTTPWebhookURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("URL 格式无效")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("仅支持 http 或 https")
	}
}
