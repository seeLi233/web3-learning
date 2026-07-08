// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title SlippageHelper
 * @notice 滑点计算工具库 — 纯函数，gas 高效
 * @dev 供前端/合约计算 minAmountOut 等滑点参数
 * 
 * 核心概念:
 *   - bps (basis points): 1 bps = 0.01%
 *   - 50 bps = 0.5% = 50/10000
 *   - maxBps = 10000 (100%)
 *
 * 使用示例:
 *   // 期望输出 1000 USDT，允许 0.5% 滑点
 *   minOut = SlippageHelper.applySlippage(1000, 50);
 *   // → 1000 * (10000 - 50) / 10000 = 995
 */
library SlippageHelper {
    /// @notice 最大 bps（100%）
    uint256 public constant MAX_BPS = 10_000;

    // ============ 自定义 Error ============
    error SlippageTooHigh();        // 滑点超过 100%（不可能）
    error InvalidBps();             // bps 超过 MAX_BPS

    // ============ 核心函数 ============

    /// @notice 根据滑点计算最小输出量
    /// @param expectedAmount 期望得到的数量
    /// @param slippageBps 滑点容忍度（basis points），如 50 = 0.5%
    /// @return minAmount 最小可接受的数量
    ///
    /// 公式: minAmount = expectedAmount * (10000 - slippageBps) / 10000
    ///
    /// 例: expectedAmount=1000e18, slippageBps=50
    ///     → 1000e18 * 9950 / 10000 = 995e18
    function applySlippage(uint256 expectedAmount, uint256 slippageBps) internal pure returns (uint256 minAmount) {
        if (slippageBps > MAX_BPS) revert InvalidBps();
        // 如果滑点为 0，直接返回期望值，省 gas
        if (slippageBps == 0) return expectedAmount;
        minAmount = (expectedAmount * (MAX_BPS - slippageBps)) / MAX_BPS;
    }

    /// @notice 根据滑点计算最大输入量（反向滑点）
    /// @param expectedAmount 期望输入的数量
    /// @param slippageBps 滑点容忍度（basis points），如 50 = 0.5%
    /// @return maxAmount 最大可接受的输入量
    ///
    /// 公式: maxAmount = expectedAmount * (10000 + slippageBps) / 10000
    ///
    /// 场景: 你要卖代币换另一种，你不知道能换到多少（输出不确定），
    ///       但你能控制最多花多少输入。此时用 maxAmountIn。
    function applySlippageReverse(uint256 expectedAmount, uint256 slippageBps) internal pure returns (uint256 maxAmount) {
        if (slippageBps > MAX_BPS) revert InvalidBps();
        maxAmount = expectedAmount * (MAX_BPS + slippageBps) / MAX_BPS;
    }

    // ============ 高级功能 ============

    /// @notice 计算两个量的滑点百分比（bps）
    /// @param expected 预期的量
    /// @param actual 实际的量
    /// @return slippageBps 滑点（bps）
    ///
    /// 公式: slippageBps = (expected - actual) / expected * 10000
    ///
    /// 例: expected=1000, actual=990
    ///     → (1000 - 990) * 10000 / 1000 = 100 bps = 1%
    function calculatedSlippage(uint256 expected, uint256 actual) internal pure returns (uint256 slippageBps) {
        if (expected == 0) return 0;
        if (actual >= expected) return 0;   // 正向滑点（实际输出多于期望）= 好事！
        // 注意: SafeMath 虽在 0.8+ 已内置溢出检查，这里递减顺序确保不 revert
        slippageBps = ((expected - actual) * MAX_BPS) / expected;
    }

    /// @notice 判断实际输出是否满足滑点要求
    /// @param expectedAmount 期望数量
    /// @param actualAmount 实际数量
    /// @param maxSlippageBps 最大容忍滑点
    /// @return acceptable 是否可以接受
    function isAcceptable(uint256 expectedAmount, uint256 actualAmount, uint256 maxSlippageBps) internal pure returns (bool acceptable) {
        uint256 minAcceptable = applySlippage(expectedAmount, maxSlippageBps);
        return actualAmount >= minAcceptable;
    }

    // ============ 场景化函数 ============

    /// @notice 为 AMM swap 计算安全的 minAmountOut
    /// @param amountIn 输入量
    /// @param reserveIn 输入代币储备
    /// @param reserveOut 输出代币储备
    /// @param feeBps 手续费(bps)，如 30 = 0.3%
    /// @param slippageBps 滑点容忍(bps)，如 50 = 0.5%
    /// @return minAmountOut 最小输出量
    /// @return expectedOut 期望输出量（不含滑点）
    ///
    /// 公式:
    ///   1. amountInWithFee = amountIn * (10000 - feeBps)
    ///   2. expectedOut = reserveOut * amountInWithFee / (reserveIn * 10000 + amountInWithFee)
    ///   3. minAmountOut = expectedOut * (10000 - slippageBps) / 10000
    function computeMinAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut, uint256 feeBps, uint256 slippageBps) internal pure returns (uint256 minAmountOut, uint256 expectedOut) {
        // Step 1: 扣除手续费后的有效输入
        uint256 amountInWithFee = amountIn * (MAX_BPS - feeBps);

        // Step 2: 恒定乘积公式算期望输出
        expectedOut = (amountInWithFee * reserveOut) / (reserveIn * MAX_BPS + amountInWithFee);

        // Step 3: 应用滑点容忍
        minAmountOut = applySlippage(expectedOut, slippageBps);
    }
}