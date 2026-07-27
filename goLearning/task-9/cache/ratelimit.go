package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ==================== 滑动窗口限流 Lua 脚本 ====================

// 滑动窗口核心思想：
//
//	不按固定的时间分桶（如 12:00~12:01 一个桶），
//	而是记录每次请求的时间戳，只统计当前时间 - 窗口大小内的请求数。
//
// 用 ZSet 实现：
//   - Score = 请求时间戳（毫秒）
//   - Member = 唯一标识（时间戳 + 自增序号）
//   - 每次请求：移除窗口外的旧记录 → 统计窗口内记录数 → 判断 → 添加新记录

const slidingWindowScript = `
	local key = KEYS[1]
	local window = tonumber(ARGV[1])  -- 窗口大小（毫秒）
	local limit = tonumber(ARGV[2])   -- 限制次数
	local now = tonumber(ARGV[3])     -- 当前时间戳（毫秒）

	-- 1. 移除窗口外的旧记录（score < now - window）
	redis.call("ZREMRANGEBYSCORE", key, 0, now - window)

	-- 2. 统计窗口内的请求数
	local count = redis.call("ZCARD", key)

	-- 3. 判断是否超限
	if count >= limit then
		return 0  -- 拒绝（限流）
	end

	-- 4. 记录本次请求（member 需唯一，用自增序号保证）
	redis.call("ZADD", key, now, now .. ":" .. redis.call("INCR", key .. ":seq"))

	-- 5. 设置过期时间，防止 ZSet 内存泄漏
	redis.call("PEXPIRE", key, window)

	return 1  -- 放行
`

// RateLimiter 滑动窗口限流器
type RateLimiter struct {
	rdb    *redis.Client
	window time.Duration // 时间窗口
	limit  int           // 限制次数
}

// NewRateLimiter 创建限流器
// window: 时间窗口
// limit: 窗口内允许的最大请求数
func NewRateLimiter(rdb *redis.Client, window time.Duration, limit int) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		window: window,
		limit:  limit,
	}
}

// Allow 判断请求是否允许通过
// key: 限流的维度（如 "rate:api:/user/login" 或 "rate:ip:192.168.1.1"）
// 返回 true 表示放行，false 表示限流
func (r *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now().UnixMilli()
	windowMs := r.window.Milliseconds()

	result, err := r.rdb.Eval(ctx, slidingWindowScript, []string{key}, windowMs, r.limit, now).Result()

	if err != nil {
		// Redis 故障时，默认放行（避免误杀，根据场景权衡）
		return true, err
	}

	return result.(int64) == 1, nil
}

// AllowN 判断 N 个请求是否允许通过
func (r *RateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	for i := 0; i < n; i++ {
		ok, err := r.Allow(ctx, key)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// ==================== 固定窗口计数器（对比用） ====================

// FixedWindowLimiter 固定窗口限流器（简单但有边界问题）
// 问题：窗口边界处可能出现瞬时 2x 流量
//
//	窗口 12:00~12:01: 100 个请求
//	窗口 12:01~12:02: 100 个请求
//	但在 12:00:59 ~ 12:01:01 这 2 秒内，可能有 200 个请求！
type FixedWindowLimiter struct {
	rdb    *redis.Client
	window time.Duration
	limit  int
}

func NewFixedWindowLimiter(rdb *redis.Client, window time.Duration, limit int) *FixedWindowLimiter {
	return &FixedWindowLimiter{rdb: rdb, window: window, limit: limit}
}

func (f *FixedWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// 用当前时间窗口作为 key 后缀
	now := time.Now().Unix()
	windowKey := fmt.Sprintf("%s:%d", key, now/int64(f.window.Seconds()))

	// INCR + 首次设置 EXPIRE
	count, err := f.rdb.Incr(ctx, windowKey).Result()
	if err != nil {
		return true, err
	}
	if count == 1 {
		f.rdb.Expire(ctx, windowKey, f.window+time.Second)
	}
	return count <= int64(f.limit), nil
}
