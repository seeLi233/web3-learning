// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title OverflowSecure
 * @notice 正确使用 unchecked + Solidity 0.8+ 自动检查的示范
 */
contract OverflowSecure {
    mapping (address => uint256) public balances;
    uint256 public totalSupply;
    uint256 public totalTransfers; // 从 0 开始，不可能溢出

    // ✅ Solidity 0.8+ 自动检查溢出
    function mint(address to, uint256 amount) external {
        balances[to] += amount; // 编译器插入溢出检查；正确使用 to 参数
        totalSupply += amount;
    }

    // ✅ 安全的批量转账（0.8+ 自动防溢出）
    function batchTransfer(address[] calldata receivers, uint256 amount) external {
        // 乘法也有溢出检查！直接用 Solidity 原生运算
        uint256 total = receivers.length * amount;
        // 如果溢出，上面那行已经 revert 了，不会走到这里
        require(balances[msg.sender] >= total, "Insufficient balance");

        for(uint256 i = 0; i < receivers.length; ) {
            balances[msg.sender] -= amount;
            balances[receivers[i]] += amount;
            unchecked { ++i; } // ✅ 循环自增用 unchecked 省 gas
        }
    }

    // ✅ 明确不会溢出的场景用 unchecked
    function incrementTotalTransfers() external {
        unchecked { totalTransfers++; }
        // totalTransfers 从 0 开始，每次 +1
        // uint256 最大值 = 1.15*10^77，永远达不到
    }

    // 🔴 反例：不要在 unchecked 里做用户输入相关的运算
    function badExample_dontDoThis(uint256 amount) external {
        // unchecked {
        //     balances[msg.sender] += amount;  // ❌ 用户输入的 amount 可能很大！
        // }
    }

    receive() external payable {}
}