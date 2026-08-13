// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ============================================================
// SSTORE3 — 用 "地址指针" 代替 "存储数据" 的极致优化
// ============================================================
// 为什么叫 SSTORE3？
// → SSTORE1 = 传统 storage 写入（~20000 gas）
// → SSTORE2 = 把数据写进合约 bytecode（~200 gas/字节 写入，读用 EXTCODECOPY）
// → SSTORE3 = 在 SSTORE2 基础上：
//    (1) 用 CREATE2 让指针地址可预测、可去重
//    (2) 用 address 的 160 bit 直接编码小数据（指针即数据）
//    (3) 共享同一份数据，多个 slot 指向同一地址
//
// 核心思想类比：
//   SSTORE1 = 把书的内容抄在本子上（每抄一个字都贵）
//   SSTORE2 = 把书印成一本书，本子上只记"这本书在图书馆的编号"
//   SSTORE3 = 多个人共用同一本书，且编号本身能编码部分信息

contract SSTORE3 {
    // ============ 状态变量 ============

    // 为什么用 mapping(address => bytes) 而不是 mapping(uint256 => bytes)？
    // → SSTORE3 的核心就是"地址即指针"，key 是地址更直观
    //    而且可以直接用 address 的 bits 编码数据
    // 作用：存储数据指针 → 数据内容（存在指针合约的 bytecode 里）
    mapping(address => bytes) private _pointerToData;

    // 记录某个数据 hash 对应的指针地址，用于去重
    // 为什么需要去重？
    // → 如果两个用户存了相同的数据，SSTORE3 只需要部署一份合约
    //    省下重复部署的 gas
    mapping(bytes32 => address) private _dataHashToPointer;

    // ============ 事件 ============
    event DataStored(address indexed pointer, uint256 size);
    event GasReport(uint256 writeGas, uint256 readGas, string label);

    // ============================================================
    // 写入：把数据存到新合约的 bytecode 里
    // ============================================================
    // 为什么用 CREATE2 而不是 CREATE？
    // → CREATE2 的地址 = keccak256(0xff, deployer, salt, bytecodeHash)
    //    只要 (deployer, salt, bytecode) 相同，地址就相同
    //    这样天然支持去重：相同数据 → 相同地址 → 不重复部署
    function write(bytes calldata data) external returns (address pointer) {
        // 计算数据 hash 用于去重
        bytes32 dataHash = keccak256(data);

        // 如果这个数据已经存过，直接返回已存在的指针
        // 为什么先查去重？
        // → 省 gas：重复部署同样的数据是浪费
        //    SSTORE3 的核心优势之一就是数据去重
        if (_dataHashToPointer[dataHash] != address(0)) {
            return _dataHashToPointer[dataHash];
        }

        // 用 CREATE2 部署一个新合约，其 bytecode 就是我们的数据
        // 为什么不能直接把 data 当 bytecode 传给 create2？
        // → CREATE2 会把传入的 bytecode 当作「构造函数」执行，
        //    部署的是它 RETURN 出来的字节码，而不是传入的字节码本身。
        //    如果直接把 data（如 0xdead...）当构造函数执行，第一个字节
        //    是非法 opcode，执行直接 revert → create2 返回 0 → 部署失败。
        //    所以要在 data 前面拼一段 initcode：把后面的 data 复制到内存并 return。
        assembly {
            // CREATE2 的参数：
            //   create2(value, offset, size, salt)
            //   - value: 发送的 ETH（这里 0）
            //   - offset: bytecode（initcode + data）在内存中的起始位置
            //   - size: initcode + data 的总长度
            //   - salt: CREATE2 的盐值（我们用 dataHash）

            let ptr := mload(0x40)

            // ===== 前 14 字节 initcode =====
            // 功能：把紧跟其后的 data 复制到内存，再 return 出去作为 runtime code
            //   PUSH2 <len>   // 0x61 <hi> <lo>  压入 data 长度
            //   PUSH1 0x0e    // 0x60 0x0e       压入 data 在 initcode 里的偏移（=14）
            //   PUSH1 0x00    // 0x60 0x00       压入内存目标偏移（0）
            //   CODECOPY      // 0x39            从 code 偏移 14 复制 len 字节到内存 0
            //   PUSH2 <len>   // 0x61 <hi> <lo>  再压入长度
            //   PUSH1 0x00    // 0x60 0x00       压入内存偏移（0）
            //   RETURN        // 0xf3            返回内存 [0, len)
            mstore8(ptr,        0x61) // PUSH2
            mstore8(add(ptr, 1), shr(8, data.length))
            mstore8(add(ptr, 2), data.length)
            mstore8(add(ptr, 3), 0x60) // PUSH1
            mstore8(add(ptr, 4), 0x0e) // data 偏移 = 14
            mstore8(add(ptr, 5), 0x60) // PUSH1
            mstore8(add(ptr, 6), 0x00) // 内存目标 0
            mstore8(add(ptr, 7), 0x39) // CODECOPY
            mstore8(add(ptr, 8), 0x61) // PUSH2
            mstore8(add(ptr, 9), shr(8, data.length))
            mstore8(add(ptr, 10), data.length)
            mstore8(add(ptr, 11), 0x60) // PUSH1
            mstore8(add(ptr, 12), 0x00) // 内存偏移 0
            mstore8(add(ptr, 13), 0xf3) // RETURN

            // 把 data 复制到 initcode 后面（偏移 14 开始）
            // data.offset 指向 calldata 中 data 的起始字节，data.length 是长度
            calldatacopy(add(ptr, 14), data.offset, data.length)

            pointer := create2(0, ptr, add(data.length, 14), dataHash)
            // 为什么 salt 用 dataHash？
            // → 相同数据 → 相同 hash → 相同地址，天然去重
        }

        // 检查部署是否成功（create2 返回 0 表示失败）
        require(pointer != address(0), "SSTORE3: deploy failed");

        // 记录映射关系
        _pointerToData[pointer] = data;
        _dataHashToPointer[dataHash] = pointer;

        emit DataStored(pointer, data.length);
    }

    // ============================================================
    // 读取：从指针合约的 bytecode 读数据
    // ============================================================
    // 为什么读操作便宜？
    // → EXTCODECOPY 是常量 gas（~2600 gas 起步），
    //    而 SLOAD 是 2100（cold）/100（warm），
    //    但 storage 写很贵，bytecode 读 vs storage 读在大量数据时优势明显
    function read(address pointer) external view returns (bytes memory) {
        // 如果 mapping 里存了，直接返回（省一次 extcodecopy）
        // 但实际上 SSTORE3 的哲学是"不存数据，只存指针"
        // 这里为了教学对比，我们总是从 bytecode 读

        // 先获取指针合约的 bytecode 长度
        // 为什么需要长度？
        // → extcodecopy 需要知道读多少字节
        uint256 codeSize;
        assembly {
            // extcodesize(addr) 返回 addr 合约的 bytecode 长度
            codeSize := extcodesize(pointer)
        }

        // 检查指针合约确实有代码
        require(codeSize > 0, "SSTORE3: empty pointer");

        // 分配结果内存
        bytes memory result = new bytes(codeSize);

        // 用 assembly 高效复制
        assembly {
            // extcodecopy(addr, memPos, codeOffset, len)
            // 从指针合约的 bytecode offset 0 开始，
            // 复制 codeSize 字节到 result 的 data 区域
            // add(result, 0x20)：跳过 32 字节的 length 前缀
            extcodecopy(pointer, add(result, 0x20), 0, codeSize)
        }

        return result;
    }

    // ============================================================
    // SSTORE3 技巧：地址本身编码数据
    // ============================================================
    // 为什么地址能编码数据？
    // → address 是 160 bit，可以装下 uint160 范围内的任何值
    //    小数据（如 uint96）可以直接存进地址，省一次部署
    // 场景：存储小额参数（费率、上限），不值得为 32 字节部署合约

    // 把 uint96 编码进地址
    function encodeUint96(uint96 value) external pure returns (address) {
        // 为什么 uint96？
        // → address 是 160 bit，uint96 是 96 bit，绰绰有余
        //    还能留 64 bit 放其他标记
        return address(uint160(value));
    }

    // 从地址解码 uint96
    function decodeUint96(address pointer) external pure returns (uint96) {
        // 为什么直接 cast？
        // → address 本身就是 uint160，直接转回 uint96 无损
        return uint96(uint160(pointer));
    }

    // 复合编码：前 8 bit 是类型标记，后 152 bit 是数据
    // 为什么需要类型标记？
    // → 一个指针地址可能指向不同含义的数据
    //    用前几个 bit 标记类型，读的时候才知道如何解码
    function encodeTyped(uint8 kind, uint152 data) external pure returns (address) {
        // 为什么用 shl(152, kind)？
        // → 把 kind 移到最高 8 bit，然后和 data 做或运算合并
        //    结果：| 8 bit kind | 152 bit data | = 160 bit = address
        return address(uint160((uint256(kind) << 152) | uint256(data)));
    }

    // 解码数据部分
    function decodeKind(address pointer) external pure returns (uint8) {
        // 为什么 shr(152, ...)？
        // → 右移 152 bit，把最高的 8 bit（kind）移到最低位
        return uint8(uint256(uint160(pointer)) >> 152);
    }

    // 解码数据部分
    function decodeTypedData(address pointer) external pure returns (uint152) {
        // 为什么用 and 而不是 shr 之后再 shl？
        // → and(pointer, 0x00...FF) 直接保留低 152 bit
        //    最高 8 bit 清零，剩下就是 data
        return uint152(uint256(uint160(pointer)) & ((1 << 152) - 1));
    }
}