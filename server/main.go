package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/config"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/middleware"
	"github.com/spiritlhl/goban/internal/monitor"
	"github.com/spiritlhl/goban/internal/routes"
)

// @title Goban API
// @version 1.0
// @description Authenticated API for Bilibili comment monitoring, keyword rules, reports, settings, and account management.
// @BasePath /api
// @securityDefinitions.basic BasicAuth
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	router.Use(middleware.CORS(cfg))

	// 设置路由
	routes.SetupRoutes(router)

	// 启动服务器
	log.Printf("服务器启动在端口 %s", cfg.Port)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Println("收到退出信号，正在关闭服务")
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("启动服务器失败: %v", err)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 HTTP 服务失败: %v", err)
	}
	monitorService.Stop()
}
