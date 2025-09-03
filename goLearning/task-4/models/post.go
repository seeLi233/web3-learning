package models

import (
	"time"
)

type Post struct {
	Id        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UserId    uint      `gorm:"not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserId" json:"user"`
	Comments  []Comment `gorm:"foreignKey:PostId" json:"comments"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdateAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
