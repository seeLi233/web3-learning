package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func LoggerMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()

		duration := time.Since(start)
		log.Printf("Request: %s %s - Status: %d - Duration: %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status(), duration)
	}
}
