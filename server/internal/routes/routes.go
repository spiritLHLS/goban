package routes

import (
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
				users.DELETE("/:id", controllers.DeleteBiliUser)
			}

			// 监控任务管理
			tasks := auth.Group("/tasks")
			{
				tasks.GET("/list", controllers.ListMonitorTasks)
				tasks.POST("/create", controllers.CreateMonitorTask)
				tasks.PUT("/:id", controllers.UpdateMonitorTask)
				tasks.DELETE("/:id", controllers.DeleteMonitorTask)
				tasks.GET("/:id/test", controllers.TestMonitorTask)
			}

			// 日志和记录
			logs := auth.Group("/logs")
			{
				logs.GET("/monitor", controllers.GetMonitorLogs)
				logs.GET("/report", controllers.GetReportRecords)
			}
		}
	}

	// 健康检查（不需要认证）
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
