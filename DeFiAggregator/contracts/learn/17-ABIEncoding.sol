// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * 17-ABIEncoding.sol
 * 深入理解 ABI 编码的 5 种方式
 *
 * 今天要搞懂的核心问题：
 * 1. abi.encode 和 abi.encodePacked 的区别？
 * 2. abi.encodeWithSelector / encodeWithSignature 怎么用？
 * 3. 什么时候用哪个？
 * 4. encodePacked 的碰撞风险是什么？
 */

contract ABIEncoding {
    
    // ============ 1. abi.encode — 标准 ABI 编码 ============
     function demoEncode(uint256 x, address addr)
        external
        pure
        returns (bytes memory)
    {
        // 每个参数严格 32 字节对齐
        // uint256(42) → 64 个十六进制字符（32 字节）
        // address → 64 个十六进制字符（左补 0 到 32 字节）
        // 总共: 64 字节
        return abi.encode(x, addr);
        // 结果示例（输入 42, 0x5B38...）:
        // 0x
        // 000000000000000000000000000000000000000000000000000000000000002a  ← 42
        // 0000000000000000000000005b38da6a701c568545dcfcb03fcb875f56beddc4  ← address
    }

    // ============ 2. abi.encodePacked — 紧凑编码 ============
    function demoEncodePacked(uint128 x, address addr)
        external
        pure
        returns (bytes memory)
    {
        // 紧凑排列，不补 0
        // uint128(42) → 16 字节（128 位）
        // address → 20 字节（地址的固定大小）
        // 总共: 36 字节
        return abi.encodePacked(x, addr);
    }

    // ============ 3. encodePacked 碰撞风险演示！ ============
    // ⚠️ 这是面试最爱考的陷阱！
    function collisionDemo()
        external
        pure
        returns (bytes32 hash1, bytes32 hash2, bool areEqual)
    {
        // 注意！这两个哈希完全一样！
        hash1 = keccak256(abi.encodePacked("ab", "c"));   // "abc"
        hash2 = keccak256(abi.encodePacked("a", "bc"));   // "abc"
        areEqual = hash1 == hash2;
        // areEqual = true ← 碰撞！

        // 安全做法：用 abi.encode（保持了结构边界，不会碰撞）
        // hash1 = keccak256(abi.encode("ab", "c"));
        // hash2 = keccak256(abi.encode("a", "bc"));
        // hash1 != hash2 ← 安全！
    }

    // ============ 4. abi.encodeWithSelector ============
    function demoEncodeWithSelector(address to, uint256 amount)
        external
        pure
        returns (bytes memory)
    {
        // 先手动算出 transfer 的函数选择器
        // keccak256("transfer(address,uint256)") 的前 4 字节 = 0xa9059cbb
        bytes4 selector = bytes4(keccak256("transfer(address,uint256)"));

        // 选择器 + ABI 编码的参数
        return abi.encodeWithSelector(selector, to, amount);
        // 结果:
        // 0xa9059cbb  ← 4 字节选择器
        // + 标准 ABI 编码的 (to, amount)
    }

    // ============ 5. abi.encodeWithSignature — 更方便 ============
    function demoEncodeWithSignature(address to, uint256 amount)
        external
        pure
        returns (bytes memory)
    {
        // 直接用函数签名字符串，选择器自动计算
        // 和上面的 encodeWithSelector 结果完全一样
        return abi.encodeWithSignature("transfer(address,uint256)", to, amount);
    }

    // ============ 6. abi.decode — 解码 ============
    function demoDecode(bytes calldata data)
        external
        pure
        returns (uint256 num, address addr)
    {
        // 把编码后的字节还原成原始数据
        (num, addr) = abi.decode(data, (uint256, address));
    }

    // 完整编解码流程演示
    function roundTrip(uint256 x, address addr)
        external
        pure
        returns (uint256 decodedX, address decodedAddr)
    {
        bytes memory encoded = abi.encode(x, addr);
        (decodedX, decodedAddr) = abi.decode(encoded, (uint256, address));
        // decodedX == x, decodedAddr == addr
    }

    // ============ 7. keccak256 — 哈希计算 ============
    // keccak256 是 Solidity 内置的哈希函数（就是 SHA3-256）
    function demoKeccak(string calldata input)
        external
        pure
        returns (bytes32)
    {
        return keccak256(bytes(input));
    }

    // keccak256 + abi.encode 的安全哈希
    function safeHash(string calldata a, string calldata b)
        external
        pure
        returns (bytes32)
    {
        return keccak256(abi.encode(a, b));  // ✅ 安全
    }

    // ============ 8. 实战：手动构造 calldata ============
    // 模拟低级 call 调用
    function buildTransferCalldata(address to, uint256 amount)
        external
        pure
        returns (bytes memory calldata_)
    {
        calldata_ = abi.encodeWithSignature(
            "transfer(address,uint256)",
            to,
            amount
        );
        // 这个 bytes 可以直接用于
        // (bool success, ) = tokenAddress.call(calldata_);
    }

    // ============ 9. 函数选择器计算 ============
    function getSelector(string calldata funcSig)
        external
        pure
        returns (bytes4)
    {
        return bytes4(keccak256(bytes(funcSig)));
        // 输入: "transfer(address,uint256)"
        // 输出: 0xa9059cbb
    }
}