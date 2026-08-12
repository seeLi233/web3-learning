package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/metrics"
)

// PrometheusMetrics 返回一个 Gin 中间件，自动采集 HTTP 指标
//
// 为什么做成返回 gin.HandlerFunc 的函数，而不是直接写一个 HandlerFunc？
// → 和项目已有的 Logger()、CORS() 保持一致的风格——"工厂函数"模式
//
//	这样中间件可以带配置参数（未来可能加 path 过滤等）
//
// 作用：每个 HTTP 请求自动执行以下操作：
//  1. 记录开始时间
//  2. 执行后续 handler（c.Next()）
//  3. 根据结果更新 Counter + Histogram
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 请求进入：记录开始时间
		// 为什么必须在 c.Next() 之前记录？
		// → 延迟 = 结束时间 - 开始时间，先记下起点，等 c.Next() 返回后再算差值
		start := time.Now()

		// 2. 获取路径模板（如 /api/v1/user/:id），而非实际路径（如 /api/v1/user/123）
		// 为什么用 FullPath() 而不是 Request.URL.Path？
		// → 实际路径 /api/v1/user/123 有无限多种（用户 ID 不同），会炸 label 基数
		//    路由模板 /api/v1/user/:id 只有有限几十个，符合 Prometheus label 设计原则
		path := c.FullPath()
		if path == "" {
			path = "unknown" // 兜底：404 时 FullPath() 可能为空
		}

		// 3. 执行后续中间件 + 业务 handler
		c.Next()

		// 4. 请求结束：采集指标
		// 为什么在 c.Next() 之后？
		// → 此时 status code 已确定（可能被业务代码改了），延迟也有了
		status := strconv.Itoa(c.Writer.Status()) // int → string，"200" "404" "500"
		method := c.Request.Method

		// 4a. Counter：请求计数 +1
		// 为什么 WithLabelValues 而不是 With？
		// → WithLabelValues 按位置传值（快），With 按 key-value 传（清晰但慢一点点）
		//    编译期已知 label 顺序的场景，用 WithLabelValues 更高效
		metrics.HttpRequestsTotal.WithLabelValues(method, path, status).Inc()

		// 4b. Histogram：记录本次请求延迟
		// Observe() 接收的是 float64 秒数
		// 为什么用 time.Since(start).Seconds() 而不是 Milliseconds()？
		// → Prometheus 社区惯例用秒作为时间单位（方便和系统指标对齐）
		metrics.HttpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}
