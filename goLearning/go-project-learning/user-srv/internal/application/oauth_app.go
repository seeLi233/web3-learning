package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/go-project-learning/project/user-srv/internal/repository/db"
)

type OAuthApp struct {
	oauthRepo *db.OAuthRepo
	userRepo  *db.UserRepo // 复用现有的用户仓库
}

func NewOAuthApp(oauthRepo *db.OAuthRepo, userRepo *db.UserRepo) *OAuthApp {
	return &OAuthApp{
		oauthRepo: oauthRepo,
		userRepo:  userRepo,
	}
}

// GitHubUser GitHub 用户信息
// 对应 GitHub API 返回的用户数据
type GitHubUser struct {
	ID        int64  `json:"id"`         // GitHub 用户 ID
	Login     string `json:"login"`      // GitHub 用户名
	Email     string `json:"email"`      // GitHub 邮箱
	Name      string `json:"name"`       // GitHub 显示名称
	AvatarURL string `json:"avatar_url"` // 头像 URL
}

// ========== 授权码相关 ==========

// CreateAuthorizationCode 创建授权码
// 业务逻辑：
// 1. 验证 client_id 是否存在
// 2. 验证 redirect_uri 是否在允许列表
// 3. 生成随机授权码
// 4. 保存到数据库（5分钟过期）
// 5. 返回授权码

func (app *OAuthApp) CreateAuthorizationCode(ctx context.Context, clientID, redirectURI, scope string, userID uint) (string, error) {
	// 1. 验证客户端
	client, err := app.oauthRepo.GetClientByClientID(ctx, clientID)
	if err != nil {
		return "", fmt.Errorf("invalid client_id")
	}

	// 2. 验证 redirect_uri
	if !validateRedirectURI(client.RedirectURIs, redirectURI) {
		return "", fmt.Errorf("invalid redirect_uri")
	}

	// 3. 生成授权码
	code, err := generateRandomToken()
	if err != nil {
		return "", err
	}

	// 4. 保存授权码
	authCode := &entity.AuthorizationCode{
		Code:        code,
		ClientID:    clientID,
		UserID:      userID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}

	if err := app.oauthRepo.SaveAuthorizationCode(ctx, authCode); err != nil {
		return "", err
	}

	return code, nil
}

// ValidateAndConsumeCode 验证并消费授权码
// 业务逻辑：
// 1. 查询授权码是否存在且未过期
// 2. 验证 client_id 和 redirect_uri 是否匹配
// 3. 删除授权码（一次性使用）
// 4. 返回用户信息
func (app *OAuthApp) ValidateAndConsumeCode(ctx context.Context, code, clientID, redirectURI string) (uint, string, error) {
	// 1. 查询授权码
	authCode, err := app.oauthRepo.GetAuthorizationCode(ctx, code)
	if err != nil {
		return 0, "", fmt.Errorf("authorization code invalid or expired")
	}

	// 2. 验证匹配
	if authCode.ClientID != clientID {
		return 0, "", fmt.Errorf("client_id mismatch")
	}
	if authCode.RedirectURI != redirectURI {
		return 0, "", fmt.Errorf("redirect_uri mismatch")
	}

	// 3. 删除授权码
	app.oauthRepo.DeleteAuthorizationCode(ctx, code)

	return authCode.UserID, authCode.Scope, nil
}

// ========== Token 相关 ==========

// CreateToken 创建 Access Token 和 Refresh Token
// 业务逻辑：
// 1. 生成随机 access_token
// 2. 生成随机 refresh_token
// 3. 保存到数据库
// 4. 返回 token
func (app *OAuthApp) CreateToken(ctx context.Context, clientID string, userID uint, scope string) (string, string, int64, error) {
	// 1. 生成 access_token
	accessTokenStr, err := generateRandomToken()
	if err != nil {
		return "", "", 0, err
	}

	// 2. 生成 refresh_token
	refreshTokenStr, err := generateRandomToken()
	if err != nil {
		return "", "", 0, err
	}

	// 3. 保存 access_token
	accessToken := &entity.OAuthAccessToken{
		Token:     accessTokenStr,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	if err := app.oauthRepo.SaveAccessToken(ctx, accessToken); err != nil {
		return "", "", 0, err
	}

	// 4. 保存 refresh_token
	refreshToken := &entity.OAuthRefreshToken{
		Token:         refreshTokenStr,
		AccessTokenID: accessToken.ID,
		ClientID:      clientID,
		UserID:        userID,
		ExpiresAt:     time.Now().Add(7 * 24 * time.Hour),
	}
	if err := app.oauthRepo.SaveRefreshToken(ctx, refreshToken); err != nil {
		return "", "", 0, err
	}

	return accessTokenStr, refreshTokenStr, 7200, nil
}

// ValidateAccessToken 验证 Access Token
func (app *OAuthApp) ValidateAccessToken(ctx context.Context, token string) (uint, string, string, error) {
	accessToken, err := app.oauthRepo.GetAccessToken(ctx, token)
	if err != nil {
		return 0, "", "", fmt.Errorf("access_token invalid or expired")
	}

	return accessToken.UserID, accessToken.ClientID, accessToken.Scope, nil
}

// RefreshToken 刷新 Token
func (app *OAuthApp) RefreshToken(ctx context.Context, refreshTokenStr, clientID string) (string, string, int64, error) {
	// 1. 验证 refresh_token
	refreshToken, err := app.oauthRepo.GetRefreshToken(ctx, refreshTokenStr)
	if err != nil {
		return "", "", 0, fmt.Errorf("refresh_token invalid or expired")
	}

	// 2. 验证 client_id
	if refreshToken.ClientID != clientID {
		return "", "", 0, fmt.Errorf("client_id mismatch")
	}

	// 3. 删除旧 token
	app.oauthRepo.DeleteRefreshToken(ctx, refreshTokenStr)
	app.oauthRepo.DeleteAccessTokenByUserID(ctx, refreshToken.UserID)

	// 4. 创建新 token
	return app.CreateToken(ctx, clientID, refreshToken.UserID, "")
}

// ========== GitHub 登录 ==========

// GitHubLogin 处理 GitHub 登录
func (app *OAuthApp) GitHubLogin(ctx context.Context, githubID int64, githubUsername, githubEmail string) (bool, uint, error) {
	// 1. 检查是否已绑定
	binding, err := app.oauthRepo.GetBindingByGitHubID(ctx, githubID)
	if err == nil {
		// 已绑定，返回用户 ID
		return false, binding.UserID, nil
	}

	// 2. 未绑定，返回需要绑定的标记
	return true, 0, nil
}

// BindGitHubAccount 绑定 GitHub 账号
func (app *OAuthApp) BindGitHubAccount(ctx context.Context, githubID int64, githubUsername, githubEmail string, userID uint) error {
	binding := &entity.UserGitHubBinding{
		UserID:         userID,
		GitHubID:       githubID,
		GitHubUsername: githubUsername,
		GitHubEmail:    githubEmail,
	}
	return app.oauthRepo.SaveGitHubBinding(ctx, binding)
}

func (app *OAuthApp) ValidateClient(ctx context.Context, clientID, clientSecret string) (*entity.OAuthClient, error) {
	client, err := app.oauthRepo.GetClientByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid client_id")
	}

	if client.ClientSecret != clientSecret {
		return nil, fmt.Errorf("invalid client_secret")
	}

	return client, nil
}

// ========== 辅助函数 ==========

// generateRandomToken 生成随机 token
func generateRandomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// validateRedirectURI 验证 redirect_uri 是否在允许列表
func validateRedirectURI(allowedURIs, redirectURI string) bool {
	// 解析 JSON 数组
	var uris []string
	if err := json.Unmarshal([]byte(allowedURIs), &uris); err != nil {
		return false
	}

	// 检查 redirectURI 是否在列表中
	for _, uri := range uris {
		if uri == redirectURI {
			return true
		}
	}
	return false
}
