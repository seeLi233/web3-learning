// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IDiamondLoupe — EIP-2535 Diamond 标准的查询接口
/// @notice "Loupe" = 珠宝放大镜，用来观察钻石的切面结构
interface IDiamondLoupe {
    /// @notice Facet 信息结构
    struct Facet {
        address facetAddress;
        bytes4[] functionSelectors;
    }

    /// @notice 返回所有 Facet 及其 selectors
    function facets() external view returns (Facet[] memory);

    /// @notice 返回所有 Facet 地址
    function facetAddresses() external view returns (address[] memory);

    /// @notice 查询某个 Facet 包含的所有函数选择器
    function facetFunctionSelectors(address _facet) external view returns (bytes4[] memory);

    /// @notice 查询某个函数选择器属于哪个 Facet
    /// @return facetAddress_ 如果未注册则返回 address(0)
    function facetAddress(bytes4 _functionSelector) external view returns (address facetAddress_);
}