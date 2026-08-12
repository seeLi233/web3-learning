package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	pb "github.com/go-project-learning/project/user-srv/api/pb"
)

func Register(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Password string `json:"password" binding:"required"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}
	// 密码加密
	// hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	// 限流检查
	ip := c.ClientIP()
	key := "rate_limit:" + ip + ":register"
	allowed, err := redis.SlidingWindowLimit(c.Request.Context(), key, time.Minute, 3)
	if err != nil {
		resp.Error(c, 50001, "限流检查失败:"+err.Error())
		return
	}
	if !allowed {
		resp.Error(c, 429, "请求过于频繁，请稍后再试")
		return
	}

	grpcResp, err := global.UserClient.CreateUser(context.Background(), &pb.CreateUserRequest{
		Name:     req.Name,
		Password: req.Password,
		Phone:    req.Phone,
		Email:    req.Email,
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

func GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, 10001, "id 参数错误")
		return
	}

	grpcResp, err := global.UserClient.GetUser(context.Background(), &pb.GetUserRequest{Id: id})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, grpcResp.Data)
}

func UpdateProfile(c *gin.Context) {
	// 从 JWT 中获取用户 ID （中间件已经解析好了）
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

	var req struct {
		Username string `json:"username"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误")
		return
	}

	grpcResp, err := global.UserClient.UpdateUser(c.Request.Context(), &pb.UpdateUserRequest{
		Id:    int64(uid),
		Name:  req.Username,
		Phone: req.Phone,
		Email: req.Email,
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

func ListUser(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	size, _ := strconv.ParseInt(c.DefaultQuery("size", "10"), 10, 32)

	grpcResp, err := global.UserClient.ListUser(context.Background(), &pb.ListUserRequest{
		Page: int32(page),
		Size: int32(size),
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}

	resp.OK(c, grpcResp.Data)
}
