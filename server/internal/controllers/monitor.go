package controllers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
	"github.com/spiritlhl/goban/internal/rules"
	"github.com/spiritlhl/goban/internal/secure"
	"github.com/spiritlhl/goban/internal/settings"
	"gorm.io/gorm"
)

type taskRequest struct {
	Name             string            `json:"name"`
	UserID           uint              `json:"user_id"`
	TargetUID        flexibleInt64     `json:"target_uid"`
	TargetUIDs       flexibleInt64List `json:"target_uids"`
	VideoCount       int               `json:"video_count"`
	CommentCount     int               `json:"comment_count"`
	Keywords         string            `json:"keywords"`
	KeywordRuleIDs   []uint            `json:"keyword_rule_ids"`
	Enabled          *bool             `json:"enabled"`
	Interval         int               `json:"interval"`
	ReportDelay      int               `json:"report_delay"`
	DailyReportLimit int               `json:"daily_report_limit"`
	MaxRetries       *int              `json:"max_retries"`
	RetryInterval    int               `json:"retry_interval"`
	ProxyURL         string            `json:"proxy_url"`
}

type taskStatusRequest struct {
	Action  string `json:"action"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type taskProgressItem struct {
	Task            models.MonitorTask  `json:"task"`
	RecentLogs      []models.MonitorLog `json:"recent_logs"`
	ProgressPercent int                 `json:"progress_percent"`
}

const (
	maxTaskTargets      = 20
	maxTaskNameLength   = 120
	maxTaskKeywords     = 4000
	maxTaskVideoCount   = 50
	maxTaskCommentCount = 500
	minTaskInterval     = 30
	maxTaskInterval     = 86400
	minTaskReportDelay  = 30
	maxTaskReportDelay  = 3600
	maxTaskDailyLimit   = 5000
	maxTaskRetries      = 10
	minTaskRetrySeconds = 1
	maxTaskRetrySeconds = 300
)

// ListMonitorTasks 获取监控任务列表
func ListMonitorTasks(c *gin.Context) {
	db := database.GetDB()
	var tasks []models.MonitorTask
	if err := db.Preload("User").Preload("Targets").Order("created_at DESC").Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "获取任务列表失败")
		return
	}
	respondOK(c, tasks)
}

// CreateMonitorTask 创建监控任务
func CreateMonitorTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	targetUIDs := normalizeTargetUIDs(req.TargetUIDs, int64(req.TargetUID))
	if req.UserID == 0 || len(targetUIDs) == 0 {
		respondError(c, http.StatusBadRequest, "请选择账号并填写至少一个UP主UID")
		return
	}
	if err := validateMonitorTaskInput(req, targetUIDs); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	db := database.GetDB()
	var user models.BiliUser
	if err := db.First(&user, req.UserID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if !user.Login {
		respondError(c, http.StatusConflict, "用户未登录")
		return
	}

	cookies, err := secure.DecryptString(user.Cookies)
	if err != nil {
		respondError(c, http.StatusConflict, "Cookie解密失败: "+err.Error())
		return
	}

	client := bili.NewBiliClient(cookies, user.UID)
	targets, err := resolveTargets(c.Request.Context(), client, targetUIDs)
	if err != nil {
		respondError(c, http.StatusBadGateway, err.Error())
		return
	}

	task := models.MonitorTask{
		Name:             strings.TrimSpace(req.Name),
		UserID:           req.UserID,
		Targets:          targets,
		VideoCount:       withDefault(req.VideoCount, "default_video_count", 5),
		CommentCount:     withDefault(req.CommentCount, "default_comment_count", 50),
		Keywords:         strings.TrimSpace(req.Keywords),
		KeywordRuleIDs:   rules.FormatRuleIDs(req.KeywordRuleIDs),
		Enabled:          true,
		Interval:         withDefault(req.Interval, "default_interval", 300),
		ReportDelay:      withDefault(req.ReportDelay, "default_report_delay", 30),
		DailyReportLimit: withDefault(req.DailyReportLimit, "default_daily_report_limit", 100),
		MaxRetries:       withDefaultPtr(req.MaxRetries, "default_max_retries", 3),
		RetryInterval:    withDefault(req.RetryInterval, "default_retry_interval", 2),
		ProxyURL:         strings.TrimSpace(req.ProxyURL),
		LastStatus:       "created",
	}
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	if task.Name == "" {
		task.Name = defaultTaskName(targets)
	}

	if err := validateTaskRules(req.KeywordRuleIDs, task.Keywords); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := db.Create(&task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建任务失败: "+err.Error())
		return
	}

	db.Preload("User").Preload("Targets").First(&task, task.ID)
	respondCreated(c, "创建成功", gin.H{"message": "创建成功", "task": task})
}

// UpdateMonitorTask 更新监控任务
func UpdateMonitorTask(c *gin.Context) {
	id := c.Param("id")
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	db := database.GetDB()
	var task models.MonitorTask
	if err := db.Preload("User").Preload("Targets").First(&task, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}

	var targets []models.MonitorTarget
	targetUIDs := normalizeTargetUIDs(req.TargetUIDs, int64(req.TargetUID))
	if err := validateMonitorTaskInput(req, targetUIDs); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(targetUIDs) > 0 {
		cookies, err := secure.DecryptString(task.User.Cookies)
		if err != nil {
			respondError(c, http.StatusConflict, "Cookie解密失败: "+err.Error())
			return
		}
		targets, err = resolveTargets(c.Request.Context(), bili.NewBiliClient(cookies, task.User.UID), targetUIDs)
		if err != nil {
			respondError(c, http.StatusBadGateway, err.Error())
			return
		}
	}

	if len(req.KeywordRuleIDs) > 0 || strings.TrimSpace(req.Keywords) != "" {
		if err := validateTaskRules(req.KeywordRuleIDs, req.Keywords); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if strings.TrimSpace(req.Name) != "" {
			task.Name = strings.TrimSpace(req.Name)
		}
		if req.VideoCount > 0 {
			task.VideoCount = req.VideoCount
		}
		if req.CommentCount > 0 {
			task.CommentCount = req.CommentCount
		}
		if req.Keywords != "" {
			task.Keywords = strings.TrimSpace(req.Keywords)
		}
		if req.KeywordRuleIDs != nil {
			task.KeywordRuleIDs = rules.FormatRuleIDs(req.KeywordRuleIDs)
		}
		if req.Enabled != nil {
			task.Enabled = *req.Enabled
		}
		if req.Interval > 0 {
			task.Interval = req.Interval
		}
		if req.ReportDelay > 0 {
			task.ReportDelay = req.ReportDelay
		}
		if req.DailyReportLimit > 0 {
			task.DailyReportLimit = req.DailyReportLimit
		}
		if req.MaxRetries != nil {
			task.MaxRetries = *req.MaxRetries
		}
		if req.RetryInterval > 0 {
			task.RetryInterval = req.RetryInterval
		}
		task.ProxyURL = strings.TrimSpace(req.ProxyURL)

		if len(targets) > 0 {
			if err := tx.Where("task_id = ?", task.ID).Delete(&models.MonitorTarget{}).Error; err != nil {
				return err
			}
			for i := range targets {
				targets[i].TaskID = task.ID
			}
			if err := tx.Create(&targets).Error; err != nil {
				return err
			}
			if task.Name == "" || task.Name == defaultTaskName(task.Targets) {
				task.Name = defaultTaskName(targets)
			}
		}
		return tx.Save(&task).Error
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}

	db.Preload("User").Preload("Targets").First(&task, task.ID)
	respondCreated(c, "更新成功", gin.H{"message": "更新成功", "task": task})
}

// DeleteMonitorTask 删除监控任务
func DeleteMonitorTask(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	var task models.MonitorTask
	if err := db.First(&task, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}
	if !requireDeleteConfirmation(c, task.Name, strconv.FormatUint(uint64(task.ID), 10)) {
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.MonitorTarget{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.MonitorLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id = ?", task.ID).Delete(&models.ReportRecord{}).Error; err != nil {
			return err
		}
		return tx.Delete(&task).Error
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	var remaining int64
	if err := db.Model(&models.MonitorTask{}).Where("id = ?", task.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		respondError(c, http.StatusInternalServerError, "删除结果校验失败")
		return
	}

	respondCreated(c, "删除成功", gin.H{"message": "删除成功", "deleted_id": task.ID})
}

func ListTaskProgress(c *gin.Context) {
	db := database.GetDB()
	var tasks []models.MonitorTask
	if err := db.Preload("User").Preload("Targets").Order("updated_at DESC").Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "获取任务进度失败")
		return
	}
	respondOK(c, buildTaskProgressItems(tasks))
}

func GetTaskProgress(c *gin.Context) {
	db := database.GetDB()
	var task models.MonitorTask
	if err := db.Preload("User").Preload("Targets").First(&task, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}
	items := buildTaskProgressItems([]models.MonitorTask{task})
	if len(items) == 0 {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}
	respondOK(c, items[0])
}

func UpdateTaskStatus(c *gin.Context) {
	var req taskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	db := database.GetDB()
	var task models.MonitorTask
	if err := db.First(&task, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}

	now := time.Now()
	updates := map[string]interface{}{}
	switch req.Action {
	case "enable":
		updates["enabled"] = true
		updates["last_status"] = "waiting"
		updates["last_error"] = ""
		updates["backoff_until"] = nil
		updates["backoff_reason"] = ""
		updates["backoff_attempt"] = 0
		updates["next_run_at"] = now
		updates["progress_message"] = "已手动启用，等待调度"
	case "disable":
		updates["enabled"] = false
		updates["last_status"] = "paused"
		updates["progress_message"] = "已手动暂停"
	case "resume", "retry_now":
		updates["enabled"] = true
		updates["last_check"] = time.Time{}
		updates["last_status"] = "waiting"
		updates["last_error"] = ""
		updates["backoff_until"] = nil
		updates["backoff_reason"] = ""
		updates["backoff_attempt"] = 0
		updates["next_run_at"] = now
		updates["progress_message"] = "已清除退避并等待立即调度"
	case "reset_stats":
		updates["checked_comments"] = 0
		updates["matched_comments"] = 0
		updates["report_count"] = 0
		updates["progress_total"] = 0
		updates["progress_done"] = 0
		updates["progress_message"] = "统计已重置"
	case "set_status":
		status := strings.TrimSpace(req.Status)
		if status == "" {
			respondError(c, http.StatusBadRequest, "缺少 status")
			return
		}
		if !validManualTaskStatus(status) {
			respondError(c, http.StatusBadRequest, "不支持的任务状态")
			return
		}
		updates["last_status"] = status
		updates["last_error"] = strings.TrimSpace(req.Message)
		updates["progress_message"] = strings.TrimSpace(req.Message)
	default:
		respondError(c, http.StatusBadRequest, "不支持的状态动作")
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.MonitorTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
			return err
		}
		if req.Action == "reset_stats" {
			if err := tx.Model(&models.MonitorTarget{}).Where("task_id = ?", task.ID).Updates(map[string]interface{}{
				"checked_comments": 0,
				"matched_comments": 0,
				"report_count":     0,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "更新任务状态失败: "+err.Error())
		return
	}

	db.Preload("User").Preload("Targets").First(&task, task.ID)
	respondCreated(c, "状态已更新", gin.H{"message": "状态已更新", "task": task})
}

// GetMonitorLogs 获取监控日志
func GetMonitorLogs(c *gin.Context) {
	page, pageSize := pagination(c)
	db := database.GetDB()
	query := db.Model(&models.MonitorLog{})

	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if level := c.Query("level"); level != "" {
		query = query.Where("level = ?", level)
	}

	var total int64
	query.Count(&total)

	var logs []models.MonitorLog
	query.Preload("Task.User").
		Preload("Task.Targets").
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&logs)

	c.JSON(http.StatusOK, gin.H{"total": total, "page": page, "page_size": pageSize, "data": logs})
}

// GetReportRecords 获取举报记录
func GetReportRecords(c *gin.Context) {
	page, pageSize := pagination(c)
	query := filteredReportQuery(c)

	var total int64
	query.Count(&total)

	var records []models.ReportRecord
	query.Preload("Task.User").
		Preload("Task.Targets").
		Order("created_at DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&records)

	c.JSON(http.StatusOK, gin.H{"total": total, "page": page, "page_size": pageSize, "data": records})
}

// ExportReportRecords 导出举报记录CSV
func ExportReportRecords(c *gin.Context) {
	query := filteredReportQuery(c)
	var records []models.ReportRecord
	if err := query.Order("created_at DESC").Limit(10000).Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="goban-report-records.csv"`)
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"时间", "任务ID", "UP主UID", "UP主", "视频BVID", "视频标题", "评论ID", "评论用户ID", "评论用户", "匹配规则", "匹配内容", "状态", "消息"})
	for _, record := range records {
		status := "失败"
		if record.Success {
			status = "成功"
		}
		_ = writer.Write([]string{
			record.CreatedAt.Format(time.RFC3339),
			strconv.FormatUint(uint64(record.TaskID), 10),
			strconv.FormatInt(record.TargetUID, 10),
			record.TargetUname,
			record.BVID,
			record.VideoTitle,
			strconv.FormatInt(record.CommentID, 10),
			strconv.FormatInt(record.CommentUserID, 10),
			record.CommentUser,
			record.KeywordRuleName,
			record.MatchedKeyword,
			status,
			record.Message,
		})
	}
	writer.Flush()
}

// TestMonitorTask 测试监控任务（手动触发一次）
func TestMonitorTask(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	var task models.MonitorTask
	if err := db.Preload("User").Preload("Targets").First(&task, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "任务不存在")
		return
	}
	if !task.User.Login {
		respondError(c, http.StatusConflict, "用户未登录")
		return
	}

	cookies, err := secure.DecryptString(task.User.Cookies)
	if err != nil {
		respondError(c, http.StatusConflict, "Cookie解密失败: "+err.Error())
		return
	}

	compiledRules, compileErrors := compiledRulesForTask(task)
	if len(compiledRules) == 0 {
		respondError(c, http.StatusBadRequest, "未设置可用关键字规则")
		return
	}

	client := bili.NewBiliClient(cookies, task.User.UID)
	client.SetRetryPolicy(task.MaxRetries, task.RetryInterval)

	var result []map[string]interface{}
	ctx := c.Request.Context()
	for _, target := range task.Targets {
		if ctx.Err() != nil {
			respondError(c, http.StatusRequestTimeout, "测试已取消")
			return
		}
		videos, err := client.GetUserVideosContext(ctx, target.UID, minInt(task.VideoCount, 3))
		if err != nil {
			log.Printf("[测试任务 %d] 获取UP主 %d 视频失败: %v", task.ID, target.UID, err)
			result = append(result, map[string]interface{}{
				"target_uid":   target.UID,
				"target_uname": target.Uname,
				"error":        err.Error(),
			})
			continue
		}

		for _, video := range videos {
			if ctx.Err() != nil {
				respondError(c, http.StatusRequestTimeout, "测试已取消")
				return
			}
			comments, err := client.GetVideoCommentsContext(ctx, video.AID, minInt(task.CommentCount, 20))
			if err != nil {
				log.Printf("[测试任务 %d] 获取视频评论失败: %v", task.ID, err)
				continue
			}

			videoResult := map[string]interface{}{
				"target_uid":   target.UID,
				"target_uname": target.Uname,
				"bvid":         video.BVID,
				"title":        video.Title,
				"comments":     len(comments),
				"matches":      []rules.MatchResult{},
			}

			matches := []rules.MatchResult{}
			for _, comment := range comments {
				matches = append(matches, rules.MatchAll(comment.Content.Message, compiledRules)...)
			}
			videoResult["matches"] = matches
			result = append(result, videoResult)
		}
	}

	respondCreated(c, "测试完成", gin.H{"message": "测试完成", "result": result, "compile_errors": compileErrors})
}

func filteredReportQuery(c *gin.Context) *gorm.DB {
	db := database.GetDB()
	query := db.Model(&models.ReportRecord{})

	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if targetUID := c.Query("target_uid"); targetUID != "" {
		query = query.Where("target_uid = ?", targetUID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("matched_keyword LIKE ? OR keyword_rule_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if success := c.Query("success"); success != "" {
		query = query.Where("success = ?", success == "true" || success == "1")
	}
	if start := parseTimeQuery(c.Query("start_time")); start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if end := parseTimeQuery(c.Query("end_time")); end != nil {
		query = query.Where("created_at <= ?", *end)
	}

	return query
}

func buildTaskProgressItems(tasks []models.MonitorTask) []taskProgressItem {
	items := make([]taskProgressItem, 0, len(tasks))
	if len(tasks) == 0 {
		return items
	}
	taskIDs := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}

	var logs []models.MonitorLog
	if err := database.GetDB().
		Where("task_id IN ?", taskIDs).
		Order("created_at DESC").
		Limit(len(taskIDs) * 5).
		Find(&logs).Error; err != nil {
		log.Printf("[任务进度] 查询最近日志失败: %v", err)
	}
	logsByTask := map[uint][]models.MonitorLog{}
	for _, row := range logs {
		if len(logsByTask[row.TaskID]) >= 3 {
			continue
		}
		logsByTask[row.TaskID] = append(logsByTask[row.TaskID], row)
	}

	for _, task := range tasks {
		items = append(items, taskProgressItem{
			Task:            task,
			RecentLogs:      logsByTask[task.ID],
			ProgressPercent: taskProgressPercent(task),
		})
	}
	return items
}

func taskProgressPercent(task models.MonitorTask) int {
	if task.ProgressTotal <= 0 {
		if task.LastStatus == "success" {
			return 100
		}
		return 0
	}
	value := int(task.ProgressDone * 100 / task.ProgressTotal)
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func validManualTaskStatus(status string) bool {
	switch status {
	case "created", "waiting", "running", "success", "warning", "error", "paused", "backoff":
		return true
	default:
		return false
	}
}

func validateMonitorTaskInput(req taskRequest, targetUIDs []int64) error {
	if len(targetUIDs) > maxTaskTargets {
		return fmt.Errorf("单个任务最多监控 %d 个UP主", maxTaskTargets)
	}
	if runeLen(strings.TrimSpace(req.Name)) > maxTaskNameLength {
		return fmt.Errorf("任务名称不能超过 %d 个字符", maxTaskNameLength)
	}
	if runeLen(strings.TrimSpace(req.Keywords)) > maxTaskKeywords {
		return fmt.Errorf("临时关键字不能超过 %d 个字符", maxTaskKeywords)
	}
	if err := validateOptionalInt("视频数", req.VideoCount, 1, maxTaskVideoCount); err != nil {
		return err
	}
	if err := validateOptionalInt("评论数", req.CommentCount, 1, maxTaskCommentCount); err != nil {
		return err
	}
	if err := validateOptionalInt("检查间隔", req.Interval, minTaskInterval, maxTaskInterval); err != nil {
		return err
	}
	if err := validateOptionalInt("举报间隔", req.ReportDelay, minTaskReportDelay, maxTaskReportDelay); err != nil {
		return err
	}
	if err := validateOptionalInt("每日举报上限", req.DailyReportLimit, 1, maxTaskDailyLimit); err != nil {
		return err
	}
	if req.MaxRetries != nil {
		if *req.MaxRetries < 0 || *req.MaxRetries > maxTaskRetries {
			return fmt.Errorf("最大重试次数必须在 0-%d 之间", maxTaskRetries)
		}
	}
	if err := validateOptionalInt("重试间隔", req.RetryInterval, minTaskRetrySeconds, maxTaskRetrySeconds); err != nil {
		return err
	}
	return validateProxyURL(req.ProxyURL)
}

func validateOptionalInt(label string, value, minValue, maxValue int) error {
	if value == 0 {
		return nil
	}
	if value < minValue || value > maxValue {
		return fmt.Errorf("%s必须在 %d-%d 之间", label, minValue, maxValue)
	}
	return nil
}

func validateProxyURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("代理地址格式无效")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5":
		return nil
	default:
		return fmt.Errorf("代理地址仅支持 http、https 或 socks5")
	}
}

func validateTaskRules(ruleIDs []uint, adHocKeywords string) error {
	if len(ruleIDs) == 0 && len(rules.ParseAdHocKeywords(adHocKeywords)) > 0 {
		return nil
	}
	if len(ruleIDs) == 0 {
		count := int64(0)
		if err := database.GetDB().Model(&models.KeywordRule{}).Where("enabled = ?", true).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("请先创建至少一个启用的关键字规则，或填写临时关键字")
		}
		return nil
	}

	var count int64
	if err := database.GetDB().Model(&models.KeywordRule{}).Where("id IN ? AND enabled = ?", ruleIDs, true).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ruleIDs)) {
		return fmt.Errorf("存在无效或已停用的关键字规则")
	}
	return nil
}

func runeLen(value string) int {
	return len([]rune(value))
}

func compiledRulesForTask(task models.MonitorTask) ([]rules.CompiledRule, []error) {
	db := database.GetDB()
	var rows []models.KeywordRule
	ruleIDs := rules.ParseRuleIDs(task.KeywordRuleIDs)
	query := db.Where("enabled = ?", true)
	if len(ruleIDs) > 0 {
		query = query.Where("id IN ?", ruleIDs)
	}
	if err := query.Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, []error{err}
	}

	return rules.CompileMany(rows, task.Keywords)
}

func resolveTargets(ctx context.Context, client *bili.BiliClient, targetUIDs []int64) ([]models.MonitorTarget, error) {
	targets := make([]models.MonitorTarget, 0, len(targetUIDs))
	for _, uid := range targetUIDs {
		uname, err := client.GetUPInfoContext(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("获取UP主 %d 信息失败: %w", uid, err)
		}
		targets = append(targets, models.MonitorTarget{UID: uid, Uname: uname})
	}
	return targets, nil
}

func normalizeTargetUIDs(list flexibleInt64List, single int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0)
	for _, uid := range list {
		if uid > 0 && !seen[uid] {
			result = append(result, uid)
			seen[uid] = true
		}
	}
	if single > 0 && !seen[single] {
		result = append(result, single)
	}
	return result
}

func defaultTaskName(targets []models.MonitorTarget) string {
	if len(targets) == 0 {
		return "未命名任务"
	}
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Uname != "" {
			names = append(names, target.Uname)
		} else {
			names = append(names, strconv.FormatInt(target.UID, 10))
		}
	}
	return strings.Join(names, ", ")
}

func withDefault(value int, key string, fallback int) int {
	if value > 0 {
		return value
	}
	return settings.GetInt(key, fallback)
}

func withDefaultPtr(value *int, key string, fallback int) int {
	if value != nil {
		return *value
	}
	return settings.GetInt(key, fallback)
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func parseTimeQuery(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type flexibleInt64 int64

func (v *flexibleInt64) UnmarshalJSON(data []byte) error {
	var number int64
	if err := json.Unmarshal(data, &number); err == nil {
		*v = flexibleInt64(number)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*v = flexibleInt64(parsed)
	return nil
}

type flexibleInt64List []int64

func (v *flexibleInt64List) UnmarshalJSON(data []byte) error {
	var numbers []int64
	if err := json.Unmarshal(data, &numbers); err == nil {
		*v = numbers
		return nil
	}

	var texts []string
	if err := json.Unmarshal(data, &texts); err == nil {
		result := make([]int64, 0, len(texts))
		for _, text := range texts {
			parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
			if err != nil {
				return err
			}
			result = append(result, parsed)
		}
		*v = result
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return nil
	}
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';' || r == ' '
	})
	result := make([]int64, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		parsed, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return err
		}
		result = append(result, parsed)
	}
	*v = result
	return nil
}
