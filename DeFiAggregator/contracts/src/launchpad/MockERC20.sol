// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @title MockERC20
 * @notice 测试用 ERC20 代币，有公开 mint 函数
 * @dev 仅用于测试环境，生产环境绝不使用！
 *      为什么单独写一个 mock 而不是用 DeFiToken？
 *      → DeFiToken 有复杂的权限和 mint 限制（onlyOwner 等）
 *      → MockERC20 的 mint 是 public 的，测试中任何人都可以铸币
 *      → 测试应该简单可控，不依赖其他合约的复杂逻辑
 */
contract MockERC20 is ERC20 {
    constructor() ERC20("Mock Token", "MTK") {}

    /**
     * @notice 公开铸币（仅测试用）
     * @dev 生产环境中 mint 必须加访问控制
     */
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}