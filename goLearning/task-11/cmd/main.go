package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task11/internal/auth"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ==================== 全局配置 ====================

// 为什么配置用环境变量而不是硬编码？
// → 12-Factor App 原则：配置和代码分离。不同环境（开发/测试/生产）
//
//	用不同 secret，但不能改代码。生产环境的 secret 永远不会出现在 git 历史中。
var (
	// JWT 签名密钥 —— 生产环境必须从环境变量读取！
	// 如果没有设置环境变量，用默认值（仅用于本地开发）
	jwtSecret = []byte(getEnv("JWT_SECRET", "dev-secret-change-in-production"))

	// Redis 连接地址
	redisAddr = getEnv("REDIS_ADDR", "localhost:6380")

	// 服务监听端口
	serverPort = getEnv("SERVER_PORT", "8080")
)

// getEnv 读环境变量，不存在时返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// ==================== 模拟用户数据 ====================

// 为了聚焦 JWT + RBAC 核心逻辑，用内存 map 模拟用户数据库
// 生产环境应该用 GORM + PostgreSQL（Day 37 学过）
var users = map[string]struct {
	Password string
	Role     string
}{
	"admin":     {Password: "admin123", Role: auth.RoleAdmin},
	"alice":     {Password: "alice123", Role: auth.RoleUser},
	"bob":       {Password: "bob123", Role: auth.RoleUser},
	"moderator": {Password: "mod123", Role: auth.RoleModerator},
}

// 模拟 Refresh Token 存储（生产环境存数据库）
var refreshTokens = map[string]string{} // userID -> refreshToken

func main() {
	// ==================== 初始化 Redis ====================
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	// 启动时 ping Redis 确认连接——失败就 fatal，不让服务无 Redis 运行
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️  Redis 连接失败 (%v)，黑名单功能将不可用", err)
		// 不 fatal——本地开发时可能没装 Redis，允许降级运行
		rdb = nil
	}

	// 创建黑名单实例（Redis 不可用时置 nil，中间件会跳过黑名单检查）
	var blacklist auth.TokenBlacklist
	if rdb != nil {
		blacklist = auth.NewRedisBlacklist(rdb, "blacklist")
	}

	// ==================== 初始化 Gin ====================
	// 为什么用 Default() 而不是 New()？
	// → Default() 自动附带了 Logger（请求日志）和 Recovery（panic 恢复）中间件
	//    这是生产环境的标准配置。New() 是裸引擎，需要手动加。
	r := gin.Default()

	// ==================== 公开路由 ====================
	// 不需要认证的路由：登录、健康检查
	r.POST("/login", loginHandler)
	r.GET("/health", healthHandler)

	// ==================== 认证路由 ====================
	// 需要有效 JWT 才能访问的路由
	authGroup := r.Group("/api")
	authGroup.Use(auth.AuthMiddleware(jwtSecret, blacklist))
	{
		// POST /api/refresh —— 用 Refresh Token 换新 Access Token
		// 注意：refresh 接口本身需要认证吗？
		// → 需要！但验证的是 Refresh Token 而非 Access Token。
		//    refresh 接口单独处理，不走 Auth 中间件（因为传的是 RT 不是 AT）
		//    放在 authGroup 外面单独注册：
		r.POST("/refresh", refreshHandler)

		// POST /api/logout —— 登出（把 token jti 加入黑名单）
		authGroup.POST("/logout", logoutHandler(blacklist))

		// GET /api/profile —— 查看自己的资料（所有角色可用）
		authGroup.GET("/profile", profileHandler)
	}

	// ==================== 管理路由 ====================
	// 需要 admin 角色才能访问的路由
	adminGroup := r.Group("/admin")
	adminGroup.Use(auth.AuthMiddleware(jwtSecret, blacklist))
	adminGroup.Use(auth.RBACMiddleware(auth.RoleAdmin))
	{
		adminGroup.GET("/users", listUsersHandler)
		adminGroup.DELETE("/user/:id", deleteUserHandler)
	}

	// ==================== 版主路由 ====================
	// 需要 moderator 或 admin 角色
	modGroup := r.Group("/mod")
	modGroup.Use(auth.AuthMiddleware(jwtSecret, blacklist))
	modGroup.Use(auth.RBACMiddleware(auth.RoleModerator, auth.RoleAdmin))
	{
		modGroup.PUT("/users/:id", updateUserHandler)
	}

	// ==================== 优雅启动 ====================
	// Day 35 学过的 graceful shutdown 模式
	srv := &http.Server{
		Addr:    ":" + serverPort,
		Handler: r,
	}

	// 在 goroutine 中启动服务，主线程监听退出信号
	go func() {
		log.Printf("🚀 API Gateway 启动在 :%s", serverPort)
		log.Printf("   GET  /health          — 健康检查（公开）")
		log.Printf("   POST /login           — 登录获取 token（公开）")
		log.Printf("   POST /refresh         — 刷新 Access Token")
		log.Printf("   POST /api/logout      — 登出（需要 JWT）")
		log.Printf("   GET  /api/profile     — 个人资料（需要 JWT）")
		log.Printf("   GET  /admin/users     — 用户列表（需要 admin）")
		log.Printf("   DELETE /admin/users/:id — 删除用户（需要 admin）")
		log.Printf("   PUT  /mod/users/:id   — 修改用户（需要 moderator/admin）")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// ==================== 等待退出信号 ====================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 收到退出信号...")

	// 给正在处理的请求 10 秒缓冲时间
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("服务关闭失败: %v", err)
	}
	log.Println("✅ 服务已安全关闭")
}

// ==================== Handler 实现 ====================

// ==================== 公开接口 ====================

// healthHandler 健康检查 —— 负载均衡器/监控系统用
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// loginHandler 用户登录 —— 验证用户名密码，返回 token pair
func loginHandler(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	// 查用户表（模拟）
	user, exists := users[req.Username]
	if !exists || user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid username or password",
			"code":  "LOGIN_INVALID_CREDENTIALS",
		})
		return
	}

	// 生成 token pair
	pair, err := auth.GenerateTokenPair(jwtSecret, req.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	// 存 Refresh Token（生产环境存数据库，这里简化）
	refreshTokens[req.Username] = pair.RefreshToken

	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(auth.AccessTokenTTL.Seconds()), // 秒数，方便前端设置定时刷新
	})
}

// refreshHandler 用 Refresh Token 换取新 Access Token
//
// 请求格式：Authorization: Bearer <refresh_token>
// 为什么放在 authGroup 外面？
// → Auth 中间件只接受 Access Token（tokenType="access" 检查），
//
//	Refresh Token 的 tokenType 是 "refresh"，会被 Auth 中间件拒绝。
//	所以 refresh 接口需要单独处理——直接解析 RT，不走通用 Auth 中间件。
func refreshHandler(c *gin.Context) {
	authHandler := c.GetHeader("Authorization")
	tokenString, ok := afterBearer(authHandler)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid authorization header",
			"code":  "REFRESH_HEADER_INVALID",
		})
		return
	}

	// 调用 RefreshAccessToken —— 内部会验证签名 + 检查 tokenType == "refresh"
	newAT, err := auth.RefreshAccessToken(jwtSecret, tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid refresh token",
			"code":  "REFRESH_TOKEN_INVALID",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": newAT,
		"token_type":   "Bearer",
		"expires_in":   int(auth.AccessTokenTTL.Seconds()),
	})
}

// afterBearer 从 Authorization header 提取 "Bearer " 后的 token
func afterBearer(header string) (string, bool) {
	if len(header) < 7 || header[:7] != "Bearer " {
		return "", false
	}
	token := header[7:]
	return token, token != ""
}

// ==================== 认证接口（需要 JWT） ====================

// logoutHandler 返回登出 handler
// 为什么是返回 handler 的函数而不是直接一个 handler？
// → logout 需要 blacklist 实例，通过闭包捕获。这是"依赖注入"的模式
func logoutHandler(blacklist auth.TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 context 获取 Auth 中间件注入的数据
		tokenID, _ := c.Get("tokenID")
		userID, _ := c.Get("userID")

		// 将 token jti 加入 Redis 黑名单
		if blacklist != nil {
			// 黑名单 TTL = token 剩余有效期
			// exp 时间从 JWT claims 获取，这里简化：用默认 AT TTL
			if err := blacklist.Add(c.Request.Context(), tokenID.(string), auth.AccessTokenTTL); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to logout",
					"code":  "LOGOUT_FAILED",
				})
				return
			}
		}

		// 同时删除 Refresh Token（防止用 RT 换新 AT 绕过黑名单）
		delete(refreshTokens, userID.(string))

		c.JSON(http.StatusOK, gin.H{
			"message": "logged out successfully",
		})
	}
}

// profileHandler 查看当前用户的个人资料
func profileHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	role, _ := c.Get("role")

	// 从模拟数据库取用户信息
	_, exists := users[userID.(string)]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"username": userID,
		"role":     role,
		"password": "***", // 敏感信息脱敏——不返回密码！
	})
}

// ==================== 管理接口（需要 admin 角色） ====================

// listUsersHandler 列出所有用户（仅 admin）
func listUsersHandler(c *gin.Context) {
	userList := make([]gin.H, 0, len(users))
	for name, u := range users {
		userList = append(userList, gin.H{
			"username": name,
			"role":     u.Role,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"users": userList,
		"total": len(userList),
	})
}

// deleteUserHandler 删除用户（仅 admin）
func deleteUserHandler(c *gin.Context) {
	userID := c.Param("id")

	if _, exists := users[userID]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 不能删除自己
	currentUser, _ := c.Get("userID")
	if userID == currentUser.(string) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}

	delete(users, userID)
	delete(refreshTokens, userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "user deleted: " + userID,
	})
}

// ==================== 版主接口（需要 moderator/admin） ====================

// updateUserHandler 修改用户信息（仅 moderator 或 admin）
func updateUserHandler(c *gin.Context) {
	userID := c.Param("id")

	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, exists := users[userID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 更新角色
	user.Role = req.Role
	users[userID] = user

	c.JSON(http.StatusOK, gin.H{
		"message":  "user updated",
		"username": userID,
		"new_role": req.Role,
	})
}
