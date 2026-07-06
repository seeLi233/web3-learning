// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @title MockToken
 * @notice 用于测试 AMM 的模拟代币
 * 任何人都可以铸造，仅用于本地测试
 */
contract MockToken is ERC20 {
    // 小数位常量
    uint8 private _decimals;

    constructor(string memory name, string memory symbol, uint8 decimals_ ) ERC20(name, symbol) {
        _decimals = decimals_;
    }

    function decimals() public view virtual override returns (uint8) {
        return _decimals;
    }

    /// @notice 任何人都可以铸造测试代币
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}