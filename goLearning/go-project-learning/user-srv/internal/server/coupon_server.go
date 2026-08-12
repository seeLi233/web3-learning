package server

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"go.uber.org/zap"
)

type CouponServer struct {
	pb.UnimplementedCouponServiceServer
	couponApp *application.CouponApp
}

func NewCouponServer(couponApp *application.CouponApp) *CouponServer {
	return &CouponServer{
		couponApp: couponApp,
	}
}

// toPbCouponTemplate 实体转 pb
func toPbCouponTemplate(t *entity.CouponTemplate) *pb.CouponTemplate {
	return &pb.CouponTemplate{
		Id:             int64(t.ID),
		Name:           t.Name,
		Type:           int32(t.Type),
		DiscountAmount: t.DiscountAmount.InexactFloat64(),
		MinAmount:      t.MinAmount.InexactFloat64(),
		StartTime:      t.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:        t.EndTime.Format("2006-01-02 15:04:05"),
		TotalCount:     int32(t.TotalCount),
		ClaimedCount:   int32(t.ClaimedCount),
		PerUserLimit:   int32(t.PerUserLimit),
		Status:         int32(t.Status),
	}
}

// toPbUserCoupon 实体转 pb
func toPbUserCoupon(c *entity.UserCoupon) *pb.UserCoupon {
	var usedAt string
	if c.UsedAt != nil {
		usedAt = c.UsedAt.Format("2006-01-02 15:04:05")
	}

	pbCoupon := &pb.UserCoupon{
		Id:         int64(c.ID),
		UserId:     int64(c.UserID),
		TemplateId: int64(c.TemplateID),
		Status:     int32(c.Status),
		ClaimedAt:  c.ClaimedAt.Format("2006-01-02 15:04:05"),
		UsedAt:     usedAt,
		OrderId:    c.OrderID,
	}

	// 关联模板信息
	if c.Template.ID > 0 {
		pbCoupon.Template = toPbCouponTemplate(&c.Template)
	}

	return pbCoupon
}

// ========================================
// ListCouponTemplates 获取可领取的优惠券列表
// ========================================
func (s *CouponServer) ListCouponTemplates(ctx context.Context, req *pb.ListCouponTemplatesRequest) (*pb.ListCouponTemplatesResponse, error) {
	templates, total, err := s.couponApp.ListTemplates(ctx, int(req.Page), int(req.Size))
	if err != nil {
		logger.Error("获取优惠券模板列表失败", zap.Error(err))
		return &pb.ListCouponTemplatesResponse{
			Code: 50001,
			Msg:  "获取优惠券列表失败",
		}, nil
	}

	pbTemplates := make([]*pb.CouponTemplate, len(templates))
	for i, t := range templates {
		pbTemplates[i] = toPbCouponTemplate(t)
	}

	return &pb.ListCouponTemplatesResponse{
		Code:  0,
		Msg:   "获取优惠券列表成功",
		Data:  pbTemplates,
		Total: int32(total),
	}, nil
}

// ========================================
// ClaimCoupon 领取优惠券
// ========================================
func (s *CouponServer) ClaimCoupon(ctx context.Context, req *pb.ClaimCouponRequest) (*pb.ClaimCouponResponse, error) {
	coupon, err := s.couponApp.ClaimCoupon(ctx, uint(req.UserId), uint(req.TemplateId))
	if err != nil {
		logger.Error(fmt.Sprintf("领取优惠券失败, userId: %d, templateId: %d", req.UserId, req.TemplateId), zap.Error(err))
		return &pb.ClaimCouponResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.ClaimCouponResponse{
		Code:   0,
		Msg:    "领取优惠券成功",
		Coupon: toPbUserCoupon(coupon),
	}, nil
}

// ========================================
// ListUserCoupons 获取我的优惠券列表
// ========================================
func (s *CouponServer) ListUserCoupons(ctx context.Context, req *pb.ListUserCouponsRequest) (*pb.ListUserCouponsResponse, error) {
	coupons, total, err := s.couponApp.ListUserCoupons(ctx, uint(req.UserId), int(req.Status), int(req.Page), int(req.Size))
	if err != nil {
		logger.Error(fmt.Sprintf("获取用户优惠券列表失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.ListUserCouponsResponse{
			Code: 50001,
			Msg:  "获取优惠券列表失败",
		}, nil
	}

	pbCoupons := make([]*pb.UserCoupon, len(coupons))
	for i, c := range coupons {
		pbCoupons[i] = toPbUserCoupon(c)
	}

	return &pb.ListUserCouponsResponse{
		Code:  0,
		Msg:   "获取优惠券列表成功",
		Data:  pbCoupons,
		Total: int32(total),
	}, nil
}

// ========================================
// GetAvailableCoupons 获取可用优惠券（下单时）
// ========================================
func (s *CouponServer) GetAvailableCoupons(ctx context.Context, req *pb.GetAvailableCouponsRequest) (*pb.GetAvailableCouponsResponse, error) {
	coupons, err := s.couponApp.GetAvailableCoupons(ctx, uint(req.UserId), req.OrderAmount)
	if err != nil {
		logger.Error(fmt.Sprintf("获取可用优惠券失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.GetAvailableCouponsResponse{
			Code: 50001,
			Msg:  "获取可用优惠券失败",
		}, nil
	}

	pbCoupons := make([]*pb.UserCoupon, len(coupons))
	for i, c := range coupons {
		pbCoupons[i] = toPbUserCoupon(c)
	}

	return &pb.GetAvailableCouponsResponse{
		Code: 0,
		Msg:  "获取可用优惠券成功",
		Data: pbCoupons,
	}, nil
}

// ========================================
// UseCoupon 使用优惠券
// ========================================
func (s *CouponServer) UseCoupon(ctx context.Context, req *pb.UseCouponRequest) (*pb.UseCouponResponse, error) {
	err := s.couponApp.UseCoupon(ctx, uint(req.CouponId), uint(req.UserId), req.OrderId)
	if err != nil {
		logger.Error(fmt.Sprintf("使用优惠券失败, couponId: %d, userId: %d", req.CouponId, req.UserId), zap.Error(err))
		return &pb.UseCouponResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.UseCouponResponse{
		Code: 0,
		Msg:  "使用优惠券成功",
	}, nil
}

// ========================================
// GetCouponDetail 获取优惠券详情
// ========================================
func (s *CouponServer) GetCouponDetail(ctx context.Context, req *pb.GetCouponDetailRequest) (*pb.GetCouponDetailResponse, error) {
	coupon, err := s.couponApp.GetCouponDetail(ctx, uint(req.CouponId), uint(req.UserId))
	if err != nil {
		logger.Error(fmt.Sprintf("获取优惠券详情失败, couponId: %d, userId: %d", req.CouponId, req.UserId), zap.Error(err))
		return &pb.GetCouponDetailResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.GetCouponDetailResponse{
		Code:   0,
		Msg:    "获取优惠券详情成功",
		Coupon: toPbUserCoupon(coupon),
	}, nil
}
