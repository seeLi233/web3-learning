# Day 1 面试题答案

## 题目 1: Solidity 中有哪些值类型和引用类型？它们在 EVM 中如何存储？

### 值类型 (Value Types)
- `bool`: 布尔值，true/false
- `int`/`uint`: 有符号/无符号整数，支持 8-256 位
- `address`: 20 字节地址
- `bytes1`-`bytes32`: 固定长度字节数组
- `enum`: 枚举类型

### 引用类型 (Reference Types)
- `string`: 动态字节数组，UTF-8 编码
- `bytes`: 动态字节数组
- `array`: 数组（固定/动态长度）
- `struct`: 结构体
- `mapping`: 映射

### EVM 存储方式
- **值类型**: 直接存储在栈(Stack)上，占用 32 字节
- **引用类型**: 存储在存储(Storage)中，变量存储的是指向存储位置的指针
- **Storage**: 永久存储在区块链上，消耗 gas
- **Memory**: 临时存储，函数结束后销毁
- **Calldata**: 只读，用于外部函数参数

---

## 题目 2: `public`、`private`、`internal`、`external` 四个可见性修饰符的区别？

### public
- 内部和外部都可以访问
- 会自动生成 getter 函数
- gas 消耗相对较高

### private
- 只能在当前合约中访问
- 继承合约也无法访问
- 最安全的修饰符

### internal
- 当前合约和继承合约可以访问
- 外部无法访问
- 默认可见性（不写修饰符时）

### external
- 只能从外部调用
- 不能在合约内部调用（除非用 this.method()）
- gas 效率最高，适合大量数据的函数参数

### 选择建议
- 状态变量的 getter: `public`
- 内部工具函数: `internal` 或 `private`
- 外部接口函数: `external`
- 需要被继承的函数: `internal`

---

## 题目 3: `pure` 和 `view` 函数的区别？调用它们需要消耗 gas 吗？

### view 函数
- 只读取状态，不修改状态
- 可以访问 `msg`, `block`, `tx` 等全局变量
- 可以读取状态变量

### pure 函数
- 不读取也不修改状态
- 不能访问状态变量
- 只能使用参数和局部变量
- 不能使用 `msg.sender`, `block.timestamp` 等

### gas 消耗
- **外部调用**: 不消耗 gas（因为不改变状态）
- **内部调用**: 消耗 gas（因为需要执行计算）
- **合约内部调用**: 消耗 gas（即使是 view/pure）

### 示例
```solidity
// view: 读取状态
function getBalance() public view returns (uint256) {
    return balances[msg.sender];  // 读取状态变量
}

// pure: 不读取状态
function add(uint256 a, uint256 b) public pure returns (uint256) {
    return a + b;  // 只使用参数
}