package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/redis"
)

const (
	codePrefix  = "sms:code:"  // sms:code:{phone} -> 验证码
	limitPrefix = "sms:limit:" // sms:limit:{phone} -> 发送频率限制
)

// SetCode 存储验证码，5分钟过期
func SetCode(ctx context.Context, phone, code string) error {
	key := fmt.Sprintf("%s%s", codePrefix, phone)
	return redis.Set(ctx, key, code, 5*time.Minute)
}

// GetCode 获取验证码
func GetCode(ctx context.Context, phone string) (string, error) {
	key := fmt.Sprintf("%s%s", codePrefix, phone)
	return redis.Get(ctx, key)
}

// DeleteCode 删除已使用的验证码
func DeleteCode(ctx context.Context, phone string) error {
	key := fmt.Sprintf("%s%s", codePrefix, phone)
	return redis.Del(ctx, key)
}

// SetLimit 设置发送频率限制， 60s 过期
func SetLimit(ctx context.Context, phone string) error {
	key := fmt.Sprintf("%s%s", limitPrefix, phone)
	return redis.Set(ctx, key, "1", 60*time.Second)
}

// CheckLimit 检查是否在限制期内（true=受限，false=可发送）
func CheckLimit(ctx context.Context, phone string) bool {
	key := fmt.Sprintf("%s%s", limitPrefix, phone)
	exists, _ := redis.Exists(ctx, key)
	return exists
}
