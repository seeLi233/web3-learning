package entity

import (
	"time"

	"gorm.io/gorm"
)

// OAuthClient OAuth 客户端应用
// 例如：商城前端、管理后台、移动端 APP
type OAuthClient struct {
	gorm.Model
	ClientID     string `gorm:"type:varchar(100);uniqueIndex;not null"` // 客户端标识
	ClientSecret string `gorm:"type:varchar(200);not null"`             // 客户端密钥
	Name         string `gorm:"type:varchar(100);not null"`             // 应用名称
	RedirectURIs string `gorm:"type:text;not null"`                     // 允许的回调地址（JSON 数组）
	Scope        string `gorm:"type:varchar(500);default:'read write'"` // 允许的权限范围
}

// TableName 指定表名
func (OAuthClient) TableName() string {
	return "oauth_clients"
}

// AuthorizationCode 授权码
// 一次性使用，5 分钟过期
type AuthorizationCode struct {
	gorm.Model
	Code        string    `gorm:"type:varchar(100);uniqueIndex;not null"` // 授权码
	ClientID    string    `gorm:"type:varchar(100);not null"`             // 客户端标识
	UserID      uint      `gorm:"not null"`                               // 用户 ID
	RedirectURI string    `gorm:"type:varchar(500);not null"`             // 回调地址
	Scope       string    `gorm:"type:varchar(500)"`                      // 授权范围
	ExpiresAt   time.Time `gorm:"not null"`                               // 过期时间
}

// TableName 指定表名
func (AuthorizationCode) TableName() string {
	return "authorization_codes"
}

// OAuthAccessToken Access Token
// 用于访问受保护资源
type OAuthAccessToken struct {
	gorm.Model
	Token     string    `gorm:"type:varchar(200);uniqueIndex;not null"` // Token 值
	ClientID  string    `gorm:"type:varchar(100);not null"`             // 客户端标识
	UserID    uint      `gorm:"not null"`                               // 用户 ID
	Scope     string    `gorm:"type:varchar(500)"`                      // 权限范围
	ExpiresAt time.Time `gorm:"not null"`                               // 过期时间
}

// TableName 指定表名
func (OAuthAccessToken) TableName() string {
	return "oauth_access_tokens"
}

// OAuthRefreshToken Refresh Token
// 用于刷新 Access Token
type OAuthRefreshToken struct {
	gorm.Model
	Token         string    `gorm:"type:varchar(200);uniqueIndex;not null"` // Token 值
	AccessTokenID uint      `gorm:"not null"`                               // 关联的 Access Token ID
	ClientID      string    `gorm:"type:varchar(100);not null"`             // 客户端标识
	UserID        uint      `gorm:"not null"`                               // 用户 ID
	ExpiresAt     time.Time `gorm:"not null"`                               // 过期时间
}

// TableName 指定表名
func (OAuthRefreshToken) TableName() string {
	return "oauth_refresh_tokens"
}

// UserGitHubBinding GitHub 绑定关系
// 记录本地用户和 GitHub 账号的关联
type UserGitHubBinding struct {
	gorm.Model
	UserID         uint   `gorm:"not null;index"`       // 本地用户 ID
	GitHubID       int64  `gorm:"uniqueIndex;not null"` // GitHub 用户 ID
	GitHubUsername string `gorm:"type:varchar(100)"`    // GitHub 用户名
	GitHubEmail    string `gorm:"type:varchar(100)"`    // GitHub 邮箱
}

// TableName 指定表名
func (UserGitHubBinding) TableName() string {
	return "user_github_bindings"
}
