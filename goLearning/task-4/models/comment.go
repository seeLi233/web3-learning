package models

import "time"

type Comment struct {
	Id        uint      `gorm:"primaryKey" json:"id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UserId    uint      `gorm:"not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserId" json:"user"`
	PostId    uint      `gorm:"not null;constraint:OnDelete:CASCADE" json:"post_id"`
	Post      Post      `gorm:"foreignKey:PostId" json:"post"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"create_at"`
}
