// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

/**
 * @title ReentrancyGuard
 * @notice 手写版重入锁 — 教学用，理解后再用 OZ 的
 * @dev 用 1/2 而非 0/1 以避免和默认值混淆
 */
abstract contract ReentrancyGuard {
    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;

    uint256 private _status;

    constructor() {
        _status = _NOT_ENTERED;
    }

    /**
     * @notice 防止重入的 modifier
     * @dev 原理：
     *  1. 进入函数时检查 _status 是否为 NOT_ENTERED
     *  2. 设为 ENTERED（上锁）
     *  3. 执行函数体
     *  4. 函数结束后恢复为 NOT_ENTERED（解锁）
     *
     *  如果在步骤 3 中发生外部调用并重入本合约任何 nonReentrant 函数，
     *  步骤 1 的检查会失败（因为 _status 仍是 ENTERED）
     */
    modifier nonReentrant {
        // 进入前检查 → 如果已经进入则拒绝
        require(_status != _ENTERED, "ReentrancyGuard: reentrant call");

        // 上锁
        _status = _ENTERED;
        
        // 执行被保护的逻辑
        _;

        // 解锁（即使 revert 也不会执行这行，但锁状态随交易回滚而恢复）
        _status = _NOT_ENTERED;
    }
}