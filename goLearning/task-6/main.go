package main

import (
	"context"
	"github/seeli/task-6/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// ===== 1. 初始化路由 =====
	r := router.SetupRouter()

	// ===== 2. 创建 HTTP Server =====
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  15 * time.Second, // 防止慢客户端攻击
		WriteTimeout: 15 * time.Second, // 防止慢响应攻击
		IdleTimeout:  60 * time.Second, // Keep-Alive 超时
	}

	// ===== 3. 在 goroutine 中监听关闭信号 =====
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		log.Printf("🛑 收到信号 %v，开始优雅关闭...", sig)

		// 给 30 秒时间完成正在处理的请求
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("⚠️ 强制关闭（超时）: %v", err)
		}
		log.Println("✅ 服务已安全退出")
	}()

	// ===== 4. 启动服务 =====
	log.Println("🚀 API Gateway 启动于 http://localhost:8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ 启动失败: %v", err)
	}
}
