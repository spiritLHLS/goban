package controllers

import (
	"strings"
	"testing"
)

func TestValidateMonitorTaskInputBounds(t *testing.T) {
	err := validateMonitorTaskInput(taskRequest{
		VideoCount:  51,
		ReportDelay: 30,
		Interval:    300,
	}, []int64{1})
	if err == nil || !strings.Contains(err.Error(), "视频数") {
		t.Fatalf("expected video count validation error, got %v", err)
	}

	err = validateMonitorTaskInput(taskRequest{
		ProxyURL:    "ftp://127.0.0.1:8080",
		ReportDelay: 30,
		Interval:    300,
	}, []int64{1})
	if err == nil || !strings.Contains(err.Error(), "代理地址") {
		t.Fatalf("expected proxy validation error, got %v", err)
	}
}

func TestValidateMonitorTaskInputTargetLimit(t *testing.T) {
	targets := make([]int64, maxTaskTargets+1)
	for i := range targets {
		targets[i] = int64(i + 1)
	}
	err := validateMonitorTaskInput(taskRequest{}, targets)
	if err == nil || !strings.Contains(err.Error(), "最多监控") {
		t.Fatalf("expected target limit error, got %v", err)
	}
}

func TestValidateKeywordRuleInputBounds(t *testing.T) {
	err := validateKeywordRuleInput(keywordRuleRequest{
		Name:    "valid",
		Pattern: strings.Repeat("广", maxKeywordRulePattern+1),
	})
	if err == nil || !strings.Contains(err.Error(), "匹配内容") {
		t.Fatalf("expected pattern length error, got %v", err)
	}
}

func TestValidateSettingsInput(t *testing.T) {
	err := validateSettingsInput(map[string]string{
		"default_report_delay": "5",
	})
	if err == nil || !strings.Contains(err.Error(), "default_report_delay") {
		t.Fatalf("expected default_report_delay range error, got %v", err)
	}

	err = validateSettingsInput(map[string]string{
		"feishu_webhook_url": "ftp://example.com/hook",
	})
	if err == nil || !strings.Contains(err.Error(), "feishu_webhook_url") {
		t.Fatalf("expected webhook url scheme error, got %v", err)
	}

	err = validateSettingsInput(map[string]string{
		"webhook_enabled": "true",
		"webhook_type":    "telegram",
	})
	if err != nil {
		t.Fatalf("expected valid settings, got %v", err)
	}
}
