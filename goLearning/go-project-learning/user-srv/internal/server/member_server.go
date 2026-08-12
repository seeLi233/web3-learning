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

type MemberServer struct {
	pb.UnimplementedMemberServiceServer
	memberApp *application.MemberApp
}

func NewMemberServer(memberApp *application.MemberApp) *MemberServer {
	return &MemberServer{
		memberApp: memberApp,
	}
}

// toPbMemberInfo 实体转 pb（抽取公共方法，避免重复代码）
func toPbMemberInfo(member *entity.MemberInfo) *pb.MemberInfo {
	// 核心知识点：*time.Time 可能为 nil，需要判空再格式化
	var levelUpTime string
	if member.LevelUpTime != nil {
		levelUpTime = member.LevelUpTime.Format("2006-01-02 15:04:05")
	}

	return &pb.MemberInfo{
		Id:          int64(member.ID),
		UserId:      int64(member.UserID),
		LevelId:     int64(member.LevelID),
		GrowthValue: int32(member.GrowthValue),
		TotalGrowth: int32(member.TotalGrowth),
		LevelUpTime: levelUpTime,
		Level: &pb.MemberLevel{
			Id:          int64(member.Level.ID),
			LevelName:   member.Level.LevelName,
			LevelValue:  int32(member.Level.LevelValue),
			MinGrowth:   int32(member.Level.MinGrowth),
			MaxGrowth:   int32(member.Level.MaxGrowth),
			Discount:    member.Level.Discount.InexactFloat64(),
			Icon:        member.Level.Icon,
			Description: member.Level.Description,
		},
	}
}

// ========================================
// GetMemberInfo 获取会员信息
// ========================================
func (s *MemberServer) GetMemberInfo(ctx context.Context, req *pb.GetMemberInfoRequest) (*pb.GetMemberInfoResponse, error) {
	member, err := s.memberApp.GetMemberInfo(ctx, uint(req.UserId))
	if err != nil {
		logger.Error(fmt.Sprintf("获取会员信息失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.GetMemberInfoResponse{
			Code: 50001,
			Msg:  "获取会员信息失败",
		}, nil
	}

	return &pb.GetMemberInfoResponse{
		Code:   0,
		Msg:    "获取会员信息成功",
		Member: toPbMemberInfo(member),
	}, nil
}

// ========================================
// AddGrowth 增加成长值
// ========================================
func (s *MemberServer) AddGrowth(ctx context.Context, req *pb.AddGrowthRequest) (*pb.AddGrowthResponse, error) {
	member, err := s.memberApp.AddGrowth(ctx, uint(req.UserId), int(req.Growth), req.SourceType, req.SourceId, req.Description)
	if err != nil {
		logger.Error(fmt.Sprintf("增加成长值失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.AddGrowthResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.AddGrowthResponse{
		Code:   0,
		Msg:    "增加成长值成功",
		Member: toPbMemberInfo(member),
	}, nil
}

// ========================================
// DeductGrowth 扣减成长值
// ========================================
func (s *MemberServer) DeductGrowth(ctx context.Context, req *pb.DeductGrowthRequest) (*pb.DeductGrowthResponse, error) {
	member, err := s.memberApp.DeductGrowth(ctx, uint(req.UserId), int(req.Growth), req.SourceType, req.SourceId, req.Description)
	if err != nil {
		logger.Error(fmt.Sprintf("扣减成长值失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.DeductGrowthResponse{
			Code: 50001,
			Msg:  err.Error(),
		}, nil
	}

	return &pb.DeductGrowthResponse{
		Code:   0,
		Msg:    "扣减成长值成功",
		Member: toPbMemberInfo(member),
	}, nil
}

// ========================================
// GetGrowthLogs 获取成长值变动日志
// ========================================
func (s *MemberServer) GetGrowthLogs(ctx context.Context, req *pb.GetGrowthLogsRequest) (*pb.GetGrowthLogsResponse, error) {
	logs, total, err := s.memberApp.GetGrowthLogs(ctx, uint(req.UserId), int(req.Page), int(req.Size))
	if err != nil {
		logger.Error(fmt.Sprintf("获取成长值日志失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.GetGrowthLogsResponse{
			Code: 50001,
			Msg:  "获取日志失败",
		}, nil
	}

	pbLogs := make([]*pb.GrowthLog, len(logs))
	for i, log := range logs {
		pbLogs[i] = &pb.GrowthLog{
			Id:          int64(log.ID),
			UserId:      int64(log.UserID),
			ChangeValue: int32(log.ChangeValue),
			SourceType:  log.SourceType,
			SourceId:    log.SourceID,
			Description: log.Description,
			CreatedAt:   log.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	return &pb.GetGrowthLogsResponse{
		Code:  0,
		Msg:   "获取日志成功",
		Data:  pbLogs,
		Total: int32(total),
	}, nil
}

// ========================================
// GetMemberBenefits 获取会员权益
// ========================================
func (s *MemberServer) GetMemberBenefits(ctx context.Context, req *pb.GetMemberBenefitsRequest) (*pb.GetMemberBenefitsResponse, error) {
	benefits, err := s.memberApp.GetMemberBenefits(ctx, uint(req.UserId))
	if err != nil {
		logger.Error(fmt.Sprintf("获取会员权益失败, userId: %d", req.UserId), zap.Error(err))
		return &pb.GetMemberBenefitsResponse{
			Code: 50001,
			Msg:  "获取权益失败",
		}, nil
	}

	return &pb.GetMemberBenefitsResponse{
		Code: 0,
		Msg:  "获取权益成功",
		Data: benefits,
	}, nil
}

// ========================================
// ListMemberLevels 获取所有等级配置
// ========================================
func (s *MemberServer) ListMemberLevels(ctx context.Context, req *pb.ListMemberLevelsRequest) (*pb.ListMemberLevelsResponse, error) {
	levels, currentLevel, err := s.memberApp.ListMemberLevels(ctx, uint(req.UserId))
	if err != nil {
		logger.Error("获取等级配置失败", zap.Error(err))
		return &pb.ListMemberLevelsResponse{
			Code: 50001,
			Msg:  "获取等级配置失败",
		}, nil
	}

	pbLevels := make([]*pb.MemberLevel, len(levels))
	for i, level := range levels {
		pbLevels[i] = &pb.MemberLevel{
			Id:          int64(level.ID),
			LevelName:   level.LevelName,
			LevelValue:  int32(level.LevelValue),
			MinGrowth:   int32(level.MinGrowth),
			MaxGrowth:   int32(level.MaxGrowth),
			Discount:    level.Discount.InexactFloat64(),
			Icon:        level.Icon,
			Description: level.Description,
		}
	}

	return &pb.ListMemberLevelsResponse{
		Code:         0,
		Msg:          "获取等级配置成功",
		Data:         pbLevels,
		CurrentLevel: int32(currentLevel),
	}, nil
}
