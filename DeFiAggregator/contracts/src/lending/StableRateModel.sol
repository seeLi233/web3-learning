// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./InterestRateModel.sol";

/// @title 稳定利率模型 — Aave 风格
/// @notice 扩展可变利率模型，新增稳定利率计算
///
/// @dev 稳定利率 = 当前可变利率 + 稳定溢价
///
/// 稳定利率并不"永久稳定"：
///   1. 借款时锁定利率 = 当前可变利率 + 溢价
///   2. 如果利用率超过 rebalanceThreshold，协议可重新调整
///   3. 稳定利率总借款单独计算加权平均值
///
/// 核心设计思想:
///   - 为长期借款用户提供利率确定性
///   - 协议通过溢价获得更高收益
///   - 通过 rebalance 防止市场极端波动时的套利
contract StableRateModel is InterestRateModel {
    // ============ 稳定利率参数 ============

    /// @notice 稳定利率溢价 — 在可变利率基础上额外加收
    /// @dev 0.02 RAY = 2% 溢价。如果可变利率 10%，稳定利率 = 12%
    uint256 public immutable stableRatePremium;

    /// @notice 再平衡阈值 — 利用率超过此值，协议可调整稳定利率
    /// @dev 0.95 RAY = 95%
    uint256 public immutable rebalanceThreshold;

    /// @notice 再平衡后的稳定利率上限 — 调整后稳定利率不能超过此值
    /// @dev 防止稳定利率无限飙高
    uint256 public immutable stableRateCap;

    // ============ 构造函数 ============

    /// @param _baseRatePerYear 基础年化利率 (ray)
    /// @param _multiplierPerYear 拐点前斜率 (ray)
    /// @param _jumpMultiplierPerYear 拐点后斜率 (ray)
    /// @param _optimalUtilizationRate 拐点利用率 (ray)
    /// @param _reserveFactor 储备因子 (ray)
    /// @param _stableRatePremium 稳定利率溢价 (ray)
    /// @param _rebalanceThreshold 再平衡阈值 (ray)
    /// @param _stableRateCap 稳定利率上限 (ray)
    constructor(
        uint256 _baseRatePerYear,
        uint256 _multiplierPerYear,
        uint256 _jumpMultiplierPerYear,
        uint256 _optimalUtilizationRate,
        uint256 _reserveFactor,
        uint256 _stableRatePremium,
        uint256 _rebalanceThreshold,
        uint256 _stableRateCap
    ) InterestRateModel(_baseRatePerYear, _multiplierPerYear, _jumpMultiplierPerYear, _optimalUtilizationRate, _reserveFactor) {
        require(_rebalanceThreshold <= RAY, "Invalid rebalance threshold");
        require(_stableRateCap <= RAY, "Invalid stable rate cap");

        stableRatePremium = _stableRatePremium;
        rebalanceThreshold = _rebalanceThreshold;
        stableRateCap = _stableRateCap;
    }

    // ============ 稳定利率计算 ============

    /// @notice 计算当前稳定借款利率
    /// @param cash 现金余额
    /// @param borrows 已借出总量
    /// @param reserves 储备金总量
    /// @return stableBorrowRate 稳定借款年化利率 (ray)
    function getStableRate(
        uint256 cash,
        uint256 borrows,
        uint256 reserves
    ) public view returns (uint256 stableBorrowRate) {
        // 先获取当前可变利率
        (uint256 variableRate, ) = getRates(cash, borrows, reserves);

        // 稳定利率 = 可变利率 + 溢价
        stableBorrowRate = variableRate + stableRatePremium;

        // 但不能超过上限
        if (stableBorrowRate > stableRateCap) {
            stableBorrowRate = stableRateCap;
        }
    }

    /// @notice 检查是否需要调整已锁定的稳定利率
    /// @dev 当利用率 > rebalanceThreshold 时，协议可以重新给稳定借款定价
    /// @param cash 现金余额
    /// @param borrows 已借出总量
    /// @param lockedStableRate 用户锁定时的稳定利率
    /// @return needsRebalance 是否需要重新调整
    /// @return newStableRate 调整后的新稳定利率（如果需要）
    function checkRebalance(uint256 cash, uint256 borrows, uint256 lockedStableRate) external view returns (bool needsRebalance, uint256 newStableRate) {
        uint256 utilizationRate = getUtilizationRate(cash, borrows, 0);

        // 利用率超过阈值 → 需要重新定价
        if (utilizationRate > rebalanceThreshold) {
            uint256 currentStableRate = getStableRate(cash, borrows, 0);

            // 只有新利率高于用户锁定的利率时才调整
            // （只向上调整，保护协议利益）
            if (currentStableRate > lockedStableRate) {
                return (true, currentStableRate);
            }
        }

        return (false, lockedStableRate);
    }

    // ============ 批量查询 ============

    /// @notice 一次性获取所有利率信息
    /// @return variableBorrowRate 可变借款利率
    /// @return stableBorrowRate 稳定借款利率
    /// @return supplyRate 存款利率
    /// @return utilizationRate 当前利用率
    function getAllRates(uint256 cash, uint256 borrows, uint256 reserves) external view returns (uint256 variableBorrowRate, uint256 stableBorrowRate, uint256 supplyRate, uint256 utilizationRate) {
        utilizationRate = getUtilizationRate(cash, borrows, reserves);
        (variableBorrowRate, supplyRate) = getRates(cash, borrows, reserves);
        stableBorrowRate = getStableRate(cash, borrows, reserves);
    }
}