package application

import (
	"context"
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
	"go.uber.org/zap"
)

type MemberApp struct {
	memberRepo *db.MemberRepo
}

func NewMemberApp(memberRepo *db.MemberRepo) *MemberApp {
	return &MemberApp{memberRepo: memberRepo}
}

// ========================================
// GetMemberInfo 获取会员信息（查不到则自动初始化）
// ========================================
func (a *MemberApp) GetMemberInfo(ctx context.Context, userId uint) (*entity.MemberInfo, error) {
	member, err := a.memberRepo.GetMemberInfo(ctx, userId)
	if err != nil {
		// 查不到记录 → 懒初始化（兼容会员功能上线前的老用户）
		if initErr := a.InitMember(ctx, userId); initErr != nil {
			return nil, fmt.Errorf("会员初始化失败: %w", initErr)
		}
		return a.memberRepo.GetMemberInfo(ctx, userId)
	}
	return member, nil
}

// ========================================
// InitMember 初始化会员信息（用户注册时调用）
// ========================================
func (a *MemberApp) InitMember(ctx context.Context, userId uint) error {
	// 查询普通会员等级（level_value = 0）
	levels, err := a.memberRepo.ListMemberLevels(ctx)
	if err != nil {
		return fmt.Errorf("查询等级配置失败: %w", err)
	}

	var defaultLevel *entity.MemberLevel
	for _, level := range levels {
		if level.LevelValue == 0 {
			defaultLevel = level
			break
		}
	}

	if defaultLevel == nil {
		return fmt.Errorf("未找到默认会员等级配置")
	}

	member := &entity.MemberInfo{
		UserID:      userId,
		LevelID:     defaultLevel.ID,
		GrowthValue: 0,
		TotalGrowth: 0,
	}

	return a.memberRepo.CreateMemberInfo(ctx, member)
}

// ========================================
// AddGrowth 增加成长值 + 自动升级检查
// ========================================
func (a *MemberApp) AddGrowth(ctx context.Context, userId uint, growth int, sourceType, sourceId, description string) (*entity.MemberInfo, error) {
	// 1. 参数校验
	if growth <= 0 {
		return nil, fmt.Errorf("成长值必须大于0")
	}

	// 1.5 确保会员信息存在（懒初始化，兼容老用户）
	if _, err := a.GetMemberInfo(ctx, userId); err != nil {
		return nil, fmt.Errorf("会员信息初始化失败: %w", err)
	}

	// 2. 原子增加成长值
	member, err := a.memberRepo.AddGrowth(ctx, userId, growth, sourceType, sourceId, description)
	if err != nil {
		return nil, fmt.Errorf("增加成长值失败: %w", err)
	}

	// 3. 检查是否需要升级
	newLevelID, shouldUpgrade := a.checkUpgrade(ctx, member)
	if shouldUpgrade {
		if err := a.memberRepo.UpdateLevel(ctx, userId, newLevelID); err != nil {
			return nil, fmt.Errorf("升级失败: %w", err)
		}
		// 重新查询获取最新的 Level 关联数据
		return a.memberRepo.GetMemberInfo(ctx, userId)
	}

	return member, nil
}

// checkUpgrade 检查是否需要升级
// 核心知识点：遍历等级配置，找到 growth_value >= min_growth 的最高等级
func (a *MemberApp) checkUpgrade(ctx context.Context, member *entity.MemberInfo) (uint, bool) {
	levels, err := a.memberRepo.ListMemberLevels(ctx)
	if err != nil {
		logger.Error("查询等级配置失败", zap.Error(err))
		return 0, false
	}
	if len(levels) == 0 {
		logger.Warn("member_levels 表为空，无法判断升级")
		return 0, false
	}

	logger.Info(fmt.Sprintf("[升级检查] userId=%d, growth=%d, currentLevelId=%d, 等级配置数量=%d",
		member.UserID, member.GrowthValue, member.LevelID, len(levels)))

	// 找到 growth_value >= min_growth 的最高等级
	var bestLevel *entity.MemberLevel
	for _, level := range levels {
		logger.Info(fmt.Sprintf("[升级检查] 遍历等级: %s(level=%d), min=%d, max=%d",
			level.LevelName, level.LevelValue, level.MinGrowth, level.MaxGrowth))
		if member.GrowthValue >= level.MinGrowth {
			if level.MaxGrowth == -1 || member.GrowthValue <= level.MaxGrowth {
				bestLevel = level
				logger.Info(fmt.Sprintf("[升级检查] 命中等级: %s", level.LevelName))
			}
		}
	}

	if bestLevel != nil && bestLevel.ID != member.LevelID {
		return bestLevel.ID, true
	}
	return 0, false
}

// ========================================
// DeductGrowth 扣减成长值 + 自动降级检查
// ========================================
func (a *MemberApp) DeductGrowth(ctx context.Context, userId uint, growth int, sourceType, sourceId, description string) (*entity.MemberInfo, error) {
	// 1. 参数校验
	if growth <= 0 {
		return nil, fmt.Errorf("扣减值必须大于0")
	}

	// 2. 确保会员信息存在 + 检查扣减后会不会变负数
	member, err := a.GetMemberInfo(ctx, userId)
	if err != nil {
		return nil, err
	}
	if member.GrowthValue < growth {
		return nil, fmt.Errorf("成长值不足，当前: %d, 扣减: %d", member.GrowthValue, growth)
	}

	// 3. 原子扣减成长值
	member, err = a.memberRepo.DeductGrowth(ctx, userId, growth, sourceType, sourceId, description)
	if err != nil {
		return nil, fmt.Errorf("扣减成长值失败: %w", err)
	}

	// 4. 检查是否需要降级
	newLevelID, shouldDowngrade := a.checkDowngrade(ctx, member)
	if shouldDowngrade {
		if err := a.memberRepo.UpdateLevel(ctx, userId, newLevelID); err != nil {
			return nil, fmt.Errorf("降级失败: %w", err)
		}
		return a.memberRepo.GetMemberInfo(ctx, userId)
	}

	return member, nil
}

// checkDowngrade 检查是否需要降级
// 核心知识点：如果当前成长值 < 当前等级的 min_growth，就降级
func (a *MemberApp) checkDowngrade(ctx context.Context, member *entity.MemberInfo) (uint, bool) {
	// 如果当前成长值 >= 当前等级门槛，不需要降级
	if member.GrowthValue >= member.Level.MinGrowth {
		return 0, false
	}

	// 找到 growth_value >= min_growth 的最高等级（降级目标）
	levels, err := a.memberRepo.ListMemberLevels(ctx)
	if err != nil || len(levels) == 0 {
		return 0, false
	}

	var targetLevel *entity.MemberLevel
	for _, level := range levels {
		if member.GrowthValue >= level.MinGrowth {
			targetLevel = level
		}
	}

	if targetLevel != nil && targetLevel.ID != member.LevelID {
		return targetLevel.ID, true
	}
	return 0, false
}

// ========================================
// GetGrowthLogs 获取成长值变动日志（分页）
// ========================================
func (a *MemberApp) GetGrowthLogs(ctx context.Context, userId uint, page, size int) ([]*entity.MemberGrowthLog, int, error) {
	// 参数校验
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}

	// 查询日志列表
	logs, err := a.memberRepo.GetGrowthLogs(ctx, userId, page, size)
	if err != nil {
		return nil, 0, err
	}

	// 查询总数
	total, err := a.memberRepo.GetGrowthLogsCount(ctx, userId)
	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ========================================
// GetMemberBenefits 获取会员权益列表
// ========================================
func (a *MemberApp) GetMemberBenefits(ctx context.Context, userId uint) ([]*pb.Benefit, error) {
	member, err := a.GetMemberInfo(ctx, userId)
	if err != nil {
		return nil, err
	}

	// 根据等级返回不同权益（硬编码，实际项目可以做成配置表）
	return a.getBenefitsByLevel(member.Level.LevelValue), nil
}

// getBenefitsByLevel 根据等级值返回权益列表
// 核心知识点：等级越高，权益越多（累积式）
func (a *MemberApp) getBenefitsByLevel(levelValue int) []*pb.Benefit {
	// 基础权益（所有等级都有）
	benefits := []*pb.Benefit{
		{Name: "基础积分", Description: "消费1元=1积分", Icon: "icon_points"},
		{Name: "生日礼包", Description: "生日当天赠送优惠券", Icon: "icon_gift"},
	}

	// 银卡及以上
	if levelValue >= 1 {
		benefits = append(benefits, &pb.Benefit{
			Name:        "包邮券",
			Description: "每月赠送2张包邮券",
			Icon:        "icon_shipping",
		})
	}

	// 金卡及以上
	if levelValue >= 2 {
		benefits = append(benefits, &pb.Benefit{
			Name:        "专属折扣",
			Description: "全场商品95折",
			Icon:        "icon_discount",
		})
	}

	// 钻石
	if levelValue >= 3 {
		benefits = append(benefits, &pb.Benefit{
			Name:        "专属客服",
			Description: "7x24小时专属客服通道",
			Icon:        "icon_service",
		})
		benefits = append(benefits, &pb.Benefit{
			Name:        "优先发货",
			Description: "订单优先打包发货",
			Icon:        "icon_priority",
		})
	}

	return benefits
}

// ========================================
// ListMemberLevels 获取所有等级配置
// ========================================
func (a *MemberApp) ListMemberLevels(ctx context.Context, userId uint) ([]*entity.MemberLevel, int, error) {
	levels, err := a.memberRepo.ListMemberLevels(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 查询用户当前等级（懒初始化）
	member, err := a.GetMemberInfo(ctx, userId)
	if err != nil {
		return levels, 0, nil
	}

	return levels, member.Level.LevelValue, nil
}
