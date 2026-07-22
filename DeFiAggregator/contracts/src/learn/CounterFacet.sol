// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title CounterFacet — 计数器 Facet
/// @notice 使用 Diamond Storage 模式存储数据
library CounterStorage {
    bytes32 internal constant STORAGE_POSITION = keccak256("diamond.standard.counter.storage");

    struct Layout {
        uint256 value;
    }

    function layout() internal pure returns (Layout storage s) {
        bytes32 position = STORAGE_POSITION;
        assembly {
            s.slot := position
        }
    }
}

contract CounterFacet {
    event Incremented(address indexed caller, uint256 newValue);

    /// @notice 递增计数器
    function increment() external returns (uint256) {
        CounterStorage.Layout storage s = CounterStorage.layout();
        s.value += 1;
        emit Incremented(msg.sender, s.value);
        return s.value;
    }

    /// @notice 查询当前计数值
    function getValue() external view returns (uint256) {
        return CounterStorage.layout().value;
    }

    /// @notice 设置计数值（用于测试初始状态）
    function setValue(uint256 _value) external {
        CounterStorage.layout().value = _value;
    }
}