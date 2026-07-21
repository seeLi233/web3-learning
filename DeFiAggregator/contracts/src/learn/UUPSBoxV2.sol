// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

/// @title UUPSBoxV2 — 可升级计数器（版本 2）
/// @notice 新增递减功能 + 增值事件带版本号
/// @dev 继承于 V1 的存储布局，新增 variables 追加在末尾
contract UUPSBoxV2 is Initializable, UUPSUpgradeable, OwnableUpgradeable {
    // ==================== 存储变量（与 V1 保持一致！）====================
    // ⚠️ 关键规则：
    // 1. 不能删除已有变量
    // 2. 不能改变变量类型
    // 3. 不能重排顺序
    // 4. 新变量只能追加在末尾

    uint256 public value;
    string public constant VERSION = "2.0.0";   // constant 可以改——不在存储里

    // 🆕 V2 新增变量（追加在末尾）
    uint256 public lastDecrementedAt;   // 最后一次递减的时间戳
    int256 public netChang;             // 净变化量（可正可负）

    // ==================== 事件 ====================
    event ValueChanged(uint256 oldValue, uint256 newValue, string version);
    event Incremented(uint256 amount);
    event Decremented(uint256 amount);

    // ==================== 初始化器 ====================
    // ⚠️ 注意：V2 不需要重新初始化！升级时不会调用 initialize()
    // 以下 initialize 仅用于全新的部署（例如测试环境直接从 V2 开始）

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(uint256 initialValue) public initializer {
        __Ownable_init(msg.sender);
        value = initialValue;
        lastDecrementedAt = 0;
        netChang = 0;
    }

    // ==================== 业务逻辑（保留 V1 功能）====================

    function increment(uint256 amount) external {
        uint256 oldValue = value;
        value += amount;
        netChang += int256(amount);
        emit ValueChanged(oldValue, value, VERSION);
        emit Incremented(amount);
    }

    // ==================== 🆕 V2 新功能 ====================

    /// @notice 减少计数
    /// @param amount 减少量
    function decrement(uint256 amount) external {
        require(value >= amount, "Underflow: insufficient value");

        uint256 oldValue = value;
        value -= amount;
        netChang -= int256(amount);
        lastDecrementedAt = block.timestamp;

        emit ValueChanged(oldValue, value, VERSION);
        emit Decremented(amount);
    }

    /// @notice 查询当前值
    function getValue() external view returns (uint256) {
        return value;
    }

    /// @notice 🆕 V2 新增：获取合约统计信息
    function getStats() external view returns (
        uint256 currentValue,
        string memory version,
        int256 totalNetChange,
        uint256 lastDecrementTime
    ) {
       return (value, VERSION, netChang, lastDecrementedAt);
    }

    // ==================== 升级授权 ====================

    function _authorizeUpgrade(address newImplementation) internal override onlyOwner { }
}