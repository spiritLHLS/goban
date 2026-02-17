package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/monitor"
	"github.com/spiritlhl/goban/internal/routes"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()

	// 初始化数据库
	if err := database.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 初始化监控服务
	monitorService := monitor.NewMonitorService()
	go monitorService.Start()

	// 设置Gin模式
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := gin.Default()

	// 配置CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 设置路由
	routes.SetupRoutes(router)

	// 启动服务器
	log.Printf("服务器启动在端口 %s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
