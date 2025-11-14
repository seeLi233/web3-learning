// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

contract MetaNodeToken is ERC20, AccessControl {
    // 定义铸造角色
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

    constructor(address initialOwner) ERC20("MateNode", "MTA") {
        _grantRole(DEFAULT_ADMIN_ROLE, initialOwner);
        // 部署者默认获得铸造权限
        _grantRole(MINTER_ROLE, initialOwner);
        // 预先 mint 一些代币到分配地址
        _mint(initialOwner, 100_000_000);
    }

    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        _mint(to, amount);
    }
}