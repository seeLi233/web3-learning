package entity

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CouponTemplate 优惠券模板
type CouponTemplate struct {
	gorm.Model
	Name           string          `gorm:"type:varchar(50);not null" json:"name"`
	Type           int             `gorm:"type:tinyint;not null;default:1" json:"type"` // 1=满减券 2=折扣券 3=无门槛券
	DiscountAmount decimal.Decimal `gorm:"type:decimal(10,2);not null;default:0" json:"discount_amount"`
	MinAmount      decimal.Decimal `gorm:"type:decimal(10,2);not null;default:0" json:"min_amount"` // 最低消费金额
	StartTime      time.Time       `gorm:"not null" json:"start_time"`
	EndTime        time.Time       `gorm:"not null" json:"end_time"`
	TotalCount     int             `gorm:"type:int;not null;default:-1" json:"total_count"` // -1=无限制
	ClaimedCount   int             `gorm:"type:int;not null;default:0" json:"claimed_count"`
	PerUserLimit   int             `gorm:"type:int;not null;default:1" json:"per_user_limit"`
	Status         int             `gorm:"type:tinyint;not null;default:1" json:"status"` // 1=启用 0=禁用
}

// UserCoupon 用户优惠券
type UserCoupon struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	UserID     uint           `gorm:"type:bigint unsigned;not null;index" json:"user_id"`
	TemplateID uint           `gorm:"type:bigint unsigned;not null;index" json:"template_id"`
	Status     int            `gorm:"type:tinyint;not null;default:1" json:"status"` // 1=未使用 2=已使用 3=已过期
	ClaimedAt  time.Time      `gorm:"not null" json:"claimed_at"`
	UsedAt     *time.Time     `json:"used_at"`
	OrderID    string         `gorm:"type:varchar(64);default:''" json:"order_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Template   CouponTemplate `gorm:"foreignKey:TemplateID" json:"template"` // 关联查询
}
