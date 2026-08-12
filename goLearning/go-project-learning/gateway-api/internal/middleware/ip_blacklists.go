package middleware

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
)

var sm sync.Map

func IPBlacklist() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取客户端 IP
		ip := c.ClientIP()
		fmt.Printf("[IPBlacklist] 检查 IP: %s\n", ip)

		// 2. 检查本地缓存（黑名单）
		if value, ok := sm.Load(ip); ok {
			fmt.Printf("[IPBlacklist] 本地缓存命中: %s = %s\n", ip, value.(string))
			if value.(string) == "blacklisted" {
				resp.Error(c, 403, "IP 已被封禁")
				c.Abort()
				return
			}
		}

		// 3. 检查 Redis（无论本地缓存是什么，都检查 Redis，确保实时性）
		redisKey := "ip_blacklist:" + ip
		fmt.Printf("[IPBlacklist] 检查 Redis key: %s\n", redisKey)
		exists, err := redis.Exists(c, redisKey)
		fmt.Printf("[IPBlacklist] Redis exists: %v, err: %v\n", exists, err)
		if err == nil && exists {
			sm.Store(ip, "blacklisted")
			resp.Error(c, 403, "IP 已被封禁")
			c.Abort()
			return
		}

		// 4. 未命中黑名单，缓存为 allowed
		sm.Store(ip, "allowed")
		c.Next()
	}
}
