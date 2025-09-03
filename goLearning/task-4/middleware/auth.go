package middleware

import (
	"net/http"
	"strings"
	"web3/task4/utils"

	"github.com/gin-gonic/gin"
)

// 校验用户是否为当前用户
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer")
		claims, err := utils.ParseJWT(tokenString)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invaild token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserId)
		c.Next()
	}
}
