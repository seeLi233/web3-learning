package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go/rpc"
	"github.com/web3-learning/solana-go-project/client"
)

func main() {

	client := client.ConnectDevnet()

	// 获取区块信息
	blockhash, err := client.GetLatestBlockhash(
		context.TODO(),
		rpc.CommitmentFinalized,
	)
	if err != nil {
		log.Fatalf("获取区块哈希失败: %v", err)
	}

	slot := blockhash.Context.Slot
	var val uint64 = 0

	block, err := client.GetBlockWithOpts(context.Background(), slot, &rpc.GetBlockOpts{
		TransactionDetails:             rpc.TransactionDetailsFull,
		MaxSupportedTransactionVersion: &val,
	})
	if err != nil {
		log.Fatalf("获取区块失败: %v", err)
	}

	// 提取信息并输出
	fmt.Printf("Solana Devnet 区块信息 (Slot: %d) \n", slot)
	fmt.Printf("区块哈希: %s \n", block.Blockhash)

	// 时间戳可能为nil
	if block.BlockTime != nil {
		fmt.Printf("时间戳： %d (Unix时间) \n", *block.BlockTime)
	} else {
		fmt.Printf("时间戳: 无")
	}

	fmt.Printf("交易数量: %d \n", len(block.Transactions))

}
