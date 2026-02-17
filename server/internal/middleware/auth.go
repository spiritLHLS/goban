package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
)

// BasicAuth Basic认证中间件
func BasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.GetConfig()

		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "需要认证",
			})
			return
		}

		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "认证格式错误",
			})
			return
		}

		payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "认证信息解码失败",
			})
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "认证信息格式错误",
			})
			return
		}

		username, password := pair[0], pair[1]
		if username != cfg.Username || password != cfg.Password {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "用户名或密码错误",
			})
			return
		}

		c.Next()
	}
}
