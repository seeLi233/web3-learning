package client

import (
	"log"

	"github.com/ethereum/go-ethereum/ethclient"
)

func ConnectSepolia(apiURL string) *ethclient.Client {
	client, err := ethclient.Dial(apiURL)
	if err != nil {
		log.Fatalf("连接 Sepolia 失败：%v", err)
	}
	log.Println("成功连接 Sepolia 测试网")
	return client
}
