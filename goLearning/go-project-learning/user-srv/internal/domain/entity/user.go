package entity

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"type:varchar(50);unique;not null" json:"username"`
	Password string `gorm:"type:varchar(255);not null;size:64" json:"password"`
	Phone    string `gorm:"type:varchar(20);unique;not null" json:"phone"`
	Email    string `gorm:"type:varchar(100);unique;not null" json:"email"`
	Status   int    `gorm:"type:int;default:1" json:"status"` // 1: active, 0: inactive

	// 一个用户存在多个订单信息
	// Orders []Order `gorm:"foreignKey:UserId"`
}
