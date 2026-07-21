// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

/// @title UUPSBoxV1 — 可升级计数器（版本 1）
/// @notice 只支持递增，不支持递减
/// @dev 使用 UUPS 代理模式 + 初始化器模式
contract UUPSBoxV1 is Initializable, UUPSUpgradeable, OwnableUpgradeable {
    // ==================== 存储变量 ====================
    // ⚠️ 重要：永远不要删除或重新排列已有的存储变量！
    // 新版本只能追加新变量到末尾

    uint256 public value;   // 计数器值
    string public constant VERSION = "1.0.0";   // 版本号（constant 不影响存储）

    // ==================== 事件 ====================
    event ValueChanged(uint256 oldValue, uint256 newValue);
    event Incremented(uint256 amount);

    // ==================== 初始化器 ====================
    // ⚠️ 代理模式不能用 constructor！
    // constructor 在 Logic 合约部署时执行，不在 Proxy 的上下文中
    // 必须用 initialize() 函数代替

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        // constructor 在代理模式下不会被调用
        // 这里什么都不做，只是为了兼容性
        _disableInitializers();
    }

    /// @notice 初始化函数 — 替代 constructor
    /// @param initialValue 初始计数值
    function initialize(uint256 initialValue) public initializer {
        // 调用父类的初始化器
        __Ownable_init(msg.sender);     // 初始 owner = 部署者
        value = initialValue;
    }

    // ==================== 业务逻辑 ====================

    /// @notice 增加计数
    /// @param amount 增加量
    function increment(uint256 amount) external {
        uint256 oldValue = value;
        value += amount;
        emit ValueChanged(oldValue, value);
        emit Incremented(amount);
    }

    /// @notice 查询当前值
    function getValue() external view returns (uint256) {
        return value;
    }

    // ==================== UUPS 升级授权 ====================
    // ⭐ 这是 UUPS 的核心：自定义升级权限！

    /// @notice 授权升级 — 只有 owner 能升级
    /// @param newImplementation 新的逻辑合约地址
    function _authorizeUpgrade(address newImplementation) internal override onlyOwner {
        // 可以在这里添加更多检查：
        // require(newImplementation != address(0), "zero address");
        // require(isRegistered(newImplementation), "not registered");
    }
}