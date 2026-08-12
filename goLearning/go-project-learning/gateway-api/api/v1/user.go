package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterUserRoutes(r *gin.RouterGroup) {
	user := r.Group("/user")
	user.Use(middleware.CORS(), middleware.IPBlacklist())
	{
		user.POST("/register", handler.Register)

		// 登录相关（无需鉴权）
		user.POST("/send-code", handler.SendCode)
		user.POST("/login/phone", handler.PhoneLogin)
		user.POST("/login/email", handler.EmailLogin)
		user.POST("/login/password", handler.PwdLogin)
		user.POST("/token/refresh", handler.RefreshToken)
		user.POST("/logout", handler.Logout) // 登出
	}

	// 需要鉴权的路由组
	auth := r.Group("/user")
	auth.Use(middleware.CORS(), middleware.JWTAuth())
	{
		// 需要登录才能访问的接口
		auth.GET("/:id", handler.GetUser)
		auth.GET("/list", handler.ListUser)
		auth.PUT("/profile", handler.UpdateProfile)
	}
}
