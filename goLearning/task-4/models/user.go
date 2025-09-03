package models

import "time"

// users 表：存储用户信息，包括 id 、 username 、 password 、 email 等字段。
type User struct {
	Id        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"not null" json:"-"`
	Email     string    `gorm:"size:200;uniqueIndex;not null" json:"email"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdateAt  time.Time `gorm:"autoUpdateTime" json:"update_at"`
	Posts     []Post    `gorm:"foreignKey:UserId" json:"posts"`
}
