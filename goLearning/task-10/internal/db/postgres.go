package db

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(host, port, user, pass, dbname string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		host, port, user, pass, dbname,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn), // 只打印警告，不刷屏
	})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	// 自动迁移（开发环境用，生产环境请用 migration 文件）
	err = db.AutoMigrate(&EventLog{})
	if err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}

	log.Println("✅ PostgreSQL 连接成功 + 表已就绪")
	return db, nil
}
