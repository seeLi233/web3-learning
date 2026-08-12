package jwt

import (
	"time"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义声明
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	jwt.RegisteredClaims
}

// GenerateToken 签发 access_token + refresh_token
// 返回值: access_token, refreshToken, expiresAt(unix 秒), error
func GenerateToken(userID uint, username, phone string) (string, string, int64, error) {
	cfg := config.Conf.JWTConfig
	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.AccessExpire) * time.Second)

	// Access Token
	accessClaims := Claims{
		UserID:   userID,
		Username: username,
		Phone:    phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    cfg.Issuer,
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", "", 0, err
	}

	// Refresh Token (更长过期时间)
	refreshExpiresAt := now.Add(time.Duration(cfg.RefreshExpire) * time.Second)
	refreshClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    cfg.Issuer,
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, _ := refreshToken.SignedString([]byte(cfg.Secret))

	return accessStr, refreshStr, expiresAt.Unix(), nil
}

// ParseToken 解析并验证 token
func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Conf.JWTConfig
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		logger.Error("jwt token 解析失败")
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// RefreshAccessToken 用 refresh_token 换新的 access_token
func RefreshAccessToken(refreshTokenStr string) (string, int64, error) {
	claims, err := ParseToken(refreshTokenStr)
	if err != nil {
		return "", 0, err
	}

	cfg := config.Conf.JWTConfig
	now := time.Now()
	expiresAt := now.Add(time.Duration(cfg.AccessExpire) * time.Second)

	newClaims := Claims{
		UserID:   claims.UserID,
		Username: claims.Username,
		Phone:    claims.Phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    cfg.Issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	str, err := token.SignedString([]byte(cfg.Secret))
	return str, expiresAt.Unix(), err

}
