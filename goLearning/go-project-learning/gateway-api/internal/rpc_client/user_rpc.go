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

func InitUserClient() {
	cfg := config.Conf.UserSrv
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接 user-srv 失败: %v", err)
		return
	}

	global.UserClient = pb.NewUserServiceClient(conn)
	log.Printf("已连接 user-srv: %s", addr)
}
