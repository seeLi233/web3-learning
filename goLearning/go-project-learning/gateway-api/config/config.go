package config

import (
	"fmt"
	"log"
	"os"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/spf13/viper"
)

type AppConfig struct {
	Server  ServerConfig      `mapstructure:"server"`
	UserSrv UserSrvConfig     `mapstructure:"user_srv"`
	Redis   config.RedisConfig `mapstructure:"redis"`
	JWT     JWTConfig         `mapstructure:"jwt"`
	GitHub  GitHubConfig      `mapstructure:"github"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type UserSrvConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type JWTConfig struct {
	Secret       string `mapstructure:"secret"`
	CookieDomain string `mapstructure:"cookie_domain"` // 父域名，如 .example.com
	CookiePath   string `mapstructure:"cookie_path"`
	CookieSecure bool   `mapstructure:"cookie_secure"`
}
type GitHubConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURI  string `mapstructure:"redirect_uri"`
}

var Conf *AppConfig

func Init() {
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}

	viper.AddConfigPath("./configs")
	viper.SetConfigName("config." + env)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}
	if err := viper.Unmarshal(&Conf); err != nil {
		log.Fatalf("解析配置失败: %v", err)
	}

	fmt.Printf("gateway-api 加载环境: %s\n", env)
}
