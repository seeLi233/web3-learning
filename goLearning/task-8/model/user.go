package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string         `gorm:"column:username;type:varchar(50);not null;uniqueIndex;comment:用户名" json:"username"`
	Email     string         `gorm:"column:email;type:varchar(100);not null;uniqueIndex;comment:邮箱" json:"email"`
	Password  string         `gorm:"column:password;type:varchar(255);not null;comment:密码哈希" json:"-"` // json:"-" 序列化时隐藏密码
	Age       int            `gorm:"column:age;type:int;default:0;comment:年龄" json:"age"`
	Status    string         `gorm:"column:status;type:varchar(20);default:'active';index;comment:状态(active/inactive/banned)" json:"status"`
	CreatedAt time.Time      `gorm:"autoCreateTime;comment:创建时间" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime;comment:更新时间" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:软删除时间" json:"deleted_at,omitempty"`
}

// TableName 自定义表名（默认是 struct 名的蛇形复数 users）
func (User) TableName() string {
	return "users"
}

// ==================== GORM Hooks ====================

// BeforeCreate 创建前钩子：验证用户名长度
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if len(u.Username) < 2 {
		return fmt.Errorf("用户名至少需要 2 个字符")
	}
	if u.Status == "" {
		u.Status = "active" // 默认状态
	}
	return nil
}

// AfterCreate 创建后钩子：打印日志
func (u *User) AfterCreate(tx *gorm.DB) error {
	fmt.Printf("✅ 用户创建成功: ID=%d, Username=%s\n", u.ID, u.Username)
	return nil
}

// BeforeUpdate 更新前钩子：验证状态
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	validStatus := map[string]bool{"active": true, "inactive": true, "banned": true}
	if !validStatus[u.Status] {
		return fmt.Errorf("无效的用户状态: %s", u.Status)
	}
	return nil
}

// AfterFind 查询后钩子：脱敏处理（示例）
func (u *User) AfterFind(tx *gorm.DB) error {
	// 比如隐藏邮箱的部分字符
	// u.Email = maskEmail(u.Email)
	return nil
}
