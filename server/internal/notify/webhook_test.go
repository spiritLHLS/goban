package notify

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spiritlhl/goban/internal/models"
)

func TestPostJSONRetriesTransientFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &Sender{client: server.Client()}
	started := time.Now()
	if err := sender.postJSON(server.URL, map[string]string{"text": "hello"}); err != nil {
		t.Fatalf("expected retry to recover, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if time.Since(started) < time.Second {
		t.Fatal("expected retry delay before second attempt")
	}
}

func TestFormatReportMessageContainsRequiredContext(t *testing.T) {
	message := formatReportMessage(models.ReportRecord{
		TargetUname:     "测试UP",
		TargetUID:       123,
		VideoTitle:      "测试视频",
		BVID:            "BV123",
		CommentUser:     "评论用户",
		CommentUserID:   456,
		KeywordRuleName: "广告规则",
		MatchedKeyword:  "广告",
		CommentContent:  "广告内容",
		CommentID:       789,
	})

	required := []string{"测试UP", "123", "测试视频", "BV123", "评论用户", "456", "广告规则", "广告", "广告内容", "789"}
	for _, value := range required {
		if !strings.Contains(message, value) {
			t.Fatalf("expected message to contain %q, got %q", value, message)
		}
	}
}
