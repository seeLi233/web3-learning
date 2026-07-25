package model

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 创建测试用的 SQLite 内存数据库
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 SQLite 内存数据库失败: %v", err)
	}

	// 执行 AutoMigrate
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}
	return db
}

// ==================== 创建用户测试 ====================

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)

	t.Run("创建正常用户", func(t *testing.T) {
		user := &User{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "secret123",
			Age:      26,
		}

		result := db.Create(user)

		if result.Error != nil {
			t.Errorf("创建用户失败: %v", result.Error)
		}
		if user.ID != 1 {
			t.Errorf("期望 ID=1, 得到 ID=%d", user.ID)
		}
	})

	t.Run("BeforeCreate Hook: 用户名太短", func(t *testing.T) {
		user := &User{
			Username: "a", // 只有 1 个字符
			Email:    "short@example.com",
			Password: "secret",
		}

		result := db.Create(user)
		if result.Error == nil {
			t.Error("期望 Hook 返回错误（用户名太短），但没有错误")
		}
	})

	t.Run("默认 Status 为 active", func(t *testing.T) {
		user := &User{
			Username: "auto-status",
			Email:    "auto@example.com",
			Password: "secret",
		}

		db.Create(user)

		var found User
		db.First(&found, user.ID)
		if found.Status != "active" {
			t.Errorf("期望 Status='active', 得到 '%s'", found.Status)
		}
		fmt.Printf("✅ 用户 %s 创建，Status=%s (Hook 自动设置)\n", found.Username, found.Status)
	})
}

// ==================== 软删除测试 ⭐ ====================

func TestSoftDelete(t *testing.T) {
	db := setupTestDB(t)

	// 先创建测试用户
	user := &User{Username: "delete_me", Email: "delete@example.com", Password: "pw"}
	db.Create(user)

	t.Run("软删除后普通查询找不到", func(t *testing.T) {
		// 执行软删除
		if err := db.Delete(&User{}, user.ID).Error; err != nil {
			t.Fatalf("软删除失败: %v", err)
		}

		// 普通查询：应该找不到
		var found User
		err := db.First(&found, user.ID).Error
		if err == nil {
			t.Error("期望 ErrRecordNotFound，但找到了已删除的用户")
		}
		if err != gorm.ErrRecordNotFound {
			t.Logf("错误类型: %v", err)
		}
	})

	t.Run("Unscoped 查询可以找到已删除记录", func(t *testing.T) {
		var found User
		err := db.Unscoped().First(&found, user.ID).Error
		if err != nil {
			t.Errorf("Unscoped 查询失败: %v", err)
		}
		if !found.DeletedAt.Valid {
			t.Error("期望 DeletedAt 被设置，但仍然是 NULL")
		}
		fmt.Printf("✅ 已删除用户 DeletedAt=%v\n", found.DeletedAt.Time)
	})

	t.Run("Unscoped 硬删除真正移除数据", func(t *testing.T) {
		// 硬删除
		db.Unscoped().Delete(&User{}, user.ID)

		// Unscoped 也找不到了
		var found User
		err := db.Unscoped().First(&found, user.ID).Error
		if err != gorm.ErrRecordNotFound {
			t.Error("期望 ErrRecordNotFound，硬删除后应该彻底消失")
		}
	})
}

// ==================== Hook 测试 ====================

func TestBeforeUpdateHook(t *testing.T) {
	db := setupTestDB(t)

	user := &User{Username: "hook-test", Email: "hook@example.com", Password: "pw"}
	db.Create(user)

	t.Run("有效状态更新成功", func(t *testing.T) {
		user.Status = "inactive"
		result := db.Updates(user)
		if result.Error != nil {
			t.Errorf("期望更新成功，但失败: %v", result.Error)
		}
	})

	t.Run("无效状态被 BeforeUpdate Hook 拦截", func(t *testing.T) {
		// 关键：先改 struct 字段，再用 Updates 写入
		// 这样 BeforeUpdate hook 里的 u.Status 才会是 "super_admin"
		user.Status = "super_admin"
		result := db.Updates(user)
		if result.Error == nil {
			t.Error("期望 Hook 拦截无效状态，但没有错误")
		} else {
			fmt.Printf("✅ Hook 正确拦截: %v\n", result.Error)
		}
	})
}

// ==================== 综合测试 ====================

func TestCRUDComplete(t *testing.T) {
	db := setupTestDB(t)

	// 1. Create
	user := &User{Username: "full-test", Email: "full@example.com", Password: "encrypted", Age: 20}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if user.ID == 0 {
		t.Error("创建后 ID 应该被赋值")
	}

	// 2. Read
	var found User
	if err := db.First(&found, user.ID).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if found.Username != "full-test" {
		t.Errorf("查询结果不一致: 期望 full-test, 得到 %s", found.Username)
	}

	// 3. Update
	db.Model(&found).Update("age", 21)
	var updated User
	db.First(&updated, user.ID)
	if updated.Age != 21 {
		t.Errorf("更新失败: 期望 Age=21, 得到 %d", updated.Age)
	}

	// 4. Delete (软删除)
	db.Delete(&updated)
	var deletedSearch User
	err := db.First(&deletedSearch, user.ID).Error
	if err != gorm.ErrRecordNotFound {
		t.Error("软删除后普通查询应返回 ErrRecordNotFound")
	}

	// 5. Unscoped 查询确认数据还在
	var stillExists User
	db.Unscoped().First(&stillExists, user.ID)
	if stillExists.ID != user.ID {
		t.Error("Unscoped 应能查到已删除数据")
	}
}

// ==================== 唯一约束测试 ====================

func TestUniqueConstraint(t *testing.T) {
	db := setupTestDB(t)

	user1 := &User{Username: "unique-test", Email: "unique@example.com", Password: "pw"}
	db.Create(user1)

	t.Run("重复用户名", func(t *testing.T) {
		user2 := &User{Username: "unique-test", Email: "different@example.com", Password: "pw"}
		result := db.Create(user2)
		if result.Error == nil {
			t.Error("期望唯一约束违反错误，但创建成功了")
		}
	})

	t.Run("重复邮箱", func(t *testing.T) {
		user2 := &User{Username: "different", Email: "unique@example.com", Password: "pw"}
		result := db.Create(user2)
		if result.Error == nil {
			t.Error("期望唯一约束违反错误，但创建成功了")
		}
	})
}
