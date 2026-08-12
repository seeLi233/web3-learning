package db

import (
	"context"
	"time"

	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"gorm.io/gorm"
)

type OAuthRepo struct {
	db *gorm.DB
}

func NewOAuthRepo(db *gorm.DB) *OAuthRepo {
	return &OAuthRepo{db: db}
}

// AutoMigrate 自动创建表
func (r *OAuthRepo) AutoMigrate() error {
	return r.db.AutoMigrate(
		&entity.OAuthClient{},
		&entity.AuthorizationCode{},
		&entity.OAuthAccessToken{},
		&entity.OAuthRefreshToken{},
		&entity.UserGitHubBinding{},
	)
}

// ========== OAuthClient 操作 ==========
// GetClientByClientID 根据 client_id 获取客户端
func (r *OAuthRepo) GetClientByClientID(ctx context.Context, clientID string) (*entity.OAuthClient, error) {
	var client entity.OAuthClient
	err := r.db.WithContext(ctx).Where("client_id = ?", clientID).First(&client).Error
	return &client, err
}

// ========== AuthorizationCode 操作 ==========

// SaveAuthorizationCode 保存授权码
func (r *OAuthRepo) SaveAuthorizationCode(ctx context.Context, code *entity.AuthorizationCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

// GetAuthorizationCode 获取未过期的授权码
func (r *OAuthRepo) GetAuthorizationCode(ctx context.Context, code string) (*entity.AuthorizationCode, error) {
	var authCode entity.AuthorizationCode
	err := r.db.WithContext(ctx).Where("code = ? AND expires_at > ?", code, time.Now()).First(&authCode).Error
	return &authCode, err
}

// DeleteAuthorizationCode 删除授权码（一次性使用）
func (r *OAuthRepo) DeleteAuthorizationCode(ctx context.Context, code string) error {
	return r.db.WithContext(ctx).Where("code = ?", code).Delete(&entity.AuthorizationCode{}).Error
}

// ========== OAuthAccessToken 操作 ==========

// SaveAccessToken 保存 Access Token
func (r *OAuthRepo) SaveAccessToken(ctx context.Context, token *entity.OAuthAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetAccessToken 获取未过期的 Access Token
func (r *OAuthRepo) GetAccessToken(ctx context.Context, token string) (*entity.OAuthAccessToken, error) {
	var accessToken entity.OAuthAccessToken
	err := r.db.WithContext(ctx).Where("token = ? AND expires_at > ?", token, time.Now()).First(&accessToken).Error
	return &accessToken, err
}

// DeleteAccessToken 删除 Access Token
func (r *OAuthRepo) DeleteAccessToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Where("token = ?", token).Delete(&entity.OAuthAccessToken{}).Error
}

// DeleteAccessTokenByUserID 删除用户的所有 Access Token
func (r *OAuthRepo) DeleteAccessTokenByUserID(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&entity.OAuthAccessToken{}).Error
}

// ========== OAuthRefreshToken 操作 ==========

// SaveRefreshToken 保存 Refresh Token
func (r *OAuthRepo) SaveRefreshToken(ctx context.Context, token *entity.OAuthRefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetRefreshToken 获取未过期的 Refresh Token
func (r *OAuthRepo) GetRefreshToken(ctx context.Context, token string) (*entity.OAuthRefreshToken, error) {
	var refreshToken entity.OAuthRefreshToken
	err := r.db.WithContext(ctx).Where("token = ? AND expires_at > ?", token, time.Now()).First(&refreshToken).Error
	return &refreshToken, err
}

// DeleteRefreshToken 删除 Refresh Token
func (r *OAuthRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Where("token = ?", token).Delete(&entity.OAuthRefreshToken{}).Error
}

// ========== UserGitHubBinding 操作 ==========

// GetBindingByGitHubID 根据 GitHub ID 获取绑定关系
func (r *OAuthRepo) GetBindingByGitHubID(ctx context.Context, githubID int64) (*entity.UserGitHubBinding, error) {
	var binding entity.UserGitHubBinding
	err := r.db.WithContext(ctx).Where("github_id = ?", githubID).First(&binding).Error
	return &binding, err
}

// SaveGitHubBinding 保存 GitHub 绑定关系
func (r *OAuthRepo) SaveGitHubBinding(ctx context.Context, binding *entity.UserGitHubBinding) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

// CleanupExpired 清理过期数据
func (r *OAuthRepo) CleanupExpired(ctx context.Context) error {
	now := time.Now()
	r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&entity.AuthorizationCode{})
	r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&entity.OAuthAccessToken{})
	r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&entity.OAuthRefreshToken{})
	return nil
}
