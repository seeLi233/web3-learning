package handler

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	"github.com/go-project-learning/project/user-srv/api/pb"
)

// AddBlacklist 添加黑名单
func AddBlacklist(c *gin.Context) {
	var req struct {
		IP       string `json:"ip" binding:"required"`
		Reason   string `json:"reason" binding:"required"`
		Deadline int64  `json:"deadline"` // 0=永久
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 从 JWT 获取操作人
	userID, _ := c.Get("userID")

	grpcResp, err := global.RiskClient.AddBlacklist(context.Background(), &pb.AddBlacklistRequest{
		Ip:       req.IP,
		Reason:   req.Reason,
		Source:   "manual",
		UserId:   uint32(userID.(uint)),
		Deadline: req.Deadline,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, nil)
}

// RemoveBlacklist 移除黑名单
func RemoveBlacklist(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		resp.Error(c, 10001, "缺少 id 参数")
		return
	}

	// 转换 id 为 uint32
	var idUint uint32
	if _, err := fmt.Sscanf(id, "%d", &idUint); err != nil {
		resp.Error(c, 10001, "id 参数格式错误")
		return
	}

	grpcResp, err := global.RiskClient.RemoveBlacklist(context.Background(), &pb.RemoveBlacklistRequest{
		Id: idUint,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, nil)
}

// ListBlacklist 获取黑名单列表
func ListBlacklist(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	size := c.DefaultQuery("size", "10")

	var pageInt, sizeInt int32
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(size, "%d", &sizeInt)

	grpcResp, err := global.RiskClient.ListBlacklist(context.Background(), &pb.ListBlacklistRequest{
		Page: pageInt,
		Size: sizeInt,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, gin.H{
		"list":  grpcResp.Data,
		"total": grpcResp.Total,
	})
}

// ListRiskConfig 获取风控配置列表
func ListRiskConfig(c *gin.Context) {
	grpcResp, err := global.RiskClient.ListRiskConfig(context.Background(), &pb.ListRiskConfigRequest{})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Data)
}

// UpdateRiskConfig 更新风控配置
func UpdateRiskConfig(c *gin.Context) {
	var req struct {
		ID        uint32 `json:"id" binding:"required"`
		RuleValue string `json:"rule_value" binding:"required"`
		Status    bool   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	grpcResp, err := global.RiskClient.UpdateRiskConfig(context.Background(), &pb.UpdateRiskConfigRequest{
		Id:        req.ID,
		RuleValue: req.RuleValue,
		Status:    req.Status,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, nil)
}

// RefreshRiskConfig 刷新风控配置
func RefreshRiskConfig(c *gin.Context) {
	grpcResp, err := global.RiskClient.RefreshRiskConfig(context.Background(), &pb.RefreshRiskConfigRequest{})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, gin.H{"message": "刷新成功"})
}
