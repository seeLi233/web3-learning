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

type MemberRepo struct {
	db *gorm.DB
}

func NewMemberRepo(db *gorm.DB) *MemberRepo {
	return &MemberRepo{db: db}
}

func (r *MemberRepo) UpdateMemberInfo(ctx context.Context, member *entity.MemberInfo) error {
	tracer := otel.GetTracer("DB-UpdateMemberInfo")
	ctx, span := tracer.Start(ctx, "DB-UpdateMemberInfo")
	defer span.End()

	return r.db.WithContext(ctx).Save(member).Error
}

// UpdateLevel 只更新 level_id（升级/降级专用）
// 核心知识点：用 Updates + map 只更新指定字段，避免 Save 更新所有字段的坑
func (r *MemberRepo) UpdateLevel(ctx context.Context, userId uint, levelID uint) error {
	tracer := otel.GetTracer("DB-UpdateLevel")
	ctx, span := tracer.Start(ctx, "DB-UpdateLevel")
	defer span.End()

	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.MemberInfo{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"level_id":      levelID,
			"level_up_time": now,
		}).Error
}

func (r *MemberRepo) GetMemberInfo(ctx context.Context, userId uint) (*entity.MemberInfo, error) {
	tracer := otel.GetTracer("DB-GetMemberInfo")
	ctx, span := tracer.Start(ctx, "DB-GetMemberInfo")
	defer span.End()

	var member entity.MemberInfo

	err := r.db.Preload("Level").WithContext(ctx).Where("user_id = ?", userId).First(&member).Error
	if err != nil {
		logger.Error(fmt.Sprintf("会员查询失败: userId[%d]", userId), zap.Error(err))
		return nil, err
	}

	return &member, err
}

func (r *MemberRepo) AddGrowth(ctx context.Context, userId uint, value int, sourceType, sourceId, description string) (*entity.MemberInfo, error) {
	tracer := otel.GetTracer("DB-AddGrowth")
	ctx, span := tracer.Start(ctx, "DB-AddGrowth")
	defer span.End()

	// 原子更新：一次 SQL 同时增加 growth_value 和 total_growth
	if err := r.db.WithContext(ctx).Model(&entity.MemberInfo{}).
		Where("user_id = ?", userId).
		Updates(map[string]interface{}{
			"growth_value": gorm.Expr("growth_value + ?", value),
			"total_growth": gorm.Expr("total_growth + ?", value),
		}).Error; err != nil {
		logger.Error(fmt.Sprintf("用户成长值更新失败,userId: %d", userId), zap.Error(err))
		return nil, err
	}

	// 创建日志
	if err := r.CreateGrowLog(ctx, &entity.MemberGrowthLog{
		UserID:      userId,
		ChangeValue: value,
		SourceType:  sourceType,
		SourceID:    sourceId,
		Description: description,
	}); err != nil {
		logger.Error(fmt.Sprintf("日志创建失败,userId: %d", userId), zap.Error(err))
		return nil, err
	}

	// 重新查询返回最新数据（原子更新后旧对象是过期的）
	return r.GetMemberInfo(ctx, userId)
}

func (r *MemberRepo) DeductGrowth(ctx context.Context, userId uint, value int, sourceType, sourceId, description string) (*entity.MemberInfo, error) {
	tracer := otel.GetTracer("DB-DeductGrowth")
	ctx, span := tracer.Start(ctx, "DB-DeductGrowth")
	defer span.End()

	// 扣减时只减 growth_value，total_growth 是累计值只增不减
	if err := r.db.WithContext(ctx).Model(&entity.MemberInfo{}).
		Where("user_id = ?", userId).
		Update("growth_value", gorm.Expr("growth_value - ?", value)).Error; err != nil {
		logger.Error(fmt.Sprintf("用户成长值扣减失败,userId: %d", userId), zap.Error(err))
		return nil, err
	}

	// 创建日志（负数）
	if err := r.CreateGrowLog(ctx, &entity.MemberGrowthLog{
		UserID:      userId,
		ChangeValue: -value,
		SourceType:  sourceType,
		SourceID:    sourceId,
		Description: description,
	}); err != nil {
		logger.Error(fmt.Sprintf("日志创建失败,userId: %d", userId), zap.Error(err))
		return nil, err
	}

	// 重新查询返回最新数据
	return r.GetMemberInfo(ctx, userId)
}

func (r *MemberRepo) CreateGrowLog(ctx context.Context, log *entity.MemberGrowthLog) error {
	tracer := otel.GetTracer("DB-CreateGrowLog")
	ctx, span := tracer.Start(ctx, "DB-CreateGrowLog")
	defer span.End()

	return r.db.WithContext(ctx).Create(log).Error
}

func (r *MemberRepo) GetGrowthLogs(ctx context.Context, userId uint, page, size int) ([]*entity.MemberGrowthLog, error) {
	tracer := otel.GetTracer("DB-GetGrowthLogs")
	ctx, span := tracer.Start(ctx, "DB-GetGrowthLogs")
	defer span.End()

	var logs []*entity.MemberGrowthLog
	err := r.db.WithContext(ctx).Model(&entity.MemberGrowthLog{}).Where("user_id = ?", userId).Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&logs).Error

	return logs, err
}

func (r *MemberRepo) GetGrowthLogsCount(ctx context.Context, userId uint) (int, error) {
	tracer := otel.GetTracer("DB-GetGrowthLogsCount")
	ctx, span := tracer.Start(ctx, "DB-GetGrowthLogsCount")
	defer span.End()

	var count int64
	err := r.db.WithContext(ctx).Model(&entity.MemberGrowthLog{}).Where("user_id = ?", userId).Count(&count).Error
	return int(count), err
}

func (r *MemberRepo) CreateMemberInfo(ctx context.Context, member *entity.MemberInfo) error {
	tracer := otel.GetTracer("DB-CreateMemberInfo")
	ctx, span := tracer.Start(ctx, "DB-CreateMemberInfo")
	defer span.End()

	return r.db.WithContext(ctx).Create(member).Error
}

func (r *MemberRepo) ListMemberLevels(ctx context.Context) ([]*entity.MemberLevel, error) {
	tracer := otel.GetTracer("DB-ListMemberLevels")
	ctx, span := tracer.Start(ctx, "DB-ListMemberLevels")
	defer span.End()

	var levels []*entity.MemberLevel
	err := r.db.WithContext(ctx).Model(&entity.MemberLevel{}).Find(&levels).Error

	return levels, err
}
