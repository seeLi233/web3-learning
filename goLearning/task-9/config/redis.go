package config

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// InitRedis 初始化 Redis 连接
// 生产环境：配置应该从环境变量读取
func InitRedis() (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "localhost:6380", // Redis 地址
		Password:     "",               // 密码（本机开发没有）
		DB:           0,                // 使用默认 DB 0
		PoolSize:     10,               // 最大连接数
		MinIdleConns: 3,                // 最小空闲连接
		MaxRetries:   30,               // 失败重试
		DialTimeout:  5 * time.Second,  // 连接超时
		ReadTimeout:  3 * time.Second,  // 读超时
		WriteTimeout: 3 * time.Second,  // 写超时
	})

	// Ping 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	fmt.Println("✅ Redis 连接成功")
	return rdb, nil
}
