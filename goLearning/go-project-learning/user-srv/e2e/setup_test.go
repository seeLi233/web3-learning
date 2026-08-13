//go:build e2e

// 为什么用 build tag 而不是塞进普通测试？
// → 让 E2E 和单元测试解耦：普通 go test 自动跳过（不碰 Docker），
//    E2E 只在 go test -tags=e2e 时显式运行。这是 Go 生态隔离慢测试的标准做法

package e2e

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	commoncfg "github.com/go-project-learning/project/common/pkg/config"
	commonredis "github.com/go-project-learning/project/common/pkg/redis"
	v1 "github.com/go-project-learning/project/gateway-api/api/v1"
	gwcfg "github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	repodb "github.com/go-project-learning/project/user-srv/internal/repository/db"
	"github.com/go-project-learning/project/user-srv/internal/server"
	"github.com/go-project-learning/project/user-srv/pkg/sms"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ============ 包级共享状态 ============
// 为什么用包级变量而不是每个测试自己 Setup？
// → 容器启动一次要几十秒，每个用例重复拉容器会让测试慢到无法接受。
//
//	TestMain 只启动一次，所有用例共享同一套基础设施，TearDown 也只清理一次
var (
	infra   *testutil.TestInfra // 容器句柄，用于 TearDown
	router  *gin.Engine         // 装配好的 gateway 路由（含全部真实中间件 + handler）
	grpcSrv *grpc.Server        // 内存中的 user-srv gRPC 服务
)

// TestMain 是所有测试的入口，先启动基础设施再跑用例
func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		// 为什么初始化失败直接 os.Exit(1) 而不是 m.Run()？
		// → 基础设施都没起来，所有用例必然失败，与其跑一堆无意义的失败，不如立即终止并给出明确原因
		fmt.Printf("❌ E2E 测试初始化失败: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	ctx := context.Background()

	// 1. 初始化日志（关键！否则 logger.Log 是 nil，下面 InitRedis 会 panic）
	// 为什么 level 用 "error"？
	// → 测试输出要干净，只让 error 级日志打印，info 日志（如"Redis 连接成功"）静默
	logger.Init("./logs/e2e_test.log", "error")

	// 2. 启动真实 MySQL + Redis 容器
	var err error
	infra, err = testutil.Setup(ctx)
	if err != nil {
		return 0, err
	}
	// 为什么 defer TearDown 而不是手动在最后调用？
	// → defer 保证无论测试正常结束还是 panic，容器都会被清理，不会泄漏
	defer infra.TearDown(ctx)

	// 3. 连接 MySQL + 建表
	// 为什么 isDev 传 false？→ 关闭全量 SQL 日志，测试输出更干净
	database.InitDB(infra.MySQL, false)
	// 注册链路会写 user + member_info 两张表，member_info 外键引用 member_level
	// 所以三张表都要迁移，否则 Insert 时外键约束报错
	if err := database.DB.AutoMigrate(
		&entity.User{},
		&entity.MemberInfo{},
		&entity.MemberLevel{},
	); err != nil {
		return 0, fmt.Errorf("AutoMigrate 失败: %w", err)
	}

	// 4. 连接 Redis（IPBlacklist 中间件和 register 限流都要用）
	commonredis.InitRedis(infra.Redis)

	// 5. 启动内存 gRPC server
	host, port, grpcSrv, err := startUserGRPC()
	if err != nil {
		return 0, err
	}
	// GracefulStop 等待进行中的 RPC 完成后再停，避免测试结束时连接被硬切断
	defer grpcSrv.GracefulStop()

	// 6. 装配 gateway 路由
	router, err = buildGatewayRouter(host, port)
	if err != nil {
		return 0, err
	}

	// 7. 基础设施全部就绪，开始跑测试用例
	return m.Run(), nil
}

// startUserGRPC 在进程内启动真实的 user-srv gRPC 服务
//
// 为什么在"进程内"启动而不是起一个独立子进程？
// → 子进程要处理启动等待、日志捕获、进程清理，复杂度翻倍。
//
//	进程内启动只是把 main.go 里"构建依赖链 + 注册服务"的代码复刻一遍，
//	但监听 127.0.0.1:0 随机端口——和真实服务唯一的区别就是没用固定 50051 端口
func startUserGRPC() (host string, port int, srv *grpc.Server, err error) {
	// 1. 构建依赖链：repo → memberApp → userApp → server
	//    （和 user-srv/cmd/main.go 里的顺序完全一致）
	userRepo := repodb.NewUserRepo(database.DB)
	memberRepo := repodb.NewMemberRepo(database.DB)
	memberApp := application.NewMemberApp(memberRepo)
	// 为什么 SMS 传空配置？
	// → 空 Provider 会让 sms.NewSender 走 default 分支返回 DevSender（只打日志不真发短信）
	//    register 链路根本不调短信，所以这里随便给个空配置即可
	smsSender := sms.NewSender(commoncfg.SMSConfig{})
	userApp := application.NewUserApp(smsSender, userRepo, memberApp)
	userService := server.NewUserServiceServer(userApp)

	// 2. 监听随机端口
	// 为什么用 "127.0.0.1:0" 而不是 ":0"？
	// → 只绑定回环地址，测试不对外暴露端口，也更安全
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", 0, nil, err
	}
	// 端口 0 让 OS 分配一个空闲端口，通过 Addr() 读回实际端口
	tcpAddr := lis.Addr().(*net.TCPAddr)

	// 3. 注册并后台启动
	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, userService)
	go func() {
		// Serve 会阻塞，放进 goroutine 让主流程继续往下走
		_ = grpcServer.Serve(lis)
	}()

	return tcpAddr.IP.String(), tcpAddr.Port, grpcServer, nil
}

// buildGatewayRouter 装配真实的 gateway Gin 路由
func buildGatewayRouter(host string, port int) (*gin.Engine, error) {
	// 1. 设置 gateway 配置（JWT 密钥给步骤三的登录链路用）
	gwcfg.Conf = &gwcfg.AppConfig{
		UserSrv: gwcfg.UserSrvConfig{Host: host, Port: port},
		JWT:     gwcfg.JWTConfig{Secret: "e2e-test-secret"},
	}

	// 2. 建立 gRPC 客户端
	// 为什么不调 rpc_client.InitUserClient()？
	// → 它在 gateway-api/internal/rpc_client 里，internal 包对 user-srv 模块不可见。
	//    这里手动 grpc.NewClient + 赋值 global.UserClient，效果完全一样，
	//    只是绕开了 internal 可见性限制
	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	global.UserClient = pb.NewUserServiceClient(conn)

	// 3. 构建 Gin 路由（复刻 gateway-api/cmd/api/main.go 的路由注册）
	gin.SetMode(gin.TestMode) // 关掉 Gin 启动日志
	r := gin.New()
	apiV1 := r.Group("/api/v1")
	v1.RegisterUserRoutes(apiV1) // 注册 /user/register、/user/:id 等真实路由

	return r, nil
}
