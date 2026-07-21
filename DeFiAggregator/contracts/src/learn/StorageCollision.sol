// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title StorageCollision — 存储布局冲突演示
/// @notice 故意制造存储冲突，直观展示"变量顺序不一致"的灾难后果
/// @dev ⚠️ 仅用于教学演示，展示不遵循存储布局铁律的后果

// ==================== V1: 正常版本 ====================
contract StorageV1 {
    uint256 public valueA;  // slot 0
    uint256 public valueB;  // slot 1
    address public owner;   // slot 2

    // 初始化函数
    function init(uint256 a, uint256 b) external {
        valueA = a;
        valueB = b;
        owner = msg.sender;
    }

    function getSlot0() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(0) }
        return val;
    }

    function getSlot1() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(1) }
        return val;
    }

    function getSlot2() external view returns (address) {
        address val;
        assembly { val := sload(2) }
        return val;
    }
}

// ==================== V2: ⚠️ 危险版本——变量顺序被打乱！ ====================
contract StorageV2Broken {
    // ⚠️⚠️⚠️ 变量顺序与 V1 不同！
    // V1: slot 0 = valueA, slot 1 = valueB, slot 2 = owner
    // V2: slot 0 = owner,   slot 1 = valueA, slot 2 = valueB
    //
    // 如果升级到 V2 后读 valueA → 实际上读的是 V1 的 owner！（存储污染）
    address public owner;       // slot 0 ← 本应该是 valueA！
    uint256 public valueA;      // slot 1 ← 本应该是 valueB！
    uint256 public valueB;      // slot 2 ← 本应该是 owner！

    function getSlot0() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(0) }
        return val;
    }

    function getSlot1() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(1) }
        return val;
    }

    function getSlot2() external view returns (address) {
        address val;
        assembly { val := sload(2) }
        return val;
    }
}

// ==================== V2: ✅ 正确版本——变量顺序不变 ====================
contract StorageV2Correct {
    // ✅ 变量顺序与 V1 完全一致
    uint256 public valueA;    // slot 0
    uint256 public valueB;    // slot 1
    address public owner;     // slot 2

    // ✅ 新变量追加在末尾
    string public description; // slot 3

    function getSlot0() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(0) }
        return val;
    }

    function getSlot1() external view returns (uint256) {
        uint256 val;
        assembly { val := sload(1) }
        return val;
    }

    function getSlot2() external view returns (address) {
        address val;
        assembly { val := sload(2) }
        return val;
    }
}