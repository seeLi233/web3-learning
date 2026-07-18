// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

/**
 * @title CrossReentrancyVault
 * @notice 跨函数重入漏洞演示
 * @dev withdraw() 先转账后记账 → 攻击者在 receive() 中调用 clear() → clear() 读到旧余额
 */
contract CrossReentrancyVault {
    mapping (address => uint256) public balances;
    mapping (address => uint256) public bonus;  // 被污染的辅助状态

    event Deposited(address indexed user, uint256 amount);
    event BonusCleared(address indexed user, uint256 bonusAmount);

    function deposit() external payable {
        require(msg.value > 0, "Must send ETH");
        balances[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    /// @notice 取款 — 漏洞：先转账
    function withdraw() external payable {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "No balance");

        (bool success, ) = msg.sender.call{value: amount}("");
        require(success, "Transfer failed");

        balances[msg.sender] = 0;
    }

    /// @notice 结算奖金 — 用 balances 计算但 withdraw 还没更新！
    /// @dev 如果在 withdraw 的 receive() 回调中调用，会读到旧 balances
    function claimBonus() external {
        uint256 userBalance = balances[msg.sender]; // ← 可能是旧值！
        require(userBalance > 0, "No balance for bonus");

        // 奖金 = 余额的 10%
        uint256 bonusAmount = userBalance / 10;
        bonus[msg.sender] += bonusAmount;

        emit BonusCleared(msg.sender, bonusAmount);
    }

    /// @notice 提现奖金
    function withdrawBonus() external payable {
        uint256 amount = bonus[msg.sender];
        require(amount > 0, "No bonus");
        bonus[msg.sender] = 0;
        (bool success, ) = msg.sender.call{value: amount}("");
        require(success, "Bonus withdraw failed");
    }

    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}