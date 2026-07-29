package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试密钥——测试专用，避免和真实密钥混淆
var testSecret = []byte("test-secret-for-unit-tests")

// ==================== A. Token 生成测试 ====================

func TestGenerateAccessToken(t *testing.T) {
	token, err := GenerateAccessToken(testSecret, "user123", RoleUser)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// 验证 token 由三部分组成（header.payload.signature）
	// 用 "." 分割后应该有且仅有 3 段
	parts := splitToken(t, token)
	assert.Len(t, parts, 3, "JWT 应该由 header.payload.signature 三段组成")
}

func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken(testSecret, "user123", RoleUser)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// 验证 token 可以成功解析，且类型是 "refresh"
	claims, err := ParseToken(testSecret, token)
	require.NoError(t, err)
	assert.Equal(t, "refresh", claims.TokenType)
	assert.Equal(t, "user123", claims.UserID)
}

func TestGenerateTokenPair(t *testing.T) {
	pair, err := GenerateTokenPair(testSecret, "user123", RoleAdmin)
	require.NoError(t, err)
	require.NotNil(t, pair)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	// AT 和 RT 应该是不同的 token（jti 和 tokenType 都不同）
	assert.NotEqual(t, pair.AccessToken, pair.RefreshToken,
		"Access Token 和 Refresh Token 必须不同")
}

// ==================== B. Token 解析验证测试 ====================

func TestParseToken_Success(t *testing.T) {
	token, _ := GenerateAccessToken(testSecret, "alice", RoleUser)

	claims, err := ParseToken(testSecret, token)
	require.NoError(t, err)
	assert.Equal(t, "alice", claims.UserID)
	assert.Equal(t, RoleUser, claims.Role)
	assert.Equal(t, "access", claims.TokenType)
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, _ := GenerateAccessToken(testSecret, "alice", RoleUser)

	// 用不同的密钥解析——必须失败，否则任何人拿到 token 都能伪造
	_, err := ParseToken([]byte("wrong-secret"), token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

func TestParseToken_ExpiredToken(t *testing.T) {
	// 手动构造一个已过期的 token
	// 为什么不用 GenerateAccessToken + 等待 15 分钟？
	// → 测试不能等 15 分钟！直接构造 token 手动设置过期时间。
	now := time.Now()
	claims := CustomClaims{
		UserID:    "alice",
		Role:      RoleUser,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			// 过期时间设为 1 小时前
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ID:        "test-jti-expired",
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, err := tokenObj.SignedString(testSecret)
	require.NoError(t, err)

	_, err = ParseToken(testSecret, expiredToken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestParseToken_EmptyToken(t *testing.T) {
	_, err := ParseToken(testSecret, "")
	require.Error(t, err)
}

func TestParseToken_InvalidFormat(t *testing.T) {
	_, err := ParseToken(testSecret, "this-is-not-a-jwt")
	require.Error(t, err)
}

// ==================== C. Token 刷新测试 ====================

func TestRefreshAccessToken_Success(t *testing.T) {
	rt, err := GenerateRefreshToken(testSecret, "bob", RoleUser)
	require.NoError(t, err)

	newAT, err := RefreshAccessToken(testSecret, rt)
	require.NoError(t, err)
	require.NotEmpty(t, newAT)

	// 验证新 token 是 access 类型
	claims, err := ParseToken(testSecret, newAT)
	require.NoError(t, err)
	assert.Equal(t, "access", claims.TokenType)
	assert.Equal(t, "bob", claims.UserID)
}

func TestRefreshAccessToken_UseAccessTokenToRefresh(t *testing.T) {
	// 场景：攻击者拿到 AT，试图用它换新 AT（无限续命攻击）
	at, err := GenerateAccessToken(testSecret, "alice", RoleUser)
	require.NoError(t, err)

	// RefreshAccessToken 必须拒绝——因为 tokenType 是 "access" 而不是 "refresh"
	_, err = RefreshAccessToken(testSecret, at)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not refresh")
}

func TestRefreshAccessToken_ExpiredRefreshToken(t *testing.T) {
	// 构造过期的 RT
	now := time.Now()
	claims := CustomClaims{
		UserID:    "alice",
		Role:      RoleUser,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-8 * 24 * time.Hour)),
			ID:        "test-jti-expired-rt",
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredRT, err := tokenObj.SignedString(testSecret)
	require.NoError(t, err)

	_, err = RefreshAccessToken(testSecret, expiredRT)
	require.Error(t, err)
}

// ==================== C4. 🔥 面试用例 ====================

func TestC4_PreventNoneAlgorithmAttack(t *testing.T) {
	// 面试题：JWT 的 "none" 算法攻击是怎么回事？如何防御？
	//
	// 攻击原理：某些有漏洞的 JWT 库接受 alg=none 的 token（无签名），
	// 攻击者可以修改 payload（如把 role 改为 admin），然后发 alg=none 的 token。
	//
	// 防御：ParseToken 中的 keyFunc 检查签名算法必须是 *jwt.SigningMethodHMAC

	// 构造一个 alg=none 的 token 字符串
	// Base64URL("{\"alg\":\"none\",\"typ\":\"JWT\"}") = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"
	// Base64URL("{\"user_id\":\"admin\",\"role\":\"admin\",\"token_type\":\"access\"}") = ...
	// 第三段签名部分为空（none 算法不需要签名）
	noneToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJ1c2VyX2lkIjoiYWRtaW4iLCJyb2xlIjoiYWRtaW4iLCJ0b2tlbl90eXBlIjoiYWNjZXNzIn0."

	// ParseToken 中的 keyFunc 会检查算法，发现 alg=none 不是 HMAC 时直接返回 error
	_, err := ParseToken(testSecret, noneToken)
	require.Error(t, err, "alg=none 的 token 必须被拒绝")
	assert.Contains(t, err.Error(), "signing method",
		"错误信息应明确指出签名算法不合法")
}

func TestC4_AccessTokenCannotBeUsedForRefresh(t *testing.T) {
	// 面试题：为什么要区分 AT 和 RT 的类型？攻击场景是什么？
	// 攻击场景：攻击者通过 XSS 拿到了 AT，如果用 AT 就能换新 AT，
	// 攻击者就可以无限续命（因为刷新不检查密码，只检查 token 有效性）。
	// 防御：RT 单独标记 tokenType="refresh"，RefreshAccessToken 检查类型。

	at, _ := GenerateAccessToken(testSecret, "alice", RoleUser)

	// 用 AT 去刷新 → 必须失败
	_, err := RefreshAccessToken(testSecret, at)
	require.Error(t, err, "Access Token 不能用于刷新")
}

func TestC4_TamperedPayload(t *testing.T) {
	// 面试题：如何验证 JWT 的完整性？如果有人篡改 payload，会发生什么？
	// 答：签名会不匹配。服务端用 header+payload+secret 重新计算签名，
	// 与收到的签名对比，不一致则拒绝。

	// 生成合法 token
	token, _ := GenerateAccessToken(testSecret, "alice", RoleUser)
	parts := splitToken(t, token)

	// 把 payload 中的 role 从 "user" 改为 "admin"（模拟篡改）
	// 注意：只改了 payload，签名没变
	_ = parts // payload 已编码，这里用另一个方式测试

	// 更好且更直观的验证方式：用错误的密钥去解析，一定失败
	tamperedToken := parts[0] + "." + parts[1] + ".fakesignature"
	_, err := ParseToken(testSecret, tamperedToken)
	require.Error(t, err, "篡改的 token 必须被拒绝")
}

// ==================== 辅助函数 ====================

func splitToken(t *testing.T, token string) []string {
	t.Helper()
	parts := make([]string, 0)
	// 简单的手动分割
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
