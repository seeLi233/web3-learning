// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title 利率模型统一接口
/// @notice 借贷协议通过此接口调用利率模型，可插拔不同实现
interface IInterestRateModel {
    /// @notice 计算当前借款利率（年化，单位：ray = 1e27）
    /// @param cash 池子里的现金余额
    /// @param borrows 已借出总量
    /// @param reserves 储备金总量
    /// @return borrowRate 借款年化利率 (ray)
    /// @return supplyRate 存款年化利率 (ray)
    function getRates(uint256 cash, uint256 borrows, uint256 reserves) external view returns (uint256 borrowRate, uint256 supplyRate);

    /// @notice 计算利用率
    /// @param cash 现金余额
    /// @param borrows 已借出总量
    /// @param reserves 储备金总量
    /// @return 利用率 (ray)
    function getUtilizationRate(uint256 cash, uint256 borrows, uint256 reserves) external pure returns (uint256);
}