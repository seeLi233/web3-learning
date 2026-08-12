package application

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
)

type CouponApp struct {
	couponRepo *db.CouponRepo
}

func NewCouponApp(couponRepo *db.CouponRepo) *CouponApp {
	return &CouponApp{couponRepo: couponRepo}
}

// ========================================
// ListTemplates 获取可领取的优惠券列表
// ========================================
func (a *CouponApp) ListTemplates(ctx context.Context, page, size int) ([]*entity.CouponTemplate, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	return a.couponRepo.ListTemplates(ctx, page, size)
}

// ========================================
// ClaimCoupon 领取优惠券
// ========================================
func (a *CouponApp) ClaimCoupon(ctx context.Context, userID, templateID uint) (*entity.UserCoupon, error) {
	// 1. 查询模板是否存在
	template, err := a.couponRepo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("优惠券模板不存在: %w", err)
	}

	// 2. 校验模板状态
	if template.Status != 1 {
		return nil, fmt.Errorf("优惠券已禁用")
	}

	// 3. 校验有效期
	now := time.Now()
	if now.Before(template.StartTime) {
		return nil, fmt.Errorf("优惠券尚未开始")
	}
	if now.After(template.EndTime) {
		return nil, fmt.Errorf("优惠券已过期")
	}

	// 4. 校验库存
	if template.TotalCount != -1 && template.ClaimedCount >= template.TotalCount {
		return nil, fmt.Errorf("优惠券已领完")
	}

	// 5. 校验用户领取上限
	count, err := a.couponRepo.CountUserCoupons(ctx, userID, templateID)
	if err != nil {
		return nil, fmt.Errorf("查询领取数量失败: %w", err)
	}
	if count >= template.PerUserLimit {
		return nil, fmt.Errorf("已达领取上限")
	}

	// 6. 原子递增已领取数量（库存扣减）
	if err := a.couponRepo.IncrementClaimedCount(ctx, templateID); err != nil {
		return nil, fmt.Errorf("领取失败: %w", err)
	}

	// 7. 创建用户优惠券记录
	coupon := &entity.UserCoupon{
		UserID:     userID,
		TemplateID: templateID,
		Status:     1, // 未使用
		ClaimedAt:  now,
	}

	if err := a.couponRepo.CreateUserCoupon(ctx, coupon); err != nil {
		return nil, fmt.Errorf("创建优惠券记录失败: %w", err)
	}

	// 8. 查询并返回完整的优惠券信息（含模板）
	return a.couponRepo.GetUserCouponByID(ctx, coupon.ID)
}

// ========================================
// ListUserCoupons 获取我的优惠券列表
// ========================================
func (a *CouponApp) ListUserCoupons(ctx context.Context, userID uint, status, page, size int) ([]*entity.UserCoupon, int, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	return a.couponRepo.ListUserCoupons(ctx, userID, status, page, size)
}

// ========================================
// GetAvailableCoupons 获取可用优惠券（下单时）
// ========================================
func (a *CouponApp) GetAvailableCoupons(ctx context.Context, userID uint, orderAmount float64) ([]*entity.UserCoupon, error) {
	return a.couponRepo.GetAvailableCoupons(ctx, userID, orderAmount)
}

// ========================================
// UseCoupon 使用优惠券
// ========================================
func (a *CouponApp) UseCoupon(ctx context.Context, couponID, userID uint, orderID string) error {
	// 1. 参数校验
	if orderID == "" {
		return fmt.Errorf("订单ID不能为空")
	}

	// 2. 查询优惠券是否存在且属于该用户
	coupon, err := a.couponRepo.GetUserCouponByID(ctx, couponID)
	if err != nil {
		return fmt.Errorf("优惠券不存在: %w", err)
	}

	// 3. 校验归属
	if coupon.UserID != userID {
		return fmt.Errorf("优惠券不属于当前用户")
	}

	// 4. 校验状态
	if coupon.Status != 1 {
		return fmt.Errorf("优惠券不可使用")
	}

	// 5. 校验有效期
	now := time.Now()
	if now.Before(coupon.Template.StartTime) || now.After(coupon.Template.EndTime) {
		return fmt.Errorf("优惠券不在有效期内")
	}

	// 6. 更新优惠券状态
	return a.couponRepo.UseCoupon(ctx, couponID, userID, orderID)
}

// ========================================
// GetCouponDetail 获取优惠券详情
// ========================================
func (a *CouponApp) GetCouponDetail(ctx context.Context, couponID, userID uint) (*entity.UserCoupon, error) {
	coupon, err := a.couponRepo.GetUserCouponByID(ctx, couponID)
	if err != nil {
		return nil, fmt.Errorf("优惠券不存在: %w", err)
	}

	// 校验归属
	if coupon.UserID != userID {
		return nil, fmt.Errorf("优惠券不属于当前用户")
	}

	return coupon, nil
}
