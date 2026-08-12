package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	"github.com/go-project-learning/project/user-srv/api/pb"
)

// ListCouponTemplates 获取可领取的优惠券列表
func ListCouponTemplates(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	grpcResp, err := global.CouponClient.ListCouponTemplates(c.Request.Context(), &pb.ListCouponTemplatesRequest{
		Page: int32(page),
		Size: int32(size),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, gin.H{
		"templates": grpcResp.Data,
		"total":     grpcResp.Total,
	})
}

// ClaimCoupon 领取优惠券
func ClaimCoupon(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	templateId, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "优惠券ID参数错误")
		return
	}

	grpcResp, err := global.CouponClient.ClaimCoupon(c.Request.Context(), &pb.ClaimCouponRequest{
		UserId:     int64(uid),
		TemplateId: int64(templateId),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Coupon)
}

// ListUserCoupons 获取我的优惠券列表
func ListUserCoupons(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	grpcResp, err := global.CouponClient.ListUserCoupons(c.Request.Context(), &pb.ListUserCouponsRequest{
		UserId: int64(uid),
		Status: int32(status),
		Page:   int32(page),
		Size:   int32(size),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, gin.H{
		"coupons": grpcResp.Data,
		"total":   grpcResp.Total,
	})
}

// GetAvailableCoupons 获取可用优惠券（下单时）
func GetAvailableCoupons(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	orderAmount, _ := strconv.ParseFloat(c.DefaultQuery("order_amount", "0"), 64)

	grpcResp, err := global.CouponClient.GetAvailableCoupons(c.Request.Context(), &pb.GetAvailableCouponsRequest{
		UserId:      int64(uid),
		OrderAmount: orderAmount,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Data)
}

// UseCoupon 使用优惠券
func UseCoupon(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	couponId, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "优惠券ID参数错误")
		return
	}

	var req struct {
		OrderID string `json:"order_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误")
		return
	}

	grpcResp, err := global.CouponClient.UseCoupon(c.Request.Context(), &pb.UseCouponRequest{
		CouponId: int64(couponId),
		UserId:   int64(uid),
		OrderId:  req.OrderID,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, nil)
}

// GetCouponDetail 获取优惠券详情
func GetCouponDetail(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	couponId, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "优惠券ID参数错误")
		return
	}

	grpcResp, err := global.CouponClient.GetCouponDetail(c.Request.Context(), &pb.GetCouponDetailRequest{
		CouponId: int64(couponId),
		UserId:   int64(uid),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Coupon)
}
