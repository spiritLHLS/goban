package middleware

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
)

const (
	authFailureLimit  = 5
	authFailureWindow = 5 * time.Minute
	authBlockDuration = 15 * time.Minute
)

type authAttempt struct {
	failures     int
	firstFailure time.Time
	blockedUntil time.Time
}

type authRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]authAttempt
}

var basicAuthLimiter = &authRateLimiter{attempts: map[string]authAttempt{}}

// BasicAuth Basic认证中间件
func BasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()
		rateKey := c.ClientIP()
		now := time.Now()
		if basicAuthLimiter.blocked(rateKey, now) {
			retryAfter := basicAuthLimiter.retryAfter(rateKey, now)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			}
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    http.StatusTooManyRequests,
				"message": "登录失败次数过多，请稍后再试",
				"error":   "登录失败次数过多，请稍后再试",
			})
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "需要认证",
				"error":   "需要认证",
			})
			return
		}

		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			basicAuthLimiter.recordFailure(rateKey, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "认证格式错误",
				"error":   "认证格式错误",
			})
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			basicAuthLimiter.recordFailure(rateKey, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "认证信息解码失败",
				"error":   "认证信息解码失败",
			})
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			basicAuthLimiter.recordFailure(rateKey, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "认证信息格式错误",
				"error":   "认证信息格式错误",
			})
			return
		}

		username, password := pair[0], pair[1]
		if username != cfg.Username || password != cfg.Password {
			basicAuthLimiter.recordFailure(rateKey, now)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "用户名或密码错误",
				"error":   "用户名或密码错误",
			})
			return
		}

		basicAuthLimiter.reset(rateKey)
		c.Next()
	}
}

func (l *authRateLimiter) blocked(key string, now time.Time) bool {
	if key == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt, ok := l.attempts[key]
	if !ok {
		return false
	}
	if !attempt.blockedUntil.IsZero() && attempt.blockedUntil.After(now) {
		return true
	}
	if now.Sub(attempt.firstFailure) > authFailureWindow {
		delete(l.attempts, key)
	}
	return false
}

func (l *authRateLimiter) retryAfter(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt := l.attempts[key]
	if attempt.blockedUntil.After(now) {
		return attempt.blockedUntil.Sub(now).Round(time.Second)
	}
	return 0
}

func (l *authRateLimiter) recordFailure(key string, now time.Time) {
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt := l.attempts[key]
	if attempt.firstFailure.IsZero() || now.Sub(attempt.firstFailure) > authFailureWindow {
		attempt = authAttempt{firstFailure: now}
	}
	attempt.failures++
	if attempt.failures >= authFailureLimit {
		attempt.blockedUntil = now.Add(authBlockDuration)
	}
	l.attempts[key] = attempt
}

func (l *authRateLimiter) reset(key string) {
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func resetBasicAuthLimiterForTest() {
	basicAuthLimiter.mu.Lock()
	defer basicAuthLimiter.mu.Unlock()
	basicAuthLimiter.attempts = map[string]authAttempt{}
}
