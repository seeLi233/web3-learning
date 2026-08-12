package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
)

const userCachePrefix = "user:"

// GetUserCache 从 Redis 读用户缓存
func GetUserCache(ctx context.Context, id int) (*entity.User, bool) {
	key := fmt.Sprintf("%s%d", userCachePrefix, id)
	data, err := redis.Get(ctx, key)
	if err != nil || data == "" {
		return nil, false
	}

	var user entity.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, false
	}
	return &user, true
}

// SetUserCache 写入用户缓存，默认 2 小时过期
func SetUserCache(ctx context.Context, user *entity.User) error {
	key := fmt.Sprintf("%s%d", userCachePrefix, user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return redis.Set(ctx, key, string(data), 2*time.Hour)
}

// DeleteUserCache 删除用户缓存
func DeleteUserCache(ctx context.Context, id int) error {
	key := fmt.Sprintf("%s%d", userCachePrefix, id)
	return redis.Del(ctx, key)
}
