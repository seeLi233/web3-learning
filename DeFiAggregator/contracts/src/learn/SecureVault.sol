// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import {ReentrancyGuard} from "./ReentrancyGuard.sol";

/**
 * @title SecureVault
 * @notice CEI + ReentrancyGuard 双重防护的安全银行
 * @dev 第一层：CEI 模式阻止单函数重入
 *      第二层：nonReentrant 阻止跨函数重入
 */
contract SecureVault is ReentrancyGuard {
    mapping (address => uint256) public balances;

    event Deposited(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);

    function deposit() external payable {
        require(msg.value > 0, "Must send ETH");
        balances[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    /// @notice 安全取款 — CEI 模式 + ReentrancyGuard
    function withdraw() external nonReentrant {
        // 1. CHECKS
        uint256 amount = balances[msg.sender];
        require(amount > 0, "No balance");

        // 2. EFFECTS（先清零，再转账）
        balances[msg.sender] = 0;

        // 3. INTERACTIONS
        (bool success, ) = msg.sender.call{value:amount}("");
        require(success, "Transfer failed");

        emit Withdrawn(msg.sender, amount);
    }

    /// @notice 另一个敏感函数 — 同样受 nonReentrant 保护
    function emergencyWithdraw(uint256 amount) external nonReentrant {
        require(balances[msg.sender] >= amount, "Insufficient");
        balances[msg.sender] -= amount;
        (bool success, ) = msg.sender.call{value: amount}("");
        require(success, "Transfer failed");
    }

    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}