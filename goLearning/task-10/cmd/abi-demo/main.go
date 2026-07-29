package main

import (
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const erc20ABI = `[
	{
		"name": "transfer",
		"type": "function",
		"inputs": [
			{ "name": "to", "type": "address"},
			{ "name": "amount", "type": "uint256"}
		],
		"outputs": [{ "name": "", "type": "bool"}]
	},
	{
		"name": "balanceOf",
		"type": "function",
		"inputs": [{ "name": "account", "type": "address"}],
		"outputs": [{ "name": "", "type": "uint256"}]
	},
	{
		"name": "Transfer",
		"type": "event",
		"inputs": [
			{ "name": "from", "type": "address", "indexed": true},
			{ "name": "to", "type": "address", "indexed": true},
			{ "name": "value", "type": "uint256", "indexed": false}
		]
	}
]`

func main() {
	// 1. 解析 ABI JSON
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		log.Fatalf("ABI 解析失败: %v", err)
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     ABI 编码解码实战                      ║")
	fmt.Println("╚══════════════════════════════════════════╝")

	// ==================== Part A: 函数选择器 ====================
	fmt.Println("\n━━━ A. 函数选择器 (Function Selector) ━━━")

	// 手动计算 selector
	transferSig := "transfer(address, uint256)"
	transferHash := crypto.Keccak256Hash([]byte(transferSig))
	transferSelector := transferHash[:4]
	fmt.Printf("1. transfer(address,uint256)\n")
	fmt.Printf("   keccak256 = 0x%x\n", transferHash)
	fmt.Printf("   前4字节   = 0x%x  ← 这就是 selector\n", transferSelector)

	balanceSig := "balanceOf(address)"
	balanceHash := crypto.Keccak256Hash([]byte(balanceSig))
	balanceSelector := balanceHash[:4]
	fmt.Printf("\n2. balanceOf(address)\n")
	fmt.Printf("   keccak256 = 0x%x\n", balanceHash)
	fmt.Printf("   前4字节   = 0x%x  ← 这就是 selector\n", balanceSelector)

	// ==================== Part B: 静态参数编码 ====================
	fmt.Println("\n━━━ B. 静态参数编码 (transfer) ━━━")

	to := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb7")
	amount := big.NewInt(1000000000000000000) // 1 token (18 decimals)

	// 用 ABI Pack 编码
	calldata, err := parsedABI.Pack("transfer", to, amount)
	if err != nil {
		log.Fatalf("Pack 失败: %v", err)
	}

	fmt.Printf("transfer(0x742d..., 1000000000000000000)\n\n")
	fmt.Printf("完整 calldata: 0x%x\n\n", calldata)
	fmt.Printf("结构拆解:\n")
	fmt.Printf("  [0:4]   selector = 0x%x\n", calldata[0:4])
	fmt.Printf("  [4:36]  to       = 0x%x  (address 左补0到32字节)\n", calldata[4:36])
	fmt.Printf("  [36:68] amount   = 0x%x  (uint256 左补0到32字节)\n", calldata[36:68])

	// 验证
	fmt.Printf("\n验证:\n")
	fmt.Printf("  0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb7\n")
	fmt.Printf("  → 32字节填充后: 0x000000000000000000000000%s\n", to.Hex()[2:])

	// ==================== Part C: 动态参数编码（手动） ====================
	fmt.Println("\n━━━ C. 动态参数编码 (手动演示) ━━━")
	fmt.Println("假设函数: foo(uint256,string)  参数: foo(42, \"Hello\")")

	// keccak256("foo(uint256,string)")[:4]
	fooSig := "foo(uint256, string)"
	fooHash := crypto.Keccak256Hash([]byte(fooSig))
	fooSelector := fooHash[:4]
	fmt.Printf("\nselector = keccak256(\"%s\")[:4] = 0x%x\n", fooSig, fooSelector)

	// 参数1: uint256 = 42（静态，直接填）
	arg1 := common.LeftPadBytes(big.NewInt(42).Bytes(), 32)
	fmt.Printf("\n参数1 (uint256=42, 静态):\n")
	fmt.Printf("  0x%x\n", arg1)

	// 参数2: string = "Hello"（动态，三区编码）
	// offset = 0x40（2个32字节参数 = 64 = 0x40，从整个 calldata 的 data 区起始位置算起）
	offset := common.LeftPadBytes(big.NewInt(64).Bytes(), 32)
	fmt.Printf("\n参数2 (string=\"Hello\", 动态，先从 offset 开始):\n")
	fmt.Printf("  offset (指向 data 区) = 0x%x  (64 = 2×32字节)\n", offset)

	// Data 区: [length] [string 填充到 32 字节]
	strBytes := []byte("Hello")
	length := common.LeftPadBytes(big.NewInt(int64(len(strBytes))).Bytes(), 32)
	paddedStr := common.RightPadBytes(strBytes, 32) // 右补 0 到 32 字节

	fmt.Printf("  Data 区:\n")
	fmt.Printf("    length = 0x%x  (5 = len(\"Hello\"))\n", length)
	fmt.Printf("    data   = 0x%x  (\"Hello\" 右补0到32字节)\n", paddedStr)

	// 拼接完整 calldata
	var fullData []byte
	fullData = append(fullData, fooSelector...) // [0:4]
	fullData = append(fullData, arg1...)        // [4:36]
	fullData = append(fullData, offset...)      // [36:68]
	fullData = append(fullData, length...)      // [68:100]
	fullData = append(fullData, paddedStr...)   // [100:132]

	fmt.Printf("\n完整 calldata (4+96+32=132 bytes):\n")
	fmt.Printf("  0x%x\n", fullData)

	fmt.Printf("\n布局图解:\n")
	fmt.Printf("  ┌──────────┬─────────────────────────────┐\n")
	fmt.Printf("  │ [0:4]    │ selector                    │\n")
	fmt.Printf("  │ [4:36]   │ arg1: uint256=42            │\n")
	fmt.Printf("  │ [36:68]  │ arg2: offset=0x40           │ ← 指向 [64:?]\n")
	fmt.Printf("  │ [68:100] │ arg2 data: length=5         │\n")
	fmt.Printf("  │ [100:132]│ arg2 data: \"Hello\" padded │\n")
	fmt.Printf("  └──────────┴─────────────────────────────┘\n")

	// ==================== Part D: 事件 Topic 编码 ====================
	fmt.Println("\n━━━ D. 事件 Topic 编码 ━━━")

	eventSig := "Transfer(address, address, uint256)"
	eventHash := crypto.Keccak256Hash([]byte(eventSig))
	fmt.Printf("\nTransfer 事件签名: %s\n", eventSig)
	fmt.Printf("  keccak256 = 0x%x\n", eventHash)
	fmt.Printf("  topic0    = 0x%x  (事件签名哈希)\n\n", eventHash)

	fmt.Println("日志结构 (indexed → topic, 非 indexed → data):")
	fmt.Printf("  topic0 = 事件签名哈希\n")
	fmt.Printf("  topic1 = from 地址 (indexed, 左补0到32字节)\n")
	fmt.Printf("  topic2 = to   地址 (indexed, 左补0到32字节)\n")
	fmt.Printf("  data   = value (非 indexed, ABI 编码的 uint256)\n")

	fmt.Println("\n关键规则:")
	fmt.Println("  • indexed 参数 → topic[1..3]，每个 32 字节")
	fmt.Println("  • 非 indexed 参数 → data 区，按 ABI 编码")
	fmt.Println("  • 最多 3 个 indexed 参数 (topic 数组最大 4 个)")

	// ==================== Part E: AbiJSON返回值的 ABiJson 解码 ====================
	fmt.Println("\n━━━ E. 解码返回值 ━━━")

	// 模拟 balanceOf 返回 4000 (0xFA0)
	mockResult := common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000FA0")
	fmt.Printf("原始返回数据: 0x%x\n", mockResult)

	// 方法1: 用 ABI UnpackIntoInterface
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", mockResult)
	if err != nil {
		log.Fatalf("解码失败: %v", err)
	}
	fmt.Printf("解码结果 (balance): %s\n", balance.String())

	// 方法2: 用 ABI Unpack
	outputs, err := parsedABI.Unpack("balanceOf", mockResult)
	if err != nil {
		log.Fatalf("解码失败: %v", err)
	}
	fmt.Printf("解码结果 (raw):   %v\n", outputs[0])

	fmt.Println("\n✅ ABI 编码解码演示完成！")
}
