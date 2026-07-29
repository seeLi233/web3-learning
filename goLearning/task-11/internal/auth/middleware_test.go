package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== 测试辅助：Mock Blacklist ====================

// mockBlacklist 用内存 map 模拟 Redis 黑名单，不需要真实 Redis
// 为什么自己写 mock 而不是用 mock 框架？
// → 保持依赖最小化——测试不需要引入第三方 mock 库。
//
//	手写 mock 只有 20 行代码，引入库反而增加维护负担。
type mockBlacklist struct {
	mu    sync.RWMutex
	items map[string]time.Time // jti -> 过期时间
}

func newMockBlacklist() *mockBlacklist {
	return &mockBlacklist{
		items: make(map[string]time.Time),
	}
}

func (m *mockBlacklist) IsBlacklisted(_ interface{}, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, exists := m.items[jti]
	if !exists {
		return false, nil
	}
	// 检查是否过期（模拟 Redis TTL）
	if time.Now().After(exp) {
		return false, nil
	}
	return true, nil
}

func (m *mockBlacklist) Add(_ interface{}, jti string, ttl interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, ok := ttl.(time.Duration)
	if !ok {
		d = 15 * time.Minute
	}
	m.items[jti] = time.Now().Add(d)
	return nil
}

// ==================== 测试辅助：Gin 测试引擎 ====================

// setupTestRouter 创建带 Auth 和 RBAC 中间件的测试路由
// 每次测试调用，获得一个干净的路由实例，避免测试间状态污染
func setupTestRouter(secret []byte, blacklist TokenBlacklist) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 公开路由
	r.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})

	// 需要认证的路由
	authGroup := r.Group("/api")
	authGroup.Use(AuthMiddleware(secret, blacklist))
	{
		authGroup.GET("/profile", func(c *gin.Context) {
			userID, _ := c.Get("userID")
			role, _ := c.Get("role")
			c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
		})
	}

	// 需要 admin 角色的路由
	adminGroup := r.Group("/admin")
	adminGroup.Use(AuthMiddleware(secret, blacklist))
	adminGroup.Use(RBACMiddleware(RoleAdmin))
	{
		adminGroup.GET("/dashboard", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "admin dashboard"})
		})
	}

	// 需要 moderator 或 admin 角色的路由
	modGroup := r.Group("/mod")
	modGroup.Use(AuthMiddleware(secret, blacklist))
	modGroup.Use(RBACMiddleware(RoleModerator, RoleAdmin))
	{
		modGroup.PUT("/content", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "content updated"})
		})
	}

	// 需要 user:write 权限的路由
	writeGroup := r.Group("/content")
	writeGroup.Use(AuthMiddleware(secret, blacklist))
	writeGroup.Use(RBACMiddleware(PermUserWrite))
	{
		writeGroup.POST("/create", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"message": "created"})
		})
	}

	return r
}

// makeRequest 发起测试请求的辅助函数
func makeRequest(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

// ==================== A. Auth 中间件测试 ====================

func TestAuthMiddleware_NoToken(t *testing.T) {
	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/api/profile", "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/api/profile", "invalid-token")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	at, err := GenerateAccessToken(testSecret, "alice", RoleUser)
	require.NoError(t, err)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/api/profile", at)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证返回的 userID 和 role 是 JWT 中的值
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "alice", resp["user_id"])
	assert.Equal(t, RoleUser, resp["role"])
}

func TestAuthMiddleware_RefreshTokenUsedAsAccessToken(t *testing.T) {
	// 场景：攻击者或粗心的开发者用 Refresh Token 访问 API
	rt, err := GenerateRefreshToken(testSecret, "alice", RoleUser)
	require.NoError(t, err)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/api/profile", rt)

	// Auth 中间件检查 tokenType == "access"，RT 会被拒绝
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MissingBearerPrefix(t *testing.T) {
	at, _ := GenerateAccessToken(testSecret, "alice", RoleUser)

	r := setupTestRouter(testSecret, nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/profile", nil)
	// 故意不加 "Bearer " 前缀
	req.Header.Set("Authorization", at)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_PublicRouteNoTokenRequired(t *testing.T) {
	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/public", "")

	// 公开路由无需 token，应该直接返回 200
	assert.Equal(t, http.StatusOK, w.Code)
}

// ==================== B. 黑名单测试 ====================

func TestAuthMiddleware_BlacklistedToken(t *testing.T) {
	mockBL := newMockBlacklist()
	at, err := GenerateAccessToken(testSecret, "alice", RoleUser)
	require.NoError(t, err)

	// 解析 token 拿到 jti
	claims, _ := ParseToken(testSecret, at)
	// 将 jti 加入黑名单
	mockBL.Add(context.Background(), claims.ID, AccessTokenTTL)

	r := setupTestRouter(testSecret, mockBL)
	w := makeRequest(r, "GET", "/api/profile", at)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ==================== C. RBAC 测试 ====================

func TestRBAC_AdminAccessAdminRoute(t *testing.T) {
	at, _ := GenerateAccessToken(testSecret, "admin", RoleAdmin)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/admin/dashboard", at)

	assert.Equal(t, http.StatusOK, w.Code, "admin 角色应该能访问 /admin 路由")
}

func TestRBAC_UserAccessAdminRoute(t *testing.T) {
	at, _ := GenerateAccessToken(testSecret, "alice", RoleUser)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/admin/dashboard", at)

	assert.Equal(t, http.StatusForbidden, w.Code, "普通 user 不能访问 /admin 路由")
}

func TestRBAC_ModeratorAccessModRoute(t *testing.T) {
	at, _ := GenerateAccessToken(testSecret, "moderator", RoleModerator)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "PUT", "/mod/content", at)

	assert.Equal(t, http.StatusOK, w.Code, "moderator 角色应该能访问 /mod 路由")
}

func TestRBAC_AdminAccessModRoute(t *testing.T) {
	// admin 也能访问 moderator 路由——因为 RBACMiddleware(RoleModerator, RoleAdmin)
	// 允许这两个角色中的任意一个
	at, _ := GenerateAccessToken(testSecret, "admin", RoleAdmin)

	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "PUT", "/mod/content", at)

	assert.Equal(t, http.StatusOK, w.Code,
		"admin 角色也应该能访问 /mod 路由（多角色允许）")
}

func TestRBAC_NoAuthMiddlewareBeforeRBAC(t *testing.T) {
	// 如果直接请求需要 RBAC 的路由但不提供 token，
	// Auth 中间件应该先拦截（返回 401），RBAC 中间件不会执行
	r := setupTestRouter(testSecret, nil)
	w := makeRequest(r, "GET", "/admin/dashboard", "")

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"缺少 token 应该先被 Auth 中间件拦截，返回 401 而非 403")
}

func TestRBAC_WildcardPermission(t *testing.T) {
	// admin 角色有 "admin:*" 通配符权限
	// hasPermission 中的通配符匹配逻辑是否正确？
	assert.True(t, hasPermission([]string{"admin:*"}, "admin:read"),
		"admin:* 应该匹配 admin:read")
	assert.True(t, hasPermission([]string{"admin:*"}, "admin:write"),
		"admin:* 应该匹配 admin:write")
	assert.False(t, hasPermission([]string{"admin:*"}, "user:read"),
		"admin:* 不应该匹配 user:read（前缀不同）")
}

// ==================== C4. 🔥 面试用例 ====================

func TestC4_CompleteLoginToAccessFlow(t *testing.T) {
	// 面试题：请描述从用户登录到访问受保护资源的完整流程，
	// 包括 token 生成、传输、验证、刷新的各个环节
	//
	// 这是一个端到端的演示，覆盖了完整的认证流水线

	secret := []byte("interview-secret")

	// Step 1: 用户登录 → 获得 token pair
	pair, err := GenerateTokenPair(secret, "alice", RoleUser)
	require.NoError(t, err)
	t.Logf("Step 1: 登录成功，AT=%s..., RT=%s...",
		pair.AccessToken[:20], pair.RefreshToken[:20])

	// Step 2: 用 AT 访问受保护 API
	claims, err := ParseToken(secret, pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "access", claims.TokenType)
	t.Logf("Step 2: AT 验证通过，userID=%s, role=%s", claims.UserID, claims.Role)

	// Step 3: AT 过期后，用 RT 刷新
	newAT, err := RefreshAccessToken(secret, pair.RefreshToken)
	require.NoError(t, err)
	t.Logf("Step 3: 刷新成功，新 AT=%s...", newAT[:20])

	// Step 4: 新 AT 可以正常使用
	claims2, err := ParseToken(secret, newAT)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims2.UserID)
	t.Logf("Step 4: 新 AT 验证通过")

	// Step 5: 登出 → AT 加入黑名单
	bl := newMockBlacklist()
	ctx := context.Background()
	_ = bl.Add(ctx, claims2.ID, AccessTokenTTL)
	blacklisted, _ := bl.IsBlacklisted(ctx, claims2.ID)
	assert.True(t, blacklisted)
	t.Logf("Step 5: 登出成功，AT jti 已加入黑名单")
}

func TestC4_CVE_NoneAlgorithmAttack(t *testing.T) {
	// 面试题：CVE 历史中 JWT 的 alg=none 漏洞是什么？你代码里如何防御？
	//
	// 这个测试验证：即使攻击者构造完美格式的 none-alg token，ParseToken 也要拒绝

	// 第一段：{"alg":"none","typ":"JWT"} → Base64URL 编码
	header := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"
	// 第二段：{"user_id":"admin","role":"admin","token_type":"access"} → Base64URL 编码
	payload := "eyJ1c2VyX2lkIjoiYWRtaW4iLCJyb2xlIjoiYWRtaW4iLCJ0b2tlbl90eXBlIjoiYWNjZXNzIn0"
	// 第三段：空（none 算法不需要签名）
	noneToken := header + "." + payload + "."

	_, err := ParseToken(testSecret, noneToken)
	require.Error(t, err, "alg=none attack must be blocked")
	t.Logf("alg=none 攻击已被成功拦截: %v", err)
}

func TestC4_CookieVsLocalStorage(t *testing.T) {
	// 面试题：Token 应该存在哪里？为什么？
	//
	// 这个测试不是验证代码，而是演示安全原理——
	// 生成 token 后"模拟"三种存储方式的风险对比

	t.Run("HttpOnly_Cookie_protection", func(t *testing.T) {
		// HttpOnly Cookie：JS 无法通过 document.cookie 读取
		// 这个属性由服务端 set-cookie 时设置，客户端 JS 被阻止访问
		// → XSS 攻击者无法窃取（但 CSRF 仍需 SameSite 防护）
		t.Log("HttpOnly Cookie: XSS-safe, CSRF needs SameSite=Strict")
	})

	t.Run("localStorage_risk", func(t *testing.T) {
		// localStorage：任何 JS 代码都能读取
		// → 一旦有 XSS 漏洞，攻击者直接 localStorage.getItem('token') 拿走
		t.Log("localStorage: vulnerable to XSS, avoid for sensitive tokens")
	})

	t.Run("memory_storage", func(t *testing.T) {
		// 内存变量：页面刷新就没了
		// → 最安全（XSS 也拿不到），但用户体验差（刷新页面要重新登录或静默刷新）
		t.Log("Memory: safest, needs silent refresh via Refresh Token in HttpOnly Cookie")
	})
}
