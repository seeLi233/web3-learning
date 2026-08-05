// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title BridgeToken — 支持跨链铸造/销毁的 ERC20 代币
/// @notice 只有被授权的桥合约才能铸造和销毁
contract BridgeToken is ERC20, Ownable {
    // ==================== 状态变量 ====================

    /// @notice 被授权的桥合约地址 → 是否授权
    mapping (address => bool) public bridges;

    // ==================== 事件 ====================

    event BridgeAdded(address indexed bridge);
    event BridgeRemoved(address indexed bridge);

    // ==================== 修饰器 ====================
    modifier onlyBridge {
        require(bridges[msg.sender], "BridgeToken: caller is not a bridge");
        _;
    }

    // ==================== 构造函数 ====================

    constructor(string memory name, string memory symbol, address initialOwner) ERC20(name, symbol) Ownable(initialOwner) {
        // 初始铸造给 owner（模拟源链上的初始供应）
        _mint(initialOwner, 1_000_000 * 10 ** decimals());
    }

    // ==================== 桥管理（仅 Owner） ====================

    function addBridge(address bridge) external onlyOwner {
        require(bridge != address(0), "BridgeToken: zero Address");
        bridges[bridge] = true;
        emit BridgeAdded(bridge);
    }

    function removeBridge(address bridge) external onlyOwner {
        bridges[bridge] = false;
        emit BridgeRemoved(bridge);
    }

    // ==================== 桥铸造/销毁（仅桥合约） ====================

    /// @notice 目标链上铸造包装代币
    function bridgeMint(address to, uint256 amount) external onlyBridge {
        _mint(to, amount);
    }

    /// @notice 目标链上销毁包装代币（用户赎回时）
    function bridgeBurn(address to, uint256 amount) external onlyBridge {
        _burn(to, amount);
    }
}