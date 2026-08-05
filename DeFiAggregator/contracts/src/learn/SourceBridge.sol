// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "./BridgeToken.sol";

/// @title SourceBridge — 源链桥合约（锁定-解锁模式）
/// @notice 锁定用户的 BridgeToken，发出跨链事件
contract SourceBridge is Ownable {
    // ==================== 状态变量 ====================

    BridgeToken public token;

    /// @notice 已处理的赎回 txId（防重放）
    mapping (bytes32 => bool) public processedUnlock;

    /// @notice 交易 nonce（用于生成唯一 txId）
    uint256 public nonce;

    // ==================== 事件 ====================

    /// @notice 资产锁定事件 → 中继者监听此事件
    event TokenLocked(bytes32 indexed txId, address indexed sender, uint256 amount, address indexed recipientOnDestChain);

    /// @notice 资产解锁事件（赎回）
    event TokenUnLocked(bytes32 indexed txId, address indexed recipient, uint256 amount);

    // ==================== 构造函数 ====================

    constructor(address _token, address initialOwner) Ownable(initialOwner) {
        require(_token != address(0), "SourceBridge: zero token address");
        token = BridgeToken(_token);
    }

    // ==================== 核心功能 ====================

    /// @notice 锁定代币，发起跨链转账
    /// @param amount 锁定的金额
    /// @param recipientOnDestChain 目标链上的接收地址
    /// @return txId 本次跨链转账的唯一 ID
    function lock(uint256 amount, address recipientOnDestChain) external returns (bytes32 txId) {
        require(amount > 0, "SourceBridge: amount is zero");
        require(recipientOnDestChain != address(0), "SourceBridge: zero recipient");

        // 生成唯一交易 ID ⭐ 面试重点：为什么用这些参数？
        txId = keccak256(abi.encodePacked(
            msg.sender, 
            amount, 
            recipientOnDestChain, 
            block.chainid,               // 包含链 ID → 防止跨链重放
            address(this),               // 包含桥地址 → 防止不同桥之间重放
            nonce++
            )
        );

        // 把用户的 token 转移到桥合约（锁定）
        require(token.transferFrom(msg.sender, address(this), amount), "SourceBridge: transfer failed");

        emit TokenLocked(txId, msg.sender, amount, recipientOnDestChain);
    }

    /// @notice 解锁代币（赎回流程的最后一步）
    /// @dev 只有 owner（模拟中继者）能调用
    function unlock(bytes32 txId, address recipient, uint256 amount) external onlyOwner {
        require(!processedUnlock[txId], "SourceBridge: already processed");
        require(recipient != address(0), "SourceBridge: zero recipient");
        require(amount > 0, "SourceBridge: amount is zero");

        processedUnlock[txId] = true;

        // 把锁定的 token 转回给用户
        require(token.transfer(recipient, amount), "SourceBridge: transfer failed");

        emit TokenUnLocked(txId, recipient, amount);
    }

    /// @notice 查询桥合约持有的 token 余额
    function lockedBalance() external view returns (uint256) {
        return token.balanceOf(address(this));
    }

    // ==================== 紧急功能 ====================

    /// @notice 紧急暂停后提取 token（仅 owner）
    function emergencyWithdraw(address to, uint256 amount) external onlyOwner {
        require(to != address(0), "SourceBridge: zero address");
        token.transfer(to, amount);
    }
}