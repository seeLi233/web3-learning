package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterMemberRouter(r *gin.RouterGroup) {
	member := r.Group("/member")
	member.Use(middleware.JWTAuth())
	{
		member.GET("/info", handler.GetMemberInfo)          // 获取会员信息
		member.POST("/growth/add", handler.AddGrowth)       // 增加成长值
		member.POST("/growth/deduct", handler.DeductGrowth) // 扣减成长值
		member.GET("/growth/logs", handler.GetGrowthLogs)   // 成长值日志
		member.GET("/benefits", handler.GetMemberBenefits)  // 会员权益
		member.GET("/levels", handler.ListMemberLevels)     // 等级配置列表
	}
}
