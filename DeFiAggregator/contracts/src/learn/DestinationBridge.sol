// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "./BridgeToken.sol";

/// @title DestinationBridge — 目标链桥合约（铸造-销毁模式）
/// @notice 监听到源链锁定事件后，铸造等量包装代币
contract DestinationBridge is Ownable {
    // ==================== 状态变量 ====================

    BridgeToken public token;

    /// @notice 已处理的铸造 txId（防重放）
    mapping (bytes32 => bool) public processedMint;

    /// @notice 已处理的赎回 txId（防重放）
    mapping (bytes32 => bool) public processBurn;

    uint256 public nonce;

    // ==================== 事件 ====================

    event TokenMinted(bytes32 indexed txId, address indexed recipient, uint256 amount);

    event TokenBurned(bytes32 indexed txId, address indexed sender, uint256 amount, address indexed recipientOnSourceChain);

    // ==================== 构造函数 ====================

    constructor(address _token, address initialOwner) Ownable(initialOwner) {
        require(_token != address(0), "DestinationBridge: zero token address");
        token = BridgeToken(_token);
    }

    // ==================== 核心功能 ====================

    /// @notice 铸造包装代币（中继者调用）
    /// @dev 只有 owner 能调用，生产环境需要签名验证
    function mint(bytes32 txId, address recipient, uint256 amount) external onlyOwner {
        require(!processedMint[txId], "DestinationBridge: already minted");
        require(recipient != address(0), "DestinationBridge: zero recipient");
        require(amount > 0, "DestinationBridge: amount is zero");

        processedMint[txId] = true;

        // 调用 BridgeToken 的 bridgeMint
        token.bridgeMint(recipient, amount);

        emit TokenMinted(txId, recipient, amount);
    }

    /// @notice 销毁包装代币，发起赎回（用户调用）
    /// @param amount 要赎回的金额
    /// @param recipientOnSourceChain 源链上的接收地址
    function burn(uint256 amount, address recipientOnSourceChain) external returns (bytes32 txId) {
        require(amount > 0, "DestinationBridge: amount is zero");
        require(recipientOnSourceChain != address(0), "DestinationBridge: zero recipient");

        txId = keccak256(abi.encodePacked(msg.sender, amount, recipientOnSourceChain, block.chainid, address(this), nonce++));
    
        // 先销毁用户的包装代币
        token.bridgeBurn(msg.sender, amount);

        emit TokenBurned(txId, msg.sender, amount, recipientOnSourceChain);
    }

    /// @notice 查询目标链上已铸造的包装代币总量
    function mintedSupply() external view returns (uint256) {
        return token.totalSupply();
    }
}