// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IOverflowVulnerable {
    function batchTransfer(address[] calldata receivers, uint256 amount) external;
    function balances() external view returns (uint256);
}

/**
 * @title OverflowAttacker
 * @notice 利用 batchTransfer 溢出漏洞的攻击合约
 */
contract OverflowAttacker {
    IOverflowVulnerable public token;
    
    constructor(address _token) {
        token = IOverflowVulnerable(_token);
    }

    /// @notice 执行 BEC 式溢出攻击
    /// @dev 原理：找到合适的 receivers.length 使 length * amount 溢出成一个很小的值
    function attackBECStyle(uint256 targetSelf, uint256 targetBalance) external {
        // 计算溢出参数：我们需要 length * amount 溢出到 targetSelf
        // uint256 最大值 = 2^256 - 1
        // 溢出公式：len * amount = targetSelf (mod 2^256)
        // 即：amount = targetSelf / len (但需要精确定向溢出)

        // 简化方案：直接找溢出点
        // 如果 receivers.length * amount > type(uint256).max 且溢出后 <= 攻击者的余额
        // 攻击者就能通过检查，然后向每个接收者转 amount

        // 实际攻击中，攻击者：
        // 1. 先 mint 一点代币给自己
        // 2. 计算溢出参数
        // 3. 调用 batchTransfer
    }

    receive() external payable{}
}