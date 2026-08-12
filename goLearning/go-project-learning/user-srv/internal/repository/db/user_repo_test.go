package db

import (
	"context"
	"os"
	"testing"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/common/pkg/logger"
	redisCli "github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
)

// ============================================================
// 仓库层集成测试
//
// 前置条件：MySQL 必须运行（docker-compose 启动的 3307 端口）
// 为什么测仓库层？
// → 仓库层是离数据库最近的代码层，这里的 bug 会导致数据写错、读不到、泄漏
//   而且仓库层测试跑得最快（不启动 gRPC Server），适合频繁运行
//
// 为什么每个测试独立建数据？
// → 测试之间不能有顺序依赖——如果"创建用户"测试失败，"查询用户"测试也要能独立跑
//   每个测试开始前清理旧数据，结束后清理新数据，保证隔离
// ============================================================

// cleanTestUsers 清理测试产生的用户数据
//
// 为什么要清理？
// → 测试产生的脏数据会干扰后续测试运行（比如唯一约束冲突）
//
//	每次测试结束都做 teardown，保证下次跑还是干净环境
func cleanTestUsers(t *testing.T, phone string) {
	t.Helper()
	// GORM 的 Unscoped 是物理删除（硬删除），去掉软删除的默认条件
	// 为什么用 Unscoped？User entity 嵌入了 gorm.Model（含 DeletedAt 字段），
	// GORM 默认会加 deleted_at IS NULL 条件。测试清理需要真正删除行，不管软删状态
	database.DB.Unscoped().Where("phone = ?", phone).Delete(&entity.User{})
}

// TestMain 初始化数据库连接，让仓库层测试能独立运行
//
// 为什么需要 TestMain？
// → database.DB 是全局变量，必须通过 database.InitDB() 初始化。
//
//	没有 TestMain 时 database.DB == nil，调用 database.DB.Unscoped() 直接 panic。
//	和 server/user_server_test.go 的 TestMain 职责相同——
//	区别是这里不需要启动 gRPC Server，只需要数据库。
func TestMain(m *testing.M) {
	// 0. 切回项目根目录
	//    为什么需要 chdir？
	//    → go test 把 CWD 设为当前包目录（internal/repository/db/），
	//      但 config.Init() 用 viper.AddConfigPath("./configs") 相对路径找配置，
	//      模块根目录是 user-srv/（上 3 级），configs/ 在 user-srv/configs/
	os.Chdir("../../..")

	// 1. 初始化配置
	config.Init()

	// 2. 初始化日志
	logger.Init("./logs/test.log", "debug")

	// 3. 初始化数据库连接
	database.InitDB(config.Conf.MySQL, true)

	// 3.5 初始化 Redis 连接
	//    为什么也要初始化 Redis？
	//    → GetUserById 先查 Redis 缓存，cache.GetUserCache() 内部调用
	//      redis.Get()，如果 Client 是 nil 会直接 panic。
	//      即使 Redis 不可用，至少初始化 Client 让代码走到查 DB 的分支。
	redisCli.InitRedis(config.Conf.Redis)

	// 4. 自动迁移测试表
	database.DB.AutoMigrate(&entity.User{})

	// 5. 运行所有测试
	code := m.Run()

	// 6. 传递退出码
	os.Exit(code)
}

func TestCreateUser(t *testing.T) {
	repo := NewUserRepo(database.DB)
	// 在 TestCreateUser 级别统一清理，避免子测试间的 defer 互相干扰
	// 为什么不用每个子测试各自 defer？
	// -> Go 的 t.Run 子测试顺序执行，A1 的 defer 会在 A2 开始前触发，
	//    A1 数据被清掉后 A2（同手机号）就没有冲突了 → 测试逻辑失效。
	//    统一清理让 A1 的数据在整个 TestCreateUser 期间始终存在。
	cleanTestUsers(t, "13801000001")
	defer cleanTestUsers(t, "13801000001")
	cleanTestUsers(t, "13801000003")
	defer cleanTestUsers(t, "13801000003")

	tests := []struct {
		name    string
		user    entity.User
		wantErr bool
	}{
		{
			name: "A1. 正常创建用户",
			user: entity.User{
				Username: "test_alice",
				Password: "hashed_password_123",
				Phone:    "13801000001",
				Email:    "alice@test.com",
				Status:   1,
			},
			wantErr: false,
		},
		{
			name: "A2. 创建重复手机号的用户 → 应报唯一约束冲突",
			user: entity.User{
				Username: "test_alice2",
				Password: "hashed_password_456",
				Phone:    "13801000001",
				Email:    "alice2@test.com",
				Status:   1,
			},
			wantErr: true, // 预期报错——phone 是 unique 约束
		},
		{
			name: "A3. 创建重复邮箱的用户 → 应报唯一约束冲突",
			user: entity.User{
				Username: "test_alice3",
				Password: "hashed_password_789",
				Phone:    "13801000003",
				Email:    "alice@test.com",
				Status:   1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := repo.Create(ctx, &tt.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && !tt.wantErr {
				// 验证创建后的用户 ID 是否被数据库自动填充
				// GORM Create 会自动回填主键 ID
				if tt.user.ID == 0 {
					t.Error("创建成功后 ID 应为非零值（GORM 应自动回填主键）")
				}
			}
		})
	}
}

func TestGetUserById(t *testing.T) {
	repo := NewUserRepo(database.DB)
	ctx := context.Background()

	// 清理上次测试可能残留的数据
	cleanTestUsers(t, "13810000099")

	// 先创建一个用户作为查询目标
	seed := &entity.User{
		Username: "test_query_user",
		Password: "pwd",
		Phone:    "13810000099",
		Email:    "query@test.com",
		Status:   1,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatalf("种子数据创建失败: %v", err)
	}
	defer cleanTestUsers(t, seed.Phone)

	t.Run("B1. 查询存在的用户", func(t *testing.T) {
		user, err := repo.GetByID(ctx, seed.ID)
		if err != nil {
			t.Fatalf("GetUserById() 不应报错: %v", err)
		}
		if user.Username != seed.Username {
			t.Errorf("用户名 = %q, want %q", user.Username, seed.Username)
		}
		if user.Phone != seed.Phone {
			t.Errorf("手机号 = %q, want %q", user.Phone, seed.Phone)
		}
	})

	t.Run("B2. 查询不存在的用户 → 应返回错误", func(t *testing.T) {
		// 为什么测不存在的情况？
		// → 面试高频：gRPC 的 NOT_FOUND 错误码处理
		//   First() 未找到时返回 gorm.ErrRecordNotFound
		_, err := repo.GetByID(ctx, 999999999)
		if err == nil {
			t.Error("查询不存在的 ID 应返回错误")
		}
	})
}

func TestGetUserByPhone(t *testing.T) {
	repo := NewUserRepo(database.DB)
	ctx := context.Background()

	cleanTestUsers(t, "13910000001")

	seed := &entity.User{
		Username: "test_phone_user",
		Password: "pwd",
		Phone:    "13910000001",
		Email:    "phone@test.com",
		Status:   1,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatalf("种子数据创建失败: %v", err)
	}
	defer cleanTestUsers(t, seed.Phone)

	t.Run("C1. 根据手机号查询", func(t *testing.T) {
		user, err := repo.GetByPhone(ctx, seed.Phone)
		if err != nil {
			t.Fatalf("GetUserByPhone() 不应报错: %v", err)
		}
		if user.Email != seed.Email {
			t.Errorf("邮箱 = %q, want %q", user.Email, seed.Email)
		}
	})
}

func TestUpdateUser(t *testing.T) {
	repo := NewUserRepo(database.DB)
	ctx := context.Background()

	cleanTestUsers(t, "13700000001")

	seed := &entity.User{
		Username: "test_update_user",
		Password: "pwd",
		Phone:    "13700000001",
		Email:    "old@test.com",
		Status:   1,
	}
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatalf("种子数据创建失败: %v", err)
	}
	defer cleanTestUsers(t, seed.Phone)

	t.Run("D1. 更新用户名和邮箱", func(t *testing.T) {
		seed.Username = "updated_name"
		seed.Email = "new@test.com"
		err := repo.Update(ctx, seed)
		if err != nil {
			t.Fatalf("UpdateUser() 不应报错: %v", err)
		}

		// 重新查询验证更新是否持久化到数据库
		// 为什么重新查而不是直接读 seed？
		// → seed 是内存中的对象，更新后它的字段已经变了
		//   但我们要验证的是数据库里的值真的被改写了——这是集成测试的核心价值
		updated, err := repo.GetByID(ctx, seed.ID)
		if err != nil {
			t.Fatalf("更新后查询失败: %v", err)
		}
		if updated.Username != "updated_name" {
			t.Errorf("数据库中用户名 = %q, want %q", updated.Username, "updated_name")
		}
		if updated.Email != "new@test.com" {
			t.Errorf("数据库中邮箱 = %q, want %q", updated.Email, "new@test.com")
		}
	})
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()

	cleanTestUsers(t, "13600000009")

	seed := &entity.User{
		Username: "test_delete_user",
		Password: "pwd",
		Phone:    "13600000009",
		Email:    "delete@test.com",
		Status:   1,
	}
	repo := NewUserRepo(database.DB)
	if err := repo.Create(ctx, seed); err != nil {
		t.Fatalf("种子数据创建失败: %v", err)
	}
	// 注意：DeleteUser 是软删除（gorm.Model 的 DeletedAt），这里用 Unscoped 清理
	defer database.DB.Unscoped().Delete(seed)

	t.Run("E1. 删除用户（软删除）", func(t *testing.T) {
		err := repo.Delete(ctx, seed.ID)
		if err != nil {
			t.Fatalf("DeleteUser() 不应报错: %v", err)
		}

		// 软删除后，正常的 First 查不到（GORM 自动加 deleted_at IS NULL）
		_, err = repo.GetByID(ctx, seed.ID)
		if err == nil {
			t.Error("软删除后 GetUserById 应返回错误（GORM 自动过滤已软删记录）")
		}
	})
}
