package handler

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/redis"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/cookie"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	pb "github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-project-learning/project/user-srv/pkg/jwt"
)

// SendCode 发送验证码
func SendCode(c *gin.Context) {
	var req struct {
		Phone string `json:"phone" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误："+err.Error())
		return
	}

	grpcResp, err := global.UserClient.SendCode(context.Background(), &pb.SendCodeRequest{
		Phone: req.Phone,
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

// PhoneLogin 手机验证码登录
func PhoneLogin(c *gin.Context) {
	var req struct {
		Phone      string `json:"phone" binding:"required"`
		VerifyCode string `json:"verify_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 限流检查
	ip := c.ClientIP()
	key := "rate_limit:" + ip + ":login"
	// 从 Redis 获取限流参数
	limitStr, err := redis.Get(c.Request.Context(), "risk_config:login_max_attempts")
	limit := 10 // 默认值
	if err == nil {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	allowed, err := redis.SlidingWindowLimit(c.Request.Context(), key, time.Minute, limit)
	if err != nil {
		resp.Error(c, 50001, "限流检查失败:"+err.Error())
		return
	}
	if !allowed {
		resp.Error(c, 429, "请求过于频繁，请稍后再试")
		return
	}

	grpcResp, err := global.UserClient.PhoneLogin(context.Background(), &pb.PhoneLoginRequest{
		Phone: req.Phone,
		Code:  req.VerifyCode,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	// 计算 Cookie 过期时间 (秒)
	maxAge := int(time.Until(time.Unix(grpcResp.ExpiresAt, 0)).Seconds())
	if maxAge < 0 {
		maxAge = 7200 // 默认 2 小时
	}

	// 写入 Cookie，实现跨子域名共享
	cookie.SetCookie(c, "access_token", grpcResp.AccessToken, maxAge)

	resp.OK(c, gin.H{
		"access_token":  grpcResp.AccessToken,
		"refresh_token": grpcResp.RefreshToken,
		"expires_at":    grpcResp.ExpiresAt,
		"user":          grpcResp.User,
	})
}

// EmailLogin 邮箱密码登录
func EmailLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 限流检查
	ip := c.ClientIP()
	key := "rate_limit:" + ip + ":login"
	// 从 Redis 获取限流参数
	limitStr, err := redis.Get(c.Request.Context(), "risk_config:login_max_attempts")
	limit := 10 // 默认值
	if err == nil {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	allowed, err := redis.SlidingWindowLimit(c.Request.Context(), key, time.Minute, limit)
	if err != nil {
		resp.Error(c, 50001, "限流检查失败:"+err.Error())
		return
	}
	if !allowed {
		resp.Error(c, 429, "请求过于频繁，请稍后再试")
		return
	}

	grpcResp, err := global.UserClient.EmailLogin(context.Background(), &pb.EmailLoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	// 计算 Cookie 过期时间
	maxAge := int(time.Until(time.Unix(grpcResp.ExpiresAt, 0)).Seconds())
	if maxAge < 0 {
		maxAge = 7200
	}
	cookie.SetCookie(c, "access_token", grpcResp.AccessToken, maxAge)

	resp.OK(c, gin.H{
		"access_token":  grpcResp.AccessToken,
		"refresh_token": grpcResp.RefreshToken,
		"expires_at":    grpcResp.ExpiresAt,
		"user":          grpcResp.User,
	})
}

// PwdLogin 通用账号密码登录
func PwdLogin(c *gin.Context) {
	var req struct {
		Account  string `json:"account" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 限流检查
	ip := c.ClientIP()
	key := "rate_limit:" + ip + ":login"
	// 从 Redis 获取限流参数
	limitStr, err := redis.Get(c.Request.Context(), "risk_config:login_max_attempts")
	limit := 10 // 默认值
	if err == nil {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}

	allowed, err := redis.SlidingWindowLimit(c.Request.Context(), key, time.Minute, limit)
	if err != nil {
		resp.Error(c, 50001, "限流检查失败:"+err.Error())
		return
	}
	if !allowed {
		resp.Error(c, 429, "请求过于频繁，请稍后再试")
		return
	}

	grpcResp, err := global.UserClient.PwdLogin(context.Background(), &pb.PwdLoginRequest{
		Account:  req.Account,
		Password: req.Password,
	})

	if err != nil {
		resp.Error(c, 50001, err.Error())
		return
	}
	if grpcResp.Code != 0 {
		// 记录登录失败
		failKey := "login_fail:" + req.Account
		count, _ := redis.Client.Incr(c.Request.Context(), failKey).Result()
		if count == 1 {
			// 第一次失败，设置过期时间
			lockTimeStr, _ := redis.Get(c.Request.Context(), "risk_config:lock_duration")
			lockTime := 900 // 默认 15 分钟
			if v, err := strconv.Atoi(lockTimeStr); err == nil {
				lockTime = v
			}
			redis.Expire(c.Request.Context(), failKey, time.Duration(lockTime)*time.Second)
		}

		// 检查是否超过最大失败次数
		maxFailStr, _ := redis.Get(c.Request.Context(), "risk_config:login_max_attempts")
		maxFail := 5 // 默认 5 次
		if v, err := strconv.Atoi(maxFailStr); err == nil {
			maxFail = v
		}

		if count >= int64(maxFail) {
			resp.Error(c, 423, "登录失败次数过多，账号已锁定")
			return
		}

		resp.Error(c, grpcResp.Code, grpcResp.Msg)
		return
	}

	// 计算 Cookie 过期时间
	maxAge := int(time.Until(time.Unix(grpcResp.ExpiresAt, 0)).Seconds())
	if maxAge < 0 {
		maxAge = 7200
	}
	cookie.SetCookie(c, "access_token", grpcResp.AccessToken, maxAge)

	resp.OK(c, gin.H{
		"access_token":  grpcResp.AccessToken,
		"refresh_token": grpcResp.RefreshToken,
		"expires_at":    grpcResp.ExpiresAt,
		"user":          grpcResp.User,
	})
}

// RefreshToken 刷新 token
func RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 直接在 gateway 本地解析 refresh_token，不走 gRPC
	newToken, expiresAt, err := jwt.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		resp.Error(c, 10004, "refresh_token 无效或已过期")
		return
	}

	resp.OK(c, gin.H{
		"access_token": newToken,
		"expires_at":   expiresAt,
	})
}

// Logout 登出 - 清除 Cookie
func Logout(c *gin.Context) {
	cookie.ClearCookie(c, "access_token")
	resp.OK(c, gin.H{"message": "已登出"})
}
