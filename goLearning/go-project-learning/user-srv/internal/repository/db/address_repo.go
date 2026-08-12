package db

import (
	"context"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/common/pkg/otel"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/cache"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AddressRepo struct {
	db *gorm.DB
}

func NewAddressRepo(db *gorm.DB) *AddressRepo {
	return &AddressRepo{db: db}
}

func (r *AddressRepo) Create(ctx context.Context, addr *entity.Address) (*entity.Address, error) {
	tracer := otel.GetTracer("DB-Create")
	ctx, span := tracer.Start(ctx, "DB-Create")
	defer span.End()

	err := r.db.WithContext(ctx).Create(addr).Error
	if err != nil {
		logger.Error("地址创建失败", zap.Error(err))
		return nil, err
	}

	return addr, err
}

func (r *AddressRepo) Delete(ctx context.Context, id uint) error {
	tracer := otel.GetTracer("DB-Delete")
	ctx, span := tracer.Start(ctx, "DB-Delete")
	defer span.End()

	err := r.db.WithContext(ctx).Delete(&entity.Address{}, id).Error
	if err == nil {
		cache.DeleteAddressCache(ctx, id)
	}
	return err
}

func (r *AddressRepo) Update(ctx context.Context, addr *entity.Address) (*entity.Address, error) {
	tracer := otel.GetTracer("DB-Update")
	ctx, span := tracer.Start(ctx, "DB-Update")
	defer span.End()

	err := r.db.WithContext(ctx).Save(addr).Error
	if err == nil {
		cache.DeleteAddressCache(ctx, addr.ID)
	}
	return addr, err
}

func (r *AddressRepo) GetByID(ctx context.Context, id uint) (*entity.Address, error) {
	tracer := otel.GetTracer("DB-GetByID")
	ctx, span := tracer.Start(ctx, "DB-GetByID")
	defer span.End()

	var address entity.Address
	err := r.db.WithContext(ctx).First(&address, id).Error
	if err != nil {
		logger.Error("地址不存在", zap.Error(err))
		return nil, err
	}
	return &address, nil
}

func (r *AddressRepo) ListByUserID(ctx context.Context, userId uint) ([]*entity.Address, error) {
	var addresses []*entity.Address

	tracer := otel.GetTracer("DB-ListByUserID")
	ctx, span := tracer.Start(ctx, "DB-ListByUserID")
	defer span.End()

	err := r.db.WithContext(ctx).Model(&entity.Address{}).Where("user_id = ?", userId).Find(&addresses).Error
	return addresses, err
}
