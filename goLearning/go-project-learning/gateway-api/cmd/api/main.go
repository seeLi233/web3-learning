package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/metrics"
	"github.com/go-project-learning/project/common/pkg/redis"
	v1 "github.com/go-project-learning/project/gateway-api/api/v1"
	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
	"github.com/go-project-learning/project/gateway-api/internal/rpc_client"
)

func main() {
	// 1. 初始化配置
	config.Init()

	// 1.5 初始化日志
	logger.Init("./logs/gateway.log", "debug")
	defer logger.Sync()

	// 1.6 初始化 Redis
	redis.InitRedis(config.Conf.Redis)

	// 2. 初始化 gRPC 客户端
	rpc_client.InitUserClient()
	rpc_client.InitOAuthClient()
	rpc_client.InitAddressClient()
	rpc_client.InitMemberClient()
	rpc_client.InitCouponClient()
	// 初始化风控客户端
	rpc_client.InitRiskClient()

	// 3. 初始化 Gin
	r := gin.Default()
	r.Use(middleware.PrometheusMetrics(), middleware.CORS(), middleware.Logger(), middleware.IPBlacklist())

	// 为什么 /metrics 放在路由组外面？
	// → /metrics 不需要 /api/v1 前缀，不需要 JWT 认证
	//    它是 Prometheus Server 访问的内部端点，应该是"最公开的路径"
	// 为什么用 gin.WrapH？
	// → promhttp.Handler() 返回的是标准库 http.Handler，不是 gin.HandlerFunc
	//    gin.WrapH() 把 http.Handler 适配成 gin.HandlerFunc，让 gin 能处理它
	r.GET("/metrics", gin.WrapH(metrics.MetricsHandler()))

	// 4. 注册路由
	apiV1 := r.Group("/api/v1")
	v1.RegisterUserRoutes(apiV1)
	v1.RegisterOAuthRoutes(apiV1)
	v1.RegisterAddressRouter(apiV1)
	v1.RegisterMemberRouter(apiV1)
	v1.RegisterCouponRouter(apiV1)
	// 注册风控路由
	v1.RegisterRiskRouter(apiV1)

	// 5. 启动
	port := config.Conf.Server.Port
	log.Printf("✅ gateway-api 启动成功 :%d", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
