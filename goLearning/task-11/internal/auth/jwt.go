package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ==================== 常量定义 ====================

const (
	// AccessTokenTTL — AT 为什么只有 15 分钟？
	// → 安全设计的"最小暴露窗口"原则：即使 AT 泄露，攻击者也只能用 15 分钟。
	//    AT 每次请求都通过网络传输（放在 Header 里），泄露风险比 RT（只传一次刷新接口）大得多。
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL — RT 为什么是 7 天？
	// → 平衡用户体验和安全：7 天内不用重新登录，但过期后必须重新认证。
	//    RT 存 HttpOnly Cookie，不会被 JS 读取，泄露风险小。
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// ==================== 自定义 Claims ====================

// CustomClaims 定义 JWT Payload 的自定义字段
// 为什么用 struct 嵌入 RegisteredClaims 而不是自己定义所有字段？
// → jwt.RegisteredClaims 提供了标准字段（exp/iat/nbf/iss/sub/jti），
//
//	库会自动校验 exp（过期时间），不用手动判断。继承标准 + 扩展自定义字段是最佳实践。
type CustomClaims struct {
	// UserID — 用户的唯一标识，token 解析后注入到 context 供后续中间件和 handler 使用
	UserID string `json:"user_id"`
	// Role — 用户角色，RBAC 中间件的判断依据。放在 JWT 里避免了每次请求都去数据库查角色
	Role string `json:"role"`
	// TokenType — "access" 或 "refresh"，解析 token 时判断类型，防止用 Refresh Token 当 Access Token 用
	TokenType string `json:"token_type"`
	// 嵌入标准 Claims — 包含 exp（过期时间）、iat（签发时间）、jti（唯一 ID，用于黑名单）
	jwt.RegisteredClaims
}

// ==================== 核心函数 ====================

// GenerateAccessToken 生成访问令牌
// 参数：
//   - secret: 服务端签名密钥。为什么通过参数传入而不是全局变量？
//     → 方便测试——测试时用测试密钥，生产环境用环境变量注入。全局变量会让测试互相干扰。
//   - userID: 用户 ID，存入 token payload
//   - role: 用户角色，存入 token payload
//
// 返回值：签名字符串 + error（生成失败时返回具体错误，调用者决定如何处理）
func GenerateAccessToken(secret []byte, userID, role string) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Role:      role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt — 过期时间。jwt 库会自动校验，过期后 Parse 会返回 token.EXPIRED 错误
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			// IssuedAt — 签发时间。用于判断"token 是否在某个时间点之后签发"（如强制所有旧 token 失效）
			IssuedAt: jwt.NewNumericDate(now),
			// ID — token 的唯一标识（jti = JWT ID）。用于黑名单：logout 时把 jti 放入 Redis，
			//   后续请求验证 token 有效后还要查 jti 是否在黑名单中
			ID: generateJTI(userID, "access"),
		},
	}

	// NewWithClaims — 创建 token 对象，指定签名算法为 HS256
	// 为什么选 HS256 而不是 RS256？
	// → HS256 是对称加密——同一个 secret 签发和验证，适合单体或小规模微服务。
	//    RS256 是非对称——私钥签发 + 公钥验证，适合多服务场景（Auth 服务签发，其他服务用公钥验证）。
	//    本项目是单体架构，HS256 足够，且比 RS256 更快。
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// SignedString — 用 secret 签名并生成最终的 JWT 字符串
	return token.SignedString(secret)
}

// GenerateRefreshToken 生成刷新令牌
// 与 AccessToken 的区别：
//  1. TokenType = "refresh"，防止混用
//  2. TTL 更长（7 天 vs 15 分钟）
//  3. 通常放在 HttpOnly Cookie 中传输
func GenerateRefreshToken(secret []byte, userID, role string) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Role:      role,
		TokenType: "refresh", // ← 标记类型为 refresh，解析时检查
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(userID, "refresh"),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// GenerateTokenPair 一次性生成 Access Token + Refresh Token 对
// 为什么封装成一对生成？
// → 保证两个 token 使用相同的签发时间和用户信息，避免各自调用时信息不一致。
//
//	这是 login 成功后的标准返回格式。
func GenerateTokenPair(secret []byte, userID, role string) (*TokenPair, error) {
	// 并发生成两个 token —— 它们互不依赖，golang 的 goroutine 可以并行执行
	// 为什么这里不用 goroutine 并发？
	// → 两个 HMAC-SHA256 签名操作很快（微秒级），goroutine 调度开销反而更大。
	//    只在 IO 密集型操作（如同时查多个数据库/Redis）时用 goroutine 才有收益。
	accessToken, err := GenerateAccessToken(secret, userID, role)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := GenerateRefreshToken(secret, userID, role)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ParseToken 解析并验证 JWT token
// 参数：
//   - secret: 签名密钥（必须和签发时相同）
//   - tokenString: 完整的 JWT 字符串（"header.payload.signature"）
//
// 返回值：
//   - *CustomClaims: 解析后的 payload 数据
//   - error: 验证失败的原因（过期/签名不匹配/格式错误等）
func ParseToken(secret []byte, tokenString string) (*CustomClaims, error) {
	// ParseWithClaims — jwt 库的核心解析函数
	// 参数 1: token 字符串
	// 参数 2: 空的 CustomClaims 结构体 —— 库会把 payload 反序列化到这个结构里
	// 参数 3: keyFunc —— 回调函数，库用返回的密钥验证签名
	//         如果签名算法不是 HS256，这里可以返回不同密钥（多密钥轮转场景）
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// 类型断言：确保签名算法是 HMAC 的实例，防止攻击者用 "none" 算法绕过签名
			// ⚠️ 这是一个安全关键检查：CVE 历史中有多个 JWT 库因为没验证 alg 而被攻破
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		})

	if err != nil {
		return nil, err
	}

	// token.Valid 由库根据 RegisteredClaims 中的 exp/nbf 等字段自动判断
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	cliams, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	return cliams, nil
}

// RefreshAccessToken 用 Refresh Token 换取新的 Access Token
// 为什么需要这个函数而不是直接用 GenerateAccessToken？
// → 必须验证 Refresh Token 的有效性：1) token 本身有效（未过期+签名正确）
//  2. tokenType 是 "refresh"（防止用 Access Token 换 Access Token）
//  3. token 不在黑名单中（这一步由调用者处理）
func RefreshAccessToken(secret []byte, refreshTokenStr string) (string, error) {
	claims, err := ParseToken(secret, refreshTokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// 类型检查：只有 refresh token 才能换新的 access token
	// 防止攻击者拿到 access token 后无限刷新
	if claims.TokenType != "refresh" {
		return "", errors.New("token type is not refresh")
	}

	// 生成新的 Access Token（保留原用户的 userID 和 role）
	return GenerateAccessToken(secret, claims.UserID, claims.Role)
}

// ==================== 辅助类型和函数 ====================

// TokenPair — login 成功后返回给客户端的 token 对
// 为什么 AccessToken 和 RefreshToken 用不同字段而不是放在一个数组里？
// → 语义明确，调用方不需要约定数组顺序。前端也更容易解析。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// generateJTI 生成 token 唯一 ID
// 格式：{userID}:{type}:{timestamp}
// 作用：
//  1. 黑名单：logout 时把 jti 存入 Redis，后续请求验证 token 后查 jti 是否在黑名单
//  2. 防重放攻击：每个 token 有唯一 ID，即使 payload 相同也无法重放
func generateJTI(userID, tokenType string) string {
	return fmt.Sprintf("%s:%s:%d", userID, tokenType, time.Now().UnixNano())
}
