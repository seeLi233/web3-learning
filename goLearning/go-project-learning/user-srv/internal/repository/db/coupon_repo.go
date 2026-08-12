package db

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CouponRepo struct {
	db *gorm.DB
}

func NewCouponRepo(db *gorm.DB) *CouponRepo {
	return &CouponRepo{db: db}
}

// ========================================
// 优惠券模板操作
// ========================================

// ListTemplates 分页查询启用中的优惠券模板
func (r *CouponRepo) ListTemplates(ctx context.Context, page, size int) ([]*entity.CouponTemplate, int, error) {
	tracer := otel.GetTracer("DB-ListTemplates")
	ctx, span := tracer.Start(ctx, "DB-ListTemplates")
	defer span.End()

	var templates []*entity.CouponTemplate
	var total int64

	// 查询总数
	if err := r.db.WithContext(ctx).Model(&entity.CouponTemplate{}).
		Where("status = ?", 1).
		Count(&total).Error; err != nil {
		logger.Error("查询优惠券模板总数失败", zap.Error(err))
		return nil, 0, err
	}

	// 分页查询
	if err := r.db.WithContext(ctx).Model(&entity.CouponTemplate{}).
		Where("status = ?", 1).
		Order("created_at desc").
		Offset((page - 1) * size).
		Limit(size).
		Find(&templates).Error; err != nil {
		logger.Error("查询优惠券模板列表失败", zap.Error(err))
		return nil, 0, err
	}

	return templates, int(total), nil
}

// GetTemplateByID 查询模板详情
func (r *CouponRepo) GetTemplateByID(ctx context.Context, id uint) (*entity.CouponTemplate, error) {
	tracer := otel.GetTracer("DB-GetTemplateByID")
	ctx, span := tracer.Start(ctx, "DB-GetTemplateByID")
	defer span.End()

	var template entity.CouponTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error; err != nil {
		logger.Error(fmt.Sprintf("查询优惠券模板失败: id[%d]", id), zap.Error(err))
		return nil, err
	}

	return &template, nil
}

// IncrementClaimedCount 原子递增已领取数量
func (r *CouponRepo) IncrementClaimedCount(ctx context.Context, templateID uint) error {
	tracer := otel.GetTracer("DB-IncrementClaimedCount")
	ctx, span := tracer.Start(ctx, "DB-IncrementClaimedCount")
	defer span.End()

	result := r.db.WithContext(ctx).Model(&entity.CouponTemplate{}).
		Where("id = ? AND (total_count = -1 OR claimed_count < total_count)", templateID).
		Update("claimed_count", gorm.Expr("claimed_count + 1"))

	if result.Error != nil {
		logger.Error(fmt.Sprintf("递增领取数量失败: templateID[%d]", templateID), zap.Error(result.Error))
		return result.Error
	}

	// 没有行被更新说明库存不足
	if result.RowsAffected == 0 {
		return fmt.Errorf("优惠券库存不足或已领完")
	}

	return nil
}

// ========================================
// 用户优惠券操作
// ========================================

// CreateUserCoupon 创建用户优惠券记录
func (r *CouponRepo) CreateUserCoupon(ctx context.Context, coupon *entity.UserCoupon) error {
	tracer := otel.GetTracer("DB-CreateUserCoupon")
	ctx, span := tracer.Start(ctx, "DB-CreateUserCoupon")
	defer span.End()

	return r.db.WithContext(ctx).Create(coupon).Error
}

// CountUserCoupons 统计用户已领取某模板的数量
func (r *CouponRepo) CountUserCoupons(ctx context.Context, userID, templateID uint) (int, error) {
	tracer := otel.GetTracer("DB-CountUserCoupons")
	ctx, span := tracer.Start(ctx, "DB-CountUserCoupons")
	defer span.End()

	var count int64
	if err := r.db.WithContext(ctx).Model(&entity.UserCoupon{}).
		Where("user_id = ? AND template_id = ?", userID, templateID).
		Count(&count).Error; err != nil {
		logger.Error(fmt.Sprintf("统计用户优惠券数量失败: userID[%d] templateID[%d]", userID, templateID), zap.Error(err))
		return 0, err
	}

	return int(count), nil
}

// ListUserCoupons 分页查询用户优惠券
func (r *CouponRepo) ListUserCoupons(ctx context.Context, userID uint, status int, page, size int) ([]*entity.UserCoupon, int, error) {
	tracer := otel.GetTracer("DB-ListUserCoupons")
	ctx, span := tracer.Start(ctx, "DB-ListUserCoupons")
	defer span.End()

	var coupons []*entity.UserCoupon
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.UserCoupon{}).Where("user_id = ?", userID)
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	// 查询总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error(fmt.Sprintf("查询用户优惠券总数失败: userID[%d]", userID), zap.Error(err))
		return nil, 0, err
	}

	// 分页查询，预加载模板信息
	if err := query.Preload("Template").
		Order("created_at desc").
		Offset((page - 1) * size).
		Limit(size).
		Find(&coupons).Error; err != nil {
		logger.Error(fmt.Sprintf("查询用户优惠券列表失败: userID[%d]", userID), zap.Error(err))
		return nil, 0, err
	}

	return coupons, int(total), nil
}

// GetUserCouponByID 查询用户优惠券详情
func (r *CouponRepo) GetUserCouponByID(ctx context.Context, id uint) (*entity.UserCoupon, error) {
	tracer := otel.GetTracer("DB-GetUserCouponByID")
	ctx, span := tracer.Start(ctx, "DB-GetUserCouponByID")
	defer span.End()

	var coupon entity.UserCoupon
	if err := r.db.WithContext(ctx).Preload("Template").Where("id = ?", id).First(&coupon).Error; err != nil {
		logger.Error(fmt.Sprintf("查询用户优惠券详情失败: id[%d]", id), zap.Error(err))
		return nil, err
	}

	return &coupon, nil
}

// GetAvailableCoupons 查询用户可用优惠券（未使用 + 在有效期内 + 满足最低消费）
func (r *CouponRepo) GetAvailableCoupons(ctx context.Context, userID uint, orderAmount float64) ([]*entity.UserCoupon, error) {
	tracer := otel.GetTracer("DB-GetAvailableCoupons")
	ctx, span := tracer.Start(ctx, "DB-GetAvailableCoupons")
	defer span.End()

	var coupons []*entity.UserCoupon
	now := time.Now()

	if err := r.db.WithContext(ctx).
		Preload("Template").
		Joins("JOIN coupon_templates ON coupon_templates.id = user_coupons.template_id").
		Where("user_coupons.user_id = ?", userID).
		Where("user_coupons.status = ?", 1). // 未使用
		Where("coupon_templates.start_time <= ?", now).
		Where("coupon_templates.end_time >= ?", now).
		Where("coupon_templates.min_amount <= ?", orderAmount).
		Where("coupon_templates.status = ?", 1).
		Order("user_coupons.created_at desc").
		Find(&coupons).Error; err != nil {
		logger.Error(fmt.Sprintf("查询可用优惠券失败: userID[%d]", userID), zap.Error(err))
		return nil, err
	}

	return coupons, nil
}

// UseCoupon 使用优惠券（更新状态和订单ID）
func (r *CouponRepo) UseCoupon(ctx context.Context, couponID, userID uint, orderID string) error {
	tracer := otel.GetTracer("DB-UseCoupon")
	ctx, span := tracer.Start(ctx, "DB-UseCoupon")
	defer span.End()

	now := time.Now()
	result := r.db.WithContext(ctx).Model(&entity.UserCoupon{}).
		Where("id = ? AND user_id = ? AND status = ?", couponID, userID, 1).
		Updates(map[string]interface{}{
			"status":   2,
			"used_at":  now,
			"order_id": orderID,
		})

	if result.Error != nil {
		logger.Error(fmt.Sprintf("使用优惠券失败: couponID[%d] userID[%d]", couponID, userID), zap.Error(result.Error))
		return result.Error
	}

	// 没有行被更新说明优惠券状态不对
	if result.RowsAffected == 0 {
		return fmt.Errorf("优惠券不存在或已被使用")
	}

	return nil
}
