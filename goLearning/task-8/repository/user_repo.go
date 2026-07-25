package repository

import (
	"errors"
	"fmt"
	"task8/model"

	"gorm.io/gorm"
)

// UserRepository 用户数据访问层
// 采用仓储模式（Repository Pattern），封装所有数据操作
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造函数（依赖注入）
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// ==================== 基础 CRUD ====================

// Create 创建用户
func (r *UserRepository) Create(user *model.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	return nil
}

// GetByID 根据 ID 查找用户
// ⭐ 注意：GORM 软删除会自动过滤 deleted_at IS NOT NULL 的记录
func (r *UserRepository) GetByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在: ID=%d", id)
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// GetByEmail 根据邮箱查找用户
func (r *UserRepository) GetByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在: email=%s", email)
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// Update 更新用户
// 用 Save 会更新所有字段，用 Updates 只更新非零值字段
func (r *UserRepository) Update(user *model.User) error {
	result := r.db.Model(user).Updates(map[string]interface{}{
		"username": user.Username,
		"email":    user.Email,
		"age":      user.Age,
		"status":   user.Status,
	})
	if result.Error != nil {
		return fmt.Errorf("更新用户失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在: ID=%d", user.ID)
	}
	return nil
}

// Delete 软删除用户
// GORM 自动将 DELETE 转换为 UPDATE SET deleted_at = NOW()
func (r *UserRepository) Delete(id int) error {
	result := r.db.Delete(&model.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除用户失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在: ID=%d", id)
	}
	return nil
}

// HardDelete 硬删除（真正从数据库删除）
func (r *UserRepository) HardDelete(id int) error {
	result := r.db.Unscoped().Delete(&model.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("硬删除用户失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在: ID=%d", id)
	}
	return nil
}

// ==================== 高级查询 ====================

// List 分页查询用户列表
func (r *UserRepository) List(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	// 先查总数
	if err := r.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询总数失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := r.db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("分页查询失败: %w", err)
	}

	return users, total, nil
}

// ListActive 查询活跃用户
func (r *UserRepository) ListActive() ([]model.User, error) {
	var users []model.User
	err := r.db.Where("status = ?", "active").Find(&users).Error
	return users, err
}

// ==================== 事务操作 ⭐ ====================

// TransferCredits 事务示例：给用户 A 减分，给用户 B 加分
func (r *UserRepository) TransferCredits(fromID, toID uint, amount int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 注意：在事务内部使用 tx，不是 r.db！

		// 1. 检查用户 A 的积分是否足够（假设有一个 credits 字段）
		// 这里用年龄模拟，实际项目中应该是独立的积分系统
		var fromUser model.User
		if err := tx.First(&fromUser, fromID).Error; err != nil {
			return fmt.Errorf("转出用户不存在: ID=%d", fromID)
		}
		if fromUser.Age < amount {
			return fmt.Errorf("积分不足: 需要 %d, 当前 %d", amount, fromUser.Age)
		}

		// 2. 执行扣减和增加
		if err := tx.Model(&fromUser).Update("age", fromUser.Age-amount).Error; err != nil {
			return err // 自动回滚
		}

		var toUser model.User
		if err := tx.First(&toUser, toID).Error; err != nil {
			return fmt.Errorf("转入用户不存在: ID=%d", toID)
		}
		if err := tx.Model(&toUser).Update("age", toUser.Age+amount).Error; err != nil {
			return err // 自动回滚
		}

		return nil
	})
}

// ==================== 软删除恢复 ====================

// Restore 恢复被软删除的用户
func (r *UserRepository) Restore(id uint) error {
	// Unscoped() 找到被软删除的记录，然后清空 DeletedAt
	var user model.User
	err := r.db.Unscoped().First(&user, id)
	if err != nil {
		return fmt.Errorf("未找到该用户（包括已删除的）: ID=%d", id)
	}
	if !user.DeletedAt.Valid {
		return fmt.Errorf("用户未被删除: ID=%d", id)
	}

	// 恢复：将 deleted_at 设回 NULL（SQLite 语法）
	if err := r.db.Unscoped().Model(&user).Update("deleted_at", nil).Error; err != nil {
		return fmt.Errorf("恢复用户失败: %w", err)
	}
	return nil
}

// GetDeletedUsers 查询所有已删除的用户（演示 Unscoped）
func (r *UserRepository) GetDeletedUsers() ([]model.User, error) {
	var users []model.User
	err := r.db.Unscoped().Where("deleted_at IS NOT NULL").Find(&users).Error
	return users, err
}
