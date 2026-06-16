package monitor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

type ReportLimiter struct {
	mu   sync.Mutex
	next time.Time
}

type reportOutcome struct {
	success  bool
	stopTask bool
	status   string
	message  string
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
	s.ctx, s.cancel = context.WithCancel(context.Background())
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
	cancel := s.cancel
	s.running = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if cronRunner != nil {
		ctx := cronRunner.Stop()
		<-ctx.Done()
	}
	s.wg.Wait()
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
		nextRunAt := now
		if !task.LastCheck.IsZero() {
			nextRunAt = task.LastCheck.Add(time.Duration(interval) * time.Second)
		}
		if task.BackoffUntil != nil && task.BackoffUntil.After(now) {
			s.updateTaskQueueState(task.ID, "backoff", task.BackoffReason, *task.BackoffUntil)
			s.addLog(task.ID, "warning", fmt.Sprintf("任务处于退避队列，等待至 %s 后自动恢复", task.BackoffUntil.Format(time.RFC3339)))
			continue
		}
		if !task.LastCheck.IsZero() && now.Sub(task.LastCheck) < time.Duration(interval)*time.Second {
			s.updateNextRun(task.ID, nextRunAt)
			continue
		}
		if !task.User.Login {
			db.Model(&models.MonitorTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
				"last_check":       now,
				"last_status":      "error",
				"last_error":       "用户未登录",
				"next_run_at":      now.Add(time.Duration(interval) * time.Second),
				"progress_message": "用户未登录，等待账号恢复",
			})
			s.addLog(task.ID, "error", "用户未登录，跳过监控")
			continue
		}

		taskID := task.ID
		ctx, ok := s.beginTask(taskID)
		if !ok {
			continue
		}
		go func() {
			defer s.wg.Done()
			select {
			case s.semaphore <- struct{}{}:
			case <-ctx.Done():
				s.clearTaskRunning(taskID)
				return
			}
			defer func() {
				<-s.semaphore
				s.clearTaskRunning(taskID)
			}()
			s.monitorTask(ctx, taskID)
		}()
	}
}

func (s *MonitorService) monitorTask(ctx context.Context, taskID uint) {
	db := database.GetDB()
	var task models.MonitorTask
	if err := db.Preload("User").Preload("Targets").First(&task, taskID).Error; err != nil {
		log.Printf("[监控任务 %d] 任务不存在: %v", taskID, err)
		return
	}

	startedAt := time.Now()
	db.Model(&task).Updates(map[string]interface{}{
		"last_check":       startedAt,
		"last_status":      "running",
		"last_error":       "",
		"backoff_until":    nil,
		"backoff_reason":   "",
		"next_run_at":      nil,
		"progress_total":   int64(len(task.Targets)),
		"progress_done":    int64(0),
		"progress_message": "任务启动",
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
		s.updateTaskProgress(task.ID, int64(len(task.Targets)), checkedTargets(task.Targets, target.ID), fmt.Sprintf("正在处理UP主 %s(%d)", target.Uname, target.UID))
		var targetChecked int64
		var targetMatched int64
		var targetReported int64
		var targetErr string
		s.updateTargetStatus(target.ID, "running", "", 0, 0, 0)

		if ctx.Err() != nil {
			s.updateTargetStatus(target.ID, "warning", "任务已取消", targetChecked, targetMatched, targetReported)
			s.finishTask(task.ID, "warning", "任务已取消", checked, matched, reported)
			s.addLog(task.ID, "warning", "任务已取消")
			return
		}
		videos, err := client.GetUserVideosContext(ctx, target.UID, task.VideoCount)
		if err != nil {
			lastErr = fmt.Sprintf("获取UP主 %s(%d) 视频失败: %v", target.Uname, target.UID, err)
			targetErr = lastErr
			log.Printf("[监控任务 %d] %s", task.ID, lastErr)
			s.addLog(task.ID, "error", lastErr)
			s.updateTargetStatus(target.ID, "warning", targetErr, targetChecked, targetMatched, targetReported)
			continue
		}

		for videoIndex, video := range videos {
			if ctx.Err() != nil {
				s.updateTargetStatus(target.ID, "warning", "任务已取消", targetChecked, targetMatched, targetReported)
				s.finishTask(task.ID, "warning", "任务已取消", checked, matched, reported)
				s.addLog(task.ID, "warning", "任务已取消")
				return
			}
			s.updateTaskProgress(task.ID, int64(len(task.Targets)), checkedTargets(task.Targets, target.ID), fmt.Sprintf("UP主 %s：读取视频 %d/%d 评论", target.Uname, videoIndex+1, len(videos)))
			comments, err := client.GetVideoCommentsContext(ctx, video.AID, task.CommentCount)
			if err != nil {
				lastErr = fmt.Sprintf("获取视频 %s 评论失败: %v", video.BVID, err)
				targetErr = lastErr
				log.Printf("[监控任务 %d] %s", task.ID, lastErr)
				s.addLog(task.ID, "error", lastErr)
				continue
			}

			for _, comment := range comments {
				if ctx.Err() != nil {
					s.updateTargetStatus(target.ID, "warning", "任务已取消", targetChecked, targetMatched, targetReported)
					s.finishTask(task.ID, "warning", "任务已取消", checked, matched, reported)
					s.addLog(task.ID, "warning", "任务已取消")
					return
				}
				checked++
				targetChecked++
				if whitelistMatcher.Contains(comment.Member.Mid, comment.Member.Uname) {
					continue
				}

				match := rules.MatchText(comment.Content.Message, compiledRules)
				if match == nil {
					continue
				}
				matched++
				targetMatched++
				s.markRuleMatched(match.RuleID)
				s.addLog(task.ID, "warning", fmt.Sprintf("发现匹配评论，规则: %s", match.RuleName))

				outcome := s.reportComment(ctx, task, target, video, comment, *match, client)
				if outcome.success {
					reported++
					targetReported++
				}
				if outcome.stopTask {
					targetErr = outcome.message
					status := "error"
					if outcome.status != "" {
						status = outcome.status
					}
					s.updateTargetStatus(target.ID, status, targetErr, targetChecked, targetMatched, targetReported)
					s.finishTask(task.ID, status, targetErr, checked, matched, reported)
					s.addLog(task.ID, "error", targetErr)
					return
				}
			}
		}
		targetStatus := "success"
		if targetErr != "" {
			targetStatus = "warning"
		}
		s.updateTargetStatus(target.ID, targetStatus, targetErr, targetChecked, targetMatched, targetReported)
		s.updateTaskProgress(task.ID, int64(len(task.Targets)), checkedTargets(task.Targets, target.ID)+1, fmt.Sprintf("UP主 %s 处理完成", target.Uname))
	}

	status := "success"
	if lastErr != "" {
		status = "warning"
	}
	s.finishTask(task.ID, status, lastErr, checked, matched, reported)
	if lastErr != "" {
		s.notifyMonitorError(task, lastErr)
	}
	s.addLog(task.ID, "info", fmt.Sprintf("监控完成：检测 %d 条，匹配 %d 条，成功举报 %d 条", checked, matched, reported))
}

func (s *MonitorService) reportComment(ctx context.Context, task models.MonitorTask, target models.MonitorTarget, video bili.VideoInfo, comment bili.CommentInfo, match rules.MatchResult, client *bili.BiliClient) reportOutcome {
	db := database.GetDB()
	var existingReport models.ReportRecord
	if err := db.Where("task_id = ? AND comment_id = ?", task.ID, comment.RPID).First(&existingReport).Error; err == nil {
		log.Printf("[监控任务 %d] 评论已举报过，跳过: %d", task.ID, comment.RPID)
		return reportOutcome{}
	}
	if reached, count, limit := s.accountDailyReportLimitReached(task); reached {
		message := fmt.Sprintf("账号今日成功举报已达上限 %d/%d，跳过评论 %d", count, limit, comment.RPID)
		log.Printf("[监控任务 %d] %s", task.ID, message)
		s.addLog(task.ID, "warning", message)
		return reportOutcome{}
	}

	delay := task.ReportDelay
	if delay <= 0 {
		delay = settings.GetInt("default_report_delay", 30)
	}
	if !s.reportLimiter.Wait(ctx, delay) {
		message := fmt.Sprintf("举报评论 %d 前任务已取消", comment.RPID)
		s.addLog(task.ID, "warning", message)
		return reportOutcome{stopTask: true, status: "warning", message: message}
	}

	err := client.ReportCommentContext(ctx, video.AID, comment.RPID, 11)
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

	outcome := reportOutcome{}
	if err != nil {
		report.Message = err.Error()
		log.Printf("[监控任务 %d] 举报失败: %v", task.ID, err)
		s.addLog(task.ID, "error", fmt.Sprintf("举报失败: %v", err))
		if bili.IsRiskControlError(err) {
			message := s.scheduleBackoff(task, err.Error())
			outcome.stopTask = true
			outcome.status = "backoff"
			outcome.message = message
			s.addLog(task.ID, "error", message)
			s.notifyMonitorError(task, message)
		}
	} else {
		report.Message = "举报成功"
		log.Printf("[监控任务 %d] 举报成功: 评论ID %d", task.ID, comment.RPID)
		s.addLog(task.ID, "info", fmt.Sprintf("举报成功: 评论ID %d", comment.RPID))
	}

	if err := db.Create(&report).Error; err != nil {
		log.Printf("[监控任务 %d] 保存举报记录失败: %v", task.ID, err)
		return reportOutcome{stopTask: outcome.stopTask, message: outcome.message}
	}
	if report.Success {
		outcome.success = true
		go func(record models.ReportRecord) {
			if err := notify.NewSender().SendReport(record); err != nil {
				log.Printf("[Webhook] 发送失败: %v", err)
			}
		}(report)
	}
	return outcome
}

func (s *MonitorService) accountDailyReportLimitReached(task models.MonitorTask) (bool, int64, int) {
	limit := task.DailyReportLimit
	if limit <= 0 {
		limit = settings.GetInt("default_daily_report_limit", 100)
	}
	if limit <= 0 {
		return false, 0, limit
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int64
	if err := database.GetDB().
		Model(&models.ReportRecord{}).
		Joins("JOIN monitor_tasks ON monitor_tasks.id = report_records.task_id").
		Where("monitor_tasks.user_id = ? AND report_records.success = ? AND report_records.created_at >= ?", task.UserID, true, startOfDay).
		Count(&count).Error; err != nil {
		log.Printf("[监控任务 %d] 查询每日举报上限失败: %v", task.ID, err)
		return false, 0, limit
	}
	return count >= int64(limit), count, limit
}

func (s *MonitorService) scheduleBackoff(task models.MonitorTask, reason string) string {
	db := database.GetDB()
	var latest models.MonitorTask
	attempt := task.BackoffAttempt + 1
	if err := db.Select("id", "backoff_attempt").First(&latest, task.ID).Error; err == nil {
		attempt = latest.BackoffAttempt + 1
	}
	delay := backoffDelay(attempt)
	until := time.Now().Add(delay)
	message := fmt.Sprintf("触发B站风控，已加入退避队列，%s 后自动重试: %s", delay.Round(time.Second), reason)
	if err := db.Model(&models.MonitorTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"last_status":      "backoff",
		"last_error":       message,
		"backoff_until":    until,
		"backoff_reason":   reason,
		"backoff_attempt":  attempt,
		"next_run_at":      until,
		"progress_message": message,
	}).Error; err != nil {
		log.Printf("[监控任务 %d] 设置退避队列失败: %v", task.ID, err)
	}
	return message
}

func backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := settings.GetInt("risk_backoff_base_seconds", 1800)
	if base < 60 {
		base = 60
	}
	maxDelay := settings.GetInt("risk_backoff_max_seconds", 86400)
	if maxDelay < base {
		maxDelay = base
	}
	delay := time.Duration(base) * time.Second
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= time.Duration(maxDelay)*time.Second {
			return time.Duration(maxDelay) * time.Second
		}
	}
	if delay > time.Duration(maxDelay)*time.Second {
		return time.Duration(maxDelay) * time.Second
	}
	return delay
}

func (s *MonitorService) checkCookiesDue() {
	interval := settings.GetInt("cookie_check_interval", 3600)
	if interval <= 0 {
		return
	}
	ctx := s.context()
	if ctx.Err() != nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(interval) * time.Second)
	refreshInterval := settings.GetInt("cookie_refresh_interval", 21600)
	if refreshInterval <= 0 {
		refreshInterval = 21600
	}
	refreshBefore := time.Now().Add(time.Duration(refreshInterval) * time.Second)

	db := database.GetDB()
	var users []models.BiliUser
	if err := db.Where("login = ? AND ((last_cookie_check IS NULL OR last_cookie_check < ?) OR expire_time < ?)", true, cutoff, refreshBefore).Limit(10).Find(&users).Error; err != nil {
		log.Printf("[Cookie检查] 查询用户失败: %v", err)
		return
	}

	for _, user := range users {
		if ctx.Err() != nil {
			return
		}
		cookies, err := secure.DecryptString(user.Cookies)
		now := time.Now()
		previousStatus := user.CookieStatus
		updates := map[string]interface{}{
			"last_cookie_check": now,
		}
		if err != nil {
			updates["login"] = false
			updates["cookie_status"] = "invalid"
			updates["cookie_message"] = "Cookie解密失败: " + err.Error()
			db.Model(&user).Updates(updates)
			if previousStatus != "invalid" {
				s.notifyCookieInvalid(user, updates["cookie_message"].(string))
			}
			continue
		}

		valid, err := bili.ValidateCookieContext(ctx, cookies)
		if err != nil {
			updates["cookie_status"] = "unknown"
			updates["cookie_message"] = err.Error()
		} else if valid {
			updates["login"] = true
			updates["cookie_status"] = "valid"
			updates["cookie_message"] = "Cookie有效，已刷新本地有效期"
			updates["expire_time"] = now.Add(30 * 24 * time.Hour)
		} else {
			updates["login"] = false
			updates["cookie_status"] = "invalid"
			updates["cookie_message"] = "Cookie已失效"
		}
		db.Model(&user).Updates(updates)
		if updates["cookie_status"] == "invalid" && previousStatus != "invalid" {
			s.notifyCookieInvalid(user, updates["cookie_message"].(string))
			s.markUserTasksCookieInvalid(user.ID, updates["cookie_message"].(string))
		}
	}
}

func (s *MonitorService) notifyCookieInvalid(user models.BiliUser, message string) {
	go func() {
		if err := notify.NewSender().SendCookieInvalid(user, message); err != nil {
			log.Printf("[Webhook] Cookie失效通知发送失败: %v", err)
		}
	}()
}

func (s *MonitorService) notifyMonitorError(task models.MonitorTask, message string) {
	go func() {
		if err := notify.NewSender().SendMonitorError(task, message); err != nil {
			log.Printf("[Webhook] 监控异常通知发送失败: %v", err)
		}
	}()
}

func (s *MonitorService) markUserTasksCookieInvalid(userID uint, message string) {
	if userID == 0 {
		return
	}
	if err := database.GetDB().Model(&models.MonitorTask{}).
		Where("user_id = ? AND enabled = ?", userID, true).
		Updates(map[string]interface{}{
			"last_status":      "error",
			"last_error":       "账号Cookie不可用: " + message,
			"progress_message": "账号Cookie不可用，等待重新登录或恢复",
		}).Error; err != nil {
		log.Printf("[Cookie检查] 更新关联任务状态失败: %v", err)
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
		"progress_message": fmt.Sprintf("完成：检测 %d 条，匹配 %d 条，举报 %d 条", checked, matched, reported),
	}
	if status == "backoff" {
		updates["progress_message"] = lastErr
	} else {
		updates["next_run_at"] = s.nextRunAt(taskID, time.Now())
	}
	if status == "success" || status == "warning" {
		now := time.Now()
		updates["last_success_at"] = now
		updates["backoff_until"] = nil
		updates["backoff_reason"] = ""
		updates["backoff_attempt"] = 0
	}
	database.GetDB().Model(&models.MonitorTask{}).Where("id = ?", taskID).Updates(updates)
}

func (s *MonitorService) nextRunAt(taskID uint, from time.Time) time.Time {
	interval := settings.GetInt("default_interval", 300)
	var task models.MonitorTask
	if err := database.GetDB().Select("id", "interval").First(&task, taskID).Error; err == nil && task.Interval > 0 {
		interval = task.Interval
	}
	if interval < 30 {
		interval = 30
	}
	return from.Add(time.Duration(interval) * time.Second)
}

func (s *MonitorService) updateNextRun(taskID uint, nextRunAt time.Time) {
	database.GetDB().Model(&models.MonitorTask{}).Where("id = ?", taskID).Update("next_run_at", nextRunAt)
}

func (s *MonitorService) updateTaskQueueState(taskID uint, status, reason string, nextRunAt time.Time) {
	database.GetDB().Model(&models.MonitorTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"last_status":      status,
		"last_error":       reason,
		"next_run_at":      nextRunAt,
		"progress_message": fmt.Sprintf("等待自动恢复：%s", reason),
	})
}

func (s *MonitorService) updateTaskProgress(taskID uint, total, done int64, message string) {
	if total < 0 {
		total = 0
	}
	if done < 0 {
		done = 0
	}
	if total > 0 && done > total {
		done = total
	}
	database.GetDB().Model(&models.MonitorTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"progress_total":   total,
		"progress_done":    done,
		"progress_message": message,
	})
}

func (s *MonitorService) updateTargetStatus(targetID uint, status, lastErr string, checked, matched, reported int64) {
	if targetID == 0 {
		return
	}
	updates := map[string]interface{}{
		"last_check":       time.Now(),
		"last_status":      status,
		"last_error":       lastErr,
		"checked_comments": gorm.Expr("checked_comments + ?", checked),
		"matched_comments": gorm.Expr("matched_comments + ?", matched),
		"report_count":     gorm.Expr("report_count + ?", reported),
	}
	database.GetDB().Model(&models.MonitorTarget{}).Where("id = ?", targetID).Updates(updates)
}

func (s *MonitorService) addLog(taskID uint, level, message string) {
	db := database.GetDB()
	now := time.Now()
	digest := logDigest(taskID, level, message)
	window := settings.GetInt("log_dedupe_window_seconds", 300)
	if window > 0 {
		var existing models.MonitorLog
		if err := db.Where("task_id = ? AND digest = ? AND created_at >= ?", taskID, digest, now.Add(-time.Duration(window)*time.Second)).
			Order("created_at DESC").
			First(&existing).Error; err == nil {
			if err := db.Model(&existing).Updates(map[string]interface{}{
				"repeat_count": gorm.Expr("repeat_count + 1"),
				"last_seen_at": now,
			}).Error; err != nil {
				log.Printf("[监控服务] 更新日志重复次数失败: %v", err)
			}
			return
		}
	}
	logEntry := models.MonitorLog{
		TaskID:      taskID,
		Level:       level,
		Message:     message,
		Digest:      digest,
		RepeatCount: 1,
		LastSeenAt:  &now,
	}
	if err := db.Create(&logEntry).Error; err != nil {
		log.Printf("[监控服务] 保存日志失败: %v", err)
	}
}

func checkedTargets(targets []models.MonitorTarget, currentID uint) int64 {
	for index, target := range targets {
		if target.ID == currentID {
			return int64(index)
		}
	}
	return 0
}

func logDigest(taskID uint, level, message string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", taskID, level, message)))
	return hex.EncodeToString(sum[:])
}

func (s *MonitorService) beginTask(taskID uint) (context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.ctx == nil || s.ctx.Err() != nil || s.runningTasks[taskID] {
		return nil, false
	}
	s.runningTasks[taskID] = true
	s.wg.Add(1)
	return s.ctx, true
}

func (s *MonitorService) context() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s *MonitorService) clearTaskRunning(taskID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runningTasks, taskID)
}

func (l *ReportLimiter) Wait(ctx context.Context, delaySeconds int) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if delaySeconds < 30 {
		delaySeconds = 30
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if l.next.After(now) {
		timer := time.NewTimer(l.next.Sub(now))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return false
		}
	}
	l.next = time.Now().Add(time.Duration(delaySeconds) * time.Second)
	return true
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
