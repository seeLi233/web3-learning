package rpc_client

import (
	"fmt"
	"log"

	"github.com/go-project-learning/project/gateway-api/config"
	"github.com/go-project-learning/project/gateway-api/global"
	pb "github.com/go-project-learning/project/user-srv/api/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitRiskClient() {
	cfg := config.Conf.UserSrv
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// 连接到 user-srv（同一个 gRPC 服务）
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接 OAuth 服务失败: %v", err)
		return
	}
	global.RiskClient = pb.NewRiskServiceClient(conn)
}
