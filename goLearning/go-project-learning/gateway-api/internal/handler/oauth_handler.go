package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	pb "github.com/go-project-learning/project/user-srv/api/pb"
)

// OAuthHandler OAuth 处理器
type OAuthHandler struct {
	oauthClient pb.OAuthServiceClient
}

func NewOAuthHandler() *OAuthHandler {
	return &OAuthHandler{
		oauthClient: global.OAuthClient, // ✅ 使用 gRPC 客户端
	}
}

// ============ 授权码模式 ============

// AuthorizePage 显示授权页面
// GET /oauth/authorize?response_type=code&client_id=xxx&redirect_uri=xxx&scope=xxx&state=xxx
func (h *OAuthHandler) AuthorizePage(c *gin.Context) {
	// 1. 获取请求参数
	responseType := c.Query("response_type")
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")

	// 2. 参数校验
	if responseType == "" || clientID == "" || redirectURI == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "缺少必要参数",
		})
		return
	}

	// 3. 调用 gRPC 验证客户端
	clientResp, err := h.oauthClient.ValidateClient(context.Background(), &pb.ValidateClientRequest{
		ClientId: clientID,
	})
	if err != nil || clientResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_client",
			"error_description": "无效的 client_id",
		})
		return
	}

	// 4. 显示授权页面（返回 HTML 或 JSON）
	c.JSON(http.StatusOK, gin.H{
		"response_type": responseType,
		"client_id":     clientID,
		"client_name":   clientResp.Name,
		"redirect_uri":  redirectURI,
		"scope":         scope,
		"state":         state,
		"message":       "请确认授权",
	})
}

// Authorize 处理授权请求
// POST /oauth/authorize
func (h *OAuthHandler) Authorize(c *gin.Context) {
	// 1. 获取请求参数
	var req struct {
		ResponseType string `form:"response_type" binding:"required"`
		ClientID     string `form:"client_id" binding:"required"`
		RedirectURI  string `form:"redirect_uri" binding:"required"`
		Scope        string `form:"scope"`
		State        string `form:"state" binding:"required"`
		Account      string `form:"account"`
		Password     string `form:"password"`
	}

	if err := c.ShouldBind(&req); err != nil {
		resp.Error(c, 10001, "参数错误: "+err.Error())
		return
	}

	// 2. 获取用户 ID（从 Cookie 或登录）
	var userID uint64
	if uid, exists := c.Get("userID"); exists {
		userID = uint64(uid.(uint))
	} else if req.Account != "" && req.Password != "" {
		// 调用用户服务登录
		loginResp, err := global.UserClient.PwdLogin(context.Background(), &pb.PwdLoginRequest{
			Account:  req.Account,
			Password: req.Password,
		})
		if err != nil {
			resp.Error(c, 10004, "登录失败: "+err.Error())
			return
		}
		if loginResp.Code != 0 {
			resp.Error(c, 10004, "登录失败: "+loginResp.Msg)
			return
		}
		if loginResp.User == nil {
			resp.Error(c, 10004, "登录失败: 返回的用户信息为空")
			return
		}
		userID = uint64(loginResp.User.Id)
	} else {
		resp.Error(c, 10004, "未登录")
		return
	}

	// 3. 根据 response_type 处理
	switch req.ResponseType {
	case "code":
		// 授权码模式
		h.handleCodeResponse(c, req.ClientID, req.RedirectURI, req.Scope, req.State, userID)
	case "token":
		// 简化模式
		h.handleTokenResponse(c, req.ClientID, req.RedirectURI, req.Scope, req.State, userID)
	default:
		resp.Error(c, 10005, "不支持的 response_type")
	}
}

// handleCodeResponse 处理授权码模式
func (h *OAuthHandler) handleCodeResponse(c *gin.Context, clientID, redirectURI, scope, state string, userID uint64) {
	// 调用 gRPC 创建授权码
	codeResp, err := h.oauthClient.CreateAuthorizationCode(c.Request.Context(), &pb.CreateCodeRequest{
		ClientId:    clientID,
		RedirectUri: redirectURI,
		Scope:       scope,
		UserId:      userID,
	})
	if err != nil {
		resp.Error(c, 50001, "创建授权码失败: "+err.Error())
		return
	}
	if codeResp.Code != 0 {
		resp.Error(c, 50001, "创建授权码失败: "+codeResp.Msg)
		return
	}

	// 返回授权码
	resp.OK(c, gin.H{
		"code":  codeResp.AuthorizationCode,
		"state": state,
	})
}

// handleTokenResponse 处理简化模式
func (h *OAuthHandler) handleTokenResponse(c *gin.Context, clientID, redirectURI, scope, state string, userID uint64) {
	// 调用 gRPC 创建 token
	tokenResp, err := h.oauthClient.CreateAccessToken(c.Request.Context(), &pb.CreateTokenRequest{
		ClientId: clientID,
		UserId:   userID,
		Scope:    scope,
	})
	if err != nil {
		resp.Error(c, 50001, "创建 token 失败: "+err.Error())
		return
	}
	if tokenResp.Code != 0 {
		resp.Error(c, 50001, "创建 token 失败: "+tokenResp.Msg)
		return
	}

	// 返回 token
	resp.OK(c, gin.H{
		"access_token":  tokenResp.AccessToken,
		"token_type":    "bearer",
		"expires_in":    tokenResp.ExpiresIn,
		"refresh_token": tokenResp.RefreshToken,
		"state":         state,
	})
}

// ============ Token 接口 ============

// Token 用授权码换取 access_token
// POST /oauth/token
func (h *OAuthHandler) Token(c *gin.Context) {
	// 1. 获取请求参数
	var req struct {
		GrantType    string `form:"grant_type" binding:"required"`
		Code         string `form:"code"`
		RedirectURI  string `form:"redirect_uri"`
		ClientID     string `form:"client_id" binding:"required"`
		ClientSecret string `form:"client_secret" binding:"required"`
		RefreshToken string `form:"refresh_token"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_request",
			"error_description": "参数错误: " + err.Error(),
		})
		return
	}

	// 2. 验证客户端
	clientResp, err := h.oauthClient.ValidateClient(c.Request.Context(), &pb.ValidateClientRequest{
		ClientId:     req.ClientID,
		ClientSecret: req.ClientSecret,
	})
	if err != nil || clientResp.Code != 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_client",
			"error_description": "无效的 client_id 或 client_secret",
		})
		return
	}

	// 3. 根据 grant_type 处理
	switch req.GrantType {
	case "authorization_code":
		h.handleAuthorizationCodeGrant(c, req.Code, req.RedirectURI, req.ClientID)
	case "refresh_token":
		h.handleRefreshTokenGrant(c, req.RefreshToken, req.ClientID)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_grant_type",
			"error_description": "不支持的 grant_type",
		})
	}
}

// handleAuthorizationCodeGrant 处理授权码模式
func (h *OAuthHandler) handleAuthorizationCodeGrant(c *gin.Context, code, redirectURI, clientID string) {
	// 1. 调用 gRPC 验证授权码
	validateResp, err := h.oauthClient.ValidateAuthorizationCode(c.Request.Context(), &pb.ValidateCodeRequest{
		Code:        code,
		ClientId:    clientID,
		RedirectUri: redirectURI,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_grant",
			"error_description": "授权码无效或已过期: " + err.Error(),
		})
		return
	}
	if validateResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_grant",
			"error_description": "授权码无效或已过期: " + validateResp.Msg,
		})
		return
	}

	// 2. 调用 gRPC 创建 token
	tokenResp, err := h.oauthClient.CreateAccessToken(c.Request.Context(), &pb.CreateTokenRequest{
		ClientId: clientID,
		UserId:   validateResp.UserId,
		Scope:    validateResp.Scope,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": "创建 token 失败: " + err.Error(),
		})
		return
	}
	if tokenResp.Code != 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":             "server_error",
			"error_description": "创建 token 失败: " + tokenResp.Msg,
		})
		return
	}

	// 3. 返回 token
	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenResp.AccessToken,
		"token_type":    "bearer",
		"expires_in":    tokenResp.ExpiresIn,
		"refresh_token": tokenResp.RefreshToken,
	})
}

// handleRefreshTokenGrant 处理刷新 token
func (h *OAuthHandler) handleRefreshTokenGrant(c *gin.Context, refreshToken, clientID string) {
	// 调用 gRPC 刷新 token
	tokenResp, err := h.oauthClient.RefreshAccessToken(c.Request.Context(), &pb.RefreshTokenRequest{
		RefreshToken: refreshToken,
		ClientId:     clientID,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_grant",
			"error_description": "refresh_token 无效或已过期: " + err.Error(),
		})
		return
	}
	if tokenResp.Code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             "invalid_grant",
			"error_description": "refresh_token 无效或已过期: " + tokenResp.Msg,
		})
		return
	}

	// 返回新 token
	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenResp.AccessToken,
		"token_type":    "bearer",
		"expires_in":    tokenResp.ExpiresIn,
		"refresh_token": tokenResp.RefreshToken,
	})
}

// ============ UserInfo 接口 ============

// UserInfo 获取用户信息
// GET /oauth/userinfo
func (h *OAuthHandler) UserInfo(c *gin.Context) {
	// 1. 从 Header 获取 access_token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "缺少 Authorization 头",
		})
		return
	}

	// 2. 提取 token
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// 3. 调用 gRPC 获取用户信息
	userInfoResp, err := h.oauthClient.GetUserInfoByToken(c.Request.Context(), &pb.GetUserInfoRequest{
		AccessToken: token,
	})
	if err != nil || userInfoResp.Code != 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":             "invalid_token",
			"error_description": "access_token 无效或已过期",
		})
		return
	}

	// 4. 返回用户信息
	c.JSON(http.StatusOK, gin.H{
		"user_id":  userInfoResp.UserId,
		"username": userInfoResp.Username,
		"phone":    userInfoResp.Phone,
		"email":    userInfoResp.Email,
		"scope":    userInfoResp.Scope,
	})
}
