package monitor

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
	"github.com/spiritlhl/goban/internal/notify"
	"github.com/spiritlhl/goban/internal/rules"
	"github.com/spiritlhl/goban/internal/secure"
	"github.com/spiritlhl/goban/internal/settings"
	white "github.com/spiritlhl/goban/internal/whitelist"
	"gorm.io/gorm"
)

type MonitorService struct {
	mu            sync.Mutex
	running       bool
	cron          *cron.Cron
	runningTasks  map[uint]bool
	semaphore     chan struct{}
	reportLimiter *ReportLimiter
}

type ReportLimiter struct {
	mu   sync.Mutex
	next time.Time
}

func NewMonitorService() *MonitorService {
	cfg := config.GetConfig()
	return &MonitorService{
		runningTasks:  map[uint]bool{},
		semaphore:     make(chan struct{}, cfg.MaxConcurrentTasks),
		reportLimiter: &ReportLimiter{},
	}
}

func (s *MonitorService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.cron = cron.New(cron.WithSeconds())
	if _, err := s.cron.AddFunc("@every 10s", s.checkTasks); err != nil {
		log.Printf("[监控服务] 注册任务检查失败: %v", err)
	}
	if _, err := s.cron.AddFunc("@every 1m", s.checkCookiesDue); err != nil {
		log.Printf("[监控服务] 注册Cookie检查失败: %v", err)
	}
	s.mu.Unlock()

	log.Println("[监控服务] 启动")
	s.cron.Run()
}

func (s *MonitorService) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cronRunner := s.cron
	s.running = false
	s.mu.Unlock()

	if cronRunner != nil {
		ctx := cronRunner.Stop()
		<-ctx.Done()
	}
	log.Println("[监控服务] 停止")
}

func (s *MonitorService) checkTasks() {
	db := database.GetDB()
	var tasks []models.MonitorTask
	if err := db.Where("enabled = ?", true).Preload("User").Preload("Targets").Find(&tasks).Error; err != nil {
		log.Printf("[监控服务] 查询任务失败: %v", err)
		return
	}

	now := time.Now()
	for _, task := range tasks {
		interval := task.Interval
		if interval <= 0 {
			interval = settings.GetInt("default_interval", 300)
		}
		if !task.LastCheck.IsZero() && now.Sub(task.LastCheck) < time.Duration(interval)*time.Second {
			continue
		}
		if !task.User.Login {
			db.Model(&models.MonitorTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
				"last_check":  now,
				"last_status": "error",
				"last_error":  "用户未登录",
			})
			s.addLog(task.ID, "error", "用户未登录，跳过监控")
			continue
		}
		if !s.markTaskRunning(task.ID) {
			continue
		}

		taskID := task.ID
		go func() {
			s.semaphore <- struct{}{}
			defer func() {
				<-s.semaphore
				s.clearTaskRunning(taskID)
			}()
			s.monitorTask(taskID)
		}()
	}
}

func (s *MonitorService) monitorTask(taskID uint) {
	db := database.GetDB()
	var task models.MonitorTask
	if err := db.Preload("User").Preload("Targets").First(&task, taskID).Error; err != nil {
		log.Printf("[监控任务 %d] 任务不存在: %v", taskID, err)
		return
	}

	startedAt := time.Now()
	db.Model(&task).Updates(map[string]interface{}{
		"last_check":  startedAt,
		"last_status": "running",
		"last_error":  "",
	})

	if len(task.Targets) == 0 {
		s.finishTask(task.ID, "warning", "未配置监控UP主", 0, 0, 0)
		s.addLog(task.ID, "warning", "未配置监控UP主，跳过")
		return
	}

	cookies, err := secure.DecryptString(task.User.Cookies)
	if err != nil {
		s.finishTask(task.ID, "error", "Cookie解密失败: "+err.Error(), 0, 0, 0)
		s.addLog(task.ID, "error", "Cookie解密失败: "+err.Error())
		return
	}

	compiledRules, compileErrors := s.compiledRulesForTask(task)
	for _, compileErr := range compileErrors {
		s.addLog(task.ID, "warning", "规则编译失败: "+compileErr.Error())
	}
	if len(compiledRules) == 0 {
		s.finishTask(task.ID, "warning", "未设置可用关键字规则", 0, 0, 0)
		s.addLog(task.ID, "warning", "未设置可用关键字规则，跳过监控")
		return
	}

	client := newClientForTask(task, cookies)
	whitelistMatcher := s.loadWhitelistMatcher()

	log.Printf("[监控任务 %d] 开始监控 %d 个UP主", task.ID, len(task.Targets))
	s.addLog(task.ID, "info", fmt.Sprintf("开始监控 %d 个UP主", len(task.Targets)))

	var checked int64
	var matched int64
	var reported int64
	var lastErr string

	for _, target := range task.Targets {
		videos, err := client.GetUserVideos(target.UID, task.VideoCount)
		if err != nil {
			lastErr = fmt.Sprintf("获取UP主 %s(%d) 视频失败: %v", target.Uname, target.UID, err)
			log.Printf("[监控任务 %d] %s", task.ID, lastErr)
			s.addLog(task.ID, "error", lastErr)
			continue
		}

		for _, video := range videos {
			comments, err := client.GetVideoComments(video.AID, task.CommentCount)
			if err != nil {
				lastErr = fmt.Sprintf("获取视频 %s 评论失败: %v", video.BVID, err)
				log.Printf("[监控任务 %d] %s", task.ID, lastErr)
				s.addLog(task.ID, "error", lastErr)
				continue
			}

			for _, comment := range comments {
				checked++
				if whitelistMatcher.Contains(comment.Member.Mid, comment.Member.Uname) {
					continue
				}

				match := rules.MatchText(comment.Content.Message, compiledRules)
				if match == nil {
					continue
				}
				matched++
				s.markRuleMatched(match.RuleID)
				s.addLog(task.ID, "warning", fmt.Sprintf("发现匹配评论，规则: %s", match.RuleName))

				if s.reportComment(task, target, video, comment, *match, client) {
					reported++
				}
			}
		}
	}

	status := "success"
	if lastErr != "" {
		status = "warning"
	}
	s.finishTask(task.ID, status, lastErr, checked, matched, reported)
	s.addLog(task.ID, "info", fmt.Sprintf("监控完成：检测 %d 条，匹配 %d 条，成功举报 %d 条", checked, matched, reported))
}

func (s *MonitorService) reportComment(task models.MonitorTask, target models.MonitorTarget, video bili.VideoInfo, comment bili.CommentInfo, match rules.MatchResult, client *bili.BiliClient) bool {
	db := database.GetDB()
	var existingReport models.ReportRecord
	if err := db.Where("task_id = ? AND comment_id = ?", task.ID, comment.RPID).First(&existingReport).Error; err == nil {
		log.Printf("[监控任务 %d] 评论已举报过，跳过: %d", task.ID, comment.RPID)
		return false
	}

	delay := task.ReportDelay
	if delay <= 0 {
		delay = settings.GetInt("default_report_delay", 6)
	}
	s.reportLimiter.Wait(delay)

	err := client.ReportComment(video.AID, comment.RPID, 11)
	report := models.ReportRecord{
		TaskID:          task.ID,
		TargetUID:       target.UID,
		TargetUname:     target.Uname,
		AVID:            video.AID,
		BVID:            video.BVID,
		VideoTitle:      video.Title,
		CommentID:       comment.RPID,
		CommentContent:  comment.Content.Message,
		CommentUser:     comment.Member.Uname,
		CommentUserID:   comment.Member.Mid,
		MatchedKeyword:  match.Matched,
		KeywordRuleName: match.RuleName,
		MatchType:       match.MatchType,
		Reason:          11,
		Success:         err == nil,
	}
	if match.RuleID > 0 {
		report.KeywordRuleID = &match.RuleID
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

	if err := db.Create(&report).Error; err != nil {
		log.Printf("[监控任务 %d] 保存举报记录失败: %v", task.ID, err)
		return false
	}
	if report.Success {
		go func(record models.ReportRecord) {
			if err := notify.NewSender().SendReport(record); err != nil {
				log.Printf("[Webhook] 发送失败: %v", err)
			}
		}(report)
	}
	return report.Success
}

func (s *MonitorService) checkCookiesDue() {
	interval := settings.GetInt("cookie_check_interval", 3600)
	if interval <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(interval) * time.Second)

	db := database.GetDB()
	var users []models.BiliUser
	if err := db.Where("login = ? AND (last_cookie_check IS NULL OR last_cookie_check < ?)", true, cutoff).Limit(10).Find(&users).Error; err != nil {
		log.Printf("[Cookie检查] 查询用户失败: %v", err)
		return
	}

	for _, user := range users {
		cookies, err := secure.DecryptString(user.Cookies)
		now := time.Now()
		updates := map[string]interface{}{
			"last_cookie_check": now,
		}
		if err != nil {
			updates["login"] = false
			updates["cookie_status"] = "invalid"
			updates["cookie_message"] = "Cookie解密失败: " + err.Error()
			db.Model(&user).Updates(updates)
			continue
		}

		valid, err := bili.ValidateCookie(cookies)
		if err != nil {
			updates["cookie_status"] = "unknown"
			updates["cookie_message"] = err.Error()
		} else if valid {
			updates["login"] = true
			updates["cookie_status"] = "valid"
			updates["cookie_message"] = "Cookie有效"
		} else {
			updates["login"] = false
			updates["cookie_status"] = "invalid"
			updates["cookie_message"] = "Cookie已失效"
		}
		db.Model(&user).Updates(updates)
	}
}

func (s *MonitorService) compiledRulesForTask(task models.MonitorTask) ([]rules.CompiledRule, []error) {
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

func (s *MonitorService) loadWhitelistMatcher() white.Matcher {
	var rows []models.WhitelistUser
	if err := database.GetDB().Where("enabled = ?", true).Find(&rows).Error; err != nil {
		log.Printf("[白名单] 加载失败: %v", err)
		return white.NewMatcher(nil)
	}
	return white.NewMatcher(rows)
}

func (s *MonitorService) markRuleMatched(ruleID uint) {
	if ruleID == 0 {
		return
	}
	now := time.Now()
	database.GetDB().Model(&models.KeywordRule{}).Where("id = ?", ruleID).Update("last_matched_at", now)
}

func (s *MonitorService) finishTask(taskID uint, status, lastErr string, checked, matched, reported int64) {
	updates := map[string]interface{}{
		"last_status":      status,
		"last_error":       lastErr,
		"checked_comments": gorm.Expr("checked_comments + ?", checked),
		"matched_comments": gorm.Expr("matched_comments + ?", matched),
		"report_count":     gorm.Expr("report_count + ?", reported),
	}
	if status == "success" || status == "warning" {
		now := time.Now()
		updates["last_success_at"] = now
	}
	database.GetDB().Model(&models.MonitorTask{}).Where("id = ?", taskID).Updates(updates)
}

func (s *MonitorService) addLog(taskID uint, level, message string) {
	db := database.GetDB()
	logEntry := models.MonitorLog{
		TaskID:  taskID,
		Level:   level,
		Message: message,
	}
	if err := db.Create(&logEntry).Error; err != nil {
		log.Printf("[监控服务] 保存日志失败: %v", err)
	}
}

func (s *MonitorService) markTaskRunning(taskID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runningTasks[taskID] {
		return false
	}
	s.runningTasks[taskID] = true
	return true
}

func (s *MonitorService) clearTaskRunning(taskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runningTasks, taskID)
}

func (l *ReportLimiter) Wait(delaySeconds int) {
	if delaySeconds < 6 {
		delaySeconds = 6
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if l.next.After(now) {
		time.Sleep(l.next.Sub(now))
	}
	l.next = time.Now().Add(time.Duration(delaySeconds) * time.Second)
}

func newClientForTask(task models.MonitorTask, cookies string) *bili.BiliClient {
	var client *bili.BiliClient
	if task.ProxyURL != "" {
		client = bili.NewBiliClientWithProxy(cookies, task.User.UID, task.ProxyURL)
	} else {
		client = bili.NewBiliClient(cookies, task.User.UID)
	}

	maxRetries := task.MaxRetries
	if maxRetries < 0 {
		maxRetries = settings.GetInt("default_max_retries", 3)
	}
	retryInterval := task.RetryInterval
	if retryInterval <= 0 {
		retryInterval = settings.GetInt("default_retry_interval", 2)
	}
	client.SetRetryPolicy(maxRetries, retryInterval)
	return client
}
