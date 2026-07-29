package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ==================== Redis 黑名单实现 ====================

// RedisBlacklist 实现 TokenBlacklist 接口，用 Redis 存储已登出的 token jti
//
// TTL 策略：
//   - "拉黑"一个 token 时，设置 TTL = 该 token 的剩余有效期
//   - 为什么不是永久存储？
//     → token 过期后本身就是无效的，不需要继续占用 Redis 内存。
//     token 过期 = 黑名单条目也自动过期，完美同步。
//
// Key 命名规范：blacklist:{jti}
//   - 为什么加前缀？
//     → Redis 是全局 key 空间，加前缀可以：1) 避免与其他业务 key 冲突
//     2) 方便运维（`KEYS blacklist:*` 看所有黑名单）
//     3) 方便清理（误操作 DEL 不会删到其他 key）
type RedisBlacklist struct {
	client *redis.Client
	prefix string // key 前缀，默认 "blacklist"
}

// NewRedisBlacklist 创建 Redis 黑名单实例
// 参数：
//   - client: go-redis 客户端（外部创建，依赖注入）
//   - prefix: key 前缀，建议传 "blacklist"
func NewRedisBlacklist(client *redis.Client, prefix string) *RedisBlacklist {
	return &RedisBlacklist{
		client: client,
		prefix: prefix,
	}
}

// buildKey 构造 Redis key："{prefix}:{jti}"
func (b *RedisBlacklist) buildKey(jti string) string {
	return fmt.Sprintf("%s:%s", b.prefix, jti)
}

// IsBlacklisted 检查 token jti 是否在黑名单中
//
// 为什么需要这个检查？
// → 场景：用户 A 在设备 1 上登出（token 被拉黑），攻击者拿到了设备 1 的 token。
//
//	攻击者用这个 token 请求 API → Auth 中间件解析 token（签名有效，未过期）→
//	查黑名单 → 发现已被拉黑 → 拒绝请求。
//
// 函数签名为什么用 interface{} 而不是 context.Context？
// → TokenBlacklist 接口定义在 middleware.go 中，不想让它直接依赖 context 包
//
//	（接口隔离原则）。实际使用时传入 context.Context，断言即可。
func (b *RedisBlacklist) IsBlacklisted(ctxRaw interface{}, jti string) (bool, error) {
	// 类型断言：确认传入的是 context.Context
	ctx, ok := ctxRaw.(context.Context)
	if !ok {
		return false, fmt.Errorf("expected context.Context, got %T", ctxRaw)
	}

	key := b.buildKey(jti)

	// EXISTS 命令——Redis 中检查 key 是否存在
	// 返回值：1 表示 key 存在（token 在黑名单），0 表示不存在（token 可用）
	result, err := b.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists: %w", err)
	}
	return result > 0, nil
}

// Add 将 token jti 加入黑名单
//
// 参数：
//   - ctxRaw: context
//   - jti: token 的唯一标识
//   - ttlRaw: 黑名单有效期（time.Duration 类型）
//
// TTL 计算——这是黑名单机制最关键的设计决策：
//
//	黑名单 TTL = token 过期时间 - 当前时间（即 token 的剩余有效期）
//	为什么？
//	→ token 本身过期后就不需要黑名单了——过期的 token 本来就无法通过验证。
//	  如果存永久，随着用户登出次数增多，Redis 内存会无限增长。
//	→ 示例：AT 在 12:15 过期，用户 12:00 登出 → TTL = 15min
func (b *RedisBlacklist) Add(ctxRaw interface{}, jti string, ttlRaw interface{}) error {
	ctx, ok := ctxRaw.(context.Context)
	if !ok {
		return fmt.Errorf("expected context.Context, got %T", ctxRaw)
	}

	ttl, ok := ttlRaw.(time.Duration)
	if !ok {
		return fmt.Errorf("expected time.Duration, got %T", ttlRaw)
	}

	key := b.buildKey(jti)

	// SET key value NX EX ttl
	// NX = "only set if Not eXists"——幂等写入，重复登出不会报错
	// EX = 设置过期秒数
	// value 设为 "1"（只需要知道存在即可，不需要存实际数据）
	if err := b.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("redis set blacklist: %w", err)
	}
	return nil
}

// ==================== 辅助函数 ====================

// CalculateBlacklistTTL 计算黑名单 TTL = token 到期时间 - 当前时间
//
// 为什么封装成独立函数？
// → 计算逻辑只在一处定义，避免重复代码。如果以后改 TTL 策略（如加 5 分钟缓冲），
//
//	只需改这一个函数。
//
// 参数：
//   - tokenExp: token 的过期时间（从 JWT claims 的 exp 字段获取）
//
// 返回值：
//   - time.Duration: 应该设置的 Redis key TTL
//   - error: token 已经过期则返回错误（不需要加入黑名单）
func CalculateBalcklistTTL(tokenExp time.Time) (time.Duration, error) {
	ttl := time.Until(tokenExp)
	if ttl < 0 {
		return 0, fmt.Errorf("token already expired, no need to blacklist")
	}
	return ttl, nil
}
