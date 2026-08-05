// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title MerkleProof
 * @notice 默克尔证明库 — 验证一个地址是否在白名单中
 * @dev 为什么自己写而不直接用 OpenZeppelin 的？
 *      → 学习目的：深入理解每一个 hash 拼接的逻辑
 *      → 实际项目可直接 import OpenZeppelin 的 MerkleProof.sol
 *
 * 核心原理回顾：
 *   Merkle Tree 是一种二叉树，叶子节点是数据 hash，每往上走一层做一次 keccak256(左+右)
 *   验证时只需要提供「兄弟路径」(proof)，逐层 hash 到根，与链上存的 root 对比
 *
 * 为什么 O(log n) 验证就够了？
 *   → n 个叶子 → 树高 ≈ log₂(n)，验证路径长度 = 树高
 *   → 100 万地址只需 ~20 次 hash，gas 约 3K
 *   → mapping 存 100 万地址需要天价 gas
 */
library MerkleProof {
    /**
     * @notice 验证 Merkle Proof
     * @param proof  兄弟哈希数组（链下生成，链上验证用）
     * @param root   Merkle Root（部署时存在合约里，整个生命周期不变）
     * @param left   待验证的叶子节点 = keccak256(abi.encodePacked(userAddress))
     * @return bool  验证成功返回 true
     *
     * 为什么 proof 放在 calldata？
     *   → proof 不会修改，calldata 比 memory 省 gas（直接从交易数据读取，不复制到内存）
     *
     * 为什么 leaf 不是直接用 address？
     *   → 我们可能需要验证更复杂的数据（address+amount），hash 之后统一变成 bytes32，更通用
     */
    function verify(bytes32[] calldata proof, bytes32 root, bytes32 left) internal pure returns (bool) {
        bytes32 computedHash = left;

        // 为什么用 for 循环逐层往上？
        // → 每层需要一个兄弟 hash（proof[i]），把 computedHash 和 brother 拼起来再 hash
        // → 从叶子走到根，路径长度正好 = proof.length
        for (uint256 i = 0; i < proof.length; i++) {
            bytes32 proofElement = proof[i];

            // --- 排序拼接（防第二原像攻击）---
            // 为什么要排序？
            // → 如果不排序，hash(A+B) 和 hash(B+A) 结果相同，攻击者可以交换左右子树伪造证明
            // → 排序后确保左小右大，左右子树不能互换
            // → 这是 Merkle Tree 安全性的关键细节！
            if (computedHash <= proofElement) {
                computedHash = keccak256(abi.encodePacked(computedHash, proofElement));
            } else {
                computedHash = keccak256(abi.encodePacked(proofElement, computedHash));
            }
        }

        // 最终算出的根 == 部署时存的根 → 证明有效
        return computedHash == root;
    }

    /**
     * @notice 验证多叶子 Merkle Proof（批量验证）
     * @dev 用于一次性验证多个地址，省 gas（一次外部调用验证多个）
     * @param proof  兄弟哈希数组
     * @param root   Merkle Root
     * @param leaves 待验证的叶子数组
     *
     * 为什么需要批量验证？
     *   → 某些场景下需要一次验证多个条件（如：白名单地址 + 对应分配额度）
     *   → 一次外部调用验证多个，比分多次调用省 gas
     */
    function verifyMultiple(bytes32[] calldata proof, bytes32 root, bytes32[] calldata leaves) internal pure returns (bool) {
        // 空数组直接拒绝——不允许"证明空集合"
        if (leaves.length == 0) {
            return false;
        }

        // 逐个验证，全部通过才算 valid
        for (uint256 i = 0; i < leaves.length; i++) {
            if (!verify(proof, root, leaves[i])) {
                return false;
            }
        }
        return true;
    }
}

// 📌 代码分段解读

// 📌 第 1 段：库声明和 verify 函数签名
// - library MerkleProof：为什么是 library 而不是 contract？→ library 是无状态的纯函数集合，不需要部署实例，直接用 using MerkleProof for ... 引用。省 gas，逻辑复用。
// - bytes32[] calldata proof：proof 用 calldata 因为只读不写，比 memory 省一次复制开销。
// - returns (bool)：只返回布尔值，不给具体位置，调用者自己判断。

// 📌 第 2 段：for 循环逐层 hash
// - 核心逻辑：computedHash = keccak256(小, 大) 排序拼接
// - 防第二原像攻击：没有排序的话，攻击者可以用同一对 (A, B) 的 hash 互换左右子树来构造假证明

// 📌 第 3 段：verifyMultiProof
// - 批量验证：循环调用 verify()，有一个不通过就返回 false
// - 适用场景：比如白名单地址 + 分配数量一起验证