package server

import (
	"context"
	"time"

	"github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/internal/application"
)

type RiskServer struct {
	pb.UnimplementedRiskServiceServer
	riskApp *application.RiskConfigApp
}

func NewRiskServer(riskApp *application.RiskConfigApp) *RiskServer {
	return &RiskServer{riskApp: riskApp}
}

// AddBlacklist 添加黑名单
func (s *RiskServer) AddBlacklist(ctx context.Context, req *pb.AddBlacklistRequest) (*pb.RiskResponse, error) {
	var deadline *time.Time
	if req.Deadline > 0 {
		t := time.Unix(req.Deadline, 0)
		deadline = &t
	}

	err := s.riskApp.AddBlacklist(ctx, req.Ip, req.Reason, req.Source, uint(req.UserId), deadline)
	if err != nil {
		return &pb.RiskResponse{Code: 50001, Msg: err.Error()}, nil
	}
	return &pb.RiskResponse{Code: 0, Msg: "success"}, nil
}

// RemoveBlacklist 移除黑名单
func (s *RiskServer) RemoveBlacklist(ctx context.Context, req *pb.RemoveBlacklistRequest) (*pb.RiskResponse, error) {
	err := s.riskApp.RemoveBlacklist(ctx, uint(req.Id))
	if err != nil {
		return &pb.RiskResponse{Code: 50001, Msg: err.Error()}, nil
	}
	return &pb.RiskResponse{Code: 0, Msg: "success"}, nil
}

// ListBlacklist 获取黑名单列表
func (s *RiskServer) ListBlacklist(ctx context.Context, req *pb.ListBlacklistRequest) (*pb.ListBlacklistResponse, error) {
	blacklists, total, err := s.riskApp.ListBlacklist(ctx, req.Page, req.Size)
	if err != nil {
		return &pb.ListBlacklistResponse{Code: 50001, Msg: err.Error()}, nil
	}

	err = s.riskApp.LoadBlacklistsToRedis(ctx)
	if err != nil {
		return &pb.ListBlacklistResponse{Code: 50001, Msg: err.Error()}, nil
	}

	var items []*pb.BlacklistItem
	for _, bl := range blacklists {
		item := &pb.BlacklistItem{
			Id:        uint32(bl.ID),
			Ip:        bl.IP,
			Reason:    bl.Reason,
			Source:    bl.Source,
			UserId:    uint32(bl.UserID),
			Status:    bl.Status,
			CreatedAt: bl.CreatedAt.Format(time.DateTime),
		}
		if bl.Deadline != nil {
			item.Deadline = bl.Deadline.Unix()
		}
		items = append(items, item)
	}

	return &pb.ListBlacklistResponse{
		Code:  0,
		Msg:   "success",
		Data:  items,
		Total: total,
	}, nil
}

// ListRiskConfig 获取风控配置列表
func (s *RiskServer) ListRiskConfig(ctx context.Context, req *pb.ListRiskConfigRequest) (*pb.ListRiskConfigResponse, error) {
	configs, err := s.riskApp.ListRiskConfig(ctx)
	if err != nil {
		return &pb.ListRiskConfigResponse{Code: 50001, Msg: err.Error()}, nil
	}

	var items []*pb.RiskConfigItem
	for _, cfg := range configs {
		items = append(items, &pb.RiskConfigItem{
			Id:          uint32(cfg.ID),
			RuleKey:     cfg.RuleKey,
			RuleValue:   cfg.RuleValue,
			Description: cfg.Description,
			Status:      cfg.Status,
		})
	}

	return &pb.ListRiskConfigResponse{
		Code: 0,
		Msg:  "success",
		Data: items,
	}, nil
}

// UpdateRiskConfig 更新风控配置
func (s *RiskServer) UpdateRiskConfig(ctx context.Context, req *pb.UpdateRiskConfigRequest) (*pb.RiskResponse, error) {
	err := s.riskApp.UpdateRiskConfig(ctx, uint(req.Id), req.RuleValue, req.Status)
	if err != nil {
		return &pb.RiskResponse{Code: 50001, Msg: err.Error()}, nil
	}
	return &pb.RiskResponse{Code: 0, Msg: "success"}, nil
}

// RefreshRiskConfig 刷新风控配置到 Redis
func (s *RiskServer) RefreshRiskConfig(ctx context.Context, req *pb.RefreshRiskConfigRequest) (*pb.RiskResponse, error) {
	err := s.riskApp.RefreshRiskConfig(ctx)
	if err != nil {
		return &pb.RiskResponse{Code: 50001, Msg: err.Error()}, nil
	}
	return &pb.RiskResponse{Code: 0, Msg: "success"}, nil
}
