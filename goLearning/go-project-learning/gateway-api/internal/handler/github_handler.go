package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	pb "github.com/go-project-learning/project/user-srv/api/pb"
)

// GitHubHandler GitHub 登录处理器
type GitHubHandler struct {
	oauthClient pb.OAuthServiceClient // ✅ 只调用 gRPC
}

// NewGitHubHandler 创建 GitHubHandler
func NewGitHubHandler() *GitHubHandler {
	return &GitHubHandler{
		oauthClient: global.OAuthClient, // ✅ 使用 gRPC 客户端
	}
}

// GitHubLogin 重定向到 GitHub 授权页
// GET /oauth/github/login
func (h *GitHubHandler) GitHubLogin(c *gin.Context) {
	cfg := config.Conf.GitHub

	// 构建 GitHub 授权 URL
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		cfg.ClientID,
		cfg.RedirectURI,
		"user:email",
		generateState(),
	)

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// GitHubCallback GitHub 回调处理
// GET /oauth/github/callback?code=xxx&state=xxx
func (h *GitHubHandler) GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		resp.Error(c, 10001, "缺少 code 参数")
		return
	}

	// 调用 gRPC 处理 GitHub 登录
	loginResp, err := h.oauthClient.GitHubLogin(c.Request.Context(), &pb.GitHubLoginRequest{
		GithubCode: code,
	})
	if err != nil {
		resp.Error(c, 10002, "GitHub 登录失败: "+err.Error())
		return
	}

	if loginResp.Code != 0 {
		resp.Error(c, loginResp.Code, loginResp.Msg)
		return
	}

	// 判断是否需要绑定
	if loginResp.NeedBind {
		// 需要绑定，返回 GitHub 用户信息
		resp.OK(c, gin.H{
			"need_bind":       true,
			"github_id":       loginResp.GithubId,
			"github_username": loginResp.GithubUsername,
			"github_email":    loginResp.GithubEmail,
			"message":         "该 GitHub 账号未绑定本地账号，请绑定或注册新账号",
		})
		return
	}

	// 已绑定，直接登录
	h.loginSuccess(c, loginResp.AccessToken, loginResp.UserId)
}

// BindGitHub 绑定 GitHub 账号到本地账号
// POST /oauth/github/bind
func (h *GitHubHandler) BindGitHub(c *gin.Context) {
	var req struct {
		GitHubID       int64  `json:"github_id" binding:"required"`
		GitHubUsername string `json:"github_username"`
		GitHubEmail    string `json:"github_email"`
		Account        string `json:"account"`
		Password       string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 1. 调用用户服务登录
	loginResp, err := global.UserClient.PwdLogin(c.Request.Context(), &pb.PwdLoginRequest{
		Account:  req.Account,
		Password: req.Password,
	})
	if err != nil || loginResp.Code != 0 {
		resp.Error(c, 10002, "登录失败，请检查账号密码")
		return
	}

	// 2. 调用 gRPC 绑定 GitHub
	bindResp, err := h.oauthClient.BindGitHubAccount(c.Request.Context(), &pb.BindGitHubRequest{
		GithubId:       req.GitHubID,
		GithubUsername: req.GitHubUsername,
		GithubEmail:    req.GitHubEmail,
		UserId:         uint64(loginResp.User.Id),
	})
	if err != nil || bindResp.Code != 0 {
		resp.Error(c, 10003, "绑定失败")
		return
	}

	// 3. 登录成功
	h.loginSuccess(c, "", uint64(loginResp.User.Id))
}

// RegisterAndBind 注册新账号并绑定 GitHub
// POST /oauth/github/register
func (h *GitHubHandler) RegisterAndBind(c *gin.Context) {
	var req struct {
		GitHubID       int64  `json:"github_id" binding:"required"`
		GitHubUsername string `json:"github_username"`
		GitHubEmail    string `json:"github_email"`
		Username       string `json:"username" binding:"required"`
		Password       string `json:"password" binding:"required"`
		Phone          string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 1. 调用用户服务注册
	registerResp, err := global.UserClient.CreateUser(c.Request.Context(), &pb.CreateUserRequest{
		Name:     req.Username,
		Password: req.Password,
		Phone:    req.Phone,
		Email:    req.GitHubEmail,
	})
	if err != nil || registerResp.Code != 0 {
		resp.Error(c, 10002, "注册失败")
		return
	}

	// 2. 调用 gRPC 绑定 GitHub
	bindResp, err := h.oauthClient.BindGitHubAccount(c.Request.Context(), &pb.BindGitHubRequest{
		GithubId:       req.GitHubID,
		GithubUsername: req.GitHubUsername,
		GithubEmail:    req.GitHubEmail,
		UserId:         uint64(registerResp.Data.Id),
	})
	if err != nil || bindResp.Code != 0 {
		resp.Error(c, 10003, "绑定失败")
		return
	}

	// 3. 登录成功
	h.loginSuccess(c, "", uint64(registerResp.Data.Id))
}

// ============ 辅助函数 ============

// loginSuccess 登录成功处理
func (h *GitHubHandler) loginSuccess(c *gin.Context, accessToken string, userID uint64) {
	// 如果没有 access_token，需要生成一个
	if accessToken == "" {
		// 这里应该调用用户服务生成 JWT token
		// 简化处理，直接返回用户 ID
		resp.OK(c, gin.H{
			"user_id": userID,
			"message": "登录成功",
		})
		return
	}

	// 设置 Cookie
	cfg := config.Conf.JWT
	maxAge := 7200
	c.SetCookie("access_token", accessToken, maxAge, cfg.CookiePath, cfg.CookieDomain, cfg.CookieSecure, true)

	resp.OK(c, gin.H{
		"access_token": accessToken,
		"user_id":      userID,
		"message":      "登录成功",
	})
}

// generateState 生成随机 state
func generateState() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}
