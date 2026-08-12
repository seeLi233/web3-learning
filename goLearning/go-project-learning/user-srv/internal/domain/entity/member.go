package entity

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type MemberLevel struct {
	gorm.Model
	LevelName   string          `gorm:"type:varchar(20);not null" json:"level_name"`
	LevelValue  int             `gorm:"type:int;uniqueIndex;not null" json:"level_value"`
	MinGrowth   int             `gorm:"type:int;not null;default:0" json:"min_growth"`
	MaxGrowth   int             `gorm:"type:int;not null;default:0" json:"max_growth"`
	Discount    decimal.Decimal `gorm:"type:decimal(3,2);not null;default:1.00" json:"discount"`
	Icon        string          `gorm:"type:varchar(255);default:''" json:"icon"`
	Description string          `gorm:"type:varchar(500);default:''" json:"description"`
	Status      int             `gorm:"type:tinyint;not null;default:1" json:"status"`
}

type MemberInfo struct {
	gorm.Model
	UserID      uint        `gorm:"type:bigint unsigned;uniqueIndex;not null" json:"user_id"`
	LevelID     uint        `gorm:"type:bigint unsigned;not null" json:"level_id"`
	GrowthValue int         `gorm:"type:int;not null;default:0" json:"growth_value"`
	TotalGrowth int         `gorm:"type:int;not null;default:0" json:"total_growth"`
	LevelUpTime *time.Time  `gorm:"column:level_up_time" json:"level_up_time"`
	Level       MemberLevel `gorm:"foreignKey:LevelID" json:"level"` // 关联查询
	User        User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type MemberGrowthLog struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"type:bigint unsigned;not null;index" json:"user_id"`
	ChangeValue int       `gorm:"type:int;not null" json:"change_value"`
	SourceType  string    `gorm:"type:varchar(30);not null" json:"source_type"`
	SourceID    string    `gorm:"type:varchar(64);default:''" json:"source_id"`
	Description string    `gorm:"type:varchar(200);default:''" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
