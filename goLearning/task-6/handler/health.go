package handler

import (
	"github/seeli/task-6/model"

	"github.com/gin-gonic/gin"
)

// HealthCheck 健康检查接口（K8s liveness/readiness probe 使用）
// GET /api/v1/health
func HealthCheck(c *gin.Context) {
	model.Success(c, gin.H{
		"status":  "ok",
		"service": "api-gateway",
		"version": "1.0.0",
	})
}
