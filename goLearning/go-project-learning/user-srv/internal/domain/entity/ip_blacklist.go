package entity

import (
	"time"

	"gorm.io/gorm"
)

type IpBlackList struct {
	gorm.Model
	IP       string     `gorm:"type:varchar(45);not null" json:"ip"`
	Reason   string     `gorm:"type:varchar(200)" json:"reason"`
	Source   string     `gorm:"type:varchar(50)" json:"source"`
	UserID   uint       `gorm:"type:bigint unsigned" json:"user_id"` // unsigned - 无符号标识，去掉负数区间
	Status   bool       `gorm:"default:true" json:"status"`
	Deadline *time.Time `json:"deadline"`
}
