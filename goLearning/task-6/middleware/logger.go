package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// CustomLogger 自定义日志中间件
// 这是理解洋葱模型的最佳示例：
//
//	c.Next() 前 = 请求进入（记录开始时间）
//	c.Next() 后 = 响应离开（记录耗时 + 状态码）
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ===== 请求阶段 =====
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		log.Printf("📥 → | %s | %s | 开始处理", method, path)

		// ===== 进入下一层（执行后续中间件 → Handler）=====
		c.Next()

		// ===== 响应阶段 =====
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()

		log.Printf("📤 ← | %s | %s | %d | %v | %s", method, path, statusCode, latency, clientIP)
	}
}
