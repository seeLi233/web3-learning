package entity

import "gorm.io/gorm"

type RiskConfig struct {
	gorm.Model
	RuleKey     string `gorm:"type:varchar(50);uniqueIndex;not null" json:"rule_key"` // uniqueIndex 给该字段创建唯一索引  效果：整张表里这个字段的值不能重复，重复插入会直接报错
	RuleValue   string `gorm:"type:varchar(100);not null" json:"rule_value"`
	Description string `gorm:"type:varchar(255)" json:"description"`
	Status      bool   `gorm:"not null;default:true" json:"status"`
}
