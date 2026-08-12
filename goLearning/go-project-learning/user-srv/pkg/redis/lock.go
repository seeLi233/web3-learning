package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/redis"
)

// LockRegister 注册防重复锁（同一个手机号 30 秒内不能重复注册）
func LockRegister(ctx context.Context, phone string) (bool, error) {
	key := fmt.Sprintf("lock:register:%s", phone)
	err := redis.Set(ctx, key, "1", 30*time.Second)
	if err != nil {
		return false, err
	}
	return true, nil
}
