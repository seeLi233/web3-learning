package db

import (
	"context"

	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"gorm.io/gorm"
)

type RiskConfigRepo struct {
	db *gorm.DB
}

func NewRiskConfigRepo(db *gorm.DB) *RiskConfigRepo {
	return &RiskConfigRepo{db: db}
}

// GetAllRiskConfigs 获取所有启用的风控配置
func (r *RiskConfigRepo) GetAllRiskConfigs(ctx context.Context) ([]entity.RiskConfig, error) {
	tracer := otel.GetTracer("DB-GetAllRiskConfigs")
	ctx, span := tracer.Start(ctx, "DB-GetAllRiskConfigs")
	defer span.End()

	var configs []entity.RiskConfig
	err := r.db.WithContext(ctx).Where("status = ?", true).Find(&configs).Error
	return configs, err
}

// GetRiskConfigByKey 根据 key 获取配置
func (r *RiskConfigRepo) GetRiskConfigByKey(ctx context.Context, key string) (entity.RiskConfig, error) {
	tracer := otel.GetTracer("DB-GetRiskConfigByKey")
	ctx, span := tracer.Start(ctx, "DB-GetRiskConfigByKey")
	defer span.End()

	var config entity.RiskConfig
	err := r.db.WithContext(ctx).Where("rule_key = ? AND status = ?", key, true).First(&config).Error
	return config, err
}
