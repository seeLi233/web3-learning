package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	rpcURL := os.Getenv("SEPOLIA_RPC_URL")
	if rpcURL == "" {
		log.Fatal("❌ 请设置环境变量 SEPOLIA_RPC_URL")
	}
	privateKeyHex := os.Getenv("PRIVATE_KEY") //  0x 开头
	if privateKeyHex == "" {
		log.Fatal("❌ 请设置环境变量 PRIVATE_KEY（测试网私钥，0x开头）")
	}
	toAddr := os.Getenv("TO_ADDRESS") // 收款地址，可选
	if toAddr == "" {
		toAddr = "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb7" // 默认测试地址
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║    发送 ETH 转账（EIP-1559）              ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	// ==================== Step 1: 连接 + 恢复地址 ====================
	fmt.Println("\n━━━ Step 1: 连接节点 + 从私钥恢复地址 ━━━")

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("❌ 连接失败: %v", err)
	}
	defer client.Close()

	// 去掉 0x 前缀，解析私钥
	privateKey, err := crypto.HexToECDSA(privateKeyHex[2:])
	if err != nil {
		log.Fatalf("❌ 私钥解析失败: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("❌ 公钥类型转换失败")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	fmt.Printf("  🧑 发送地址: %s\n", fromAddress.Hex())

	ctx := context.Background()

	// ==================== Step 2: 查余额 + nonce ====================
	fmt.Println("\n━━━ Step 2: 查余额 + nonce ━━━")

	balance, err := client.BalanceAt(ctx, fromAddress, nil)
	if err != nil {
		log.Fatalf("❌ 查余额失败: %v", err)
	}
	fmt.Printf("  💰 ETH 余额: %s ETH\n", weiToEther(balance))

	nonce, err := client.NonceAt(ctx, fromAddress, nil) // nil = 用 latest，非 pending
	if err != nil {
		log.Fatalf("❌ 查 nonce 失败: %v", err)
	}
	fmt.Printf("  🔢 Nonce:    %d\n", nonce)

	// ==================== Step 3: 获取费用参数 ====================
	fmt.Println("\n━━━ Step 3: 获取 EIP-1559 费用参数 ━━━")

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("❌ 查链ID失败: %v", err)
	}
	fmt.Printf("  ⛓️  Chain ID: %s\n", chainID.String())

	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("❌ 查区块头失败: %v", err)
	}

	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatalf("❌ 查建议小费失败: %v", err)
	}

	// maxFeePerGas = baseFee * 2 + tip（防止下一个区块 baseFee 涨了导致交易卡住）
	// 2 倍是安全经验值，花不完的会退款
	maxFeePerGas := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), tip)

	fmt.Printf("  🔥 Base Fee:      %s Gwei\n", weiToGwei(header.BaseFee))
	fmt.Printf("  💸 Tip (小费):    %s Gwei\n", weiToGwei(tip))
	fmt.Printf("  🏷️  MaxFeePerGas:  %s Gwei\n", weiToGwei(maxFeePerGas))

	// ==================== Step 4: 构造交易 ====================
	fmt.Println("\n━━━ Step 4: 构造 EIP-1559 交易 ━━━")

	to := common.HexToAddress(toAddr)
	value := big.NewInt(100000000000000) // 0.0001 ETH

	// 先估算 gas limit，再创建 tx（因为 Transaction 是不可变对象，没有 SetGas 方法）
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    &to,
		Value: value,
	})
	if err != nil {
		// EstimateGas 失败时用 ETH 转账的硬编码值
		fmt.Printf("  ⚠️  EstimateGas 失败，使用默认值 21000: %v\n", err)
		gasLimit = 21000
	} else {
		fmt.Printf("  ⛽ 估算 Gas: %d\n", gasLimit)
		gasLimit = gasLimit * 110 / 100 // +10% buffer
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasFeeCap: maxFeePerGas,
		GasTipCap: tip,
		Gas:       gasLimit, // ← 直接在创建时设置，没有 SetGas()
		To:        &to,
		Value:     value,
		Data:      nil, // 纯 ETH 转账不需要 data
	})

	fmt.Printf("  📤 收款地址: %s\n", to.Hex())
	fmt.Printf("  💵 转账金额: %s ETH (100000000000000 wei)\n", weiToEther(value))
	fmt.Printf("  ⛽ Gas Limit: %d\n", gasLimit)

	// 预估费用
	estFee := new(big.Int).Mul(maxFeePerGas, big.NewInt(int64(gasLimit)))
	fmt.Printf("  💰 预估最大费用: %s ETH\n", weiToEther(estFee))

	// ==================== Step 5: 签名交易 ====================
	fmt.Println("\n━━━ Step 5: 签名交易 ━━━")

	signer := types.LatestSignerForChainID(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		log.Fatalf("❌ 签名失败: %v", err)
	}

	fmt.Printf("  ✍️  已签名，Tx Hash: %s\n", signedTx.Hash().Hex())

	// 验证签名（从签名恢复发送者）
	recoveredAddr, err := types.Sender(signer, signedTx)
	if err != nil {
		log.Fatalf("❌ 恢复发送者失败: %v", err)
	}
	fmt.Printf("  ✅ 签名验证通过，发送者: %s\n", recoveredAddr.Hex())

	// ==================== Step 6: 广播 + 等待确认 ====================
	fmt.Println("\n━━━ Step 6: 广播交易 + 等待确认 ━━━")

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatalf("❌ 广播失败: %v", err)
	}

	fmt.Printf("  📡 交易已广播！\n")
	fmt.Printf("  🔗 查看: https://sepolia.etherscan.io/tx/%s\n", signedTx.Hash().Hex())
	fmt.Println("\n  ⏳ 等待确认中...")

	// 等待确认（最多等 60 秒)
	receipt, err := waitForTx(client, signedTx.Hash(), 30*time.Second)
	if err != nil {
		fmt.Printf("  ⚠️ 等待超时，交易可能还在 pending\n")
		fmt.Printf("  手动检查: https://sepolia.etherscan.io/tx/%s\n", signedTx.Hash().Hex())
	} else {
		fmt.Printf("\n  ✅ 交易已确认！\n")
		fmt.Printf("  区块号:   %d\n", receipt.BlockNumber)
		fmt.Printf("  Gas 实际消耗: %d\n", receipt.GasUsed)
		fmt.Printf("  状态:     %s\n", map[uint64]string{1: "✅ 成功", 0: "❌ Reverted"}[receipt.Status])

		// 有效 Gas 价格 = baseFee + min(tip, maxFeePerGas - baseFee)
		effectiveGasPrice := new(big.Int).Add(
			header.BaseFee,
			tx.GasTipCap(),
		)
		actualFee := new(big.Int).Mul(effectiveGasPrice, big.NewInt(int64(receipt.GasUsed)))
		fmt.Printf("  实际费用: %s ETH\n", weiToEther(actualFee))
	}

	fmt.Println("\n✅ 交易流程演示完成！")
}

// ==================== 工具函数 ====================

func weiToEther(wei *big.Int) string {
	f := new(big.Float).SetInt(wei)
	return new(big.Float).Quo(f, big.NewFloat(1e18)).Text('f', 6)
}

func weiToGwei(wei *big.Int) string {
	f := new(big.Float).SetInt(wei)
	return new(big.Float).Quo(f, big.NewFloat(1e9)).Text('f', 2)
}

// waitForTx 轮询等待交易确认
func waitForTx(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		receipt, err := client.TransactionReceipt(context.Background(), txHash)
		if err == nil {
			return receipt, nil
		}
		// "not found" 是正常情况，说明还没上链

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case <-ticker.C:
			fmt.Print(".")
		}
	}
}
