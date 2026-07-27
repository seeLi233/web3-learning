package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLockHeld    = errors.New("lock: 锁已被其他人持有")
	ErrLockExpired = errors.New("lock: 锁已过期")
)

// ==================== Lua 脚本 ====================

// 释放锁的 Lua 脚本：只有持有者才能释放
const unlockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`

// 锁续期 Lua 脚本：只有持有者才能续期
const renewScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("EXPIRE", KEYS[1], ARGV[2])
else
	return 0
end
`

// ==================== Lock 结构 ====================

// Lock 分布式锁
type Lock struct {
	rdb     *redis.Client
	key     string
	value   string // 持有者标识（UUID）
	ttl     time.Duration
	renewCh chan struct{} // 停止续期的信号
}

// NewLock 创建分布式锁
func NewLock(rdb *redis.Client, key string, ttl time.Duration) *Lock {
	return &Lock{
		rdb:   rdb,
		key:   key,
		value: uuid.New().String(), // 唯一标识当前持有者
		ttl:   ttl,
	}
}

// Lock 获取锁
// 返回 ErrLockHeld 表示锁已被其他人持有
func (l *Lock) Lock(ctx context.Context) error {
	// 原子操作：SET key value NX EX ttl
	ok, err := l.rdb.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	if !ok {
		return ErrLockHeld
	}
	return nil
}

// LockWithRetry 带重试的获取锁
func (l *Lock) LockWithRetry(ctx context.Context, maxRetries int, retryInterval time.Duration) error {
	for i := 0; i < maxRetries; i++ {
		err := l.Lock(ctx)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrLockHeld) {
			return err
		}
		// 等待后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryInterval):
		}
	}
	return ErrLockHeld
}

// Unlock 释放锁（Lua 脚本原子校验 + 删除）
func (l *Lock) Unlock(ctx context.Context) error {
	// 停止续期
	if l.renewCh != nil {
		close(l.renewCh)
	}

	result, err := l.rdb.Eval(ctx, unlockScript, []string{l.key}, l.value).Result()
	if err != nil {
		return err
	}
	if result.(int64) == 0 {
		return ErrLockExpired
	}
	return nil
}

// StartRenew 启动 Watch Dog 自动续期
// 在后台协程定期续期，防止业务未完成锁就过期
func (l *Lock) StartRenew(ctx context.Context, interval time.Duration) {
	l.renewCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-l.renewCh:
				return // 停止续期
			case <-ticker.C:
				// 续期到原始 TTL（不是累加）
				l.rdb.Eval(ctx, renewScript, []string{l.key}, l.value, int(l.ttl.Seconds()))
			}
		}
	}()
}

// WithLock 自动加锁/解锁的便捷方法
// 用法:
//
//	lock := NewLock(rdb, "lock:order:1001", 30*time.Second)
//	err := lock.WithLock(ctx, 10, 100*time.Millisecond, func() error {
//	    // 业务逻辑
//	    return nil
//	})
func (l *Lock) WithLock(ctx context.Context, maxRetries int, retryInterval time.Duration, fn func() error) error {
	// 加锁
	if err := l.LockWithRetry(ctx, maxRetries, retryInterval); err != nil {
		return err
	}

	// 启动 Watch Dog（每 TTL/3 续期一次）
	l.StartRenew(ctx, l.ttl/3)

	// 确保释放锁
	defer l.Unlock(context.Background()) // 用新 context，取消时也能解锁

	// 执行业务逻辑
	return fn()
}
