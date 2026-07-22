// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IDiamondCut} from "./IDiamondCut.sol";
import {IDiamondLoupe} from "./IDiamondLoupe.sol";
import {DiamondLib} from "./DiamondLib.sol";

/// @title Diamond — EIP-2535 Diamond Proxy
/// @notice 核心代理合约，负责路由所有调用到对应的 Facet
contract Diamond {
    /// @notice 构造函数：设置 owner 并初始化基本 Facet
    /// @param _owner Diamond 的所有者
    /// @param _diamondCut 初始 Facet 列表
    constructor(address _owner, IDiamondCut.FacetCut[] memory _diamondCut) {
        // 设置 owner
        DiamondLib.diamondStorage().owner = _owner;

        // 执行初始 diamondCut
        DiamondLib.diamondCut(_diamondCut, address(0), "");

        // ⭐ 关键：如果 diamondCut 中包含 DiamondLoupeFacet，
        // 则需要把自己支持的函数（diamondCut / facets / facetAddress 等）
        // 也注册到路由表中。
        // 这里 diamondCut 函数通过 fallback 路由 → DiamondLib 处理
    }

    /// @notice 核心路由：根据 msg.sig 找到对应 Facet 并 delegatecall
    fallback() external payable {
        DiamondLib.DiamondStorage storage ds = DiamondLib.diamondStorage();
        address facet = ds.selectorToFacet[msg.sig];
        require(facet != address(0), "Diamond: function not found");

        // 内联汇编实现 delegatecall
        assembly {
            // 复制 calldata 到内存
            calldatacopy(0, 0, calldatasize())

            // delegatecall
            let result := delegatecall(gas(), facet, 0, calldatasize(), 0, 0)

            // 复制返回数据到内存
            returndatacopy(0, 0, returndatasize())

            // 根据结果返回或 revert
            switch result
            case 0 {
                revert(0, returndatasize())
            } 
            default {
                return(0, returndatasize())
            }
        }   
    }

    /// @notice 支持接收 ETH
    receive() external payable {}
}