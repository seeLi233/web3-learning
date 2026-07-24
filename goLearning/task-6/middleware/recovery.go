package middleware

import (
	"github/seeli/task-6/model"
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// CustomRecovery 自定义 panic 恢复中间件
// 作用：捕获 Handler 中任何 panic，防止整个进程崩溃，返回 500
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 打印完整堆栈到日志（方便排查）
				log.Printf("💥 PANIC: %v\n%s", err, string(debug.Stack()))

				// 给客户端返回统一的 500 错误（不暴露内部细节）
				model.InternalError(c, "服务器内部错误，请稍后重试")
				c.Abort()
			}
		}()
		c.Next()
	}
}
