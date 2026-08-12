package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/pkg/jwt"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// 优先从 Header 读取 (API 调用)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr = parts[1]
			}
		}

		// Header 没有则从 Cookie 读取 (浏览器跨域 SSO)
		if tokenStr == "" {
			tokenStr, _ = c.Cookie("access_token")
		}

		if tokenStr == "" {
			resp.Error(c, 10004, "缺少认证信息")
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(tokenStr)
		if err != nil {
			resp.Error(c, 10004, "token 无效: "+err.Error())
			c.Abort()
			return
		}

		// 将用户信息写入 Context，后续 handler 可通过 c.Get("userID") 获取
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("phone", claims.Phone)
		c.Next()
	}
}
