package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/settings"
)

func GetSettings(c *gin.Context) {
	values, err := settings.All()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败"})
		return
	}
	cfg := config.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"settings": values,
		"runtime": gin.H{
			"port":                 cfg.Port,
			"db_path":              cfg.DBPath,
			"max_concurrent_tasks": cfg.MaxConcurrentTasks,
		},
	})
}

func UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := settings.Save(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	values, _ := settings.All()
	c.JSON(http.StatusOK, gin.H{"message": "保存成功", "settings": values})
}
