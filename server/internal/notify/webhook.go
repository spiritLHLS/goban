package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spiritlhl/goban/internal/models"
	"github.com/spiritlhl/goban/internal/settings"
)

type Sender struct {
	client *http.Client
}

func NewSender() *Sender {
	timeout := settings.GetInt("webhook_timeout", 8)
	if timeout <= 0 {
		timeout = 8
	}
	return &Sender{
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

func (s *Sender) SendReport(record models.ReportRecord) error {
	return s.send(formatReportMessage(record))
}

func (s *Sender) SendCookieInvalid(user models.BiliUser, reason string) error {
	return s.send(fmt.Sprintf(
		"[goban] Cookie已失效\nUP主: %s (%d)\n原因: %s",
		user.Uname,
		user.UID,
		reason,
	))
}

func (s *Sender) SendMonitorError(task models.MonitorTask, reason string) error {
	return s.send(fmt.Sprintf(
		"[goban] 监控异常\n任务: %s (#%d)\n原因: %s",
		task.Name,
		task.ID,
		reason,
	))
}

func (s *Sender) send(message string) error {
	if !settings.GetBool("webhook_enabled", false) {
		return nil
	}
	webhookType := strings.ToLower(settings.Get("webhook_type", "none"))

	switch webhookType {
	case "telegram":
		return s.sendTelegram(message)
	case "feishu":
		return s.sendFeishu(message)
	case "dingtalk":
		return s.sendDingTalk(message)
	default:
		return nil
	}
}

func (s *Sender) sendTelegram(message string) error {
	token := settings.Get("telegram_bot_token", "")
	chatID := settings.Get("telegram_chat_id", "")
	if token == "" || chatID == "" {
		return fmt.Errorf("Telegram Webhook 未配置 bot token 或 chat id")
	}

	body := map[string]string{
		"chat_id": chatID,
		"text":    message,
	}
	return s.postJSON(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token), body)
}

func (s *Sender) sendFeishu(message string) error {
	url := settings.Get("feishu_webhook_url", "")
	if url == "" {
		return fmt.Errorf("飞书 Webhook URL 未配置")
	}

	body := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": message,
		},
	}
	return s.postJSON(url, body)
}

func (s *Sender) sendDingTalk(message string) error {
	url := settings.Get("dingtalk_webhook_url", "")
	if url == "" {
		return fmt.Errorf("钉钉 Webhook URL 未配置")
	}

	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}
	return s.postJSON(url, body)
}

func (s *Sender) postJSON(url string, body interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("Webhook 返回 HTTP %d", resp.StatusCode)
		}
		if attempt == 0 {
			time.Sleep(time.Second)
		}
	}
	return lastErr
}

func formatReportMessage(record models.ReportRecord) string {
	return fmt.Sprintf(
		"[goban] 已举报评论\nUP主: %s (%d)\n视频: %s (%s)\n评论用户: %s (%d)\n匹配规则: %s\n匹配内容: %s\n评论内容: %s\n评论ID: %d",
		record.TargetUname,
		record.TargetUID,
		record.VideoTitle,
		record.BVID,
		record.CommentUser,
		record.CommentUserID,
		record.KeywordRuleName,
		record.MatchedKeyword,
		truncate(record.CommentContent, 160),
		record.CommentID,
	)
}

func truncate(value string, limit int) string {
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}
