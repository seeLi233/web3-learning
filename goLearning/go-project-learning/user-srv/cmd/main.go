package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/metrics"
	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/middleware"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
	"github.com/go-project-learning/project/user-srv/internal/server"
	"github.com/go-project-learning/project/user-srv/pkg/sms"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger.Init("./logs/server.log", "debug")
	defer logger.Sync()

	// 初始化配置
	config.Init()

	// 添加这一行：初始化数据库
	database.InitDB(config.Conf.MySQL, true)
	// 初始化 OAuth
	oauthRepo := db.NewOAuthRepo(database.DB)
	userRepo := db.NewUserRepo(database.DB)
	addressRepo := db.NewAddressRepo(database.DB)
	memberRepo := db.NewMemberRepo(database.DB)
	couponRepo := db.NewCouponRepo(database.DB)
	riskConfigRepo := db.NewRiskConfigRepo(database.DB)
	ipBlacklistRepo := db.NewIpBlacklistRepo(database.DB)

	oauthApp := application.NewOAuthApp(oauthRepo, userRepo)
	memberApp := application.NewMemberApp(memberRepo) // 必须在 userApp 之前，因为 userApp 依赖 memberApp
	smsSender := sms.NewSender(config.Conf.SMSConfig)
	userApp := application.NewUserApp(smsSender, userRepo, memberApp)
	addressApp := application.NewAddressApp(addressRepo)
	couponApp := application.NewCouponApp(couponRepo)
	riskConfigApp := application.NewRiskConfigApp(riskConfigRepo, ipBlacklistRepo)

	// 迁移 OAuth 表
	if err := oauthRepo.AutoMigrate(); err != nil {
		log.Fatalf("OAuth 表迁移失败: %v", err)
	}

	// 自动建表（表不存在则创建，已存在则跳过）
	if err := database.DB.AutoMigrate(
		&entity.User{},
		&entity.Address{},
		&entity.MemberLevel{},
		&entity.MemberInfo{},
		&entity.MemberGrowthLog{},
		&entity.CouponTemplate{},
		&entity.UserCoupon{},
		&entity.IpBlackList{},
		&entity.RiskConfig{},
	); err != nil {
		log.Fatalf("自动迁移数据库表失败: %v", err)
	}

	redis.InitRedis(config.Conf.Redis)

	cfg := config.Conf.Consul

	configs := api.DefaultConfig()
	configs.Address = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	client, err := api.NewClient(configs)
	if err != nil {
		logger.Error("创建 Consul 客户端失败", zap.Error(err))
		return
	}

	// 为什么：
	// 	- K8s 会给每个 Pod 分配一个集群内部 IP（如 10.244.1.5）
	// 	- Consul 注册时必须用这个 Pod IP，其他服务才能通过 Consul 发现并连接到它
	// 	- K8s 的 Downward API 可以把 Pod IP 注入到环境变量 POD_IP 中
	podID := os.Getenv("POD_IP")
	if podID == "" {
		podID = "127.0.0.1"
	}

	// 拆分 IP 和 端口
	ip, portStr, err := net.SplitHostPort(podID + ":50051")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}

	// 组装注册信息
	reg := &api.AgentServiceRegistration{
		Name:    "user-grpc-service",
		Address: ip,
		Port:    port,
		// 健康检查：定期探测服务是否存活
		Check: &api.AgentServiceCheck{
			TCP:                            podID + ":50051",
			Interval:                       "10s", // 每 10s 检查一次
			Timeout:                        "3s",  // 超时 3s
			DeregisterCriticalServiceAfter: "30s", // 异常 30s 后自动注销
		},
	}

	client.Agent().ServiceRegister(reg)

	// 1. 监听端口
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		log.Fatalf("端口监听失败: %v", err)
		// logger.Fatel("端口监听失败", zap.Error(err))
	}

	// 2. 创建gRPC服务器
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.GrpcMetricsInterceptor, // 第 1 层：指标采集（最外层，记录完整耗时）
			globalLogInterceptor,              // 第 2 层：日志打印
		),
	)

	// 3. 注册服务（从 internal/service 引入）
	userService := server.NewUserServiceServer(userApp)
	pb.RegisterUserServiceServer(grpcServer, userService)

	oauthServer := server.NewOAuthServer(oauthApp, userApp)
	pb.RegisterOAuthServiceServer(grpcServer, oauthServer)

	addressServer := server.NewAddressServer(addressApp)
	pb.RegisterAddressServiceServer(grpcServer, addressServer)

	memberServer := server.NewMemberServer(memberApp)
	pb.RegisterMemberServiceServer(grpcServer, memberServer)

	couponServer := server.NewCouponServer(couponApp)
	pb.RegisterCouponServiceServer(grpcServer, couponServer)

	riskServer := server.NewRiskServer(riskConfigApp)
	pb.RegisterRiskServiceServer(grpcServer, riskServer)

	reflection.Register(grpcServer)

	riskConfigApp.LoadRiskConfigsToRedis(context.Background())

	// 4. 启动服务
	// if err := grpcServer.Serve(lis); err != nil {
	// 	log.Fatalf("服务启动失败: %v", err)
	// 	// logger.Fatel("服务启动失败", zap.Error(err))
	// }

	// 4.1. 信号监听（buffered channel）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 4.2. 启动 metrics HTTP server（用 http.Server 而非 ListenAndServe）
	metricsServer := &http.Server{Addr: ":8082", Handler: nil}
	http.Handle("/metrics", metrics.MetricsHandler())

	go func() {
		log.Println("📊 Metrics 端点启动在 :8082")
		// 为什么用 ListenAndServe 的错误不等于 http.ErrServerClosed？
		// → Shutdown() 会让 ListenAndServe 返回 http.ErrServerClosed，这是正常退出
		//    如果返回其他错误，才是真的异常
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ Metrics server 异常退出: %v", err)
		}
	}()

	// 4.3. 启动 gRPC server（也放入 goroutine）
	go func() {
		log.Println("✅ 用户服务启动成功 :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("⚠️ gRPC server 异常退出: %v", err)
		}
	}()

	// 4.4. 阻塞等待信号
	<-quit
	log.Println("🛑 收到关闭信号，开始优雅关闭...")

	// 4.5. gRPC GracefulStop（先停——不再接受新 RPC 请求，等待进行中的完成）
	grpcServer.GracefulStop()
	log.Println("✅ gRPC 服务已优雅关闭")

	// 4.6. Metrics HTTP Shutdown（5s 超时兜底）
	ctx, cancle := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancle()
	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️ Metrics server 关闭超时: %v", err)
	} else {
		log.Println("✅ Metrics 服务已优雅关闭")
	}
}

func globalLogInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// 1. 请求进入：前置日志
	start := time.Now()
	logger.Info(fmt.Sprintf("[全局日志] 开始请求 | 接口: %s | 请求参数: %v", info.FullMethod, req))

	// 2. 执行真正的业务入口
	resp, err = handler(ctx, req)

	// 3. 请求结束：后置日志
	cost := time.Since(start)
	logger.Info(fmt.Sprintf("[全局日志] 结束请求 | 接口: %s | 耗时: %v | 错误: %v", info.FullMethod, cost, err))

	return resp, err
}
