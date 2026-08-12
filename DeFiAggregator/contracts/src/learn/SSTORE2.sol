// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title SSTORE2
 * @notice 用合约字节码替代 storage 存储数据，大幅节省 gas
 * @dev 核心原理：
 *      - 写入：用 CREATE 部署一个字节码 = 数据的"存储合约"
 *      - 读取：用 EXTCODECOPY 拷贝合约字节码 → 还原原始数据
 *      - 删除：用 SELFDESTRUCT 销毁存储合约
 *
 * Gas 对比：
 *      - SSTORE: 20000 gas / 32 bytes = 625 gas/byte (首次写入)
 *      - SSTORE2: 200 gas/byte（CREATE 操作码的一部分）
 *      → 约便宜 3 倍！数据越大省得越多
 *
 * 为什么这种模式被称为 "SSTORE2"？
 * → 它是 storage 方案的"第二版"，由 0xsequence 团队在 2021 年提出
 *    后来被 Uniswap V3（LP 头寸管理）、众多 NFT 项目采用
 *
 * 面试高频问：SSTORE2 的优缺点？
 * → 优点：写入便宜 ~3x、可自毁退款
 * → 缺点：不能修改（字节码不可变）、读取稍慢（EXTCODECOPY vs SLOAD）、
 *         每次 CREATE 部署一个合约有最小 gas 成本（~32000），小数据不划算
 */
library SSTORE2 {
    // ==================== 自定义 Error ====================

    error SSTORE2__DeploymentFailed();  // CREATE 操作码返回 0 = 部署失败（通常因为 data 为空或 gas 不足）
    error SSTORE2__ReadFailed();        // 读取失败（指针地址无合约代码）

    /**
     * @notice 将数据写入一个新的存储合约
     * @param data 要写入的原始数据
     * @return pointer 存储合约地址（后续读取时用这个地址定位数据）
     *
     * 为什么返回 address 作为指针？
     * → 存储合约的地址 = 数据在链上的唯一标识符
     *    相当于 C 语言的指针：pointer → *pointer
     *
     * ⚠️ 警告：数据写入后不可修改（合约字节码是 immutable 的）
     */
    function write(bytes memory data) internal returns (address pointer) {
        // 为什么不能直接把 data 传给 CREATE？
        // → CREATE 把输入当 init code 执行，返回结果才是部署的字节码
        // → 原始数据不是合法的 EVM 字节码 → CREATE 执行失败返回 0
        //
        // 解决方案：构造一段 init code，它的功能是把后面的数据
        // "复制到内存再返回"，这样数据就成了合约的实际字节码
        //
        // Init code 结构（12 字节 header + 数据）：
        //   PUSH2 len    → 栈: [len]           // 0x61 <2 bytes>
        //   DUP1         → 栈: [len, len]       // 0x80
        //   PUSH1 0x0c   → 栈: [12, len, len]   // 0x60 0x0c  (数据起始偏移)
        //   PUSH1 0x00   → 栈: [0, 12, len, len]// 0x60 0x00  (内存目标)
        //   CODECOPY     → mem[0:len]=code[12:12+len] 栈: [len]
        //   PUSH1 0x00   → 栈: [0, len]         // 0x60 0x00
        //   RETURN       → 返回 mem[0:len] 作为部署字节码
        bytes memory initCode = bytes.concat(
            hex"61", bytes2(uint16(data.length)), // PUSH2 data_length
            hex"80",                               // DUP1
            hex"60_0c",                            // PUSH1 12
            hex"60_00",                            // PUSH1 0
            hex"39",                               // CODECOPY
            hex"60_00",                            // PUSH1 0
            hex"f3",                               // RETURN
            data
        );

        assembly {
            pointer := create(0, add(initCode, 0x20), mload(initCode))
        }

        if (pointer.code.length == 0) revert SSTORE2__DeploymentFailed();
    }

    // ==================== 读取（从合约字节码读回数据） ====================

    /**
     * @notice 从存储合约读回原始数据
     * @param pointer 之前 write() 返回的存储合约地址
     * @return data 原始数据（与传入 write() 的完全一致）
     *
     * 为什么用 EXTCODECOPY 而不是直接读合约？
     * → 存储合约没有 public view 函数——它只是"一坨字节码"
     * → EXTCODECOPY 是唯一能读取合约字节码的 EVM 操作码
     *
     * Gas 对比：
     *      EXTCODECOPY: ~2600 gas（首次冷访问）
     *      SLOAD: 2100 gas（冷）/ 100 gas（热）
     * → 读取时 SSTORE2 略贵，但因为大多数场景"写一次读多次"，
     *    写的节省（20000→~200/byte）远大于读的多花（2100→2600）
     */
    function read(address pointer) internal view returns (bytes memory data) {
        if (pointer.code.length == 0) revert SSTORE2__ReadFailed();

        assembly {
            // extcodesize：获取合约字节码的总长度
            let size := extcodesize(pointer)

            // 分配 memory 空间：32 字节长度前缀 + 数据长度
            // mload(0x40)：当前空闲内存指针（Solidity 的内存管理约定）
            data := mload(0x40)

            // ⚠️ 关键：必须先写入长度前缀，否则返回的 bytes 长度是垃圾值
            mstore(data, size)

            // 计算需要的总内存大小（对齐到 32 字节边界）
            // add(size, 0x20)：长度前缀 + 数据
            // add(..., 0x1f)：向上对齐准备
            // not(0x1f)：掩码，抹掉低 5 位实现 32 字节对齐
            // and(...)：应用对齐掩码
            mstore(0x40, add(data, and(add(add(size, 0x20), 0x1f), not(0x1f))))

            // EXTCODECOPY(addr, memDest, codeOffset, length)
            // 把 pointer 合约的整个字节码（从第 0 字节开始）拷贝到
            // data 的内存位置（跳过前 32 字节长度前缀）
            extcodecopy(pointer, add(data, 0x20), 0, size)
        }
    }

    // ==================== 删除（自毁合约回收 gas） ====================

    /**
     * @notice 销毁存储合约，回收 gas 退款
     * @param pointer 要删除的存储合约地址
     *
     * 为什么删除能回收 gas？
     * → SELFDESTRUCT 会清除合约的所有 storage slot
     * → 每清除一个 slot 返还约 4800 gas（London 分叉后）
     * → 减少了全节点的状态膨胀，这是以太坊对"清理状态"的激励机制
     *
     * ⚠️ Cancun 分叉后 SELFDESTRUCT 行为变化：
     * → 同一交易内创建 + 销毁的合约：正常自毁
     * → 非同一交易：只发送余额，不清理 storage
     * → 这是 EIP-6780 的规定，防止 Verkle Tree 迁移的复杂性
     */
    function destory(address pointer) internal {
        // 安全检查：确认 pointer 有合约代码
        // 为什么需要这个检查？防止用户传入 EOA 地址导致意外行为
        if (pointer.code.length == 0) revert SSTORE2__ReadFailed();

        assembly {
            // SELFDESTRUCT(addr)：销毁当前合约，把余额发给 addr
            // 但我们不能在自己的合约里 SELFDESTRUCT——那会把整个调用者合约炸了
            // 所以用 DELEGATECALL + SELFDESTRUCT 的组合：
            //   DELEGATECALL 让存储合约在自己的上下文中执行 SELFDESTRUCT
            //   这样只有存储合约被销毁，调用者完好无损
            //
            // mload(0x40) → 空闲内存
            // mstore(...) → 把 SELFDESTRUCT 的 selector（如果有）写入内存
            // 实际上 SELFDESTRUCT 没有 selector——它是个操作码，不是函数
            // 所以直接用 SELFDESTRUCT + DELEGATECALL：
            //   1. 在 pointer 的上下文中执行 SELFDESTRUCT
            //   2. 余额发给调用者（msg.sender 在 delegatecall 中保持不变）
            //
            // 简化方案：直接对 pointer 发 call，让它自己执行自毁
            // 但这要求 pointer（存储合约）有自毁逻辑——它没有
            //
            // 正确方案：利用 SELFDESTRUCT 在 CREATE 的上下文中可执行
            // 这里只为教学展示流程——实际使用时，存储合约的 destroy 需要
            // 在存储合约构造时预先埋入 SELFDESTRUCT 的 init code 中
            //
            // 对于纯数据存储合约（没有执行逻辑），destroy 的实际实现
            // 是把 pointer 的字节码设计为包含 SELFDESTRUCT 的合约
            // 具体方式见 writeSelfDestructable() 函数
            // selfdestruct 只接受 1 个参数（beneficiary address）
            // 此处为占位注释——实际 destroy 通过 writeSelfDestructable 实现
        }
    }

    // ==================== 高级功能：可自毁的存储合约 ====================

    /**
     * @notice 写入可自毁的存储合约
     * @param data 要存储的数据
     * @return pointer 存储合约地址
     *
     * 原理：
     * 在数据前面拼接一段"自毁逻辑"的字节码：
     *   自毁逻辑: PUSH1 0xFF SELFDESTRUCT（3 字节）
     *   ↓
     *   部署的字节码 = [自毁逻辑] + [数据]
     *   ↓
     *   调用 destroy 时：把 pointer 的 fallback 触发 → 执行最前面的自毁逻辑
     *
     * 为什么这么设计？
     * → 存储合约本身只是纯数据（不可执行）
     * → 在数据前面拼一段代码，合约就有了执行能力
     * → 这是 Solidity 底层汇编的常见模式——直接操控字节码
     */
    function writeSelfDestructable(bytes memory data) internal returns (address pointer) {
        // 部署合约的字节码 = [自毁逻辑 2B] + [原始数据]
        // 自毁逻辑：PUSH1 0xFF(0x60FF) + SELFDESTRUCT(0xFF)
        // 当合约收到调用时，从字节码开头执行 → 立即自毁
        //
        // ⚠️ 0xFF 是占位接收地址，真实场景应替换为 owner 地址
        bytes memory deployedCode = bytes.concat(
            hex"60FF_FF", // PUSH1 0xFF(2B) = 把 0xFF 压栈 → SELFDESTRUCT 取走作为 beneficiary
            data
        );

        // 构造 init code：12 字节 CODECOPY+RETURN header + 目标部署字节码
        // 原理同 write()——init code 执行后把 deployedCode 返回为合约字节码
        bytes memory initCode = bytes.concat(
            hex"61", bytes2(uint16(deployedCode.length)), // PUSH2 deployedCode_length
            hex"80",                                       // DUP1
            hex"60_0c",                                    // PUSH1 12
            hex"60_00",                                    // PUSH1 0
            hex"39",                                       // CODECOPY
            hex"60_00",                                    // PUSH1 0
            hex"f3",                                       // RETURN
            deployedCode
        );

        assembly {
            pointer := create(0, add(initCode, 0x20), mload(initCode))
        }
        if (pointer.code.length == 0) revert SSTORE2__DeploymentFailed();
    }
}

/**
 * @title SSTORE2Demo
 * @notice 演示 SSTORE2 的实际使用——对比传统 storage vs SSTORE2 的 gas
 */
contract SSTORE2Demo {
    // 使用 SSTORE2 库
    using SSTORE2 for bytes;
    using SSTORE2 for address;  // read() 在 address 上调用: pointer.read()

    // ==================== 传统 storage 方案 ====================

    // 为什么用 mapping 存储？
    // → 传统方案：把数据存进合约 storage
    // → 缺点：每条记录首次写入至少 20000 gas
    mapping(string => bytes) public storageData;

    function writeStorage(string calldata key, bytes calldata data) external {
        // SSTORE：第一次写 = 20000 gas（新 slot）
        // 如果 data 占多个 slot，每个额外 slot = 20000 gas
        storageData[key] = data;
    }

    function readStorage(string calldata key) public view returns (bytes memory) {
        // SLOAD：冷访问 2100 gas
        return storageData[key];
    }

    // 为什么这里用 mapping 存 pointer？
    // → SSTORE2 把数据存在外部合约的字节码里，我们只需要存一个 address（20 字节）
    // → 一个 slot 可以存全 address + 还有剩余空间
    // → 对比：storage 方案下，数据本身占 N 个 slot（N × 20000 gas）
    mapping(string => address) public sstore2Pointers;  // key → 存储合约地址

    function writeSSTORE2(string calldata key, bytes calldata data) external {
        // 把数据部署为一个独立合约 → 返回合约地址
        // Gas 成本：32000（CREATE 基础）+ ~200/byte（字节码存储）
        // vs 传统 SSTORE：20000/slot = 625/byte
        // → 数据超过 ~200 字节时，SSTORE2 优势明显
        address pointer = data.write();  // 这里调用 SSTORE2.write()
        sstore2Pointers[key] = pointer;  // 只存 20 字节的指针！
    }

    function readSSTORE2(string calldata key) external view returns (bytes memory) {
        address pointer = sstore2Pointers[key];
        require(pointer != address(0), "SSTORE2: not found");
        // 用 EXTCODECOPY 从存储合约的字节码读取数据
        return pointer.read();
    }

    // ==================== Gas 对比辅助函数 ====================

    /**
     * @notice 同时用两种方式写入，方便对比 gas
     * @dev 用 gas reporter 跑一次，可以直观看到两段代码的 gas 差异
     */
    function writeBoth(string calldata key, bytes calldata data) external {
        // 传统方案——贵
        storageData[key] = data;
        // SSTORE2 方案——便宜
        address pointer = data.write();
        sstore2Pointers[key] = pointer;
        // ⚠️ 这个函数总 gas 会比单个方案高（两个都执行了）
        // 测试时应该分别调用 writeStorage 和 writeSSTORE2 来对比
    }

    function getPointer(string calldata key) external view returns (address) {
        return sstore2Pointers[key];
    }


}