package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
)

// ListMonitorTasks 获取监控任务列表
func ListMonitorTasks(c *gin.Context) {
	db := database.GetDB()
	var tasks []models.MonitorTask
	db.Preload("User").Order("created_at DESC").Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

// CreateMonitorTask 创建监控任务
func CreateMonitorTask(c *gin.Context) {
	var req struct {
		UserID       uint   `json:"user_id" binding:"required"`
		TargetUID    int64  `json:"target_uid" binding:"required"`
		VideoCount   int    `json:"video_count"`
		CommentCount int    `json:"comment_count"`
		Keywords     string `json:"keywords" binding:"required"`
		Interval     int    `json:"interval"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 设置默认值
	if req.VideoCount <= 0 {
		req.VideoCount = 5
	}
	if req.CommentCount <= 0 {
		req.CommentCount = 50
	}
	if req.Interval <= 0 {
		req.Interval = 300
	}

	// 验证用户
	db := database.GetDB()
	var user models.BiliUser
	if err := db.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "用户不存在"})
		return
	}

	if !user.Login {
		c.JSON(http.StatusOK, gin.H{"error": "用户未登录"})
		return
	}

	// 获取目标UP主信息
	client := bili.NewBiliClient(user.Cookies, user.UID)
	targetUname, err := client.GetUPInfo(req.TargetUID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "获取UP主信息失败: " + err.Error()})
		return
	}

	// 创建任务
	task := models.MonitorTask{
		UserID:       req.UserID,
		TargetUID:    req.TargetUID,
		TargetUname:  targetUname,
		VideoCount:   req.VideoCount,
		CommentCount: req.CommentCount,
		Keywords:     req.Keywords,
		Enabled:      true,
		Interval:     req.Interval,
	}

	if err := db.Create(&task).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "创建任务失败"})
		return
	}

	// 预加载关联数据
	db.Preload("User").First(&task, task.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "创建成功",
		"task":    task,
	})
}

// UpdateMonitorTask 更新监控任务
func UpdateMonitorTask(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		VideoCount   int    `json:"video_count"`
		CommentCount int    `json:"comment_count"`
		Keywords     string `json:"keywords"`
		Enabled      *bool  `json:"enabled"`
		Interval     int    `json:"interval"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	db := database.GetDB()
	var task models.MonitorTask
	if err := db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "任务不存在"})
		return
	}

	// 更新字段
	if req.VideoCount > 0 {
		task.VideoCount = req.VideoCount
	}
	if req.CommentCount > 0 {
		task.CommentCount = req.CommentCount
	}
	if req.Keywords != "" {
		task.Keywords = req.Keywords
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	if req.Interval > 0 {
		task.Interval = req.Interval
	}

	if err := db.Save(&task).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "更新失败"})
		return
	}

	db.Preload("User").First(&task, task.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "更新成功",
		"task":    task,
	})
}

// DeleteMonitorTask 删除监控任务
func DeleteMonitorTask(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	var task models.MonitorTask
	if err := db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "任务不存在"})
		return
	}

	// 删除关联的日志和举报记录
	db.Where("task_id = ?", task.ID).Delete(&models.MonitorLog{})
	db.Where("task_id = ?", task.ID).Delete(&models.ReportRecord{})

	// 删除任务
	db.Delete(&task)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetMonitorLogs 获取监控日志
func GetMonitorLogs(c *gin.Context) {
	taskIDStr := c.Query("task_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	db := database.GetDB()
	query := db.Model(&models.MonitorLog{})

	if taskIDStr != "" {
		taskID, _ := strconv.Atoi(taskIDStr)
		query = query.Where("task_id = ?", taskID)
	}

	var total int64
	query.Count(&total)

	var logs []models.MonitorLog
	query.Preload("Task").
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"page_size": pageSize,
		"data":  logs,
	})
}

// GetReportRecords 获取举报记录
func GetReportRecords(c *gin.Context) {
	taskIDStr := c.Query("task_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	db := database.GetDB()
	query := db.Model(&models.ReportRecord{})

	if taskIDStr != "" {
		taskID, _ := strconv.Atoi(taskIDStr)
		query = query.Where("task_id = ?", taskID)
	}

	var total int64
	query.Count(&total)

	var records []models.ReportRecord
	query.Preload("Task").
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&records)

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"page_size": pageSize,
		"data":  records,
	})
}

// TestMonitorTask 测试监控任务（手动触发一次）
func TestMonitorTask(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	var task models.MonitorTask
	if err := db.Preload("User").First(&task, id).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "任务不存在"})
		return
	}

	if !task.User.Login {
		c.JSON(http.StatusOK, gin.H{"error": "用户未登录"})
		return
	}

	// 创建客户端
	client := bili.NewBiliClient(task.User.Cookies, task.User.UID)

	// 获取视频
	videos, err := client.GetUserVideos(task.TargetUID, task.VideoCount)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "获取视频失败: " + err.Error()})
		return
	}

	log.Printf("[测试任务 %d] 获取到 %d 个视频", task.ID, len(videos))

	var result []map[string]interface{}

	for i, video := range videos {
		if i >= 3 { // 最多测试3个视频
			break
		}

		// 获取评论
		comments, err := client.GetVideoComments(video.AID, 20) // 最多20条
		if err != nil {
			log.Printf("[测试任务 %d] 获取视频评论失败: %v", task.ID, err)
			continue
		}

		videoResult := map[string]interface{}{
			"bvid":     video.BVID,
			"title":    video.Title,
			"comments": len(comments),
			"matches":  []string{},
		}

		// 检查关键字
		for _, comment := range comments {
			for _, keyword := range parseKeywords(task.Keywords) {
				if containsKeyword(comment.Content.Message, keyword) {
					matches := videoResult["matches"].([]string)
					matches = append(matches, fmt.Sprintf("评论ID=%d, 内容=%s", comment.RPID, comment.Content.Message))
					videoResult["matches"] = matches
				}
			}
		}

		result = append(result, videoResult)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "测试完成",
		"result":  result,
	})
}

func parseKeywords(keywordsStr string) []string {
	var keywords []string
	for _, k := range splitKeywords(keywordsStr, ",") {
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	return keywords
}

func splitKeywords(s, sep string) []string {
	var result []string
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	var current string
	for _, ch := range s {
		if string(ch) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func containsKeyword(text, keyword string) bool {
	textLower := toLowerCase(text)
	keywordLower := toLowerCase(keyword)
	return contains(textLower, keywordLower)
}

func toLowerCase(s string) string {
	var result string
	for _, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result += string(ch + 32)
		} else {
			result += string(ch)
		}
	}
	return result
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
