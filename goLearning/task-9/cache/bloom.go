package cache

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/redis/go-redis/v9"
)

// SimpleBloom 简化版布隆过滤器（基于 Redis Bitmap）
// 核心思想：用多个 Hash 函数映射到 Bitmap 位
// - 添加：将所有 Hash 位置设为 1
// - 查询：所有位都是 1 → "可能存在"；任意位是 0 → "一定不存在"
type SimpleBloom struct {
	rdb       *redis.Client
	key       string // Redis Bitmap key
	hashFuncs int    // Hash 函数数量
	size      uint64 // Bitmap 大小
}

// NewSimpleBloom 创建布隆过滤器
func NewSimpleBloom(rdb *redis.Client, key string, size uint64, hashFuncs int) *SimpleBloom {
	return &SimpleBloom{
		rdb:       rdb,
		key:       key,
		size:      size,
		hashFuncs: hashFuncs,
	}
}

func (b *SimpleBloom) hash(data string, i int) uint64 {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%s:%d", data, i)))
	return h.Sum64() % b.size
}

// hash 计算第 i 个 Hash 函数的值
func (b *SimpleBloom) Add(ctx context.Context, data string) error {
	for i := 0; i < b.hashFuncs; i++ {
		offset := b.hash(data, i)
		if err := b.rdb.SetBit(ctx, b.key, int64(offset), 1).Err(); err != nil {
			return err
		}
	}
	return nil
}

// Contains 检查元素是否可能存在
// true → 可能存在（有误判）
// false → 一定不存在
func (b *SimpleBloom) Contains(ctx context.Context, data string) (bool, error) {
	for i := 0; i < b.hashFuncs; i++ {
		offset := b.hash(data, i)
		bit, err := b.rdb.GetBit(ctx, b.key, int64(offset)).Result()
		if err != nil {
			return false, err
		}
		if bit == 0 {
			return false, nil // 任意位是 0 → 一定不存在
		}
	}
	return true, nil // 所有位都是 1 → 可能存在
}
