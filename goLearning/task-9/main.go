package main

import (
	"context"
	"fmt"
	"log"
	"task9/cache"
	"task9/config"
	"time"
)

func main() {
	// 1. 连接 Redis
	rdb, err := config.InitRedis()
	if err != nil {
		log.Fatalf("❌ %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	c := cache.NewCache(rdb)

	fmt.Println("\n========== Redis 缓存演示 ==========")

	// ==================== 基础 KV ====================
	fmt.Println("\n📝 1. String 基础操作")
	c.Set(ctx, "greeting", "Hello, Redis!", 10*time.Minute)
	val, _ := c.Get(ctx, "greeting")
	fmt.Printf("   GET greeting → %s\n", val)

	exists, _ := c.Exists(ctx, "greeting")
	fmt.Printf("   EXISTS greeting → %v\n", exists)

	c.Del(ctx, "greeting")
	exists, _ = c.Exists(ctx, "greeting")
	fmt.Printf("   DEL 后 EXISTS → %v\n", exists)

	// ==================== JSON 缓存 ====================
	fmt.Println("\n📝 2. JSON 对象缓存")
	type User struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}

	c.SetJSON(ctx, "user:1", user, 5*time.Minute)
	var cached User
	err = c.GetJSON(ctx, "user:1", &cached)
	if err == nil {
		fmt.Printf("   GET user:1 → %+v\n", cached)
	}

	// ==================== Hash 操作 ====================
	fmt.Println("\n📝 3. Hash 操作")
	c.HSet(ctx, "user_hash:1", "name", "Alice", "age", "25", "role", "admin")
	name, _ := c.HGet(ctx, "user_hash:1", "name")
	all, _ := c.HGetAll(ctx, "user_hash:1")
	fmt.Printf("   HGET user_hash:1 name → %s\n", name)
	fmt.Printf("   HGETALL user_hash:1 → %v\n", all)

	// ==================== 计数器 ====================
	fmt.Println("\n📝 4. 原子计数器")
	c.Del(ctx, "counter") // 先清空计数器
	for i := 0; i < 5; i++ {
		val, _ := c.Incr(ctx, "counter")
		fmt.Printf("   INCR counter → %d\n", val)
	}

	// ==================== 随机 TTL（防雪崩） ====================
	fmt.Println("\n📝 5. 随机 TTL（防缓存雪崩）")
	for i := 0; i < 5; i++ {
		ttl := cache.RandomTTL(60*time.Second, 30*time.Second)
		fmt.Printf("   key_%d TTL = %v\n", i, ttl)
	}

	// ==================== 分布式锁 ====================
	fmt.Println("\n📝 6. 分布式锁演示")
	lock := cache.NewLock(rdb, "lock:demo", 10*time.Second)

	// 获取锁
	if err := lock.Lock(ctx); err != nil {
		fmt.Printf("   ❌ 获取锁失败: %v\n", err)
	} else {
		fmt.Println("   ✅ 获取锁成功")

		// 尝试重复获取（应该失败）
		lock2 := cache.NewLock(rdb, "lock:demo", 10*time.Second)
		if err := lock2.Lock(ctx); err != nil {
			fmt.Printf("   ✅ 重复获取被拒绝: %v\n", err)
		}

		// 释放锁
		lock.Unlock(ctx)
		fmt.Println("   ✅ 锁已释放")

		// 释放后可重新获取
		lock3 := cache.NewLock(rdb, "lock:demo", 10*time.Second)
		if err := lock3.Lock(ctx); err == nil {
			fmt.Println("   ✅ 释放后可以重新获取锁")
			lock3.Unlock(ctx)
		}
	}

	// ==================== WithLock 便捷用法 ====================
	fmt.Println("\n📝 7. WithLock 自动加锁/解锁")
	lock4 := cache.NewLock(rdb, "lock:order:1001", 10*time.Second)
	err = lock4.WithLock(ctx, 3, 50*time.Millisecond, func() error {
		fmt.Println("   🔒 在锁保护下执行业务逻辑...")
		time.Sleep(100 * time.Millisecond)
		fmt.Println("   ✅ 业务完成")
		return nil
	})
	if err != nil {
		fmt.Printf("   ❌ 执行失败: %v\n", err)
	}

	// ==================== 滑动窗口限流 ====================
	fmt.Println("\n📝 8. 滑动窗口限流器")
	limiter := cache.NewRateLimiter(rdb, 1*time.Second, 5) // 1秒内最多5次

	testKey := "rate:test"
	for i := 1; i <= 7; i++ {
		ok, _ := limiter.Allow(ctx, testKey)
		if ok {
			fmt.Printf("   请求 #%d → ✅ 放行\n", i)
		} else {
			fmt.Printf("   请求 #%d → ⛔ 限流!\n", i)
		}
	}

	// 等待窗口过后再试
	fmt.Println("   等待 1 秒后...")
	time.Sleep(1 * time.Second)
	ok, _ := limiter.Allow(ctx, testKey)
	fmt.Printf("   窗口过后请求 → %v\n", map[bool]string{true: "✅ 放行", false: "⛔ 限流"}[ok])

	// ==================== 布隆过滤器 ====================
	fmt.Println("\n📝 9. 布隆过滤器")
	bloom := cache.NewSimpleBloom(rdb, "bloom:users", 100000, 3)

	// 添加 3 个用户
	bloom.Add(ctx, "alice")
	bloom.Add(ctx, "bob")
	bloom.Add(ctx, "charlie")
	fmt.Println("   添加: alice, bob, charlie")

	// 查询存在的
	for _, name := range []string{"alice", "bob", "charlie"} {
		exists, _ := bloom.Contains(ctx, name)
		fmt.Printf("   查询 %s → %v (可能存在)\n", name, exists)
	}

	// 查询不存在的
	for _, name := range []string{"david", "eve"} {
		exists, _ := bloom.Contains(ctx, name)
		fmt.Printf("   查询 %s → %v (一定不存在)\n", name, exists)
	}

	// ==================== 缓存穿透防护 ====================
	fmt.Println("\n📝 10. 缓存穿透防护 (GetOrSet)")
	hitCount := 0
	for i := 0; i < 5; i++ {
		result, err := c.GetOrSet(ctx, "user:999", 30*time.Second, 5*time.Second, func() (interface{}, error) {
			hitCount++
			return nil, nil // 模拟 DB 中不存在
		})
		fmt.Printf("   第%d次: result=%v err=%v\n", i+1, result, err)
	}
	fmt.Printf("   实际查询 DB: %d 次 (5次请求只查了1次DB)\n", hitCount)

	fmt.Println("\n🎉 Redis 缓存层演示完成！")
}
