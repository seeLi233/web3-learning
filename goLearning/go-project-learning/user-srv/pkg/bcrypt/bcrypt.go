package bcrypt

import "golang.org/x/crypto/bcrypt"

// HashPassword 密码加密
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 校验密码
func CheckPassword(hashPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashPassword), []byte(password))
	return err == nil
}
