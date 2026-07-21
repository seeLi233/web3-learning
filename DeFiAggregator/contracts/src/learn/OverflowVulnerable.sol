// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title OverflowVulnerable
 * @notice 模拟 0.7.x 时代的溢出漏洞（教学用途）
 * @dev 用 assembly 绕过 0.8+ 内置检查，方便教学演示
 */
contract OverflowVulnerable {
    // Token 余额
    mapping (address => uint256) public balances;
    uint256 public totalSupply;

    // 向用户铸造代币
    function mint(address to, uint256 amount) external {
        balances[to] = _unsafeAdd(balances[to], amount);
        totalSupply = _unsafeAdd(totalSupply, amount);
    }

    /// @notice 批量转账 — 经典的 BEC 代币漏洞点
    /// @param receivers 接收者数组
    /// @param amount 每人转账金额
    function batchTransfer(address[] calldata receivers, uint256 amount) external {
        uint256 total = _unsafeMul(uint256(receivers.length), amount);
        // 🔴 漏洞：如果 receivers.length * amount 溢出，total 会变成很小的数
        // 攻击者可以用很少的代币满足 require，但转账循环中每人转的是 amount
        require(balances[msg.sender] >= total, "Insufficient balance");

        // 但这里转转的是 amount（原始值），不是 total！
        for (uint256 i = 0; i < receivers.length; ) {
            balances[msg.sender] = _unsafeSub(balances[msg.sender], amount);
            balances[receivers[i]] = _unsafeAdd(balances[receivers[i]], amount);
            unchecked { ++i; }
        }
    }
    // ======== 以下是用 assembly 模拟 0.7.x 的无检查运算 ========

    function _unsafeAdd(uint256 a, uint256 b) internal pure returns (uint256 c) {
        assembly { c := add(a, b) } // 纯加法，没有溢出检查
    }

    function _unsafeSub(uint256 a, uint256 b) internal pure returns (uint256 c) {
        assembly { c := sub(a, b) } // 纯减法，没有下溢检查
    }

    function _unsafeMul(uint256 a, uint256 b) internal pure returns (uint256 c) {
        assembly { c := mul(a, b) } // 纯乘法，没有溢出检查
    }

    // 方便测试：存 ETH
    receive() external payable {}

    // 提币函数 — 也有下溢漏洞
    function withdraw(uint256 amount) external {
        require(balances[msg.sender] >= amount, "Insufficient");
        balances[msg.sender] = _unsafeSub(balances[msg.sender], amount);
        payable(msg.sender).transfer(amount);
    }
}