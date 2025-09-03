package utils

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var (
	jwtSecret []byte
)

type Claims struct {
	UserId uint `json:"user_id"`
	jwt.StandardClaims
}

func GenerateJWT(userId uint) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserId: userId,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
			IssuedAt:  time.Now().Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	//privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	//if err != nil {
	//	return "", err
	//}

	// 生成随机密钥（长度至少等于哈希输出大小，HS384 需要 48 字节）
	privateKey := make([]byte, 48)
	if _, err := rand.Read(privateKey); err != nil {
		return "", err
	}

	jwtSecret = privateKey
	return token.SignedString(privateKey)
}

func ParseJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, err
}
