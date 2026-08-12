package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/common/pkg/logger"
	redisCli "github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
	"github.com/go-project-learning/project/user-srv/pkg/sms"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// ============================================================
// gRPC 端到端集成测试（使用 bufconn 内存连接）
//
// 为什么用 bufconn 而不是真实 TCP 端口？
// -> bufconn 是 gRPC 官方提供的测试工具，在内存中模拟网络连接
//   好处：
//     1. 不占用真实端口（避免端口冲突，CI 环境也能跑）
//     2. 比 TCP 快（没有网络栈开销）
//     3. 不需要改防火墙或安全组配置
//   缺点：
//     不能测试真实的网络超时、连接断开等网络层行为
//     但业务逻辑层的正确性完全能验证——这对面试已经够了
//
// 测试链路：Test Client -> bufconn -> gRPC Server -> Application -> Repository -> MySQL/Redis
// ============================================================

const bufSize = 1024 * 1024 // 1MB 缓冲区

var (
	testLis      *bufconn.Listener // 内存监听器，替代 TCP net.Listen
	testClient   pb.UserServiceClient
	testUserRepo *db.UserRepo // 仓库实例，供测试用例查询用
	testCleanup  []func()     // 测试结束后执行的清理函数列表
)

// cleanupUser 生成一个清理函数，用于测试结束后删除测试用户
//
// 为什么返回 func() 而不是直接执行？
// -> 这样可以把清理函数注册到 testCleanup 列表，
//
//	TestMain 在所有测试跑完后统一执行清理
func cleanupUser(phone string) func() {
	return func() {
		database.DB.Unscoped().Where("phone = ?", phone).Delete(&entity.User{})
	}
}

// TestMain 是 Go 测试的标准入口——在所有测试之前执行一次 setup，之后执行 teardown
//
// 为什么用 TestMain 而不是 init()？
// -> init() 在包加载时自动执行，你无法控制顺序，也无法做 teardown
//
//	TestMain 让你显式控制：setup -> run tests -> teardown
func TestMain(m *testing.M) {
	// ====== Setup：初始化所有依赖 ======

	// 0. 切回项目根目录
	//    为什么需要 chdir？
	//    -> go test 会把 CWD 设为当前包目录（internal/server/），
	//       但 config.Init() 用 viper.AddConfigPath("./configs") 相对路径找配置，
	//       所以必须回到模块根目录（user-srv/）才能找到 configs/config.dev.yaml
	os.Chdir("../..")

	// 1. 初始化配置（读取测试配置文件）
	//    为什么用测试专用配置？
	//    -> 测试环境的 MySQL/Redis 端口和开发环境可能不同（如 CI 中），
	//       分离配置避免测试污染开发数据库
	config.Init()

	// 2. 初始化日志（测试日志写到单独文件，不乱控制台）
	logger.Init("./logs/test.log", "debug")

	// 3. 初始化数据库连接
	database.InitDB(config.Conf.MySQL, true)

	// 3.5 初始化 Redis 连接
	//    为什么也要初始化 Redis？
	//    -> GetUserById 先查 Redis 缓存，cache.GetUserCache() 内部调用
	//       redis.Get()，如果 Client 是 nil 会 panic
	redisCli.InitRedis(config.Conf.Redis)

	// 4. 自动迁移测试表（确保表结构是最新的）
	database.DB.AutoMigrate(&entity.User{})

	// 4.5 清理上次遗留的测试数据
	//    为什么只靠 cleanupUser(phone) 不够？
	//    -> phone 是动态生成的（时间戳），但 username 是硬编码的。
	//       上次测试用 phone=139A 创建了 "test_rpc_alice"，
	//       这次用 phone=139B 去 Delete Where phone=139A → 找不到旧记录！
	//       所以需要按 username 前缀做一次宽清理。
	database.DB.Unscoped().Where("username LIKE ?", "test_rpc_%").Delete(&entity.User{})

	// 5. 创建 Repository 和 Application（依赖注入）
	testUserRepo = db.NewUserRepo(database.DB)
	memberRepo := db.NewMemberRepo(database.DB)
	memberApp := application.NewMemberApp(memberRepo)
	smsSender := &sms.DevSender{}
	userApp := application.NewUserApp(smsSender, testUserRepo, memberApp)

	// 6. 创建 gRPC Server（使用 bufconn 内存连接）
	testLis = bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	userService := NewUserServiceServer(userApp)
	pb.RegisterUserServiceServer(grpcServer, userService)

	// 在 goroutine 中启动 gRPC Server
	// 为什么用 goroutine？
	// -> grpcServer.Serve() 是阻塞调用，不放 goroutine 里会卡住整个测试
	go func() {
		if err := grpcServer.Serve(testLis); err != nil {
			// 忽略 Server 关闭时的正常错误（grpc.ErrServerStopped）
			fmt.Printf("测试 gRPC Server 退出: %v\n", err)
		}
	}()

	// 7. 创建 gRPC Client（同样走 bufconn）
	//    为什么每次拨号都要 NewClient？
	//    -> bufconn 连接是临时的，不像 TCP 可以保持长连接
	//       我们在每个测试用例里调用 newTestClient() 创建独立连接
	conn, err := grpc.NewClient(
		"passthrough:///bufnet", // passthrough 让 gRPC 不做 DNS 解析
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return testLis.Dial() // 真正的连接走内存通道
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()), // 测试不走 TLS
	)
	if err != nil {
		logger.Fatal("测试 gRPC Client 创建失败")
	}
	testClient = pb.NewUserServiceClient(conn)

	// 8. 运行所有测试
	code := m.Run()

	// ====== Teardown：清理 ======
	for _, fn := range testCleanup {
		fn()
	}

	conn.Close()
	grpcServer.GracefulStop()
	testLis.Close()

	// os.Exit 传递测试退出码（0=全部通过，非0=有失败）
	// m.Run() 的返回值不能直接 return，必须 os.Exit
	// （这是 Go 测试框架的要求）
	os.Exit(code)
}

// ============================================================
// 测试用例
// ============================================================

func TestCreateUser_RPC(t *testing.T) {
	phone := fmt.Sprintf("139%08d", time.Now().UnixNano()%100000000)
	testCleanup = append(testCleanup, cleanupUser(phone))

	tests := []struct {
		name    string
		req     *pb.CreateUserRequest
		wantMsg string // 期望的响应消息
		wantErr bool   // 是否期望业务错误（code != 0）
	}{
		{
			name: "A1. 正常创建用户",
			req: &pb.CreateUserRequest{
				Name:     "test_rpc_alice",
				Phone:    phone,
				Email:    fmt.Sprintf("alice_%s@test.com", phone),
				Password: "Test123456",
			},
			wantMsg: "创建成功",
			wantErr: false,
		},
		{
			name: "A2. 重复手机号 -> 应返回手机号已注册",
			req: &pb.CreateUserRequest{
				Name:     "test_rpc_alice2",
				Phone:    phone, // 和 A1 相同的手机号
				Email:    fmt.Sprintf("alice2_%s@test.com", phone),
				Password: "Test123456",
			},
			wantMsg: "手机号已注册",
			wantErr: true,
		},
		{
			name: "A3. 重复用户名 -> 应返回用户名已存在",
			req: &pb.CreateUserRequest{
				Name:     "test_rpc_alice", // 和 A1 相同的用户名
				Phone:    fmt.Sprintf("139%08d", (time.Now().UnixNano()+1)%100000000),
				Email:    fmt.Sprintf("alice3_%s@test.com", phone),
				Password: "Test123456",
			},
			wantMsg: "用户名已存在",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := testClient.CreateUser(ctx, tt.req)
			if err != nil {
				t.Fatalf("RPC 调用失败（网络层错误）: %v", err)
			}

			if tt.wantErr {
				// 期望业务错误——code 不为 0
				if resp.Code == 0 {
					t.Errorf("期望业务错误但 code = 0, msg = %q", resp.Msg)
				}
			} else {
				if resp.Code != 0 {
					t.Errorf("期望成功但 code = %d, msg = %q", resp.Code, resp.Msg)
				}
			}

			// 用 contains 做子串匹配而不是精确比较
			// 为什么不用 ==？
			// -> 服务端返回的 Msg 可能附加了参数信息，如
			//    "手机号已注册[139xxxx]" vs 测试期望的 "手机号已注册"
			//    用 contains 做子串匹配更健壮，不会因为格式细节导致测试脆断
			if !contains(resp.Msg, tt.wantMsg) {
				t.Errorf("Msg = %q, 期望包含 %q", resp.Msg, tt.wantMsg)
			}
		})
	}
}

func TestGetUser_RPC(t *testing.T) {
	phone := fmt.Sprintf("138%08d", time.Now().UnixNano()%100000000)
	testCleanup = append(testCleanup, cleanupUser(phone))

	ctx := context.Background()

	// 先创建一个用户作为查询目标
	createResp, err := testClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name:     "test_rpc_query",
		Phone:    phone,
		Email:    fmt.Sprintf("query_%s@test.com", phone),
		Password: "Test123456",
	})
	if err != nil || createResp.Code != 0 {
		t.Fatalf("种子用户创建失败: err=%v, resp=%v", err, createResp)
	}

	t.Run("B1. 查询存在的用户", func(t *testing.T) {
		// 注意：CreateUserResponse 不直接返回 ID，我们需要通过 phone 查出来
		// 这里演示的是通过 ID 查询——实际需要先从 DB 拿到 ID
		// 改进方法：在 CreateUserResponse.Data 里加 Id 字段，或者用 GetUserByPhone

		// 我们先用已知的 phone 去 DB 查到 ID
		user, err := testUserRepo.GetByPhone(ctx, phone)
		if err != nil {
			t.Fatalf("通过手机号查 ID 失败: %v", err)
		}

		resp, err := testClient.GetUser(ctx, &pb.GetUserRequest{Id: int64(user.ID)})
		if err != nil {
			t.Fatalf("RPC 调用失败: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("查询失败: code=%d, msg=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil {
			t.Fatal("响应 Data 不应为 nil")
		}
		if resp.Data.Phone != phone {
			t.Errorf("Phone = %q, want %q", resp.Data.Phone, phone)
		}
	})

	t.Run("B2. 查询不存在的用户 -> 应返回错误", func(t *testing.T) {
		resp, err := testClient.GetUser(ctx, &pb.GetUserRequest{Id: 99999999})
		if err != nil {
			// 服务器当前对 "用户不存在" 返回 gRPC error
			// （user_server.go GetUser 中 return &pb.GetUserResponse{...}, err 同时返回了 resp 和 err，
			//   gRPC 框架优先传递 err 给客户端，resp 变为 nil）
			// TODO: 修复服务器后改用 resp.Code 判断
			return
		}
		// 如果服务器修复为纯业务响应，则检查 code 字段
		if resp.Code == 0 {
			t.Error("查询不存在的用户应返回非零 code")
		}
	})
}

func TestUpdateUser_RPC(t *testing.T) {
	phone := fmt.Sprintf("137%08d", time.Now().UnixNano()%100000000)
	testCleanup = append(testCleanup, cleanupUser(phone))

	ctx := context.Background()

	// 创建种子用户
	_, err := testClient.CreateUser(ctx, &pb.CreateUserRequest{
		Name:     "test_rpc_update",
		Phone:    phone,
		Email:    fmt.Sprintf("old_%s@test.com", phone),
		Password: "Test123456",
	})
	if err != nil {
		t.Fatalf("种子用户创建失败: %v", err)
	}

	// 查到 ID
	user, err := testUserRepo.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("通过手机号查 ID 失败: %v", err)
	}

	t.Run("C1. 更新用户名", func(t *testing.T) {
		resp, err := testClient.UpdateUser(ctx, &pb.UpdateUserRequest{
			Id:   int64(user.ID),
			Name: "updated_rpc_name",
		})
		if err != nil {
			t.Fatalf("RPC 调用失败: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("更新失败: code=%d, msg=%s", resp.Code, resp.Msg)
		}

		// 验证数据库中确实更新了
		// 这就是集成测试的价值——不只测 RPC 返回，还验证数据落盘
		updated, err := testUserRepo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("更新后查询失败: %v", err)
		}
		if updated.Username != "updated_rpc_name" {
			t.Errorf("数据库中的用户名 = %q, want %q", updated.Username, "updated_rpc_name")
		}
	})
}

// ============================================================
// 辅助函数
// ============================================================

// contains 检查字符串 s 是否包含子串 substr
// 为什么用 contains 而不是 ==？
// -> 服务端错误消息格式可能变化（如附加参数），contains 更宽容
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
