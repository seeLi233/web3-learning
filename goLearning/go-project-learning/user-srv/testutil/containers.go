package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

// TestInfra 聚合测试所需的全部真实依赖
// 为什么用 struct 聚合而不是返回一堆零散变量？
// → E2E 测试需要"多个依赖一起用"，打包成一个结构体，测试里一句 s, _ := Setup() 就绪，
//
//	同时 TearDown() 集中清理，避免每个测试文件重复写关闭逻辑
type TestInfra struct {
	MySQL      config.MysqlConfig // MySQL 连接串（GORM 直接吃这个）
	Redis      config.RedisConfig // Redis 连接串（go-redis 直接吃这个）
	containers []testcontainers.Container
}

// Setup 启动 MySQL + Redis 两个真实容器，返回就绪的基础设施
//
// 为什么用 context.WithTimeout 包一层超时？
// → 拉镜像可能很慢（首次几百 MB），没有超时测试会无限卡住
//
//	CI 里网络差时也能快速失败并给出明确报错，而不是 hang 死
func Setup(ctx context.Context) (*TestInfra, error) {
	// 5 分钟：首次拉 mysql:8.0(~500MB) + redis:7-alpine + ryuk 镜像可能要 1-3 分钟，
	// 5 秒（原值）连 ryuk 镜像都拉不下来就超时，报 "reaper: context deadline exceeded"
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	infra := &TestInfra{containers: []testcontainers.Container{}}

	// ---------- 1. 启动 MySQL ----------
	// 为什么用 mysql.Run 而不是 testcontainers.GenericContainer？
	// → 官方 module 封装好了镜像参数、默认端口 3306、等待策略，几行代码搞定
	//   自己拼 GenericContainer 要手写 Image/Env/PortBind/WaitFor，容易错
	mysqlC, err := mysql.Run(ctx, "mysql:8.0",
		mysql.WithDatabase("e2e_test"),     // 自动创建名为 e2e_test 的库
		mysql.WithUsername("root"),         // 用户名
		mysql.WithPassword("e2e_password"), // 密码（测试专用，不碰生产）
		// 等待策略：容器"就绪"的标准是能成功建立 MySQL 连接，而不是"进程起来了"
		// 为什么不用 wait.ForLog("ready for connections")？
		// → ForListeningPort 只测端口通，ForLog 等日志字样；二者都可能"提前返回"
		//    WithUsername+WithPassword 内部会真正连一次库，确保 SQL 层面就绪
	)
	if err != nil {
		return nil, fmt.Errorf("启动 MySQL 失败: %w", err)
	}

	// 为什么用 mysqlC.Host() + MappedPort() 而不是 ConnectionString() 再拆分？
	// → Host() 返回容器映射到宿主机的 IP，MappedPort("3306/tcp") 返回随机端口
	//   二者直接组成 config.MysqlConfig，省掉 DSN 字符串的解析环节
	mysqlHost, _ := mysqlC.Host(ctx)
	mysqlPort, _ := mysqlC.MappedPort(ctx, "3306/tcp")
	infra.MySQL = config.MysqlConfig{
		Host:     mysqlHost,
		Port:     int(mysqlPort.Num()),
		Username: "root",
		Password: "e2e_password",
		Dbname:   "e2e_test",
		Charset:  "utf8mb4",
	}

	// // ConnectionString 返回形如 root:e2e_password@tcp(127.0.0.1:随机端口)/e2e_test
	// // 为什么端口是随机的？
	// // → 容器映射到宿主机的随机空闲端口，避免和本机已占用的 3306 冲突，也支持并行测试
	// mysqlDSN, err := mysqlC.ConnectionString(ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("获取 MySQL DSN 失败: %w", err)
	// }
	// // GORM 需要这个参数，否则会报 "invalid connection: this driver doesn't support ..."
	// // 为什么加 parseTime=true&loc=Local？
	// // → MySQL 的 DATETIME 默认按 UTC 字符串返回，Go 的 time.Time 需要本地时区
	// //    parseTime=true 让 driver 把 DATETIME 直接解析成 time.Time，避免时区错乱
	// mysqlDSN = mysqlDSN + "?parseTime=true&loc=Local"

	// infra.MySQLDSN = mysqlDSN
	// 必须把 mysqlC 加入 containers 列表，否则 TearDown 不会清理 MySQL 容器（泄漏）
	infra.containers = append(infra.containers, mysqlC)

	// ---------- 2. 启动 Redis ----------
	// 为什么 E2E 还要真 Redis？
	// → 你的 register 处理器里有 redis.SlidingWindowLimit 限流，
	//   没有真 Redis 这步直接 error，链路断在中间。mock 限流又测不到真实行为
	redisC, err := redis.Run(ctx, "redis:7-alpine",
		// 等待策略：发送 PING 命令收到 PONG 才算就绪
		// 为什么用 wait.ForLog("Ready to accept connections")？
		// → Redis 启动极快，日志出现这行时一定已能接受命令，比轮询更简单可靠
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	if err != nil {
		// 失败时清理已启动的 MySQL，避免容器泄漏
		infra.TearDown(ctx)
		return nil, fmt.Errorf("启动 Redis 失败: %w", err)
	}

	redisHost, _ := redisC.Host(ctx)
	redisPort, _ := redisC.MappedPort(ctx, "6379/tcp")
	infra.Redis = config.RedisConfig{
		Host:     redisHost,
		Port:     int(redisPort.Num()),
		DB:       0,
		PoolSize: 10,
	}

	// MustConnectionString 获取 host:port
	// redisHost, err := redisC.ConnectionString(ctx)
	// if err != nil {
	// 	infra.TearDown(ctx)
	// 	return nil, fmt.Errorf("获取 Redis 连接串失败: %w", err)
	// }

	// infra.RedisDSN = redisHost
	infra.containers = append(infra.containers, redisC)

	return infra, nil
}

// TearDown 关闭所有容器，释放资源
//
// 为什么显式 Terminate 而不等测试进程退出自动回收？
// → testcontainers 默认有 Ryuk 守护进程会在测试结束后回收，但显式 Terminate
//
//	让资源立即释放，本地反复跑测试时不会堆积几百个僵尸容器吃掉内存
func (i *TestInfra) TearDown(ctx context.Context) {
	for _, c := range i.containers {
		_ = c.Terminate(ctx)
	}
	i.containers = nil
}
