package errorcode

import (
	"testing"
)

// 测试：创建普通业务错误
// Success      = 0
//
//	ParamError   = 10001 // 参数错误
//	UserNotFound = 10002 // 用户不存在
//	UserExists   = 10003 // 用户已存在
//	TokenInvalid = 10004 // token无效
//	DBErr        = 20001 // 数据库错误
//	RedisErr     = 20002 // Redis错误
//	ThirdApiErr  = 30001 // 第三方接口错误
//	SystemErr    = 50001 // 系统错误
//	UnknownErr   = 50002 // 未知错误
func TestNewBizError(t *testing.T) {
	// 表驱动用例
	tests := []struct {
		name string
		code int
		msg  string
		// 预期结果
		wantErrStr string
		wantJSON   string
	}{
		{
			name:       "参数错误",
			code:       10001,
			msg:        "参数错误",
			wantErrStr: "code:10001, msg:参数错误",
			wantJSON:   `{"code":10001,"msg":"参数错误"}`,
		},
		{
			name:       "用户不存在",
			code:       10002,
			msg:        "用户不存在",
			wantErrStr: "code:10002, msg:用户不存在",
			wantJSON:   `{"code":10002,"msg":"用户不存在"}`,
		},
		{
			name:       "用户已存在",
			code:       10003,
			msg:        "用户已存在",
			wantErrStr: "code:10003, msg:用户已存在",
			wantJSON:   `{"code":10003,"msg":"用户已存在"}`,
		},
		{
			name:       "token无效",
			code:       10004,
			msg:        "token无效",
			wantErrStr: "code:10004, msg:token无效",
			wantJSON:   `{"code":10004,"msg":"token无效"}`,
		},
		{
			name:       "数据库错误",
			code:       20001,
			msg:        "数据库错误",
			wantErrStr: "code:20001, msg:数据库错误",
			wantJSON:   `{"code":20001,"msg":"数据库错误"}`,
		},
		{
			name:       "Redis错误",
			code:       20002,
			msg:        "Redis错误",
			wantErrStr: "code:20002, msg:Redis错误",
			wantJSON:   `{"code":20002,"msg":"Redis错误"}`,
		},
		{
			name:       "第三方接口错误",
			code:       30001,
			msg:        "第三方接口错误",
			wantErrStr: "code:30001, msg:第三方接口错误",
			wantJSON:   `{"code":30001,"msg":"第三方接口错误"}`,
		},
		{
			name:       "系统错误",
			code:       50001,
			msg:        "系统错误",
			wantErrStr: "code:50001, msg:系统错误",
			wantJSON:   `{"code":50001,"msg":"系统错误"}`,
		},
		{
			name:       "未知错误",
			code:       50002,
			msg:        "未知错误",
			wantErrStr: "code:50002, msg:未知错误",
			wantJSON:   `{"code":50002,"msg":"未知错误"}`,
		},
	}

	// 遍历执行
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBizError(tt.code, tt.msg)

			// 1. 校验错误字符串
			if err.Error() != tt.wantErrStr {
				t.Errorf("Error(): %q, want: %q", err.Error(), tt.wantErrStr)
			}

			// 2. 校验 JSON 格式
			if err.ToJSON() != tt.wantJSON {
				t.Errorf("ToJSON(): %q, want: %q", err.ToJSON(), tt.wantJSON)
			}
		})
	}
}

// 基准测试：创建错误性能
func BenchmarkNewBizError(b *testing.B) {
	// 固定测试数据
	code := 10001
	msg := "参数错误"

	// b.N 自动循环
	for i := 0; i < b.N; i++ {
		NewBizError(code, msg)
	}
}
