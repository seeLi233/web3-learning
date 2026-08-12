package consul

import (
	"fmt"
	"net"
	"strconv"

	"github.com/go-project-learning/project/common/pkg/config"
	"github.com/go-project-learning/project/common/pkg/logger"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

var ConsulClient *api.Client

func InitConsulService(cfg config.ConsulServerConfig) {
	configs := api.DefaultConfig()
	configs.Address = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	client, err := api.NewClient(configs)
	if err != nil {
		logger.Error("创建 Consul 客户端失败", zap.Error(err))
		return
	}

	ConsulClient = client
}

func RegisterService(serverName, hostPort string) error {
	// 拆分 IP 和 端口
	ip, portStr, err := net.SplitHostPort(hostPort)

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}

	// 组装注册信息
	reg := &api.AgentServiceRegistration{
		Name:    serverName,
		Address: ip,
		Port:    port,
		// 健康检查：定期探测服务是否存活
		Check: &api.AgentServiceCheck{
			TCP:                            "host.docker.internal:" + portStr,
			Interval:                       "10s", // 每 10s 检查一次
			Timeout:                        "3s",  // 超时 3s
			DeregisterCriticalServiceAfter: "30s", // 异常 30s 后自动注销
		},
	}

	ConsulClient.Agent().ServiceRegister(reg)
	return nil
}
