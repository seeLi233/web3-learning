package errorcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
)

type BizError struct {
	Code    int    `json:"code"` // 错误码
	Message string `json:"msg"`  // 错误信息
	Cause   error  `json:"-"`    // 原始错误（用于 Unwarp）
	Stack   string `json:"-"`    // 调用对战（不上报前端）
}

// 实现 Go 原生 error 接口
func (e *BizError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("code:%d, msg:%s, cause:%s", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("code:%d, msg:%s", e.Code, e.Message)
}

// Unwrap 实现官方 Unwrap 接口 （支持 erroes.Is 和 errors.As）
func (e *BizError) Unwrap() error {
	return e.Cause
}

// ==============================================
// 对外提供的构造函数（业务代码只调用这 2 个）
// ==============================================

// New 新建一个业务错误（最常用）
func NewBizError(code int, msg string) *BizError {
	return &BizError{
		Code:    code,
		Message: msg,
		Stack:   stack(),
	}
}

// Wrap 包装底层错误（数据库/第三方/系统错误）
func Wrap(code int, msg string, cause error) error {
	if cause == nil {
		return nil
	}
	return &BizError{
		Code:    code,
		Message: msg,
		Cause:   cause,
		Stack:   stack(),
	}
}

// ==============================================
// 工具方法：获取堆栈
// ==============================================

func stack() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	return string(buf[:n])
}

// ==============================================
// 工具方法：快速从 err 中取出 BizError （用于接口返回）
// ==============================================
func FromError(err error) (*BizError, bool) {
	if err == nil {
		return nil, false
	}
	var be *BizError
	if ok := errors.As(err, &be); ok {
		return be, true
	}
	// 非业务错误，统一转为系统错误
	return &BizError{
		Code:    UnknownErr,
		Message: Msg(UnknownErr),
	}, false
}

// ==============================================
// 工具方法：ToJSON 将 BizError 转为 JSON 字符串（用于接口返回）
// ==============================================
func (e *BizError) ToJSON() string {
	bs, _ := json.Marshal(e)
	return string(bs)
}
