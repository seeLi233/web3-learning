// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/// @title SimpleFlashBorrower
/// @notice 简单的闪电贷借款人 — 纯粹借了再还，用于测试
contract SimpleFlashBorrower {
    using SafeERC20 for IERC20;

    event FlashLoanReceived(address token, uint256 amount, uint256 fee);

    function onFlashLoan(
        address /* initiator */,
        address token,
        uint256 amount,
        uint256 fee,
        bytes calldata /* data */
    ) external returns (bytes32) {
        // 计算还款总额
        uint256 totalRepay = amount + fee;

        // ⭐ 关键：把借来的钱 + 手续费转回给 FlashLoan 合约
        // FlashLoan 合约会检查自己的余额，所以必须用 safeTransfer 把 token 还回去
        IERC20(token).safeTransfer(msg.sender, totalRepay);

        emit FlashLoanReceived(token, amount, fee);

        return keccak256("ERC3156FlashBorrower.onFlashLoan");
    }
}