// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "./Delegation.sol";

/**
 * @title TestVoteToken
 * @notice 测试用代币 — 继承 Delegation 实现委托投票
 * @dev 供 Delegation.test.ts 使用
 */
contract TestVoteToken is ERC20, Delegation {
    constructor() ERC20("Test Vote Token", "TVT") {}

    // ============ 铸币/销毁 ============

    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }

    function burn(address from, uint256 amount) external {
        _burn(from, amount);
    }

    // ============ OZ v5 _update hook → Delegation._afterTokenTransfer ============
    /// @dev OZ v5 使用 _update 而非 _afterTokenTransfer；
    ///      Delegation 定义了自定义的 _afterTokenTransfer 来处理投票权重变更，
    ///      需要在 _update 之后手动调用以连接两条链路
    function _update(address from, address to, uint256 amount) internal override {
        super._update(from, to, amount);
        _afterTokenTransfer(from, to, amount);
    }
}
