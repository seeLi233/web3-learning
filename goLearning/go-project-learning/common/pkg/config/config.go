package config

import (
	"fmt"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var Conf *AppConfig
var IsDev bool = false

type AppConfig struct {
	Server     ServerConfig       `mapstructure:"server"`
	MySQL      MysqlConfig        `mapstructure:"mysql"`
	Redis      RedisConfig        `mapstructure:"redis"`
	RabbitMQ   RabbitMQConfig     `mapstructure:"rabbitmq"`
	Consul     ConsulServerConfig `mapstructure:"consul"`
	Jaeger     JaegerConfig       `mapstructure:"jaeger"`
	JWTConfig  JWTConfig          `mapstructure:"jwt"`
	SMSConfig  SMSConfig          `mapstructure:"sms"`
	SMTPConfig SMTPConfig         `mapstructure:"smtp"`
}

type MysqlConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Dbname   string `mapstructure:"dbname"`
	Charset  string `mapstructure:"charset"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	DB       int    `mapstructure:"db"`
	Password string `mapstructure:"password"`
	PoolSize int    `mapstructure:"pool_size"`
}

type RabbitMQConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type ServerConfig struct {
	Name     string `mapstructure:"name"`
	Port     int    `mapstructure:"port"`
	LogLevel string `mapstructure:"log-level"`
}

type ConsulServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type JaegerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	AccessExpire  int    `mapstructure:"access_expire"`
	RefreshExpire int    `mapstructure:"refresh_expire"`
	Issuer        string `mapstructure:"issuer"`
}

type SMSConfig struct {
	Provider  string `mapstructure:"provider"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	SignName  string `mapstructure:"sign_name"`
	Template  string `mapstructure:"template"`
}

type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

func Init() {
	// 1.环境变量 ENV 区分环境
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}

	if env == "dev" {
		IsDev = true
	} else {
		IsDev = false
	}

	// 2.设置配置文件路径、名称、类型
	viper.AddConfigPath("./configs")
	viper.SetConfigName("config." + env)
	viper.SetConfigType("yaml")

	// 3.开启[环境变量自动覆盖]
	viper.AutomaticEnv()
	// 映射自定环境变量名绑定配置字段
	// _ = viper.BindEnv("app.port", "APP_PORT")
	// _ = viper.BindEnv("mysql.pwd", "MYSQL_PWD")

	// 4.读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	// 5.解析到结构体
	if err := viper.Unmarshal(&Conf); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	// 6.开启[热加载]监听配置文件修改
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("配置文件已修改，自动重载中...")
		_ = viper.Unmarshal(&Conf)
	})

	fmt.Printf("当前加载环境: %s\n", env)
}
