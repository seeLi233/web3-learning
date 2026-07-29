package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 最小 ERC20 ABI（只包含我们需要调用的 view 函数）
const erc20ViewABI = `[
    {
        "name": "name",
        "type": "function",
        "stateMutability": "view",
        "inputs": [],
        "outputs": [{"name": "", "type": "string"}]
    },
    {
        "name": "symbol",
        "type": "function",
        "stateMutability": "view",
        "inputs": [],
        "outputs": [{"name": "", "type": "string"}]
    },
    {
        "name": "decimals",
        "type": "function",
        "stateMutability": "view",
        "inputs": [],
        "outputs": [{"name": "", "type": "uint8"}]
    },
    {
        "name": "totalSupply",
        "type": "function",
        "stateMutability": "view",
        "inputs": [],
        "outputs": [{"name": "", "type": "uint256"}]
    },
    {
        "name": "balanceOf",
        "type": "function",
        "stateMutability": "view",
        "inputs": [{"name": "account", "type": "address"}],
        "outputs": [{"name": "", "type": "uint256"}]
    }
]`

func main() {
	rpcURL := os.Getenv("SEPOLIA_RPC_URL")
	if rpcURL == "" {
		log.Fatal("❌ 请设置环境变量 SEPOLIA_RPC_URL")
	}

	// 1. 连接节点
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("❌ 连接失败: %v", err)
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(erc20ViewABI))
	if err != nil {
		log.Fatalf("❌ ABI 解析失败: %v", err)
	}

	ctx := context.Background()

	// ==================== 选择目标合约 ====================
	// Sepolia 测试网上的 ERC20 合约地址
	// 方案A: Sepolia USDC (模拟)
	tokenAddr := common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")
	// 方案B: 如果上面那个不行，换成 Sepolia LINK
	// tokenAddr := common.HexToAddress("0x779877A7B0D9E8603169DdbD7836e478b4624789")

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║    eth_call 调用合约只读方法              ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("\n📄 合约地址: %s\n", tokenAddr.Hex())

	// ==================== A. 调用无参函数 name() ====================
	fmt.Println("\n━━━ A. 调用 name() ━━━")

	name := callViewString(ctx, client, parsedABI, tokenAddr, "name")
	fmt.Printf("  ✅ name() = \"%s\"\n", name)

	// ==================== B. 调用无参函数 symbol() ====================
	fmt.Println("\n━━━ B. 调用 symbol() ━━━")

	symbol := callViewString(ctx, client, parsedABI, tokenAddr, "symbol")
	fmt.Printf("  ✅ symbol() = \"%s\"\n", symbol)

	// ==================== C. 调用 decimals() ====================
	fmt.Println("\n━━━ C. 调用 decimals() ━━━")

	decimals := callViewUint8(ctx, client, parsedABI, tokenAddr, "decimals")
	fmt.Printf("  ✅ decimals() = %d\n", decimals)

	// ==================== D. 调用 totalSupply() ====================
	fmt.Println("\n━━━ D. 调用 totalSupply() ━━━")

	supply := callViewBigInt(ctx, client, parsedABI, tokenAddr, "totalSupply")
	fmt.Printf("  ✅ totalSupply() = %s (原始 wei)\n", supply.String())
	// 按 decimals 格式化
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	human := new(big.Float).Quo(
		new(big.Float).SetInt(supply),
		new(big.Float).SetInt(divisor),
	)
	fmt.Printf("  📊 格式化后   = %s %s\n", human.Text('f', 2), symbol)

	// ==================== E. 调用带参函数 balanceOf(address) ====================
	fmt.Println("\n━━━ E. 调用 balanceOf(address) ━━━")

	// 查几个地址的余额
	addrs := []struct {
		label string
		addr  common.Address
	}{
		{"Vitalik (Sepolia)", common.HexToAddress("0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")},
		{"Uniswap Router", common.HexToAddress("0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008")},
	}

	for _, a := range addrs {
		bal := callViewBigIntWithArg(ctx, client, parsedABI, tokenAddr, "balanceOf", a.addr)
		humanBal := new(big.Float).Quo(
			new(big.Float).SetInt(bal),
			new(big.Float).SetInt(divisor),
		)
		fmt.Printf("  %s: %s %s\n", a.label, humanBal.Text('f', 6), symbol)
	}

	fmt.Println("\n✅ 合约调用演示完成！")

	// ==================== F. 底层原理演示 ====================
	fmt.Println("\n━━━ F. 底层原理：手动 eth_call ━━━")
	fmt.Println("(演示 balanceOf 的完整调用链路)")

	userAddr := common.HexToAddress("0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045")

	// F1: ABI Pack → calldata
	calldata, err := parsedABI.Pack("balanceOf", userAddr)
	if err != nil {
		log.Fatalf("Pack 失败: %v", err)
	}
	fmt.Printf("\n  ① ABI.Pack(\"balanceOf\", 0xd8dA...) → calldata\n")
	fmt.Printf("     0x%x\n", calldata)

	// F2: 构造 CallMsg
	msg := ethereum.CallMsg{
		To:   &tokenAddr,
		Data: calldata,
	}
	fmt.Printf("\n  ② 构造 CallMsg{To: %s, Data: calldata}\n", tokenAddr.Hex())

	// F3: 执行 eth_call
	result, err := client.CallContract(ctx, msg, nil) // nil = latest block
	if err != nil {
		log.Printf("  eth_call 失败 (该合约可能不存在于 Sepolia): %v", err)
		fmt.Println("  💡 提示: 换一个 Sepolia ERC20 地址试试")
	} else {
		fmt.Printf("\n  ③ eth_call → 返回原始数据\n")
		fmt.Printf("     0x%x\n", result)

		// F4: ABI Unpack → Go 类型
		var balance *big.Int
		err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
		if err != nil {
			log.Fatalf("Unpack 失败: %v", err)
		}
		fmt.Printf("\n  ④ ABI.UnpackIntoInterface → *big.Int\n")
		fmt.Printf("     balance = %s\n", balance.String())
	}
}

// ==================== 工具函数 ====================

// callViewString 调用返回 string 的无参 view 函数
func callViewString(ctx context.Context, client *ethclient.Client, parsedABI abi.ABI, contractAddr common.Address, method string) string {

	calldata, err := parsedABI.Pack(method)
	if err != nil {
		log.Fatalf("Pack(%s) 失败: %v", method, err)
	}

	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &contractAddr,
		Data: calldata,
	}, nil)
	if err != nil {
		log.Fatalf("CallContract(%s) 失败: %v", method, err)
	}

	var out string
	err = parsedABI.UnpackIntoInterface(&out, method, result)
	if err != nil {
		log.Fatalf("Unpack(%s) 失败: %v", method, err)
	}
	return out
}

// callViewUint8 调用返回 uint8 的无参 view 函数
func callViewUint8(ctx context.Context, client *ethclient.Client, parsedABI abi.ABI, contractAddr common.Address, method string) uint8 {
	calldata, _ := parsedABI.Pack(method)
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &contractAddr,
		Data: calldata,
	}, nil)
	if err != nil {
		log.Fatalf("CallContract(%s) 失败: %v", method, err)
	}

	var out uint8
	parsedABI.UnpackIntoInterface(&out, method, result)
	return out
}

// callViewBigInt 调用返回 uint256 的无参 view 函数
func callViewBigInt(ctx context.Context, client *ethclient.Client, parsedABI abi.ABI, contractAddr common.Address, method string) *big.Int {
	calldata, _ := parsedABI.Pack(method)
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &contractAddr,
		Data: calldata,
	}, nil)
	if err != nil {
		log.Fatalf("CallContract(%s) 失败: %v", method, err)
	}

	var out *big.Int
	parsedABI.UnpackIntoInterface(&out, method, result)
	return out
}

// callViewBigIntWithArg 调用返回 uint256 的单参 view 函数
func callViewBigIntWithArg(ctx context.Context, client *ethclient.Client, parsedABI abi.ABI, contractsAddr common.Address, method string, arg common.Address) *big.Int {
	calldata, _ := parsedABI.Pack(method, arg)
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &contractsAddr,
		Data: calldata,
	}, nil)
	if err != nil {
		log.Fatalf("CallContract(%s) 失败: %v", method, err)
	}

	var out *big.Int
	parsedABI.UnpackIntoInterface(&out, method, result)
	return out
}
