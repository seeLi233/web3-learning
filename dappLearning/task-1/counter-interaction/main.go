package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/web3-learning/counter-interaction/internal/contract"
)

func main() {

	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("加载.env文件失败: %v", err)
	}

	privateKeyStr := os.Getenv("PRIVATE_KEY")
	rpcURL := os.Getenv("SEPOLIA_RPC")
	if privateKeyStr == "" || rpcURL == "" {
		log.Fatalf("请在.env文件中设置 Privete_key 和 Sepolia_RPC")
	}

	//rpcURL := flag.String("api", "", "Infura Sepolia API地址")
	//privKey := flag.String("key", "", "私钥")
	// 1. 连接 Sepolia 测试网 RPC 节点
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("无法连接到 RPC 节点: %v", err)
	}
	defer client.Close()
	fmt.Println("已经连接到 Sepolia 测试网")

	// 2.加载私钥(用于签名交易)
	privateKey, err := crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		log.Fatalf("私钥解析失败: %v", err)
	}

	//publickey := privateKey.Public().(*ecdsa.PublicKey)
	//fromAddr := crypto.PubkeyToAddress(*publickey)

	//nonce, err := client.PendingNonceAt(context.Background(), fromAddr)

	// 3.准备交易发送者信息 (包括链 ID、 gas 价格等)
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("获取链 ID 失败: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("创建交易签名失败: %v", err)
	}
	//auth.Nonce = big.NewInt(int64(nonce))

	// 设置 gas 参数 (可根据网络情况调整)
	auth.GasLimit = 300000
	auth.GasPrice, err = client.SuggestGasPrice(context.Background()) // 自动获取建议 gas 价格
	if err != nil {
		log.Fatalf("获取 gas 价格失败: %v", err)
	}

	// 4.部署合约
	//var contractAddr common.Address
	//var txHash types.Transaction

	contractAddr, txHash, _, err := contract.DeployContract(auth, client)
	if err != nil {
		log.Fatalf("合约部署失败: %v", err)
	}
	fmt.Printf("合约部署交易已发送, 哈希: %s\n", txHash.Hash().Hex())
	fmt.Printf("等待部署确认, 合约地址(部署后有效): %s\n", contractAddr.Hex())

	// 等待部署交易上链确认(约 10-30 秒, 视网络情况)
	_, err = bind.WaitMined(context.Background(), client, txHash)
	if err != nil {
		log.Fatalf("等待部署交易确认失败: %v", err)
	}
	fmt.Println("合约部署成功!")

	// 7.实例化合约
	// contractAddr := common.HexToAddress("0xabbbE3ec0C7B2bCB21642674a7726125084f2806") // 激活已有部署的合约地址
	counterInstance, err := contract.NewContract(contractAddr, client)
	if err != nil {
		log.Fatalf("实例化合约失败: %v", err)
	}

	// 8.调用 Increment 方法 (增加计数)
	tx, err := counterInstance.Increment(auth)
	if err != nil {
		log.Fatalf("调用 Increment 失败: %v", err)
	}
	fmt.Printf("发送 Increment 交易, 哈希: %s\n", tx.Hash().Hex())
	_, err = bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Fatalf("等待 Increment 确认失败: %v", err)
	}

	// 9.查询当前计数 (只读操作，无需 gas)
	count, err := counterInstance.Count(&bind.CallOpts{})
	if err != nil {
		log.Fatalf("查询计数失败: %v", err)
	}
	fmt.Printf("当前计数: %d\n", count)

	// 10.调用 Decrement 方法 (减少计数)
	tx, err = counterInstance.Decrement(auth)
	if err != nil {
		log.Fatalf("调用 Decrement 失败: %v", err)
	}
	fmt.Printf("发送 Decrement 交易, 哈希: %s\n", tx.Hash().Hex())
	_, err = bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Fatalf("等待 Decrement 确认失败: %v", err)
	}

	// 11.再次查询计数
	count, err = counterInstance.Count(&bind.CallOpts{})
	if err != nil {
		log.Fatalf("再次查询计数失败: %v", err)
	}
	fmt.Printf("Decrement 后计数: %d\n", count)
}
