package cache

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/redis"
)

const addressCachePrefix = "address:"

// DeleteAddressCache 删除用户缓存
func DeleteAddressCache(ctx context.Context, id uint) error {
	key := fmt.Sprintf("%s%d", addressCachePrefix, id)
	return redis.Del(ctx, key)
}
