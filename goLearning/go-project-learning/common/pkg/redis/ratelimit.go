package redis

import (
	"context"
	"fmt"
	"time"
)

// 滑动窗口限流 Lua 脚本
// ZREMRANGEBYSCORE 删除集合中 min ≤ score ≤ max 所有成员; 0:分数下限 min; now - window * 1000:分数上限 max
// ZCARD 集合内成员个数；键不存在时返回 0
// ZADD Redis zset 添加指令,第一个数值是分数 score，后面是成员 value; now; now .. math.random(): .. 是 Lua 字符串拼接, math.random() 生成 0~1 之间随机小数, 最终拼成类似 17812345678900.123456 这样的字符串，当作 zset 的成员 member
// EXPIRE 到时间后 Redis 自动删除整个 key; key：前面限流用的有序集合 zset 键; window：时间窗口，单位秒（比如 60 代表 60 秒窗口）
const slidingWindowScript = `
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- 删除窗口外的记录
-- ZREMRANGEBYSCORE key min max：删除 score 在 [min, max] 范围的成员
redis.call('ZREMRANGEBYSCORE', key, 0, now - window * 1000)

-- 统计当前窗口内的请求数
local count = redis.call('ZCARD', key)

-- 判断是否超限
if count < limit then
	-- 未超额, 添加当前请求
	redis.call('ZADD', key, now, now .. math.random())
	-- 设置 key 过期时间
	redis.call('EXPIRE', key, window)
	return 1
else
	return 0
end
`

// SlidingWindowLimit 滑动窗口限流
// key: 限流 key （如 "rate_limit:123:login")
// window: 时间窗口 (如 1 分钟)
// limit: 最大请求数
// 返回: (是否允许，错误)
func SlidingWindowLimit(ctx context.Context, key string, window time.Duration, limit int) (bool, error) {
	// 当前时间戳 (毫秒)
	now := time.Now().UnixMilli()

	// 执行 Lua 脚本
	result, err := Client.Eval(ctx, slidingWindowScript, []string{key},
		int(window.Seconds()), limit, now).Int()
	if err != nil {
		return false, fmt.Errorf("限流检查失败: %v", err)
	}

	return result == 1, nil
}
