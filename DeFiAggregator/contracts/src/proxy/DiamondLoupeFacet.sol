// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IDiamondLoupe} from "./IDiamondLoupe.sol";
import {DiamondLib} from "./DiamondLib.sol";

/// @title DiamondLoupeFacet — 钻石放大镜 Facet
/// @notice 实现 EIP-2535 的查询接口，可以查看 Diamond 的所有 Facet 结构
contract DiamondLoupeFacet is IDiamondLoupe {
    /// @inheritdoc IDiamondLoupe
    function facets() external view override returns (Facet[] memory) {
        return DiamondLib.facets();
    }

    /// @inheritdoc IDiamondLoupe
    function facetAddresses() external view override returns (address[] memory) {
        return DiamondLib.facetAddresses();
    }

    /// @inheritdoc IDiamondLoupe
    function facetFunctionSelectors(address _facet) external view override returns (bytes4[] memory) {
        return DiamondLib.facetFunctionSelectors(_facet);
    }

    /// @inheritdoc IDiamondLoupe
    function facetAddress(bytes4 _functionSelector) external view override returns (address) {
        return DiamondLib.facetAddress(_functionSelector);
    }
}