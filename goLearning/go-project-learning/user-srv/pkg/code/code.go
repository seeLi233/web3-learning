package code

import (
	"fmt"
	"math/rand/v2"
)

// GenerateCode 生成 6 位数字验证码
func GenerateCode() string {
	return fmt.Sprintf("%06d", rand.IntN(1000000))
}
