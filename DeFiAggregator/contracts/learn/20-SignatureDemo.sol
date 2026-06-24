// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * 学习目标: 理解以太坊签名的底层机制
 *
 * 三个核心概念:
 * 1. ecrecover — 从签名恢复签名者地址
 * 2. EIP-191 — 签名数据前缀标准
 * 3. EIP-712 — 类型化结构化数据签名
 */
contract SignatureDemo {

    // ===== 1. 签名是如何生成的？（链下） =====
    // 在 JavaScript/TypeScript 中:
    //
    //    const messageHash = ethers.solidityPackedKeccak256(
    //      ['address', 'uint256'],
    //      [receiver, amount]
    //    );
    //    const signature = await signer.signMessage(
    //      ethers.getBytes(messageHash)
    //    );
    //    // signature = 0x... (130 字符的十六进制字符串 = 65 字节)
    //    // 前 32 字节 = r
    //    // 中 32 字节 = s
    //    // 最后 1 字节 = v (27 或 28)
    //
    //    const sig = ethers.Signature.from(signature);
    //    // sig.r, sig.s, sig.v

    // ===== 2. ecrecover 演示 =====
    function verifySignature(bytes32 messageHash, uint8 v, bytes32 r, bytes32 s)  public pure returns (address recoveredSigner) {
        // ⚠️ ecrecover 的安全注意事项:

        // ① 必须加 EIP-191 前缀 "\x19Ethereum Signed Message:\n32"
        //    这 32 是 messageHash 的长度（字节数）
        bytes32 ethSignedMessageHash = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", messageHash));

        // ② ecrecover 如果签名无效，返回 address(0)，不会 revert
        recoveredSigner = ecrecover(ethSignedMessageHash, v, r, s);

        // ③ 在生产代码中必须检查:
        //    require(recoveredSigner != address(0), "Invalid signature");
    }

    // ===== 3. EIP-712 域名分隔符演示 =====
    bytes32 private constant _DOMAIN_TYPEHASH = keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");

    bytes32 private immutable _DOMAIN_SEPARATOR;

    constructor() {
        _DOMAIN_SEPARATOR = keccak256(abi.encode(
            _DOMAIN_TYPEHASH, 
            keccak256(bytes("SignatureDemo")),
            keccak256(bytes("1")),
            block.chainid,
            address(this)
        ));
    }

    function getDomainSeparator() external view returns (bytes32) {
        return _DOMAIN_SEPARATOR;
    }

    // ===== 4. 为什么不能用 encodePacked 做签名哈希？ =====
    function demonstarteEncodingCollision() public pure returns (bytes32, bytes32) {
        // encodePacked 有碰撞风险！
        bytes32 hash1 = keccak256(abi.encodePacked("ab", "c"));     // "abc"
        bytes32 hash2 = keccak256(abi.encodePacked("a", "bc"));     // 也是 "abc"
        // hash1 == hash2 <- 完全相同！碰撞!

        // encode 没有碰撞：
        bytes32 hash3 = keccak256(abi.encode("ab", "c"));   // 32字节 "ab" + 32字节 "c"
        bytes32 hash4 = keccak256(abi.encode("a", "bc"));  // 32字节 "a" + 32字节 "bc"
        // hash3 ≠ hash4 ← 不会碰撞

        return (hash1, hash3);  // hash1 == hash2 (碰撞), hash3 ≠ hash4 (安全)
    }

    // ===== 5. 签名 malleability（可塑性）问题 =====
    // 以太坊签名的一个数学性质:
    // 如果 (r, s, v) 是有效签名，那么 (r, n-s, 28-v) 也是有效签名
    // 其中 n = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
    //
    // 解决方案（OpenZeppelin 的做法）:
    // 强制 s 不能大于 n/2，确保签名的唯一性
    //
    // 但在实际使用 OpenZeppelin 时，ECDSA.tryRecover 已经处理了这个
}