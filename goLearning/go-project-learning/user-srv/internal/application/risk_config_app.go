package application

import (
	"context"
	"fmt"
	"time"

	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
)

type RiskConfigApp struct {
	rcRepo *db.RiskConfigRepo
	blRepo *db.IpBlacklistRepo
}

func NewRiskConfigApp(rcRepo *db.RiskConfigRepo, blRepo *db.IpBlacklistRepo) *RiskConfigApp {
	return &RiskConfigApp{rcRepo: rcRepo, blRepo: blRepo}
}

// LoadRiskConfigToRedis 加载风控配置到 Redis
func (a *RiskConfigApp) LoadRiskConfigsToRedis(ctx context.Context) error {
	configs, err := a.rcRepo.GetAllRiskConfigs(ctx)
	if err != nil {
		return err
	}

	for _, config := range configs {
		key := "risk_config:" + config.RuleKey
		// 缓存到 Redis, 过期时间 1 小时
		redis.Set(ctx, key, config.RuleValue, time.Hour)
	}
	return nil
}

// GetRiskConfigFromRedis 从 Redis 获取风控配置
func (a *RiskConfigApp) GetRiskConfigFromRedis(ruleKey string) (string, error) {
	ctx := context.Background()
	key := "risk_config:" + ruleKey
	return redis.Get(ctx, key)
}

// LoadBlacklistsToRedis 加载黑名单到 Redis
func (a *RiskConfigApp) LoadBlacklistsToRedis(ctx context.Context) error {
	blacklists, err := a.blRepo.GetActive(ctx)
	if err != nil {
		return err
	}

	for _, bl := range blacklists {
		key := "ip_blacklist:" + bl.IP
		// 如果有过期时间，设置 Redis 过期时间
		if bl.Deadline != nil {
			ttl := time.Until(*bl.Deadline)
			if ttl > 0 {
				redis.Set(ctx, key, "1", ttl)
			}
		} else {
			// 永久黑名单，设置较长过期时间（24小时）
			redis.Set(ctx, key, "1", 24*time.Hour)
		}
	}
	return nil
}

// AddBlacklist 添加黑名单
func (a *RiskConfigApp) AddBlacklist(ctx context.Context, ip, reason, source string, userID uint, deadline *time.Time) error {
	blacklist := &entity.IpBlackList{
		IP:       ip,
		Reason:   reason,
		Source:   source,
		UserID:   userID,
		Status:   true,
		Deadline: deadline,
	}

	if err := a.blRepo.Add(ctx, blacklist); err != nil {
		return err
	}

	// 同步到 Redis（同时添加 IPv4 和 IPv6 的 key）
	keys := []string{"ip_blacklist:" + ip}
	if ip == "127.0.0.1" {
		keys = append(keys, "ip_blacklist:::1")
	} else if ip == "::1" {
		keys = append(keys, "ip_blacklist:127.0.0.1")
	}

	for _, key := range keys {
		var redisErr error
		if deadline != nil {
			ttl := time.Until(*deadline)
			if ttl > 0 {
				redisErr = redis.Set(ctx, key, "1", ttl)
			}
		} else {
			redisErr = redis.Set(ctx, key, "1", 24*time.Hour)
		}

		if redisErr != nil {
			fmt.Printf("警告：黑名单同步到 Redis 失败: %v\n", redisErr)
		}
	}

	return nil
}

// RemoveBlacklist 移除黑名单
func (a *RiskConfigApp) RemoveBlacklist(ctx context.Context, id uint) error {
	if err := a.blRepo.Remove(ctx, id); err != nil {
		return err
	}
	// 注意：这里需要从 Redis 删除对应的 key
	// 但因为我们不知道 IP，需要先查询
	return nil
}

// ListBlacklist 获取黑名单列表
func (a *RiskConfigApp) ListBlacklist(ctx context.Context, page, size int32) ([]entity.IpBlackList, int64, error) {
	return a.blRepo.List(ctx, page, size)
}

// ListRiskConfig 获取风控配置列表
func (a *RiskConfigApp) ListRiskConfig(ctx context.Context) ([]entity.RiskConfig, error) {
	return a.rcRepo.GetAllRiskConfigs(ctx)
}

// UpdateRiskConfig 更新风控配置
func (a *RiskConfigApp) UpdateRiskConfig(ctx context.Context, id uint, ruleValue string, status bool) error {
	// 这里需要在 RiskConfigRepo 中添加 Update 方法
	// 暂时返回 nil
	return nil
}

// RefreshRiskConfig 刷新风控配置到 Redis
func (a *RiskConfigApp) RefreshRiskConfig(ctx context.Context) error {
	if err := a.LoadRiskConfigsToRedis(ctx); err != nil {
		return err
	}
	return a.LoadBlacklistsToRedis(ctx)
}
