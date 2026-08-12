package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	"github.com/go-project-learning/project/user-srv/api/pb"
)

// GetMemberInfo 获取当前用户的会员信息
func GetMemberInfo(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	grpcResp, err := global.MemberClient.GetMemberInfo(c.Request.Context(), &pb.GetMemberInfoRequest{
		UserId: int64(uid),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Member)
}

// AddGrowth 增加成长值（订单完成、评价、签到等场景）
func AddGrowth(c *gin.Context) {
	var req struct {
		UserID      int64  `json:"user_id" binding:"required"`
		Growth      int32  `json:"growth" binding:"required"`
		SourceType  string `json:"source_type" binding:"required"`
		SourceID    string `json:"source_id"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误")
		return
	}

	grpcResp, err := global.MemberClient.AddGrowth(c.Request.Context(), &pb.AddGrowthRequest{
		UserId:      req.UserID,
		Growth:      req.Growth,
		SourceType:  req.SourceType,
		SourceId:    req.SourceID,
		Description: req.Description,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Member)
}

// DeductGrowth 扣减成长值（管理员/退款场景）
func DeductGrowth(c *gin.Context) {
	var req struct {
		UserID      int64  `json:"user_id" binding:"required"`
		Growth      int32  `json:"growth" binding:"required"`
		SourceType  string `json:"source_type" binding:"required"`
		SourceID    string `json:"source_id"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误")
		return
	}

	grpcResp, err := global.MemberClient.DeductGrowth(c.Request.Context(), &pb.DeductGrowthRequest{
		UserId:      req.UserID,
		Growth:      req.Growth,
		SourceType:  req.SourceType,
		SourceId:    req.SourceID,
		Description: req.Description,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	resp.OK(c, grpcResp.Member)
}

// GetGrowthLogs 获取成长值变动日志
func GetGrowthLogs(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	// 分页参数，默认 page=1, size=10
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	grpcResp, err := global.MemberClient.GetGrowthLogs(c.Request.Context(), &pb.GetGrowthLogsRequest{
		UserId: int64(uid),
		Page:   int32(page),
		Size:   int32(size),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	// 返回日志列表 + 总数
	resp.OK(c, gin.H{
		"logs":  grpcResp.Data,
		"total": grpcResp.Total,
	})
}

// GetMemberBenefits 获取当前用户的会员权益
func GetMemberBenefits(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	grpcResp, err := global.MemberClient.GetMemberBenefits(c.Request.Context(), &pb.GetMemberBenefitsRequest{
		UserId: int64(uid),
	})
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

// ListMemberLevels 获取所有等级配置（含当前用户等级高亮）
func ListMemberLevels(c *gin.Context) {
	userId, exists := c.Get("userID")
	if !exists {
		resp.Error(c, 10001, "用户未登录")
		return
	}

	uid, ok := userId.(uint)
	if !ok {
		resp.Error(c, 10001, "用户ID类型错误")
		return
	}

	grpcResp, err := global.MemberClient.ListMemberLevels(c.Request.Context(), &pb.ListMemberLevelsRequest{
		UserId: int64(uid),
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
		"levels":        grpcResp.Data,
		"current_level": grpcResp.CurrentLevel,
	})
}
