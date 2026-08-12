package cookie

import (
	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/gateway-api/config"
)

// SetCookie 设置 Cookie（自动读取配置的 domain/path/secure）
func SetCookie(c *gin.Context, name, value string, maxAge int) {
	cfg := config.Conf.JWT
	c.SetCookie(
		name,
		value,
		maxAge,
		cfg.CookiePath,   // path
		cfg.CookieDomain, // domain，如 .example.com
		cfg.CookieSecure, // secure
		true,             // httpOnly - 防止 JS 读取
	)
}

// GetCookie 读取 Cookie
func GetCookie(c *gin.Context, name string) (string, error) {
	return c.Cookie(name)
}

// ClearCookie 清除 Cookie
func ClearCookie(c *gin.Context, name string) {
	cfg := config.Conf.JWT
	c.SetCookie(
		name,
		"", // value 置空
		-1, // maxAge -1 表立删除
		cfg.CookiePath,
		cfg.CookieDomain,
		cfg.CookieSecure,
		true,
	)
}

// SetCookieWithDomain 指定域名设置 SSO Cookie（用于授权中心）
func SetCookieWithDomain(c *gin.Context, name, value, domain string, maxAge int) {
	c.SetCookie(
		name,
		value,
		maxAge,
		"/",
		domain,
		false, // 生产环境改为 true
		true,
	)
}
