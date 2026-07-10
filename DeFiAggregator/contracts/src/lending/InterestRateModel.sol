// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./IInterestRateModel.sol";

/// @title 可变利率模型 — 拐点模型
/// @notice Compound/Aave 风格的多段线性利率模型
///
/// @dev 利率公式 (所有值以 ray = 1e27 表示):
///
///  利用率 U = totalBorrows / (cash + totalBorrows - reserves)
///
///  如果 U ≤ optimalUtilizationRate:
///    borrowRate = baseRate + U × slope1
///
///  如果 U > optimalUtilizationRate:
///    borrowRate = baseRate + optimalUtilizationRate × slope1
///               + (U - optimalUtilizationRate) × slope2
///
///  存款利率:
///    supplyRate = borrowRate × U × (1e27 - reserveFactor)
///
///  例子 (用 1e18 便于理解):
///    baseRate = 2%, slope1 = 10%, slope2 = 50%, optimal = 80%
///    U = 50%  → borrowRate = 2% + 50%×10% = 7%
///    U = 95%  → borrowRate = 2% + 80%×10% + 15%×50% = 17.5%
contract InterestRateModel is IInterestRateModel {
    // ============ 精度常量 ============
    uint256 public constant RAY = 1e27;

    // ============ 模型参数 (以 RAY 为单位) ============

    /// @notice 基础年化利率 (U=0% 时的最低利率)
    /// @dev 0.02 RAY = 2% 年化
    uint256 public immutable baseRatePerYear;

    /// @notice 拐点前斜率 — 利用率每增加 1%，利率增加 slope1%
    /// @dev 0.10 RAY = 每 1% 利用率增加 0.10% 利率
    uint256 public immutable multiplierPerYear;

    /// @notice 拐点后斜率 — 超过拐点后的惩罚性斜率
    uint256 public immutable jumpMultiplierPerYear;

    /// @notice 拐点利用率 — 超过此值后切换到高斜率
    uint256 public immutable optimalUtilizationRate;

    /// @notice 储备因子 — 协议从利息中的抽成比例
    /// @dev 0.10 RAY = 10%
    uint256 public immutable reserveFactor;

    // ============ 构造函数 ============

    /// @param _baseRatePerYear 基础年化利率 (ray)
    /// @param _multiplierPerYear 拐点前斜率 (ray)
    /// @param _jumpMultiplierPerYear 拐点后斜率 (ray)
    /// @param _optimalUtilizationRate 拐点利用率 (ray)
    /// @param _reserveFactor 储备因子 (ray)
    constructor(
        uint256 _baseRatePerYear,
        uint256 _multiplierPerYear,
        uint256 _jumpMultiplierPerYear,
        uint256 _optimalUtilizationRate,
        uint256 _reserveFactor
    ) {
        // 参数校验
        require(_optimalUtilizationRate > 0 && _optimalUtilizationRate <= RAY, "Invalid optimal rate");
        require(_reserveFactor <= RAY, "Invalid reserve factor");

        baseRatePerYear = _baseRatePerYear;
        multiplierPerYear = _multiplierPerYear;
        jumpMultiplierPerYear = _jumpMultiplierPerYear;
        optimalUtilizationRate = _optimalUtilizationRate;
        reserveFactor = _reserveFactor;
    }

    // ============ 核心计算函数 ============

    /// @inheritdoc IInterestRateModel
    function getUtilizationRate(uint256 cash, uint256 borrows, uint256 reserves) public pure override returns (uint256) {
        if (borrows == 0) return 0;

        uint256 totalLiquidity = cash + borrows - reserves;
        if (totalLiquidity == 0) return 0;

        // U = borrows / (cash + borrows - reserves)  (以 RAY 精度)
        return (borrows * RAY) / totalLiquidity;
    }

    /// @inheritdoc IInterestRateModel
    function getRates(uint256 cash, uint256 borrows, uint256 reserves) public view override returns (uint256 borrowRate, uint256 supplyRate) {
        // Step 1: 计算利用率
        uint256 utilizationRate = getUtilizationRate(cash, borrows, reserves);

        // Step 2: 根据利用率计算借款利率
        borrowRate = _getBorrowRate(utilizationRate);

        // Step 3: 存款利率 = 借款利率 × 利用率 × (1 - reserveFactor)
        // supplyRate = borrowRate × U × (1 - reserveFactor)
        uint256 oneMinusReserveFactor = RAY - reserveFactor;
        supplyRate = (borrowRate * utilizationRate) / RAY;
        supplyRate = (supplyRate * oneMinusReserveFactor) / RAY;
    }

    /// @notice 根据利用率计算借款利率
    /// @param utilizationRate 当前利用率 (ray)
    /// @return borrowRate 借款年化利率 (ray)
    function _getBorrowRate(uint256 utilizationRate) internal view returns (uint256) {
        // 情况 1: 利用率 ≤ 拐点 → 低斜率
        if (utilizationRate <= optimalUtilizationRate) {
            // borrowRate = baseRate + utilizationRate × slope1
            return baseRatePerYear + (utilizationRate * multiplierPerYear) / RAY;
        }

        // 情况 2: 利用率 > 拐点 → 基础 + 拐点内 + 拐点外
        // 拐点内的利率增量
        uint256 normalRate = baseRatePerYear + (optimalUtilizationRate * multiplierPerYear) / RAY;

        // 拐点外的超额利用率
        uint256 excessUtilization = utilizationRate - optimalUtilizationRate;

        // 超额部分用高斜率
        uint256 excessRate = (excessUtilization * jumpMultiplierPerYear) / RAY;

        return normalRate + excessRate;
    }

    // ============ 查询函数 (供外部调用) ============

    /// @notice 模拟计算：如果借出 amount，新的利率是多少？
    /// @param cash 当前现金
    /// @param borrows 当前借款
    /// @param amount 准备借出的金额（0 则只计算当前利率）
    /// @return newBorrowRate 借款后的新借款利率
    /// @return newSupplyRate 借款后的新存款利率
    function simulateBorrow(uint256 cash, uint256 borrows, uint256 amount) external view returns (uint256 newBorrowRate, uint256 newSupplyRate) {
        require(amount <= cash, "Insufficient cash for simulation");
        uint256 newBorrows = borrows + amount;
        uint256 newCash = cash - amount;
        return getRates(newCash, newBorrows, 0);
    }

    /// @notice 模拟计算：如果还款 amount，新的利率是多少？
    function simulateRepay(uint256 cash, uint256 borrows, uint256 amount) external view returns (uint256 newBorrowRate, uint256 newSupplyRate) {
        uint256 newBorrows = borrows > amount ? borrows - amount : 0;
        uint256 newCash = cash + amount;
        return getRates(newCash, newBorrows, 0);
    }
}