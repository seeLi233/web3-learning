package rpc_client

import (
	"fmt"

	"github.com/go-project-learning/project/common/pkg/logger"
	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/global"
	"github.com/go-project-learning/project/user-srv/api/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitAddressClient() {
	cfg := config.Conf.UserSrv
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal("连接地址服务失败", zap.Error(err))
	}

	global.AddressClient = pb.NewAddressServiceClient(conn)
	logger.Info("地址服务连接成功")
}
