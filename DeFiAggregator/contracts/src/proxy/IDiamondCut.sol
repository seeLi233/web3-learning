// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IDiamondCut — EIP-2535 Diamond 标准的 Facet 管理接口
/// @notice 定义如何添加、替换、删除 Facet
interface IDiamondCut {
    /// @notice Facet 操作类型
    enum FacetCutAction {
        Add,        // 0 — 新增 selector→facet 映射
        Replace,    // 1 — 替换已有映射
        Remove      // 2 — 删除已有映射
    }

    /// @notice 一次 Facet 变更的数据结构
    struct FacetCut {
        address facetAddress;       // Facet 合约地址（Remove 时使用 address(0)）
        FacetCutAction action;      // 操作类型
        bytes4[] functionSelectors; // 受影响的函数选择器列表
    }
    
    /// @notice 执行一次钻石切割（增删改 Facet）
    /// @param _diamondCut 一组 Facet 变更操作
    /// @param _init 初始化合约地址（address(0) 表示不需要初始化）
    /// @param _calldata 初始化调用的 calldata
    /// @dev 会触发 DiamondCut 事件
    function diamondCut(FacetCut[] calldata _diamondCut, address _init, bytes calldata _calldata) external;

    /// @notice Facet 变更事件
    event DiamonCut(FacetCut[] _diamondCut, address _init, bytes _calldata);
}