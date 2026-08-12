package jwt

import (
	"time"

	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	jwt.RegisteredClaims
}

// ParseToken 解析 token（gateway 中间件用）
func ParseToken(tokenStr string) (*Claims, error) {
	cfg := config.Conf.JWT
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// RefreshAccessToken 刷新 access_token
func RefreshAccessToken(refreshTokenStr string) (string, int64, error) {
	claims, err := ParseToken(refreshTokenStr)
	if err != nil {
		return "", 0, err
	}

	cfg := config.Conf.JWT
	now := time.Now()
	expiresAt := now.Add(2 * time.Hour) // access_token 2小时

	newClaims := Claims{
		UserID:   claims.UserID,
		Username: claims.Username,
		Phone:    claims.Phone,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	str, err := token.SignedString([]byte(cfg.Secret))
	return str, expiresAt.Unix(), err
}
