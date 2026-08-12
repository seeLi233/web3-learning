package entity

import "gorm.io/gorm"

type Address struct {
	gorm.Model
	UserID    uint   `gorm:"index;not null"`             // 关联用户
	Receiver  string `gorm:"type:varchar(50);not null"`  // 收件人
	Phone     string `gorm:"type:varchar(50);not null"`  // 联系电话
	Province  string `gorm:"type:varchar(50);not null"`  // 省
	City      string `gorm:"type:varchar(50);not null"`  // 市
	District  string `gorm:"type:varchar(50);not null"`  // 区
	Detail    string `gorm:"type:varchar(200);not null"` // 详细地址
	IsDefault bool   `gorm:"default:false"`              // 是否默认地址
}
