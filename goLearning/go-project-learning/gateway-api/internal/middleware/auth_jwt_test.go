package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/pkg/jwt"
	"github.com/go-project-learning/project/gateway-api/pkg/resp"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// ============================================================
// 测试辅助函数
// ============================================================

// initTestRouter 创建一个带 JWT 中间件的测试路由
//
// 为什么每次测试都新建一个 Gin 引擎？
// → Gin 默认使用全局模式（gin.Default() 内部自动设置 gin.Mode()），
//
//	但测试时我们需要隔离状态——每个子测试创建独立引擎，避免中间件状态互相影响
//
// 测试路由设计：GET /test → 经过 JWTAuth() → 返回用户信息
//
//	这样我们可以通过检查响应码和响应体来判断中间件行为是否正确
//
// ⚠️ 注意：config.Conf 是全局变量，测试时必须初始化，
// 否则 ParseToken() 访问 config.Conf.JWT.Secret 时会 nil pointer panic。
func initTestRouter() *gin.Engine {
	// 设置为测试模式，关掉 Gin 启动时的 DEBUG 日志
	gin.SetMode(gin.TestMode)

	// 初始化全局配置（测试环境），否则 jwt.ParseToken() 会 nil panic
	config.Conf = &config.AppConfig{
		JWT: config.JWTConfig{
			Secret: "test-secret",
		},
	}

	r := gin.New()
	r.Use(JWTAuth())

	// 受保护的路由：只有通过 JWT 校验才能访问
	// 为什么返回用户信息而不是简单 "ok"？
	// → 这样我们能断言中间件是否正确地注入了上下文中的 userID/username
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		phone, _ := c.Get("phone")

		resp.OK(c, gin.H{
			"user_id":  userID,
			"username": username,
			"phone":    phone,
		})
	})

	return r
}

// ============================================================
// JWTAuth 中间件完整测试
// ============================================================

func TestJWTAuth_MissingToken(t *testing.T) {
	// 表驱动用例：覆盖各种缺少 token 的场景
	tests := []struct {
		name       string
		setupReq   func() *http.Request // 构建请求的方式各不相同
		wantCode   int                  // 期望的 HTTP 状态码
		wantErrKey string               // 期望响应中包含的错误关键词
	}{
		{
			name: "A1. Header 中没有 Authorization",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				// 什么都不带——模拟未登录用户直接访问
				return req
			},
			wantCode:   http.StatusOK,
			wantErrKey: "缺少认证信息",
		},
		{
			name: "A2. Authorization 为空字符串",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "")
				// 空的 Authorization 头——前端可能发了但值为空
				return req
			},
			wantCode:   http.StatusOK,
			wantErrKey: "缺少认证信息",
		},
		{
			name: "A3. 既没有 Header 也没有 Cookie",
			setupReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				// 确保 Cookie 也是空的（默认就是空的，这里显式说明）
				return req
			},
			wantCode:   http.StatusOK,
			wantErrKey: "缺少认证信息",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := initTestRouter()
			req := tt.setupReq()
			w := httptest.NewRecorder() // httptest.ResponseRecorder 实现了 http.ResponseWriter，用于捕获响应

			router.ServeHTTP(w, req)

			// 1. 检查 HTTP 状态码
			if w.Code != tt.wantCode {
				t.Errorf("状态码 = %d, want %d", w.Code, tt.wantCode)
			}

			// 2. 检查响应体中包含预期的错误信息
			body := w.Body.String()
			if !contains(body, tt.wantErrKey) {
				t.Errorf("响应体 = %q, 期望包含 %q", body, tt.wantErrKey)
			}
		})
	}
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string // 各种错误格式的 Authorization 头
		wantErrKey string // 期望的错误关键词
	}{
		{
			name:       "B1. 不是 Bearer 前缀（Basic 认证）",
			authHeader: "Basic dXNlcjpwYXNz",
			// Basic 是另一种认证方式，但我们的中间件只接受 Bearer
			wantErrKey: "缺少认证信息",
		},
		{
			name:       "B2. Bearer 后面没有空格（格式错误）",
			authHeader: "Bearerxyz123",
			// strings.SplitN 按空格切分后只有一部分，不是 "Bearer token" 格式
			wantErrKey: "缺少认证信息",
		},
		{
			name:       "B3. 裸 Token 没有前缀",
			authHeader: "eyJhbGciOiJIUzI1NiJ9.xxx.yyy",
			// 直接传 token 没有 "Bearer " 前缀
			wantErrKey: "缺少认证信息",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := initTestRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			body := w.Body.String()
			if !contains(body, tt.wantErrKey) {
				t.Errorf("响应体 = %q, 期望包含 %q", body, tt.wantErrKey)
			}
		})
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		wantErrKey string
	}{
		{
			name:       "C1. 完全伪造的 Token（三段式但内容随机）",
			token:      "eyJhbGciOiJIUzI1NiJ9.INVALID_PAYLOAD.INVALID_SIGNATURE",
			wantErrKey: "token 无效",
		},
		{
			name:       "C2. 只有一段的 Token（格式完全错误）",
			token:      "not-a-valid-jwt-at-all",
			wantErrKey: "token 无效",
		},
		{
			// 为什么测试过期 token 很重要？
			// → 面试高频：JWT 注销怎么做？答案之一就是靠过期时间自动失效
			//   如果过期校验失败，用户就能拿旧 token 永久访问——这就是安全漏洞
			name: "C3. 🔥 已过期的 Token",
			token: func() string {
				// 使用 config.Conf.JWT.Secret 相同的密钥（"test-secret"）签名，
				// 这样 ParseToken 能成功解密。但 token 1 毫秒前已过期，
				// jwt.ParseWithClaims 会返回过期错误 → 预期 "token 无效"
				claims := jwt.Claims{
					UserID:   1,
					Username: "test",
					RegisteredClaims: jwtlib.RegisteredClaims{
						ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-1 * time.Millisecond)),
						IssuedAt:  jwtlib.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					},
				}
				t := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
				str, _ := t.SignedString([]byte("test-secret"))
				return str
			}(),
			wantErrKey: "token 无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := initTestRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			body := w.Body.String()
			if !contains(body, tt.wantErrKey) {
				t.Errorf("响应体 = %q, 期望包含 %q", body, tt.wantErrKey)
			}
		})
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	tests := []struct {
		name    string
		userID  uint
		user    string
		phone   string
		wantMsg string
	}{
		{
			name:    "E1. 有效 token — 正常用户",
			userID:  1,
			user:    "alice",
			phone:   "13800138000",
			wantMsg: "success",
		},
		{
			name:    "E2. 有效 token — 另一个用户",
			userID:  42,
			user:    "bob",
			phone:   "13900139000",
			wantMsg: "success",
		},
		{
			name:    "E3. phone 为空也不影响认证",
			userID:  99,
			user:    "charlie",
			phone:   "",
			wantMsg: "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 用测试 secret 签发一个有效 token
			claims := jwt.Claims{
				UserID:   tt.userID,
				Username: tt.user,
				Phone:    tt.phone,
				RegisteredClaims: jwtlib.RegisteredClaims{
					ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(1 * time.Hour)),
					IssuedAt:  jwtlib.NewNumericDate(time.Now()),
				},
			}
			token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
			tokenStr, _ := token.SignedString([]byte("test-secret"))

			router := initTestRouter()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			body := w.Body.String()
			if !contains(body, tt.wantMsg) {
				t.Errorf("期望 success, got: %q", body)
			}
			// 验证中间件正确注入了 Context 值到响应中
			if !contains(body, tt.user) {
				t.Errorf("响应应包含用户名 %q, got: %q", tt.user, body)
			}
		})
	}
}

func TestJWTAuth_CookieAuth(t *testing.T) {
	// 为什么单独测 Cookie 方式？
	// → 你的 auth_jwt.go 支持双通道认证：Header 优先，Cookie 兜底
	//   Cookie 方式是 SSO（单点登录）场景的关键——浏览器自动带 Cookie，
	//   不需要前端手动设 Authorization 头
	t.Run("D1. Header 优先 — Header 和 Cookie 同时存在时用 Header", func(t *testing.T) {
		// 这个用例验证优先级逻辑：Header 不为空 → 用 Header，忽略 Cookie
		router := initTestRouter()
		req, _ := http.NewRequest("GET", "/test", nil)

		// 同时设置 Header（无效）和 Cookie
		req.Header.Set("Authorization", "Bearer invalid-token")
		req.AddCookie(&http.Cookie{
			Name:  "access_token",
			Value: "some-valid-looking-token",
		})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 预期：因为 Header 有值，所以优先解析 Header 中的 token
		// Header 中的 token 无效 → 直接返回错误，不会回退到 Cookie
		body := w.Body.String()
		if !contains(body, "token 无效") {
			t.Errorf("应优先使用 Header 中的 token，解析失败应报 token 无效，got: %q", body)
		}
	})

	t.Run("D2. 只有 Cookie，没有 Header — 走 Cookie 通道", func(t *testing.T) {
		// 这个用例验证 Cookie 兜底逻辑
		router := initTestRouter()
		req, _ := http.NewRequest("GET", "/test", nil)

		// 不设 Authorization Header，只设 Cookie
		req.AddCookie(&http.Cookie{
			Name:  "access_token",
			Value: "cookie-token-value",
		})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 预期：Cookie 中的 token 也无法解析（格式不对无法解析）
		body := w.Body.String()
		if !contains(body, "缺少认证信息") && !contains(body, "token 无效") {
			t.Errorf("预期认证失败，got: %q", body)
		}
	})
}

// ============================================================
// 辅助函数
// ============================================================

// contains 检查字符串 s 是否包含子串 substr
func contains(s, substr string) bool {
	// 边界保护：空字符串不包含任何有意义的子串
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
