// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

/**
 * @title ReentrancyVault
 * @notice 故意留了重入漏洞的银行合约 — 用于教学演示
 * @dev 漏洞：withdraw() 先转账后更新余额，违反 CEI 模式
 */
contract ReentrancyVault {
    mapping (address => uint256) public balances;

    event Deposited(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);

    /// @notice 存款
    function deposit() external payable {
        require(msg.value > 0, "Must send ETH");
        balances[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    /// @notice 取款 — ⚠️ 有漏洞！
    /// @dev 先执行外部调用（INTERACTIONS），后更新状态（EFFECTS）
    function withdraw() external payable {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "No balance");

        // ❌ 漏洞：先转账...
        (bool success, ) = msg.sender.call{value:amount}("");
        require(success, "Transfer failed");

        // ❌ 后记账 → 攻击者已在 receive() 中重入，此时余额早已被提空
        balances[msg.sender] = 0;

        emit Withdrawn(msg.sender, msg.value);
    }

    /// @notice 查看合约余额
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}