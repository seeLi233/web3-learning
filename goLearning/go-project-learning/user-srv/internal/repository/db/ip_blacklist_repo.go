package db

import (
	"context"
	"time"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"gorm.io/gorm"
)

type IpBlacklistRepo struct {
	db *gorm.DB
}

func NewIpBlacklistRepo(db *gorm.DB) *IpBlacklistRepo {
	return &IpBlacklistRepo{db: db}
}

// Add 添加黑名单
func (r *IpBlacklistRepo) Add(ctx context.Context, blacklist *entity.IpBlackList) error {
	return r.db.WithContext(ctx).Create(blacklist).Error
}

// Remove 移除黑名单（软删除）
func (r *IpBlacklistRepo) Remove(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.IpBlackList{}, id).Error
}

// List 获取黑名单列表
func (r *IpBlacklistRepo) List(ctx context.Context, page, size int32) ([]entity.IpBlackList, int64, error) {
	var blacklists []entity.IpBlackList
	var total int64

	r.db.WithContext(ctx).Model(&entity.IpBlackList{}).Count(&total)
	err := r.db.WithContext(ctx).
		Offset(int((page - 1) * size)).
		Limit(int(size)).
		Order("created_at DESC").
		Find(&blacklists).Error

	return blacklists, total, err
}

// GetByIP 根据 IP 查询黑名单
func (r *IpBlacklistRepo) GetByIP(ctx context.Context, ip string) (*entity.IpBlackList, error) {
	var blacklist entity.IpBlackList
	err := r.db.WithContext(ctx).Where("ip = ? AND status = ?", ip, true).First(&blacklist).Error
	if err != nil {
		return nil, err
	}
	return &blacklist, nil
}

// GetActive 获取所有生效的黑名单（同步到 Redis 用）
func (r *IpBlacklistRepo) GetActive(ctx context.Context) ([]entity.IpBlackList, error) {
	var blacklists []entity.IpBlackList
	err := r.db.WithContext(ctx).
		Where("status = ? AND (deadline IS NULL OR deadline > ?)", true, time.Now()).
		Find(&blacklists).Error
	return blacklists, err
}
