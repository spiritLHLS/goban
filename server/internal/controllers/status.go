package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
)

func GetMonitorStatus(c *gin.Context) {
	db := database.GetDB()

	var totalTasks int64
	var enabledTasks int64
	var totalUsers int64
	var invalidUsers int64
	var reportSuccess int64
	db.Model(&models.MonitorTask{}).Count(&totalTasks)
	db.Model(&models.MonitorTask{}).Where("enabled = ?", true).Count(&enabledTasks)
	db.Model(&models.BiliUser{}).Count(&totalUsers)
	db.Model(&models.BiliUser{}).Where("login = ? OR cookie_status = ?", false, "invalid").Count(&invalidUsers)
	db.Model(&models.ReportRecord{}).Where("success = ?", true).Count(&reportSuccess)

	var totals struct {
		Checked int64
		Matched int64
		Reports int64
	}
	db.Model(&models.MonitorTask{}).Select("COALESCE(SUM(checked_comments), 0) AS checked, COALESCE(SUM(matched_comments), 0) AS matched, COALESCE(SUM(report_count), 0) AS reports").Scan(&totals)

	var recentTasks []models.MonitorTask
	db.Preload("Targets").Preload("User").Order("last_check DESC").Limit(10).Find(&recentTasks)

	c.JSON(http.StatusOK, gin.H{
		"now":              time.Now(),
		"total_tasks":      totalTasks,
		"enabled_tasks":    enabledTasks,
		"total_users":      totalUsers,
		"invalid_users":    invalidUsers,
		"checked_comments": totals.Checked,
		"matched_comments": totals.Matched,
		"report_count":     totals.Reports,
		"report_success":   reportSuccess,
		"recent_tasks":     recentTasks,
	})
}
