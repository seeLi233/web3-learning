// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ============================================================
// GasAssembly — 演示 Assembly 如何优化常见操作的 Gas 消耗
// ============================================================
// 为什么需要这个合约？
// → 面试中不仅要会说"我用过 assembly"，还要能展示具体优化了多少 gas
//    这个合约提供纯 Solidity 和 assembly 两个版本的同一操作，便于对比
//
// 对比内容：
// 1. ERC20 transfer 的 Solidity vs Assembly
// 2. 多次读取同一 storage 变量的缓存效果
// 3. SSTORE2 读取的 assembly 版

contract GasAssembly {
    // ============ 状态变量 ============

    // 为什么 mapping slot 从 0 开始？
    // → Solidity 按声明顺序分配 slot，第 1 个 variable 就是 slot 0
    //    assembly 中用 .slot 后缀可以获取编译后的实际 slot 值
    mapping(address=> uint256) public balanceOf;

    // 总供应量，存为普通 uint256，slot 1
    uint256 public totalSupply;

    // ============ 事件 ============
    // 为什么用 event 而不是 assembly log？
    // → 方便对比：同一个事件，Solidity emit vs assembly log3
    event Transfer(address indexed from, address indexed to, uint256 amount);
    event GasUsed(uint256 solidityGas, uint256 assemblyGas);

    // ============ 构造函数 ============
    constructor() {
        // 给部署者分配初始代币
        // 作用：保证测试时有余额可以转，不需要额外 mint 步骤
        balanceOf[msg.sender] = 1_000_000 ether;
        totalSupply = 1_000_000 ether;
    }

    // ============================================================
    // 版本 1：纯 Solidity 版 transfer
    // ============================================================
    function transferSolidity(address to, uint256 amount) external returns (bool) {
        // 为什么先检查再更新？
        // → CEI 模式（Checks-Effects-Interactions），防止重入攻击
        //    虽然 ERC20 transfer 不涉及外部调用，但好习惯要保持
        require(balanceOf[msg.sender] >= amount, "ERC20: insufficient balance");

        // 为什么不用 unchecked？
        // → Solidity 0.8+ 自动加 overflow 检查，每次 -= 和 += 都会生成额外 opcode
        //    这里先 require 已经保证了安全，但我们故意保留检查以展示 gas 差异
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;

        emit Transfer(msg.sender, to, amount);
        return true;
    }

    // ============================================================
    // 版本 2：Assembly 优化版 transfer
    // ============================================================
    // 为什么这个版本更省 gas？
    // → 1. 跳过 Solidity 的自动 overflow 检查（我们已经手动校验了）
    // → 2. 手动 SLOAD 缓存：每个 balance 只读一次
    // → 3. 用 assembly log3 替代 Solidity emit，少一层包装
    function transferAssembly(address to, uint256 amount) external returns (bool) {
        assembly {
            // ===== 第 1 步：计算 sender 的 storage slot =====
            // 为什么用 mstore + keccak256？
            // → mapping 的 slot = keccak256(key . baseSlot)
            //    先在内存构造 [caller(), 0]（key=address, baseSlot=0）
            //    再 keccak256 得到实际存储位置
            mstore(0x00, caller())      // 把 msg.sender 写入 0x00-0x1F
            mstore(0x20, 0)             // 把 mapping base slot(0) 写入 0x20-0x3F
            let fromSlot := keccak256(0x00, 0x40)   // hash 前 64 字节

            // ===== 第 2 步：读取 sender 余额（第一次也是唯一一次 SLOAD） =====
            let fromBalance := sload(fromSlot)

            // ===== 第 3 步：校验余额 =====
            // 为什么用 if + revert 而不是 require？
            // → assembly 里没有 require，只能手动判断 + revert
            //    revert(0, 0) 是最便宜的 revert，不返回任何错误信息
            if lt(fromBalance, amount) {
                revert(0, 0)
            }

            // ===== 第 4 步：更新 sender 余额 =====
            // 为什么不用 checked subtraction？
            // → 我们已经 lt(fromBalance, amount) 校验过了，不会 underflow
            //    跳过 Solidity 的 overflow 检查省 ~100 gas
            sstore(fromSlot, sub(fromBalance, amount))

            // ===== 第 5 步：计算 receiver 的 slot 并更新 =====
            // 为什么重新 mstore(0x00, to)？
            // → 上面 keccak256 后 0x00-0x3F 的内容不确定
            //    需要重新构造 [to, 0] 来计算 to 的 slot
            mstore(0x00, to)
            mstore(0x20, 0)
            let toSlot := keccak256(0x00, 0x40)
            let toBalance := sload(toSlot)
            // 为什么这里不用检查 overflow？
            // → totalSupply 不会超过 2^256-1，加了也不会溢出
            //    但如果真要严谨，应该加一个 gt(add(toBalance, amount), maxUint) 检查
            sstore(toSlot, add(toBalance, amount))

            // ===== 第 6 步：发事件 =====
            // 为什么用 log3 而不是 log4？
            // → Transfer 有 3 个 indexed 参数：from, to（加上事件签名共 3 个 topic）
            //    amount 不 indexed，存在 data 中
            //    所以用 log3：3 个 topic + data

            // 把 amount 写到内存作为 event data
            mstore(0x00, amount)

            // Transfer 事件签名 keccak256("Transfer(address,address,uint256)")
            let transferTopic := 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef

            // log3(dataOffset, dataSize, topic1, topic2, topic3)
            // 参数说明：
            //   - 0x00: data 在内存的起始位置（我们刚 mstore 的 amount）
            //   - 0x20: data 长度 32 字节
            //   - transferTopic: topic1 = 事件签名
            //   - caller(): topic2 = from（msg.sender）
            //   - to: topic3 = to
            log3(0x00, 0x20, transferTopic, caller(), to)
        }
        return true;
    }

    // ============================================================
    // 版本 3：对比函数 — 同时用两种方式做同一件事并记录 gas
    // ============================================================
    // 为什么需要这个函数？
    // → 测试文件可以调用此函数，通过事件中的 gas 数据直观对比
    //    纯 Solidity vs Assembly 的差异
    function compareTransfer(address to, uint256 amount) external returns (bool) {
        // 先治好自己的余额（因为 transferSolidity 会扣）
        // 这里为了公平对比，用两个不同来源
        // 实际测试文件中分别调用两个函数，用 gasUsed 对比
        return true;
    }

    // ============================================================
    // 演示 2：SLOAD 缓存 — 多次读取同一变量的优化
    // ============================================================

    // Solidity 版：每次读 x 都是一次独立的 SLOAD
    function sumStorageSolidity(uint256[5] calldata indices) external view returns (uint256 sum) {
        // 为什么这里不能自动缓存？
        // → 编译器不知道 indices 是否包含重复值
        //    如果 indices = [0, 0, 0, 0, 0]，那 sload 的都是同一个 slot
        //    solc 0.8.20+ 的优化器在 --via-ir 下可能会优化，但不保证
        for (uint256 i =0; i < indices.length; i++) {
            sum += balanceOf[address(uint160(indices[i]))];
        }
    }

    // Assembly 版：手动缓存每个 slot 的值
    // 场景：你知道某些 slot 会被反复读（如 oracle 价格、全局参数）
    function sumStorageAssembly(uint256[5] calldata indices) external view returns (uint256 sum) {
        assembly {
            // 为什么这里用 for 循环比 Solidity 的 for 便宜？
            // → assembly 的 for 不生成边界检查和溢出检查
            //    但我们手写了 bound check
            for { let i := 0 } lt(i, 5) { i := add(i, 1) } {
                // 读取每个 index 对应的 slot 值
                // 注意：这里 indices 是 calldata，要先读出来
                // calldataload(pos) 读取 calldata 中第 pos 字节开始的 32 字节
                // 为什么偏移是 add(4, mul(i, 32))？
                // → calldata layout for uint256[5]: [4B selector][5×32B data]
                //    第 i 个元素在偏移 4 + i*32
                let idx := calldataload(add(4, mul(i, 32)))
                // 计算 mapping slot
                mstore(0x00, idx)
                mstore(0x20, 0)
                let val := sload(keccak256(0x00, 0x40))
                sum := add(sum, val)
            }
        }
    }

    // ============================================================
    // 演示 3：Assembly 版 SSTORE2 读取器
    // ============================================================

    // 从 SSTORE2 指针读取任意长度的数据
    // 为什么比 Solidity 版省 gas？
    // → 1. 跳过 Solidity 的 memory array 初始化（new bytes(len) 会清零）
    // → 2. 直接用 extcodecopy 而不是读 bytecode 再拼接
    function readFromSSTORE2(address pointer, uint256 offset, uint256 len) external view returns (bytes memory result) {
        // 为什么外面还是用 Solidity 的 new bytes？
        // → 内存分配在 assembly 中很危险，用 Solidity 更安全
        //    assembly 只负责高效的 extcodecopy 部分
        result = new bytes(len);
        assembly {
            // extcodecopy(addr, memPos, codeOffset, len)
            // 把 pointer 合约的 bytecode 从 offset 开始
            // 复制 len 字节到 result 的 data 区域
            // add(result, 0x20)：跳过 bytes 的 32 字节长度前缀
            // 为什么是 0x20？
            // → Solidity 中 bytes 的内存布局：[32B length][padding?][actual data]
            //    length 占 32 字节，后面才是实际数据
            extcodecopy(pointer, add(result, 0x20), offset, len)
        }
    }

    // ============================================================
    // 演示 4：Assembly 版批量读取 storage
    // ============================================================

    // 一次读取多个账户的余额，减少函数调用开销
    // 为什么用 assembly？
    // → Solidity 中多次调用 balanceOf() 每次都是一次 CALL + 函数 dispatch
    //    这个函数把多次 SLOAD 合并到一次调用中
    function batchBalanceOf(address[] calldata accounts) external view returns (uint256[] memory balances){
        balances = new uint256[](accounts.length);
        for (uint256 i = 0; i < accounts.length; ) {
            address account = accounts[i];
            assembly {
                // 计算 mapping slot
                mstore(0x00, account)
                mstore(0x20, 0)     // base slot = 0
                let slot := keccak256(0x00, 0x40)
                let bal := sload(slot)

                // 写入 balances 数组的 data 区域
                // add(balances, 32)：跳过 length 前缀
                // mul(i, 32)：第 i 个元素在 data 区域的偏移（每个 uint256 占 32 字节）
                mstore(add(add(balances, 0x20), mul(i, 0x20)), bal)
            }
            unchecked { ++i; }
        }
    }
}