package routes

import (
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/controllers"
	"github.com/spiritlhl/goban/internal/middleware"
)

func SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		// 需要认证的路由
		auth := api.Group("")
		auth.Use(middleware.BasicAuth())
		{
			// B站用户管理
			users := auth.Group("/users")
			{
				users.GET("/list", controllers.ListBiliUsers)
				users.GET("/login", controllers.LoginUser)
				users.GET("/loginCheck", controllers.LoginCheck)
				users.GET("/loginCancel", controllers.LoginCancel)
				users.POST("/loginByCookie", controllers.LoginByCookie)
				users.POST("/:id/check", controllers.CheckBiliUserCookie)
				users.DELETE("/:id", controllers.DeleteBiliUser)
			}

			// 监控任务管理
			tasks := auth.Group("/tasks")
			{
				tasks.GET("/list", controllers.ListMonitorTasks)
				tasks.GET("/progress", controllers.ListTaskProgress)
				tasks.POST("/create", controllers.CreateMonitorTask)
				tasks.GET("/:id/progress", controllers.GetTaskProgress)
				tasks.POST("/:id/status", controllers.UpdateTaskStatus)
				tasks.PUT("/:id", controllers.UpdateMonitorTask)
				tasks.DELETE("/:id", controllers.DeleteMonitorTask)
				tasks.GET("/:id/test", controllers.TestMonitorTask)
			}

			// 关键字规则管理
			keywords := auth.Group("/keywords")
			{
				keywords.GET("/list", controllers.ListKeywordRules)
				keywords.POST("/create", controllers.CreateKeywordRule)
				keywords.POST("/preview", controllers.PreviewKeywordRules)
				keywords.PUT("/:id", controllers.UpdateKeywordRule)
				keywords.DELETE("/:id", controllers.DeleteKeywordRule)
			}

			// 白名单管理
			whitelist := auth.Group("/whitelist")
			{
				whitelist.GET("/list", controllers.ListWhitelistUsers)
				whitelist.POST("/create", controllers.CreateWhitelistUser)
				whitelist.PUT("/:id", controllers.UpdateWhitelistUser)
				whitelist.DELETE("/:id", controllers.DeleteWhitelistUser)
			}

			// 系统配置和状态
			auth.GET("/settings", controllers.GetSettings)
			auth.PUT("/settings", controllers.UpdateSettings)
			auth.GET("/status", controllers.GetMonitorStatus)
			auth.GET("/docs", controllers.GetAPIDocs)
			auth.GET("/docs/openapi.json", controllers.GetOpenAPISpec)

			// 日志和记录
			logs := auth.Group("/logs")
			{
				logs.GET("/monitor", controllers.GetMonitorLogs)
				logs.GET("/report", controllers.GetReportRecords)
				logs.GET("/report/export", controllers.ExportReportRecords)
			}
		}
	}

	pprofRoutes := router.Group("/debug/pprof")
	pprofRoutes.Use(middleware.BasicAuth())
	{
		pprofRoutes.GET("/", gin.WrapF(pprof.Index))
		pprofRoutes.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofRoutes.GET("/profile", gin.WrapF(pprof.Profile))
		pprofRoutes.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofRoutes.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofRoutes.GET("/trace", gin.WrapF(pprof.Trace))
		for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
			pprofRoutes.GET("/"+name, gin.WrapH(pprof.Handler(name)))
		}
	}

	// 健康检查（不需要认证）
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 静态文件服务（用于Docker部署）
	// 检查web/dist目录是否存在
	distPath := "./web/dist"
	if _, err := os.Stat(distPath); err == nil {
		// 服务静态文件
		router.StaticFS("/assets", http.Dir(filepath.Join(distPath, "assets")))

		// SPA路由处理 - 所有非API和非静态文件的请求都返回index.html
		router.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "API路由不存在"})
				return
			}
			if path == "/debug" || strings.HasPrefix(path, "/debug/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "调试路由不存在"})
				return
			}
			c.File(filepath.Join(distPath, "index.html"))
		})
	}
}
