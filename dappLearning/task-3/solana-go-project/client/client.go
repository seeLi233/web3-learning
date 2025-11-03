package client

import (
	"context"
	"log"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

func ConnectDevnet() *rpc.Client {
	// Solana Devnet 测试网节点
	rpcURL := rpc.DevNet_RPC
	client := rpc.New(rpcURL)

	return client
}

func NewDevWsClient() *ws.Client {
	rpcURL := rpc.DevNet_RPC
	wsClient, err := ws.Connect(context.Background(), rpcURL)
	if err != nil {
		log.Fatalf("WSClient 创建失败: %v", err)
	}

	return wsClient
}
