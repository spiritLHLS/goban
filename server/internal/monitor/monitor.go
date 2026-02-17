package monitor

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
)

type MonitorService struct {
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
}

func NewMonitorService() *MonitorService {
	return &MonitorService{
		running:  false,
		stopChan: make(chan struct{}),
	}
}

func (s *MonitorService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Println("[监控服务] 启动")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			log.Println("[监控服务] 停止")
			return
		case <-ticker.C:
			s.checkTasks()
		}
	}
}

func (s *MonitorService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopChan)
		s.running = false
	}
}

func (s *MonitorService) checkTasks() {
	db := database.GetDB()

	var tasks []models.MonitorTask
	if err := db.Where("enabled = ?", true).Preload("User").Find(&tasks).Error; err != nil {
		log.Printf("[监控服务] 查询任务失败: %v", err)
		return
	}

	for _, task := range tasks {
		// 检查是否到达监控间隔
		if time.Since(task.LastCheck) < time.Duration(task.Interval)*time.Second {
			continue
		}

		// 检查用户登录状态
		if !task.User.Login {
			s.addLog(task.ID, "error", "用户未登录，跳过监控")
			continue
		}

		// 执行监控
		go s.monitorTask(&task)
	}
}

func (s *MonitorService) monitorTask(task *models.MonitorTask) {
	db := database.GetDB()

	// 更新最后检查时间
	db.Model(task).Update("last_check", time.Now())

	log.Printf("[监控任务 %d] 开始监控 UP主: %s (UID: %d)", task.ID, task.TargetUname, task.TargetUID)
	s.addLog(task.ID, "info", fmt.Sprintf("开始监控 UP主: %s", task.TargetUname))

	// 创建B站客户端（支持代理）
	var client *bili.BiliClient
	if task.ProxyURL != "" {
		log.Printf("[监控任务 %d] 使用代理: %s", task.ID, task.ProxyURL)
		client = bili.NewBiliClientWithProxy(task.User.Cookies, task.User.UID, task.ProxyURL)
		s.addLog(task.ID, "info", fmt.Sprintf("使用代理: %s", task.ProxyURL))
	} else {
		client = bili.NewBiliClient(task.User.Cookies, task.User.UID)
	}
	
	// 设置重试策略
	if task.MaxRetries > 0 && task.RetryInterval > 0 {
		client.SetRetryPolicy(task.MaxRetries, task.RetryInterval)
		log.Printf("[监控任务 %d] 设置重试策略: 最大重试%d次, 基础间隔%d秒", task.ID, task.MaxRetries, task.RetryInterval)
	}

	// 获取UP主的最新视频
	videos, err := client.GetUserVideos(task.TargetUID, task.VideoCount)
	if err != nil {
		log.Printf("[监控任务 %d] 获取视频列表失败: %v", task.ID, err)
		s.addLog(task.ID, "error", fmt.Sprintf("获取视频列表失败: %v", err))
		return
	}

	log.Printf("[监控任务 %d] 获取到 %d 个视频", task.ID, len(videos))

	// 解析关键字
	keywords := parseKeywords(task.Keywords)
	if len(keywords) == 0 {
		s.addLog(task.ID, "warning", "未设置关键字，跳过监控")
		return
	}

	// 遍历视频
	for _, video := range videos {
		// 获取视频评论
		comments, err := client.GetVideoComments(video.AID, task.CommentCount)
		if err != nil {
			log.Printf("[监控任务 %d] 获取视频 %s 的评论失败: %v", task.ID, video.BVID, err)
			s.addLog(task.ID, "error", fmt.Sprintf("获取视频 %s 评论失败: %v", video.BVID, err))
			continue
		}

		log.Printf("[监控任务 %d] 视频 %s 获取到 %d 条评论", task.ID, video.BVID, len(comments))

		// 检查评论
		for _, comment := range comments {
			matchedKeyword := checkKeywords(comment.Content.Message, keywords)
			if matchedKeyword != "" {
				log.Printf("[监控任务 %d] 发现匹配评论: %s (关键字: %s)", task.ID, comment.Content.Message, matchedKeyword)
				s.addLog(task.ID, "warning", fmt.Sprintf("发现匹配评论，关键字: %s", matchedKeyword))

				// 执行举报
				s.reportComment(task, &video, &comment, matchedKeyword, client)

				// 使用配置的举报间隔，防止频繁举报
				reportDelay := task.ReportDelay
				if reportDelay <= 0 {
					reportDelay = 6 // 默认6秒，确保不超过B站1分钟10次限制
				}
				log.Printf("[监控任务 %d] 等待 %d 秒后继续...", task.ID, reportDelay)
				time.Sleep(time.Duration(reportDelay) * time.Second)
			}
		}
	}

	log.Printf("[监控任务 %d] 监控完成", task.ID)
	s.addLog(task.ID, "info", "监控完成")
}

func (s *MonitorService) reportComment(task *models.MonitorTask, video *bili.VideoInfo, comment *bili.CommentInfo, keyword string, client *bili.BiliClient) {
	db := database.GetDB()

	// 检查是否已经举报过
	var existingReport models.ReportRecord
	result := db.Where("task_id = ? AND comment_id = ?", task.ID, comment.RPID).First(&existingReport)
	if result.Error == nil {
		log.Printf("[监控任务 %d] 评论已举报过，跳过: %d", task.ID, comment.RPID)
		return
	}

	// 执行举报
	err := client.ReportComment(video.AID, comment.RPID, 11) // 11=传谣类

	report := models.ReportRecord{
		TaskID:         task.ID,
		AVID:           video.AID,
		BVID:           video.BVID,
		VideoTitle:     video.Title,
		CommentID:      comment.RPID,
		CommentContent: comment.Content.Message,
		CommentUser:    comment.Member.Uname,
		MatchedKeyword: keyword,
		Reason:         11,
		Success:        err == nil,
	}

	if err != nil {
		report.Message = err.Error()
		log.Printf("[监控任务 %d] 举报失败: %v", task.ID, err)
		s.addLog(task.ID, "error", fmt.Sprintf("举报失败: %v", err))
	} else {
		report.Message = "举报成功"
		log.Printf("[监控任务 %d] 举报成功: 评论ID %d", task.ID, comment.RPID)
		s.addLog(task.ID, "info", fmt.Sprintf("举报成功: 评论ID %d", comment.RPID))
	}

	// 保存举报记录
	if err := db.Create(&report).Error; err != nil {
		log.Printf("[监控任务 %d] 保存举报记录失败: %v", task.ID, err)
	}
}

func (s *MonitorService) addLog(taskID uint, level, message string) {
	db := database.GetDB()
	logEntry := models.MonitorLog{
		TaskID:  taskID,
		Level:   level,
		Message: message,
	}
	db.Create(&logEntry)
}

// parseKeywords 解析关键字字符串
func parseKeywords(keywordsStr string) []string {
	var keywords []string
	parts := strings.Split(keywordsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			keywords = append(keywords, part)
		}
	}
	return keywords
}

// checkKeywords 检查文本是否包含关键字
func checkKeywords(text string, keywords []string) string {
	text = strings.ToLower(text)
	for _, keyword := range keywords {
		keyword = strings.ToLower(keyword)
		if strings.Contains(text, keyword) {
			return keyword
		}
	}
	return ""
}
