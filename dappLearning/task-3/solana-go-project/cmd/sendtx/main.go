package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/web3-learning/solana-go-project/client"
)

func main() {
	client := client.ConnectDevnet()

	fromPrivKeyPath := "./config/wallet-keypair.json"
	privateKeyBytes, err := os.ReadFile(os.ExpandEnv(fromPrivKeyPath))
	if err != nil {
		log.Fatalf("读取私钥失败: %v", err)
	}

	var fromPrivKey solana.PrivateKey
	if err := json.Unmarshal(privateKeyBytes, &fromPrivKey); err != nil {
		log.Fatalf("解析私钥失败: %v", err)
	}

	walletAddr := fromPrivKey.PublicKey()

	balance, err := client.GetBalance(context.TODO(), walletAddr, rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("获取余额失败: %v \n", err)
	}

	fmt.Printf("钱包地址: %s \n", walletAddr.String())
	fmt.Printf("余额: %d SOL \n", balance.Value/1e9) // 1 Sol = 1e9 lamports

	// 转账
	toKey := flag.String("toKey", "7EfbSJzGZhjNJZQKZJK2BRNimjibywu6YPDJ8a68Gk3x", "转账钱包的地址")
	toAddr := solana.MustPublicKeyFromBase58(*toKey)

	// 获取最新区块哈希 (用于交易生效)
	recentBlock, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		log.Fatalf("获取最新区块哈希失败: %v", err)
	}

	// 转账金额
	lamports := uint64(1) * 1e9
	instruction := system.NewTransferInstruction(
		lamports,
		walletAddr,
		toAddr,
	).Build()

	tx, err := solana.NewTransaction([]solana.Instruction{instruction}, recentBlock.Value.Blockhash, solana.TransactionPayer(walletAddr))
	if err != nil {
		log.Fatalf("创建交易失败: %v", err)
	}

	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == walletAddr {
			return &fromPrivKey
		}
		return nil
	})

	sig, err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		log.Fatalf("发送交易失败: %v", err)
	}

	fmt.Printf("转账成功!交易签名: %s", sig.String())
}
