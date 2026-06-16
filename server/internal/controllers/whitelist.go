package controllers

import (
	"net/http"
	"strconv"
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
		respondError(c, http.StatusInternalServerError, "获取白名单失败")
		return
	}
	respondOK(c, rows)
}

func CreateWhitelistUser(c *gin.Context) {
	var req whitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if req.UID <= 0 && strings.TrimSpace(req.Uname) == "" {
		respondError(c, http.StatusBadRequest, "UID 和用户名至少填写一个")
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
		respondError(c, http.StatusInternalServerError, "创建白名单失败: "+err.Error())
		return
	}
	respondCreated(c, "创建成功", gin.H{"message": "创建成功", "user": row})
}

func UpdateWhitelistUser(c *gin.Context) {
	var req whitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	db := database.GetDB()
	var row models.WhitelistUser
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "白名单不存在")
		return
	}
	row.UID = req.UID
	row.Uname = strings.TrimSpace(req.Uname)
	row.Remark = strings.TrimSpace(req.Remark)
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if row.UID <= 0 && row.Uname == "" {
		respondError(c, http.StatusBadRequest, "UID 和用户名至少填写一个")
		return
	}
	if err := db.Save(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新白名单失败: "+err.Error())
		return
	}
	respondCreated(c, "更新成功", gin.H{"message": "更新成功", "user": row})
}

func DeleteWhitelistUser(c *gin.Context) {
	db := database.GetDB()
	var row models.WhitelistUser
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "白名单不存在")
		return
	}
	if !requireDeleteConfirmation(c, row.Uname, strconv.FormatInt(row.UID, 10), strconv.FormatUint(uint64(row.ID), 10)) {
		return
	}
	if err := db.Delete(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除白名单失败: "+err.Error())
		return
	}
	var remaining int64
	if err := db.Model(&models.WhitelistUser{}).Where("id = ?", row.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		respondError(c, http.StatusInternalServerError, "删除结果校验失败")
		return
	}
	respondCreated(c, "删除成功", gin.H{"message": "删除成功", "deleted_id": row.ID})
}
