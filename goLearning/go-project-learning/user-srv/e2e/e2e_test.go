//go:build e2e

// 为什么用 build tag 而不是塞进普通测试？
// → 让 E2E 和单元测试解耦：普通 go test 自动跳过（不碰 Docker），
//    E2E 只在 go test -tags=e2e 时显式运行。这是 Go 生态隔离慢测试的标准做法

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-project-learning/project/common/pkg/database"
	"github.com/go-project-learning/project/user-srv/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jw "github.com/go-project-learning/project/gateway-api/pkg/jwt"
	jwtlib "github.com/golang-jwt/jwt/v5"
)

// ============ 测试辅助 ============

// apiResp 与 gateway-api/pkg/resp.Response 结构一致，用于反序列化响应
type apiResp struct {
	Code int32       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// doRegister 发送一个注册请求，返回解析后的响应
//
// 为什么每个用例传一个独立的 clientIP？
// → register 有 3次/分钟/IP 的滑动窗口限流。httptest 默认所有请求同源 IP，
//
//	多个用例共用会误触限流导致 flaky。给每个用例独立 IP = 数据隔离，互不干扰
func doRegister(t *testing.T, clientIP string, body map[string]string) apiResp {
	t.Helper() // 标记为辅助函数，报错时定位到调用者而不是这里

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/register", strings.NewReader(string(jsonBody)))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = clientIP + ":1234" // 独立 IP，绕开限流碰撞

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req) // 走真实路由，全程无 mock

	var r apiResp
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	return r
}

// ============ A 组：注册成功全链路 ============

func TestRegisterFullChain(t *testing.T) {
	setupTest(t) // ← 新增这一行：前清 + 后清

	resp := doRegister(t, "10.0.0.1", map[string]string{
		"name":     "alice",
		"password": "pass123",
		"phone":    "13800000001",
		"email":    "alice@test.com",
	})

	// 断言 1：网关返回成功（code=0）
	require.Equal(t, int32(0), resp.Code, "期望注册成功，实际: %s", resp.Msg)

	// 断言 2：数据库真的落盘了
	// 为什么这条断言是 E2E 的灵魂？
	// → 单测只断言"handler 调用了 CreateUser"，E2E 直接查真实数据库，
	//    确认这条记录真实存在，中间任何一环断了（gRPC 没通/事务没提交）都会暴露
	var user entity.User
	err := database.DB.Where("username = ?", "alice").First(&user).Error
	require.NoError(t, err, "注册后应能在数据库查到该用户")

	// 断言 3：密码是 bcrypt 哈希，不是明文
	// 为什么重要？
	// → 如果将来有人误删 CreateUser 里的 bcrypt.HashPassword，
	//    单测（mock 了 repo）测不到，E2E 会立刻在这里炸掉
	assert.NotEqual(t, "pass123", user.Password)
	assert.True(t, strings.HasPrefix(user.Password, "$2a$"), "密码应为 bcrypt 哈希")
}

// ============ B 组：唯一性校验（跨服务透传错误码） ============

func TestRegisterDuplicate(t *testing.T) {
	setupTest(t) // ← 新增这一行：前清 + 后清
	// 先正常注册 bob
	first := doRegister(t, "10.0.0.2", map[string]string{
		"name": "bob", "password": "pass123", "phone": "13800000002", "email": "bob@test.com",
	})
	require.Equal(t, int32(0), first.Code)

	tests := []struct {
		name     string
		body     map[string]string
		wantCode int32
		wantMsg  string
	}{
		{
			// 为什么重复用户名是 10003？→ user-srv 服务端返回的错误码，
			// 网关原样透传，验证了"错误码能跨 gRPC 边界正确传递"
			name:     "B1. 重复用户名",
			body:     map[string]string{"name": "bob", "password": "x", "phone": "13899999999", "email": "other@test.com"},
			wantCode: 10003,
			wantMsg:  "用户名已存在",
		},
		{
			name:     "B2. 重复手机号",
			body:     map[string]string{"name": "carol", "password": "x", "phone": "13800000002", "email": "carol@test.com"},
			wantCode: 10005,
			wantMsg:  "手机号已注册",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doRegister(t, "10.0.0.3", tt.body)
			assert.Equal(t, tt.wantCode, resp.Code, "期望错误码 %d, 实际 %d (%s)", tt.wantCode, resp.Code, resp.Msg)
			assert.Contains(t, resp.Msg, tt.wantMsg)
		})
	}
}

// ============ C 组：参数校验 ============

func TestRegisterValidation(t *testing.T) {
	setupTest(t) // ← 新增这一行：前清 + 后清
	// 故意缺 name（binding:"required" 会拦截）
	resp := doRegister(t, "10.0.0.4", map[string]string{
		"password": "pass123",
	})
	assert.Equal(t, int32(10001), resp.Code)
	assert.Contains(t, resp.Msg, "参数错误")
}

// ============ D 组：认证全链路（注册 → JWT → 受保护查询） ============

func TestGetUserFullChain(t *testing.T) {
	setupTest(t) // ← 新增这一行：前清 + 后清
	// 1. 注册一个用户
	reg := doRegister(t, "10.0.0.5", map[string]string{
		"name": "dave", "password": "pass123", "phone": "13800000005", "email": "dave@test.com",
	})
	require.Equal(t, int32(0), reg.Code)

	// 2. 从数据库拿到真实 user ID
	var user entity.User
	require.NoError(t, database.DB.Where("username = ?", "dave").First(&user).Error)

	// 3. 签发一个有效 JWT
	// 为什么签名密钥必须是 "e2e-test-secret"？
	// → 必须与 setup_test.go 里 buildGatewayRouter 设置的 gwcfg.Conf.JWT.Secret 一致，
	//    否则网关的 JWTAuth 中间件 ParseToken 会解析失败
	claims := jw.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Phone:    user.Phone,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token, _ := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).
		SignedString([]byte("e2e-test-secret"))

	// 4. 带 token 请求受保护的 GetUser 接口
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/user/%d", user.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "10.0.0.5:1234"

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var r apiResp
	_ = json.Unmarshal(w.Body.Bytes(), &r)

	// 断言：认证通过 + 查到用户信息
	require.Equal(t, int32(0), r.Code, "期望查询成功, 实际: %s", r.Msg)
	assert.Contains(t, fmt.Sprintf("%v", r.Data), "dave", "响应数据应包含用户名 dave")
}
