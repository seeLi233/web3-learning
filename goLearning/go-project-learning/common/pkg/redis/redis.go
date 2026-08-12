package redis

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-redis/redis/extra/redisotel/v8"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"go.uber.org/zap"
)

var (
	Client *redis.Client
	ctx    = context.Background()
	Rs     *redsync.Redsync
)

func InitRedis(cfg config.RedisConfig) {
	// cfg := configs.Conf.Redis

	redisAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	client.AddHook(redisotel.NewTracingHook())

	// 测试连通性
	_, err := client.Ping(ctx).Result()
	if err != nil {
		logger.Fatal("Redis 连接失败", zap.Error(err))
	}

	logger.Info("Redis 连接成功")

	// Redsync 分布式锁（核心）
	pool := goredis.NewPool(client)
	Rs = redsync.New(pool)

	Client = client
}

// ----------------------------------------------------
// 常用工具方法封装
// ----------------------------------------------------

// Set 存 key-value, 带过期时间
// 0 表示永不过期
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return Client.Set(ctx, key, value, expiration).Err()
}

// Get 根据 key 获取值
func Get(ctx context.Context, key string) (string, error) {
	return Client.Get(ctx, key).Result()
}

func HSet(ctx context.Context, key string, field string, value interface{}) error {
	return Client.HSet(ctx, key, field, value).Err()
}

func HSetData(ctx context.Context, key string, fields ...interface{}) error {
	return Client.HSet(ctx, key, fields...).Err()
}

func HGet(ctx context.Context, key string, field string) (string, error) {
	return Client.HGet(ctx, key, field).Result()
}

func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return Client.HGetAll(ctx, key).Result()
}

// Del 删除 key
func Del(ctx context.Context, key string) error {
	return Client.Del(ctx, key).Err()
}

// Expire 给 key 设置过期时间
func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return Client.Expire(ctx, key, expiration).Err()
}

// Exists 判断 key 是否存在
func Exists(ctx context.Context, key string) (bool, error) {
	n, err := Client.Exists(ctx, key).Result()
	return n > 0, err
}

// =======================================ZSet 排行榜=======================================
func ZAdd(ctx context.Context, key string, score float64, member interface{}) error {
	z := &redis.Z{
		Score:  score,
		Member: member,
	}
	return Client.ZAdd(ctx, key, z).Err()
}

func ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return Client.ZRevRange(ctx, key, start, stop).Result()
}

// =======================================获取所有 Keys=======================================

func Keys(ctx context.Context, pattern string) ([]string, error) {
	return Client.Keys(ctx, pattern).Result()
}

// =======================================过期时间设置=======================================
// 默认过期时间
func GetDefaultExp() time.Duration {
	return GetExperationTime(2 * time.Hour)
}

// 防止缓存雪崩
func GetExperationTime(dTime time.Duration) time.Duration {
	return dTime + time.Duration(rand.IntN(1800))*time.Second
}

// =======================================分布式锁=======================================

func LockKey(key string, timer time.Duration) (*redsync.Mutex, error) {
	mutex := Rs.NewMutex(key, redsync.WithExpiry(timer))
	if err := mutex.LockContext(ctx); err != nil {
		return nil, err
	}
	return mutex, nil
}

func UnLockKey(mutex *redsync.Mutex) {
	_, _ = mutex.UnlockContext(ctx)
}

// LockMultiGoods 对多个Keys加锁（按ID排序，防死锁）
func LockMultiKeys(keys []string) ([]*redsync.Mutex, error) {
	var mutexes []*redsync.Mutex
	for _, key := range keys {
		mutex := Rs.NewMutex(
			key,
			redsync.WithExpiry(10*time.Second),
			redsync.WithTries(1),
		)
		if err := mutex.LockContext(ctx); err != nil {
			// 加锁失败，释放已加的锁
			UnlockMultiKeys(mutexes)
			return nil, err
		}
		mutexes = append(mutexes, mutex)
	}
	return mutexes, nil
}

// UnlockMultiGoods 释放多个锁
func UnlockMultiKeys(mutexes []*redsync.Mutex) {
	for _, m := range mutexes {
		_, _ = m.UnlockContext(ctx)
	}
}

// =======================================分布式锁=======================================
// 原子减法
func HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return Client.HIncrBy(ctx, key, field, incr).Result()
}
