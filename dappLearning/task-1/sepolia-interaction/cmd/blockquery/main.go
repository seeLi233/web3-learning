package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/web3-learning/sepolia-interaction/client"
)

// - 编写 Go 代码，使用 ethclient 连接到 Sepolia 测试网络。
// - 实现查询指定区块号的区块信息，包括区块的哈希、时间戳、交易数量等。
// - 输出查询结果到控制台。

func main() {
	// 通过命令行参数接收 Infura API 地址和区块
	apiURL := flag.String("api", "", "Infura Sepolia API地址(必填)")
	blockNum := flag.Uint64("num", 0, "查询的区块号(必填, 如1000000)")
	flag.Parse()

	// 校验参数
	if *apiURL == "." || *blockNum == 0 {
		log.Fatal("请指定 API 地址和区块号, 例如: go run main.go -https://... -num 10000000")
	}

	// 连接区块链
	client := client.ConnectSepolia(*apiURL)
	defer client.Close()

	// 查询区块
	block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(*blockNum)))
	if err != nil {
		log.Fatalf("查询区块失败: %v", err)
	}

	// 输出区块详情
	printBlockInfo(block)
}

// 打印区块信息
func printBlockInfo(block *types.Block) {
	fmt.Println("\n=====区块信息=====")
	fmt.Printf("区块号: %d\n", block.Number().Uint64())
	fmt.Printf("区块号哈希: %s\n", block.Hash().Hex())
	fmt.Printf("父区块哈希: %s\n", block.ParentHash().Hex())
	fmt.Printf("时间戳: %d (Unix 时间, 可转换为北京时间)\n", block.Time())
	fmt.Printf("交易数量: %d\n", len(block.Transactions()))
	fmt.Printf("矿工地址: %s\n", block.Coinbase().Hex())
	fmt.Printf("区块大小: %d bytes\n", block.Size())
	fmt.Println("==========")
}
