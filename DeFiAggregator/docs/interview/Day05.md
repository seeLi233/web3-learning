# Day 5 面试准备: 继承 + 接口 + 抽象合约 + C3 线性化

## 1. Solidity 的多重继承遵循什么规则？C3 线性化是什么？（必考高频题）

### 一句话总结

C3 线性化是 Solidity（和 Python）使用的多重继承解析算法，保证继承链的**单调性**和**一致性**。

### 三条规则

```
规则 1: 子合约永远排在父合约之前
规则 2: 声明顺序决定优先级（左边的 > 右边的）
规则 3: 整个继承图必须满足单调性（一个合约在所有链中顺序一致）
```

### 经典菱形继承推导

```
        A
       / \
      B   C
       \ /
        D

D is B, C  →  C3 线性化: D → B → C → A
```

### 手动推导过程

```
L(D) = D + merge(L(B), L(C), [B, C])

L(B) = B + merge(L(A), [A]) = [B, A]
L(C) = C + merge(L(A), [A]) = [C, A]

merge([B, A], [C, A], [B, C]):
  - B 不在任何链的尾部 → 取出 B
  - merge([A], [C, A], [C]):
  - A 在 [C, A] 的尾部 → 跳过
  - C 不在任何链的尾部 → 取出 C
  - merge([A], [A], []):
  - A 不在任何链的尾部 → 取出 A

结果: L(D) = [D, B, C, A]
```

### ⚠️ 最容易答错的问题

> **super 不等于直接父合约！**

```
D.foo() 中 super.foo() → B.foo()
B.foo() 中 super.foo() → C.foo()（不是 A！）
C.foo() 中 super.foo() → A.foo()
```

**super 是按照 C3 线性化顺序确定的"下一个"合约**，不是直接父合约。

### 菱形继承不会重复执行

每个合约的函数在 super 调用链中**只执行一次**。D→B→C→A，A.foo() 只被调用一次。

### 声明顺序的影响

```solidity
contract D is B, C { ... }  // C3: D → B → C → A
contract E is C, B { ... }  // C3: E → C → B → A  ← B和C的顺序换了！
```

---

## 2. abstract contract 和 interface 的区别？各自的使用场景？

### 对比表格（必须记住）

| 特性 | interface | abstract contract |
|------|-----------|-------------------|
| 状态变量 | ❌ 不能有 | ✅ 可以有 |
| 构造函数 | ❌ 不能有 | ✅ 可以有 |
| 函数实现 | ❌ 全部无实现 | ✅ 部分可实现 |
| modifier | ❌ 不能有 | ✅ 可以有 |
| 可见性 | 只能 external | public / internal / external |
| 多重继承 | ✅ 推荐（无钻石问题） | ⚠️ 有钻石问题 |
| 继承其他接口/合约 | ✅ 可以 | ✅ 可以 |

### 为什么 interface 不能有构造函数？

interface 是纯接口规范，不是"合约"。构造函数用于初始化状态变量，而 interface 没有状态变量，所以它不需要也不能有构造函数。

### 选择原则

```
做标准 → interface（ERC20/ERC721/ERC1155）
做基类 → abstract（Ownable/AccessControl/Pausable）

interface 定义"能做什么"（API）
abstract 提供"怎么做"（默认实现）
```

### 实际例子

```solidity
// IERC20 定义标准
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
}

// ERC20 提供默认实现
abstract contract ERC20 is IERC20 {
    mapping(address => uint256) private _balances;

    function transfer(address to, uint256 amount) external override returns (bool) {
        // ... 完整实现
    }
}
```

---

## 3. 如何设计一个可扩展的合约接口系统？

### 设计原则

1. **接口优先（Interface-First）**
   - 先用 interface 定义 API，再实现
   - 接口小而精，单一职责
   - ERC20 只定义代币标准，ERC20Permit 单独定义签名授权

2. **抽象合约提供默认实现**
   - 提取公共逻辑到 abstract contract
   - 通过 virtual/override 机制允许定制
   - 例子：Ownable 提供 owner + onlyOwner 默认实现

3. **组合优于继承**
   ```
   大而全的 IEverything ❌
   组合多个小接口 ✅
   
   ERC20 + ERC20Permit + ERC20Votes = 完整治理代币
   ```

4. **接口版本化**
   ```solidity
   interface IERC20V2 is IERC20 {
       // 保持向后兼容，新增功能
       function permit(...) external;
   }
   ```

5. **super 调用链**
   - 每个父合约的逻辑通过 super 依次执行
   - 确保所有层的钩子都被触发

6. **考虑存储兼容性**
   - 使用 EIP-1967 固定存储槽
   - 新版本只能追加状态变量，不能删除或重排

---

## 4. ERC20 的 approve 函数有什么安全问题？如何解决？（扩展题）

### Race Condition（竞态条件）

```
场景：
1. Alice approve(Bob, 100)
2. Alice 想改成 approve(Bob, 50)
3. Bob 监听 mempool，抢在 Alice 改之前 transferFrom 100
4. Bob 又花掉新的 50
→ Bob 总共花掉 150 ❌
```

### 三种解决方案

| 方案 | 做法 | 优点 | 缺点 |
|------|------|------|------|
| 先归零再赋值 | approve(0) → approve(100) | 简单直接 | 两笔交易，gas 高 |
| increase/decreaseAllowance | +10 或 -10 | 原子操作，OZ 推荐 | 需要额外函数 |
| permit (EIP-2612) | 链下签名授权 | 无 approve 交易 | 需要实现签名验证 |

### increaseAllowance 原理

```solidity
function increaseAllowance(address spender, uint256 addedValue) public returns (bool) {
    _approve(msg.sender, spender, allowance(msg.sender, spender) + addedValue);
    return true;
}

function decreaseAllowance(address spender, uint256 subtractedValue) public returns (bool) {
    uint256 currentAllowance = allowance(msg.sender, spender);
    require(currentAllowance >= subtractedValue);
    _approve(msg.sender, spender, currentAllowance - subtractedValue);
    return true;
}
```

---

## 5. super 关键字的调用顺序？多个父合约时如何决策？（扩展题）

### 核心规则

> super **不等于**直接父合约。super 按照 C3 线性化顺序调用"下一个"。

### 完整示例

```solidity
contract A {
    function foo() public virtual {
        emit Log("A");
    }
}

contract B is A {
    function foo() public virtual override {
        emit Log("B");
        super.foo();  // → C.foo()，不是 A.foo()！
    }
}

contract C is A {
    function foo() public virtual override {
        emit Log("C");
        super.foo();  // → A.foo()
    }
}

contract D is B, C {
    function foo() public override(B, C) {
        emit Log("D");
        super.foo();  // → B.foo()
    }
}

// 调用 D.foo() 输出: D → B → C → A
// B.foo() 中的 super 调的是 C，因为 C3 顺序是 D→B→C→A
```

### 直接指定 vs super

```solidity
// 直接指定：只调 A.foo()，不走 C3 链
A.foo();

// super：按 C3 顺序调下一个
super.foo();
```

### 必须显式列出所有被重写的合约

```solidity
contract D is B, C {
    // override(B, C) 必须列出所有重写的父合约
    function foo() public override(B, C) {
        super.foo();
    }
}
```

---

## 6. ERC20 继承链分析（OZ 源码级理解）

```
IERC20 (interface)
  ↑
IERC20Metadata (interface)
  ↑
Context (abstract)  ← _msgSender() / _msgData()
  ↑
ERC20 (contract)  ← implements IERC20Metadata
  ↑
ERC20Burnable (contract)  ← burn / burnFrom
  ↑
DeFiToken (contract)  ← 我们的代币 (同时继承 Ownable)

C3 线性化: DeFiToken → ERC20Burnable → ERC20 → Ownable → Context
```

---

## 7. 今日易错点总结

| 陷阱 | 原因 |
|------|------|
| super 就是直接父合约 | ❌ super 是 C3 线性化的下一个，不是直接 parent |
| interface 可以有 constructor | ❌ interface 是纯规范，没有状态变量就不需要构造函数 |
| is 后面的顺序无所谓 | ❌ 声明顺序决定 C3 线性化优先级 |
| abstract 合约不能被 new | ✅ 正确，abstract 是不完整的 |
| interface 函数可以不写 virtual | ✅ 正确，interface 所有函数隐式 virtual |
| C3 线性化后 A.foo() 会被调两次 | ❌ 每个合约函数在 super 链中只执行一次 |
| 菱形继承在 Solidity 中被禁止 | ❌ Solidity 允许菱形继承，只用 C3 线性化解决冲突 |
