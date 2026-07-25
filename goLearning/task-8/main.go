package main

import (
	"fmt"
	"log"
	"task8/config"
	"task8/model"
	"task8/repository"
)

func main() {
	// 1. 连接数据库（演示用 SQLite，不需要装 PostgreSQL）
	//    生产环境改用 InitPostgresFromEnv()
	db, err := config.InitSQLite("task8.db")
	if err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功")

	// 2. AutoMigrate: 自动根据 struct 创建/更新表
	if err := db.AutoMigrate(&model.User{}); err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v", err)
	}
	fmt.Println("✅ 数据库表迁移完成")

	// 3. 创建 Repository
	userRepo := repository.NewUserRepository(db)

	// 4. 演示 CRUD 操作
	fmt.Println("\n========== CRUD 操作演示 ==========")

	// ---------- Create ----------
	fmt.Println("\n📝 创建用户...")
	users := []model.User{
		{Username: "alice", Email: "alice@example.com", Password: "hashed_pass_1", Age: 25},
		{Username: "bob", Email: "bob@example.com", Password: "hashed_pass_2", Age: 30},
		{Username: "charlie", Email: "charlie@example.com", Password: "hashed_pass_3", Age: 35},
	}
	for i := range users {
		if err := userRepo.Create(&users[i]); err != nil {
			log.Printf("⚠️ 创建用户失败: %v", err)
		} else {
			fmt.Printf("  ✅ 创建用户: %s (ID=%d)\n", users[i].Username, users[i].ID)
		}
	}

	// ---------- Read ----------
	fmt.Println("\n🔍 查询用户...")
	user, err := userRepo.GetByID(1)
	if err != nil {
		log.Printf("⚠️ %v", err)
	} else {
		fmt.Printf("  ✅ 查到用户: ID=%d, Username=%s, Email=%s, Age=%d\n",
			user.ID, user.Username, user.Email, user.Age)
	}

	// ---------- Update ----------
	fmt.Println("\n✏️ 更新用户...")
	user.Age = 26
	if err := userRepo.Update(user); err != nil {
		log.Printf("⚠️ %v", err)
	} else {
		fmt.Printf("  ✅ 更新成功: ID=%d, NewAge=%d\n", user.ID, user.Age)
	}

	// ---------- Page Query ----------
	fmt.Println("\n📄 分页查询...")
	allUsers, total, _ := userRepo.List(1, 10)
	fmt.Printf("  ✅ 共 %d 条记录, 当前页 %d 条\n", total, len(allUsers))
	for _, u := range allUsers {
		fmt.Printf("     ID=%d, Username=%s\n", u.ID, u.Username)
	}

	// ---------- Soft Delete ----------
	fmt.Println("\n🗑️ 软删除用户 ID=2...")
	if err := userRepo.Delete(2); err != nil {
		log.Printf("⚠️ %v", err)
	} else {
		fmt.Println("  ✅ 软删除成功（deleted_at 已更新）")
	}

	// 删除后查询：3 个用户 → 删除 1 个 → 只能查到 2 个
	activeUsers, _, _ := userRepo.List(1, 10)
	fmt.Printf("  📊 删除后剩余用户数: %d (原 3 个，删除 1 个)\n", len(activeUsers))

	// ---------- 查询已删除的用户 ----------
	fmt.Println("\n🔎 查询已删除的用户（Unscoped）...")
	deletedUsers, _ := userRepo.GetDeletedUsers()
	fmt.Printf("  ✅ 已删除用户数: %d\n", len(deletedUsers))
	for _, u := range deletedUsers {
		fmt.Printf("     ID=%d, Username=%s, DeletedAt=%v\n", u.ID, u.Username, u.DeletedAt.Time)
	}

	// ---------- Restore ----------
	fmt.Println("\n🔄 恢复已删除的用户...")
	if err := userRepo.Restore(2); err != nil {
		log.Printf("⚠️ %v", err)
	} else {
		fmt.Println("  ✅ 恢复成功")
	}

	// 恢复后再查
	activeUsers, _, _ = userRepo.List(1, 10)
	fmt.Printf("  📊 恢复后用户数: %d\n", len(activeUsers))

	// ---------- Transaction ----------
	fmt.Println("\n💱 事务：Alice(25) → Bob(30) 转账 5 积分（用 age 模拟）...")
	if err := userRepo.TransferCredits(1, 2, 5); err != nil {
		log.Printf("⚠️ 转账失败: %v", err)
	} else {
		alice, _ := userRepo.GetByID(1)
		bob, _ := userRepo.GetByID(2)
		fmt.Printf("  ✅ 转账成功! Alice: %d, Bob: %d\n", alice.Age, bob.Age)
	}

	fmt.Println("\n🎉 所有操作演示完成！")
}
