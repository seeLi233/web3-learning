package router

import (
	"github/seeli/task-6/handler"
	"github/seeli/task-6/middleware"

	"github.com/gin-gonic/gin"
)

// SetupRouter 组装路由和中间件
// 中间件执行顺序：CustomLogger → CustomRecovery → CORS → 路由 Handler
func SetupRouter() *gin.Engine {
	// ===== 1. 创建 Engine（默认带 Logger + Recovery，我们用自己的替换）=====
	r := gin.New()

	// ===== 2. 注册全局中间件 =====
	r.Use(
		middleware.CustomLogger(),   // ① 最外层：记录请求日志
		middleware.CustomRecovery(), // ② 中间层：捕获 panic
		middleware.CORS(),           // ③ 内层：处理跨域
	)

	// ===== 3. 健康检查（不需要版本前缀）=====
	r.GET("/health", handler.HealthCheck)

	// ===== 4. API v1 路由组 =====
	v1 := r.Group("/api/v1")
	{
		// 用户资源
		users := v1.Group("/users")
		{
			users.GET("", handler.ListUser)          // GET    /api/v1/users
			users.GET("/:id", handler.GetUser)       // GET    /api/v1/users/:id
			users.POST("", handler.CreateUser)       // POST   /api/v1/users
			users.PATCH("/:id", handler.UpdateUser)  // PATCH  /api/v1/users/:id
			users.DELETE("/:id", handler.DeleteUser) // DELETE /api/v1/users/:id
		}

		// 后续可扩展更多资源...
		// orders := v1.Group("/orders")
		// products := v1.Group("/products")
	}

	return r
}
