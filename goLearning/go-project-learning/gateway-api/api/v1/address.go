package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterAddressRouter(r *gin.RouterGroup) {

	addr := r.Group("/address")
	addr.Use(middleware.JWTAuth())
	{
		addr.POST("", handler.CreateAddress)
		addr.GET("/:id", handler.GetAddress)
		addr.GET("/list", handler.ListAddress)
		addr.PUT("/:id", handler.UpdateAddress)
		addr.DELETE("/:id", handler.DeleteAddress)
	}

}
