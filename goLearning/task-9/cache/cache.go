package cache

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("cache: key not found")
)

// Cache 缓存工具封装
type Cache struct {
	rdb *redis.Client
}

// NewCache 创建缓存实例
func NewCache(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

// ==================== 基础操作 ====================

// Get 获取字符串值
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return val, err
}

// GetJSON 获取并反序列化 JSON 对象
func (c *Cache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// Set 设置键值对
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// SetJSON 序列化为 JSON 并存储
func (c *Cache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, ttl)
}

// Del 删除键
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists 判断键是否存在
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// Expire 设置过期时间
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// Incr 原子递增
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// HSet / HGet / HGetAll — Hash 操作
func (c *Cache) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.rdb.HSet(ctx, key, values...).Err()
}

func (c *Cache) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.rdb.HGet(ctx, key, field).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	return val, err
}

func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, key).Result()
}

// ==================== 缓存穿透防护 ====================

// GetOrSet 缓存穿透防护版 GetOrSet
// 如果 key 不存在，调用 fallback 函数获取数据并回填缓存
// 如果 fallback 返回空结果，缓存空值（短暂 TTL）防止穿透
func (c *Cache) GetOrSet(ctx context.Context, key string, ttl time.Duration, emptyTTL time.Duration, fallback func() (interface{}, error)) (interface{}, error) {
	// 1. 先查缓存
	val, err := c.rdb.Get(ctx, key).Result()
	if err == nil {
		// 命中：检查是否为空值标记
		if val == "__NULL__" {
			return nil, ErrNotFound
		}
		return val, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err // Redis 异常
	}

	// 2. 缓存未命中 → 查数据源
	data, err := fallback()
	if err != nil {
		return nil, err
	}

	// 3. 回填缓存
	if data == nil {
		// 缓存空值（短 TTL），防止穿透
		c.rdb.Set(ctx, key, "__NULL__", emptyTTL)
		return nil, ErrNotFound
	}

	c.rdb.Set(ctx, key, data, ttl)
	return data, nil
}

// RandomTTL 随机化过期时间，防止缓存雪崩
// baseTTL + random(0, maxJitter)
func RandomTTL(baseTTL time.Duration, maxJitter time.Duration) time.Duration {
	jitter := time.Duration(rand.Int64N(int64(maxJitter)))
	return baseTTL + jitter
}

// ==================== Pipeline 批量操作 ====================

// MSet 批量设置
func (c *Cache) MSet(ctx context.Context, kvs map[string]interface{}, ttl time.Duration) error {
	pipe := c.rdb.Pipeline()
	for k, v := range kvs {
		pipe.Set(ctx, k, v, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}
