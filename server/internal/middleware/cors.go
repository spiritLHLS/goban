package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
)

// CORS restricts browser API access to configured origins while allowing
// same-origin and non-browser requests that do not carry an Origin header.
func CORS(cfg *config.Config) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range cfg.AllowedOrigins {
		allowed[origin] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowed[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Vary", "Origin")
			} else if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "600")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
