// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title EIP1967Explainer — 深入理解 EIP-1967 存储槽机制
/// @notice 手动演示 ERC1967 存储槽的读写，验证"暗格"隔离性
/// @dev 教学合约，不用于生产环境
contract EIP1967Explainer {
    // ==================== EIP-1967 标准存储槽 ====================
    // ⭐ 面试必记：这三个槽位的值和计算方式

    /// @dev keccak256("eip1967.proxy.implementation") - 1
    bytes32 private constant IMPLEMENTATION_SLOT = 
        0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;

    /// @dev keccak256("eip1967.proxy.admin") - 1
    bytes32 private constant ADMIN_SLOT =
        0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103;
    
    /// @dev keccak256("eip1967.proxy.beacon") - 1
    bytes32 private constant BEACON_SLOT =
        0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50;

    // ==================== 事件 ====================
    event ImplementationSet(address indexed oldImpl, address indexed newImpl);
    event AdminSet(address indexed oldAdmin, address indexed newAdmin);
    event Debug(string message, uint256 value);

    // ==================== 实现地址管理 ====================

    /// @notice 设置 implementation 地址（模拟代理的升级操作）
    /// @param newImpl 新逻辑合约地址
    function setImplementation(address newImpl) external {
        address oldImpl = getImplementation();
        assembly {
            sstore(IMPLEMENTATION_SLOT, newImpl)
        }
        emit ImplementationSet(oldImpl, newImpl);
    }

    /// @notice 读取 implementation 地址
    function getImplementation() public view returns (address impl) {
        assembly {
            impl := sload(IMPLEMENTATION_SLOT)
        }
    }

    // ==================== Admin 地址管理 ====================

    /// @notice 设置 admin 地址
    function setAdmin(address newAdmin) external {
        address oldAdmin = getAdmin();
        assembly {
            sstore(ADMIN_SLOT, newAdmin)
        }
        emit AdminSet(oldAdmin, newAdmin);
    }

    /// @notice 读取 admin 地址
    function getAdmin() public view returns (address admin) {
        assembly {
            admin := sload(ADMIN_SLOT)
        }
    }

    // ==================== 存储隔离演示 ====================

    // ⚠️ 这些普通变量从 slot 0 开始分配
    uint256 public normalUint;      // slot 0
    address public normalAddress;   // slot 1
    bool public normalBool;         // slot 2

    /// @notice 设置普通变量（slot 0~2）
    function setNormalVars(uint256 u, address a, bool b) external {
        normalUint = u;
        normalAddress = a;
        normalBool = b;
    }

    /// @notice ⭐ 核心演示：读取普通 slot 和 EIP-1967 slot
    /// @dev 证明它们互不干扰
    function demonstrateIsolation() external view returns (
        uint256 slot0,
        uint256 slot1,
        uint256 slot2,
        address impl,
        address admin,
        uint256 implSlotNum,
        uint256 adminSlotNum
    ) {
        // 通过 Solidity 变量读取（slot 0, 1, 2）
        slot0 = normalUint;
        slot1 = uint256(uint160(normalAddress));
        slot2 = normalBool ? 1 : 0;

        // 通过 assembly 读取 EIP-1967 槽
        impl = getImplementation();
        admin = getAdmin();

        // 展示槽位号
        implSlotNum = uint256(IMPLEMENTATION_SLOT);
        adminSlotNum = uint256(ADMIN_SLOT);
    }

    // ==================== 存储槽值计算演示 ====================

    /// @notice ⭐ 面试演示：验证槽位值的计算过程
    /// @dev 手动计算 keccak256(string) - 1，验证等于标准槽值
    function verifySlotCalculation() external pure returns (
        bytes32 implCalculated,
        bytes32 adminCalculated,
        bool implMatches,
        bool adminMatches
    ) {
        // 手动计算：keccak256 返回 bytes32，需要先转 uint256 才能做减法
        implCalculated = bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1);
        adminCalculated = bytes32(uint256(keccak256("eip1967.proxy.admin")) - 1);

        // 与标准值对比
        implMatches = implCalculated == IMPLEMENTATION_SLOT;
        adminMatches = adminCalculated == ADMIN_SLOT;

        // 预期：implMatches == true, adminMatches == true
    }

    // ==================== 存储槽碰撞概率演示 ====================

    /// @notice 展示为什么碰撞概率 ≈ 0
    /// @dev IMPLEMENTATION_SLOT 的值远大于合约可能的最大 slot 数
    function demonstrateNoCollision() external pure returns (
        uint256 maxPossibleSlot,
        uint256 implSlotValue,
        bool cannotCollide
    ) {
        // 假设合约有 2^64 个变量（实际上 Solidity 限制远小于这个数，
        // 因为每个合约有 24KB 代码大小限制）
        maxPossibleSlot = 2 ** 64;

        // EIP-1967 implementation slot 的实际值
        implSlotValue = uint256(IMPLEMENTATION_SLOT);

        // implSlotValue ≈ 0x360... (一个巨大的数，远大于 2^64)
        cannotCollide = implSlotValue > maxPossibleSlot;
        // 预期：true —— 碰撞概率 ≈ 0
    }
}