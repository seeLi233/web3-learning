package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/web3-learning/sepolia-interaction/client"
)

// - 准备一个 Sepolia 测试网络的以太坊账户，并获取其私钥。
// - 编写 Go 代码，使用 ethclient 连接到 Sepolia 测试网络。
// - 构造一笔简单的以太币转账交易，指定发送方、接收方和转账金额。
// - 对交易进行签名，并将签名后的交易发送到网络。
// - 输出交易的哈希值。

func main() {
	// 通过命令行参数接收必要信息 (避免硬编码敏感信息)
	apiURL := flag.String("api", "", "Infura Sepolia API 地址(必填)")
	privateKey := flag.String("key", "", "发送方私钥 (64位十六进制, 无 0x 前缀， 必填)")
	toAddr := flag.String("to", "", "接收方地址 (带 0x 前缀， 必填)")
	amountWei := flag.String("amount", "1000000000000000", "转账金额 (单位: wei, 默认0.001 ETH)")
	flag.Parse()

	// 校验参数
	if *apiURL == "" || *privateKey == "" || *toAddr == "" {
		log.Fatal("请指定 API 地址、私钥和接收地址，例如: go run main.go -api https://... -key 你的私钥 -to 接收地址")
	}

	// 连接区块链
	client := client.ConnectSepolia(*apiURL)
	defer client.Close()

	// 解析私钥
	privKey, err := crypto.HexToECDSA(*privateKey)
	if err != nil {
		log.Fatalf("私钥解析失败 (确保是64位十六进制): %v", err)
	}

	// 推到发送方地址
	publickey := privKey.Public().(*ecdsa.PublicKey)
	fromAddr := crypto.PubkeyToAddress(*publickey)
	log.Printf("发送方地址: %s", fromAddr.Hex())

	// 解析接收方地址
	toAddress := common.HexToAddress(*toAddr)
	//if !toAddress.IsValid() {
	//	log.Fatal("接收方地址无效 (需带 0x 前缀)")
	//}

	// 解析转账金额 (wei)
	amount := new(big.Int)
	amount.SetString(*amountWei, 10) // 从字符串解析为 big.Int
	if amount.Cmp(big.NewInt(0)) <= 0 {
		log.Fatal("转正金额必须大于 0")
	}

	// 构建并发送交易
	txHash, err := sendTransaction(client, privKey, fromAddr, toAddress, amount)
	if err != nil {
		log.Fatalf("交易发送失败: %v", err)
	}

	// 输出结果
	fmt.Printf("\n 交易已提交，哈希: %d\n", &txHash)
	fmt.Printf("可在 Etherscan 查看: https://sepolia.etherscan.io/tx/%d\n", &txHash)
}

func sendTransaction(client *ethclient.Client, privateKey *ecdsa.PrivateKey, from common.Address, to common.Address, amount *big.Int) (string, error) {
	// 1.获取发送方 nonce (未确认交易计数)
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		return "", fmt.Errorf("获取 nonce 失败: %w", err)
	}

	// 2.获取建议的 Gas 价格
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", fmt.Errorf("获取 Gas 价格失败: %w", err)
	}

	// 3.固定 Gas Limit (简单转账为 21000)
	gasLimit := uint64(21000)

	// 4.构造交易 (LegacyTx 兼容大多数节点)
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     nil, // 无附加数据
	})

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// 5.签名交易
	//chainID := big.NewInt(11155111)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("交易签名失败: %w", err)
	}

	// 6.发送交易到网络
	if err := client.SendTransaction(context.Background(), signedTx); err != nil {
		return "", fmt.Errorf("提交交易失败: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}
