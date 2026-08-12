package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/internal/handler"
	"github.com/go-project-learning/project/gateway-api/internal/middleware"
)

func RegisterCouponRouter(r *gin.RouterGroup) {
	coupon := r.Group("/coupon")
	coupon.Use(middleware.JWTAuth())
	{
		coupon.GET("/list", handler.ListCouponTemplates)      // 获取可领取的优惠券列表
		coupon.POST("/claim/:id", handler.ClaimCoupon)        // 领取优惠券
		coupon.GET("/my", handler.ListUserCoupons)            // 获取我的优惠券列表
		coupon.GET("/available", handler.GetAvailableCoupons) // 获取可用优惠券（下单时）
		coupon.POST("/use/:id", handler.UseCoupon)            // 使用优惠券
		coupon.GET("/detail/:id", handler.GetCouponDetail)    // 获取优惠券详情
	}
}
