package bili

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAPICodeErrorClassifiesRiskControl(t *testing.T) {
	err := apiCodeError("举报失败", "操作频繁，请稍后再试", 0)
	if !IsRiskControlError(err) {
		t.Fatalf("expected risk-control error, got %T %v", err, err)
	}

	err = apiCodeError("获取评论失败", "风控校验失败", -412)
	if !IsRiskControlError(err) {
		t.Fatalf("expected code -412 to be risk-control error, got %T %v", err, err)
	}
}

func TestAPICodeErrorLeavesOrdinaryErrorsRetryable(t *testing.T) {
	err := apiCodeError("获取视频列表失败", "系统错误", -500)
	if IsRiskControlError(err) {
		t.Fatalf("ordinary API errors should not be risk-control errors: %v", err)
	}
	if !strings.Contains(err.Error(), "code=-500") {
		t.Fatalf("expected error to keep API code, got %q", err.Error())
	}
}

func TestRetryWithBackoffHonorsCancelledContext(t *testing.T) {
	client := &BiliClient{MaxRetries: 3, RetryInterval: 30}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := client.retryWithBackoff(ctx, func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected cancelled context error")
	}
	if called {
		t.Fatal("operation should not run after context cancellation")
	}
}

func TestRetryWithBackoffRetriesRateLimitError(t *testing.T) {
	client := &BiliClient{MaxRetries: 1, RetryInterval: 30}
	attempts := 0
	started := time.Now()

	err := client.retryWithBackoff(context.Background(), func() error {
		attempts++
		if attempts == 1 {
			return &RateLimitError{Message: "rate limited", RetryAfter: 10 * time.Millisecond}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to recover from rate limit, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if time.Since(started) < 10*time.Millisecond {
		t.Fatal("expected retry-after delay before second attempt")
	}
}

func TestValidateCookieContextHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	valid, err := ValidateCookieContext(ctx, "")
	if err == nil {
		t.Fatal("expected cancelled context error")
	}
	if valid {
		t.Fatal("cancelled validation should not report a valid cookie")
	}
}

func TestExtractCookiesFromPollResponsesHandleNil(t *testing.T) {
	if got := ExtractCookiesFromWebPollResponse(nil, nil); got != "" {
		t.Fatalf("expected empty cookie string for nil web response, got %q", got)
	}
	if got := ExtractCookiesFromTVPollResponse(nil); got != "" {
		t.Fatalf("expected empty cookie string for nil TV response, got %q", got)
	}
}

func TestExtractCookiesFromWebPollResponseDecodesQuery(t *testing.T) {
	resp := &QRCodePollResponse{}
	resp.Data.Code = 0
	resp.Data.URL = "https://www.bilibili.com/?DedeUserID=42&SESSDATA=sess%2Bdata%3D%3D&bili_jct=csrf%2Btoken&DedeUserID__ckMd5=md5%2Bvalue&sid=sid%2Fvalue"

	got := ExtractCookiesFromWebPollResponse(resp, nil)
	for _, want := range []string{
		"bili_jct=csrf+token",
		"SESSDATA=sess+data==",
		"DedeUserID=42",
		"DedeUserID__ckMd5=md5+value",
		"sid=sid/value",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected cookie string %q to contain %q", got, want)
		}
	}
}

func TestExtractCookiesFromTVPollResponseRequiresCoreCookies(t *testing.T) {
	resp := &QRCodePollResponse{}
	resp.Data.Code = 0
	resp.Data.CookieInfo.Cookies = []struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		HttpOnly int    `json:"http_only"`
		Expires  int64  `json:"expires"`
		Secure   int    `json:"secure"`
	}{
		{Name: "DedeUserID", Value: "42"},
		{Name: "bili_jct", Value: "csrf-token"},
	}

	if got := ExtractCookiesFromTVPollResponse(resp); got != "" {
		t.Fatalf("expected missing SESSDATA to produce empty cookie string, got %q", got)
	}
}
