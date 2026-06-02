package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
)

type whitelistRequest struct {
	UID     int64  `json:"uid"`
	Uname   string `json:"uname"`
	Remark  string `json:"remark"`
	Enabled *bool  `json:"enabled"`
}

func ListWhitelistUsers(c *gin.Context) {
	var rows []models.WhitelistUser
	if err := database.GetDB().Order("created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取白名单失败"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func CreateWhitelistUser(c *gin.Context) {
	var req whitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if req.UID <= 0 && strings.TrimSpace(req.Uname) == "" {
		c.JSON(http.StatusOK, gin.H{"error": "UID 和用户名至少填写一个"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := models.WhitelistUser{
		UID:     req.UID,
		Uname:   strings.TrimSpace(req.Uname),
		Remark:  strings.TrimSpace(req.Remark),
		Enabled: enabled,
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "创建白名单失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建成功", "user": row})
}

func UpdateWhitelistUser(c *gin.Context) {
	var req whitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	db := database.GetDB()
	var row models.WhitelistUser
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "白名单不存在"})
		return
	}
	row.UID = req.UID
	row.Uname = strings.TrimSpace(req.Uname)
	row.Remark = strings.TrimSpace(req.Remark)
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if row.UID <= 0 && row.Uname == "" {
		c.JSON(http.StatusOK, gin.H{"error": "UID 和用户名至少填写一个"})
		return
	}
	if err := db.Save(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "更新白名单失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "user": row})
}

func DeleteWhitelistUser(c *gin.Context) {
	db := database.GetDB()
	var row models.WhitelistUser
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "白名单不存在"})
		return
	}
	if err := db.Delete(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "删除白名单失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
