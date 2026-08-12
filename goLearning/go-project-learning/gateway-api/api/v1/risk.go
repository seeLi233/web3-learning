package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterRiskRouter(r *gin.RouterGroup) {
	risk := r.Group("/risk")
	risk.Use(middleware.JWTAuth())
	{
		// 黑名单管理
		risk.POST("/blacklist", handler.AddBlacklist)
		risk.DELETE("/blacklist/:id", handler.RemoveBlacklist)
		risk.GET("/blacklist", handler.ListBlacklist)

		// 风控配置管理
		risk.GET("/config", handler.ListRiskConfig)
		risk.PUT("/config", handler.UpdateRiskConfig)

		// 刷新配置
		risk.POST("/config/refresh", handler.RefreshRiskConfig)
	}
}
