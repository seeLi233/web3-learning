package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== 角色定义 ====================

// 为什么角色用常量而不是直接用字符串？
// → 1. 编译期检查——打错字（如 "adimn"）编译器不会报错，但 MiddlewareRoleAdmin 打错会编译失败
//  2. IDE 自动补全——不用记住每个角色字符串的准确拼写
//  3. 集中管理——所有角色在一个地方定义，改角色名只需改这里
const (
	RoleAdmin     = "admin"
	RoleUser      = "user"
	RoleModerator = "moderator"
)

// ==================== 权限定义 ====================

// 为什么权限用 "resource:action" 格式？
// → 这是 RBAC 标准命名约定（如 AWS IAM），语义清晰：
//
//	"user:read" — 读用户信息、"article:delete" — 删除文章
//	支持通配符扩展（如 "user:*" 表示所有用户操作）
const (
	PermUserRead   = "user:read"
	PermUserWrite  = "user:write"
	PermUserDelete = "user:delete"
	PermAdminAll   = "admin:*" // 管理员的超级权限，匹配所有 admin 相关操作
)

// rolePermissions 角色→权限映射表
// 为什么用 map[string][]string 而不是存数据库？
// → 本次练习聚焦 RBAC 核心逻辑。生产环境应该存数据库（角色-权限多对多表），
//
//	但面试中你至少要能画出"用户→角色→权限"三层架构并解释这个映射表的作用。
//	写死在这里的好处：无外部依赖，测试快速，改权限只需改这一个地方。
var rolePermissions = map[string][]string{
	RoleAdmin: {
		PermUserRead,
		PermUserWrite,
		PermUserDelete,
		PermAdminAll,
	},
	RoleModerator: {
		PermUserRead,
		PermUserWrite,
	},
	RoleUser: {
		PermUserRead,
	},
}

// ==================== Auth 中间件 ====================

// AuthMiddleware 返回一个 Gin 中间件，验证请求中的 JWT Access Token
// 为什么是返回 gin.HandlerFunc 的函数而不是直接一个 gin.HandlerFunc？
// → 函数式选项模式——调用方可以传入不同配置（不同 secret、不同黑名单实现），
//
//	而不是只能用全局变量。这叫"依赖注入"，让中间件可测试。
//
// 参数：
//   - secret: JWT 签名密钥
//   - blacklist: 黑名单接口（Redis 实现），用于检查已登出的 token
func AuthMiddleware(secret []byte, blacklist TokenBlacklist) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步：从 Authorization Header 提取 token
		// 标准格式：Authorization: Bearer <token>
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
				"code":  "AUTH_HEADER_MISSING",
			})
			c.Abort() // ← 停止后续中间件和 handler
			return    // ← 必须 return，否则会继续执行后面的 c.Next()
		}

		// strings.CutPrefix 检查 "Bearer " 前缀
		// 为什么用 CutPrefix 而不是 Split？
		// → Split 遇到 token 中包含 "Bearer " 字符串会错误分割。
		//    CutPrefix 语义准确："去掉前缀，拿到后面的内容"
		tokenString, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format, expected 'Bearer <token>'",
				"code":  "AUTH_HEADER_INVALID",
			})
			c.Abort()
			return
		}

		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "token is empty",
				"code":  "AUTH_TOKEN_EMPTY",
			})
			c.Abort()
			return
		}

		// 第二步：解析验证 JWT
		claims, err := ParseToken(secret, tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
				"code":  "AUTH_TOKEN_INVALID",
			})
			c.Abort()
			return
		}

		// 第三步：类型检查——必须是 Access Token，不能用 Refresh Token
		if claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "refresh token cannot be used for API access",
				"code":  "AUTH_WRONG_TOKEN_TYPE",
			})
			c.Abort()
			return
		}

		// 第四步：检查黑名单（已登出的 token）
		// 为什么黑名单检查需要接口而不是直接调 Redis？
		// → 面向接口编程：测试时可以用内存 map 替代 Redis，不需要真实 Redis 环境。
		if blacklist != nil {
			blacklisted, err := blacklist.IsBlacklisted(c.Request.Context(), claims.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to check token blacklist",
					"code":  "AUTH_BLACKLIST_ERROR",
				})
				c.Abort()
				return
			}
			if blacklisted {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "token has been revoked",
					"code":  "AUTH_TOKEN_REVOKED",
				})
				c.Abort()
				return
			}
		}

		// 第五步：注入用户信息到 context，供后续中间件和 handler 使用
		// c.Set() 的 key 为什么不用字符串字面量？
		// → 全局范围内使用 const contextKey 可以防止 key 冲突。
		//    但 Gin 的 c.Set/c.Get 内部用的是 map[string]interface{}，不是 context.Context，
		//    所以用 const 字符串常量就足够——后续中间件用相同的 const 取数据。
		c.Set("userID", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("tokenID", claims.ID)

		// 放行！进入下一个中间件（通常是 RBAC 中间件或 handler）
		c.Next()
	}
}

// ==================== RBAC 中间件 ====================

// RBACMiddleware 返回一个 Gin 中间件，检查用户是否有访问所需资源的权限
//
// 为什么是 "接收权限列表，返回中间件" 的设计？
// → 不同路由需要不同权限：/api/users GET 需要 user:read，
//
//	/api/users POST 需要 user:write。注册路由时传入不同权限列表即可。
//
// 使用示例：
//
//	adminGroup := r.Group("/admin")
//	adminGroup.Use(RBACMiddleware(RoleAdmin))  // ← 所有 /admin/* 需要 admin 角色
//
//	r.GET("/api/users", AuthMiddleware(...), RBACMiddleware(PermUserRead), handler)
func RBACMiddleware(requirePerms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 第一步：从 context 中获取当前用户的角色
		// 这个值由 AuthMiddleware 在验证 JWT 后通过 c.Set("role", ...) 注入
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "role not found in context, auth middleware required",
				"code":  "RBAC_NO_ROLE",
			})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "invalid role type",
				"code":  "RBAC_INVALID_ROLE_TYPE",
			})
			c.Abort()
			return
		}

		// 第二步：查找该角色拥有的权限列表
		perms, roleExists := rolePermissions[roleStr]
		if !roleExists {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "unknown role: " + roleStr,
				"code":  "RBAC_UNKNOWN_ROLE",
			})
			c.Abort()
			return
		}

		// 第三步：检查是否满足要求
		// 支持两种粒度，匹配逻辑不同：
		//   - 角色名（如 "admin"、"moderator"）：OR 逻辑，任一角色匹配即放行
		//     → RBACMiddleware(RoleModerator, RoleAdmin) 表示 moderator 或 admin 均可
		//   - 权限字符串（如 "user:write"、"admin:*"）：AND 逻辑，所有权限都必须具备
		//     → 如果传的是纯权限列表，用户角色需同时拥有所有这些权限
		// 优先按角色名匹配（OR），失败后再按权限匹配（AND）

		// Pass 1: 尝试角色名匹配——任一命中即放行
		for _, required := range requirePerms {
			if roleStr == required {
				c.Next()
				return
			}
		}

		// Pass 2: 角色不匹配，按权限检查——所有权限都必须具备
		for _, required := range requirePerms {
			if !hasPermission(perms, required) {
				c.JSON(http.StatusForbidden, gin.H{
					"error":    "insufficient permissions",
					"code":     "RBAC_FORBIDDEN",
					"required": required,
					"role":     roleStr,
				})
				c.Abort()
				return
			}
		}

		// 权限检查通过，放行
		c.Next()
	}
}

// ==================== 辅助函数 ====================

// hasPermission 检查权限列表中是否包含目标权限
// 为什么支持通配符匹配（admin:* 匹配 admin:read、admin:write 等）？
// → 通配符大幅简化权限管理：给 admin 角色设一个 "admin:*" 就覆盖所有管理操作，
//
//	无需逐个添加每个管理权限。
func hasPermission(perms []string, target string) bool {
	for _, p := range perms {
		// 精确匹配
		if p == target {
			return true
		}
		// 通配符匹配："admin:*" 匹配 "admin:read"、"admin:write" 等
		// 逻辑：把两段都按 ":" 切开，比较前缀相同且一个是通配符
		if matchesWildcard(p, target) {
			return true
		}
	}
	return false
}

// matchesWildcard 通配符权限匹配
// 示例：p="admin:*", target="admin:read" → true
//
//	p="user:*",  target="admin:read" → false（前缀不同）
func matchesWildcard(pattern, target string) bool {
	pParts := strings.SplitN(pattern, ":", 2)
	tParts := strings.SplitN(target, ":", 2)
	if len(pParts) != 2 || len(tParts) != 2 {
		return false
	}
	return pParts[0] == tParts[0] && pParts[1] == "*"
}

// ==================== Token 黑名单接口 ====================

// TokenBlacklist 定义 token 黑名单需要实现的方法
// 为什么定义接口而不是直接用 Redis？
// → 1. 测试方便：测试时用内存 map 模拟，不需要真实 Redis
//  2. 实现可替换：以后可以换成数据库或其他存储，中间件代码不需要改
//  3. 接口隔离：中间件只需要 IsBlacklisted + Add 两个方法，不关心底层实现细节
type TokenBlacklist interface {
	// IsBlacklisted 检查 token 的 jti 是否在黑名单中
	IsBlacklisted(ctx interface{}, jti string) (bool, error)
	// Add 将 token 的 jti 加入黑名单，TTL 过后自动删除
	Add(ctx interface{}, jti string, ttl interface{}) error
}
