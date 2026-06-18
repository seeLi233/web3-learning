# Day 3 面试准备: 控制流 + 数组 + 数据位置 + EVM Slot

## 1. storage vs memory vs calldata（必考高频题）

### 一句话区分

| 位置 | 类比 | 持久化 | 可写 | Gas | 场景 |
|------|------|--------|------|-----|------|
| `storage` | 硬盘 | ✅ | ✅ | 最贵 | 状态变量 |
| `memory` | 内存 | ❌ | ✅ | 便宜 | 临时计算 |
| `calldata` | ROM | ❌ | ❌ | 最省 | external 参数 |

### 赋值规则（必背）

```
storage  → storage  = 指针（改一个影响另一个）
storage  → memory   = 复制（⚠️ 大数组杀 gas）
memory   → memory   = 指针
memory   → storage  = 复制
calldata → memory   = 复制
```

### 面试常见追问

**Q: 为什么 `calldata` 只能在 `external` 函数用？**
A: `public` 函数可以被内部调用，内部调用时没有 calldata（是 JUMP 进来的），编译器无法保证数据来源，所以只允许 external。

**Q: struct 用 storage 指针和 memory 副本的区别？**
```solidity
User storage u = users[i];  // 指针，修改 u.balance → 改了链上数据
User memory u = users[i];   // 副本，修改 u.balance → 只改了内存，链上不变
```

---

## 2. EVM Slot 布局（必考高频题）

### 基本规则

- EVM 有 2^256 个存储槽 (slot)，每个 32 字节
- 状态变量按声明顺序分配到 slot 0, 1, 2 ...
- 基本类型占一个 slot（除非打包）

### 变量打包

多个小型变量可以**共享同一个 slot**（省 gas）：

```solidity
contract Packing {
    uint128 a;  // slot 0 ─┐
    uint128 b;  // slot 0 ─┘ 共享！总共只占 1 个 slot
    uint256 c;  // slot 1 ─── 单独占据
    uint64 d;   // slot 2 ─┐
    uint64 e;   // slot 2 ─┤
    uint128 f;  // slot 2 ─┘ 三个共享 slot 2
}
```

打包条件：相邻变量，且它们的总大小 ≤ 32 字节。

### 面试追问

**Q: mapping 怎么存？**
A: mapping 的 key 不直接存值。规则：`keccak256(abi.encode(key, slot))` 计算出实际存储位置。mapping 本身只占一个 slot（那个 slot 永远是空位，防止碰撞）。

**Q: 动态数组怎么存？**
A: slot 存的是数组长度，实际元素从 `keccak256(slot)` 开始连续存储。

**Q: 为什么要了解 slot 布局？**
A: 1) 审计时能用 `sload` 读任意 slot 检查隐藏变量；2) 升级合约时 slot 对齐是关键（存储碰撞）；3) 优化 gas。

---

## 3. 数组 Gas 特性

### 操作 Gas 对比

| 操作 | Gas | 说明 |
|------|-----|------|
| `push()` | ~20000 (冷) / ~5000 (热) | 写新 slot |
| `pop()` | ~5000 | 清空 + 减长度 |
| `delete arr[i]` | ~5000 | 只清零，长度不变 |
| `arr[i] = arr[n-1]; pop()` | ~10000 | swap-and-pop，O(1) |
| `for(i=0; i<arr.length; i++)` | 随长度线性增长 | 可能耗尽 gas |
| 遍历 storage 数组 | 每读一个元素 ~2100 (SLOAD) | 很贵 |
| 遍历 memory 数组 | ~3 gas/次 | 极便宜 |

### 面试追问

**Q: 为什么 Solidity 没有 `arr.remove(i)` 这样的内置函数？**
A: 因为 EVM 存储模型是 key-value，数组是"连续的 key"。删除中间元素意味着要把后面所有元素向前移一位 → O(n) gas，这太贵了。所以 Solidity 选择不提供，让开发者根据场景自己选 swap-and-pop 或保持空洞。

**Q: 遍历大数组的安全做法？**
- `require(arr.length <= MAX_BATCH)` 限制输入
- Pull-over-push 模式（用户自己来 claim，而非合约循环所有用户）
- 分页处理

---

## 4. 控制流编写题

**Q: 实现一个函数，返回数组中所有大于 x 的元素的索引列表**

```solidity
function indicesGt(uint[] memory arr, uint x) public pure returns (uint[] memory) {
    // 第一遍：数数量
    uint count = 0;
    for (uint i = 0; i < arr.length; i++) {
        if (arr[i] > x) count++;
    }
    // 第二遍：填值
    uint[] memory result = new uint[](count);
    uint idx = 0;
    for (uint i = 0; i < arr.length; i++) {
        if (arr[i] > x) {
            result[idx] = i;
            idx++;
        }
    }
    return result;
}
```

关注点：为什么要两遍循环？因为 memory 数组创建时必须指定长度。

---

## 5. 今日易错点总结

| 陷阱 | 原因 |
|------|------|
| `if (1)` | Solidity 没有 truthy/falsy |
| `delete arr[i]` 让数组留空洞 | delete 只清零，不改变长度 |
| `arr.length = 5` 只在 storage | memory 数组 length 是只读的 |
| 循环无上限 | gas 耗尽，交易回滚但 gas 不退 |
| storage → memory 复制大数组 | 每个元素一次 SLOAD，线性膨胀 |
| modifier 顺序 | 从左到右执行，影响结果 |
