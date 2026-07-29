package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	rpcURL := os.Getenv("SEPOLIA_RPC_URL")
	if rpcURL == "" {
		log.Fatal("❌ 请设置环境变量 SEPOLIA_RPC_URL")
	}

	// 连接节点
	fmt.Println("正在连接 Sepolia 节点...")
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("❌ 连接失败: %v", err)
	}
	defer client.Close()
	fmt.Println("✅ 连接成功！")

	ctx := context.Background()

	// ==================== 1. 查最新区块 ====================
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("❌ 获取区块头失败: %v", err)
	}
	chainID, _ := client.ChainID(ctx)
	fmt.Printf("\n链 ID: %s | 最新区块: %d\n", chainID, header.Number.Uint64())

	// ==================== 2. 查 ETH 余额 ====================
	// Sepolia 上的一个活跃地址（Vitalik 的 Sepolia 地址，用于测试）
	addr := common.HexToAddress("0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")

	balance, err := client.BalanceAt(ctx, addr, nil) // nil = latest block
	if err != nil {
		log.Fatalf("❌ 查余额失败: %v", err)
	}
	fmt.Printf("\n📧 地址: %s\n", addr.Hex())
	fmt.Printf("💰 ETH 余额: %s ETH\n", weiToEther(balance))

	// ==================== 3. 判断是 EOA 还是合约 ====================
	code, err := client.CodeAt(ctx, addr, nil)
	if err != nil {
		log.Printf("查 CodeAt 失败: %v", err)
	} else {
		if len(code) > 0 {
			fmt.Printf("📄 类型: 合约 (bytecode %d bytes)\n", len(code))
		} else {
			fmt.Printf("👤 类型: EOA (外部账户)\n")
		}
	}

	// ==================== 4. 用一个合约地址对比 ====================
	// Sepolia 上的 Uniswap V2 Router（这是个合约地址，肯定有 bytecode）
	contractAddr := common.HexToAddress("0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008")
	code2, err := client.CodeAt(ctx, contractAddr, nil)
	if err != nil {
		log.Printf("查合约 CodeAt 失败: %v", err)
	} else {
		fmt.Printf("\n📧 地址: %s\n", contractAddr.Hex())
		if len(code2) > 0 {
			fmt.Printf("📄 类型: 合约 (bytecode %d bytes)\n", len(code2))
		}
	}

	// ==================== 5. 查一笔最近的交易 ====================
	// 从最新区块里取第一笔交易
	block, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("❌ 获取区块失败: %v", err)
	}

	if len(block.Transactions()) > 0 {
		tx := block.Transactions()[0]
		fmt.Printf("\n========== 最新区块中的第一笔交易 ==========\n")
		fmt.Printf("Tx Hash:   %s\n", tx.Hash().Hex())
		fmt.Printf("Nonce:     %d\n", tx.Nonce())

		// 交易发起者（需要从签名恢复，Ethers.js 自动做，go-ethereum 需要 receipt）
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err == nil {
			fmt.Printf("状态:      %s\n", txStatus(receipt.Status))
			fmt.Printf("区块号:    %d\n", receipt.BlockNumber)
			fmt.Printf("Gas Used:  %d\n", receipt.GasUsed)
			fmt.Printf("日志数量:  %d\n", len(receipt.Logs))
		}

		if tx.To() != nil {
			fmt.Printf("To:        %s\n", tx.To().Hex())
		} else {
			fmt.Printf("To:        (合约创建)\n")
		}
		fmt.Printf("Value:     %s ETH\n", weiToEther(tx.Value()))

		// Gas 信息
		fmt.Printf("Gas Limit: %d\n", tx.Gas())
		switch tx.Type() {
		case 0:
			fmt.Printf("类型:      Legacy (固定 gasPrice)\n")
			fmt.Printf("Gas Price: %d Gwei\n", new(big.Int).Div(tx.GasPrice(), big.NewInt(1e9)))
		case 2:
			fmt.Printf("类型:      EIP-1559 (动态费用)\n")
			fmt.Printf("GasTipCap: %d Gwei\n", new(big.Int).Div(tx.GasTipCap(), big.NewInt(1e9)))
			fmt.Printf("GasFeeCap: %d Gwei\n", new(big.Int).Div(tx.GasFeeCap(), big.NewInt(1e9)))
		}
		fmt.Printf("Data 长度: %d bytes\n", len(tx.Data()))
		fmt.Println("==============================================")
	} else {
		fmt.Println("\n⚠️ 最新区块中没有交易（可能是空块）")
	}
	// // 查网络 ID
	// networkID, err := client.NetworkID(ctx)
	// if err != nil {
	// 	log.Fatalf("❌ 获取 NetworkID 失败: %v", err)
	// }
	// fmt.Printf("Network ID: %s\n", networkID.String())

	// // 查链 ID
	// chainID, err := client.ChainID(ctx)
	// if err != nil {
	// 	log.Fatalf("❌ 获取 ChainID 失败: %v", err)
	// }
	// fmt.Printf("Chain ID:  %s (Sepolia = 11155111)\n", chainID.String())

	// // 查最新区块
	// header, err := client.HeaderByNumber(ctx, nil) // nil = 最新区块
	// if err != nil {
	// 	log.Fatalf("❌ 获取区块头失败: %v", err)
	// }

	// fmt.Println()
	// fmt.Println("========== 最新区块信息 ==========")
	// fmt.Printf("区块号:   %d\n", header.Number.Uint64())
	// fmt.Printf("区块哈希: %s\n", header.Hash().Hex())
	// fmt.Printf("时间戳:   %d\n", header.Time)
	// fmt.Printf("Gas Limit: %d\n", header.GasLimit)
	// fmt.Printf("Base Fee:  %s wei\n", header.BaseFee.String())
	// fmt.Println("==================================")
}

// ==================== 工具函数 ====================

func weiToEther(wei *big.Int) string {
	f := new(big.Float).SetInt(wei)
	result := new(big.Float).Quo(f, big.NewFloat(1e18))
	return result.Text('f', 6)
}

func txStatus(status uint64) string {
	if status == 1 {
		return "✅ 成功"
	}
	return "❌ 失败(Reverted)"
}
