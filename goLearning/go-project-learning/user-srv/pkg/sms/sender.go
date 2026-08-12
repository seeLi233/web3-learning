package sms

import (
	"fmt"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/logger"
)

// Sender 短信发送接口
type Sender interface {
	Send(phone, code string) error
}

// DevSender 开发环境实现 （只打印日志，不发真消息）
type DevSender struct{}

func (d *DevSender) Send(phone, code string) error {
	// 开发环境直接打印验证码， 不用调用真实 API
	logger.Info(fmt.Sprintf("[SMS-DEV] 向 %s 发送验证码: %s", phone, code))
	return nil
}

// AliyunSender 阿里云短信实现 （后续对接用）
type AliyunSender struct {
	AccessKey string
	SecretKey string
	SignName  string
	Template  string
}

func (a *AliyunSender) Send(phone, code string) error {
	// TODO：对接阿里云短信 SDK
	// github.com/aliyun/alibabacloud-sdk-go/services/dysmsapi-20170525
	return fmt.Errorf("阿里云短信暂未实现")
}

// NewSender 根据配置创建对应的 Sender
func NewSender(cfg config.SMSConfig) Sender {
	switch cfg.Provider {
	case "aliyun":
		return &AliyunSender{
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
			SignName:  cfg.SignName,
			Template:  cfg.Template,
		}
	default:
		return &DevSender{}
	}
}
