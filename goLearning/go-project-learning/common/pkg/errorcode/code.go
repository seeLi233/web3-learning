package errorcode

const (
	Success       = 0
	ParamError    = 10001 // 参数错误
	UserNotFound  = 10002 // 用户不存在
	UserExists    = 10003 // 用户已存在
	TokenInvalid  = 10004 // token无效
	PhoneExists   = 10005 // 新增
	EmailExists   = 10006 // 新增
	PasswordError = 10007 // 新增（登录用）
	DBErr         = 20001 // 数据库错误
	RedisErr      = 20002 // Redis错误
	ThirdApiErr   = 30001 // 第三方接口错误
	SystemErr     = 50001 // 系统错误
	UnknownErr    = 50002 // 未知错误
)

// 错误码 -> 中文说明(可用于自动生成接口文档)
func Msg(code int) string {
	switch code {
	case Success:
		return "success"
	case ParamError:
		return "参数错误"
	case UserNotFound:
		return "用户不存在"
	case UserExists:
		return "用户已存在"
	case TokenInvalid:
		return "token无效"
	case DBErr:
		return "数据库错误"
	case RedisErr:
		return "Redis错误"
	case ThirdApiErr:
		return "第三方接口错误"
	case SystemErr:
		return "系统错误"
	default:
		return "未知错误"
	}
}
