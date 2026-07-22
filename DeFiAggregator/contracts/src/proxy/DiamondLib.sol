// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IDiamondCut} from "./IDiamondCut.sol";
import {IDiamondLoupe} from "./IDiamondLoupe.sol";

/// @title DiamondLib — Diamond 内部库
/// @notice 封装 selector→facet 路由映射和 diamondCut 核心逻辑
library DiamondLib {
    /// @notice Diamond 存储的槽位 = keccak256("diamond.standard.diamond.storage") - 1
    /// @dev 用 -1 保持和 EIP-1967 风格一致（碰撞概率基本为 0）
    bytes32 internal constant DIAMOND_STORAGE_POSITION = 
        0xc8fcad8db84d3cc18b4c41d551ea0ee66dd599cde068d998e57d5e09332c131b;

    /// @notice Diamond 的核心数据结构
    struct DiamondStorage {
        // selector → facet 地址映射（路由表核心）
        mapping (bytes4 => address) selectorToFacet;
        // facet 地址 → selector 列表（反向索引，供 Loupe 查询）
        mapping (address => bytes4[]) facetToSelectors;
        // 所有已注册的 facet 地址集合
        address[] facetAddresses;
        // 判断 selector 是否已注册（防止 Add 冲突）
        mapping (bytes4 => bool) selectorExists;
        // 所有权
        address owner;
    }

    /// @notice 获取 Diamond 存储指针
    function diamondStorage() internal pure returns (DiamondStorage storage ds) {
        bytes32 position = DIAMOND_STORAGE_POSITION;
        assembly {
            ds.slot := position
        }
    }

    /// @notice 核心：执行 diamondCut 操作
    function diamondCut(IDiamondCut.FacetCut[] memory _diamondCut, address _init, bytes memory _calldata) internal {
        DiamondStorage storage ds = diamondStorage();

        for (uint256 i = 0; i < _diamondCut.length; i++) {
            IDiamondCut.FacetCut memory cut = _diamondCut[i];
            IDiamondCut.FacetCutAction action = cut.action;

            if (action == IDiamondCut.FacetCutAction.Add) {
                _addFacet(ds, cut.facetAddress, cut.functionSelectors);
            } else if (action == IDiamondCut.FacetCutAction.Replace) {
                _replaceFacet(ds, cut.facetAddress, cut.functionSelectors);
            } else if (action == IDiamondCut.FacetCutAction.Remove) {
                _removeFacet(ds, cut.facetAddress, cut.functionSelectors);
            }
        }

        emit IDiamondCut.DiamonCut(_diamondCut, _init, _calldata);

        // 如果传入了初始化合约，执行初始化（delegatecall）
        if (_init != address(0)) {
            _initializeDiamondCut(_init, _calldata);
        }
    }

    // ==================== Add ====================
    function _addFacet(DiamondStorage storage ds, address _facetAddress, bytes4[] memory _selectors) internal {
        require(_facetAddress != address(0), "Diamond: facet address zero");

        bool isNewFacet = ds.facetToSelectors[_facetAddress].length == 0;
        if (isNewFacet) {
            ds.facetAddresses.push(_facetAddress);
        }

        for (uint256 i = 0; i < _selectors.length; i++) {
            bytes4 selector = _selectors[i];
            require(!ds.selectorExists[selector], "Diamond: selector already exists");

            ds.selectorToFacet[selector] = _facetAddress;
            ds.facetToSelectors[_facetAddress].push(selector);
            ds.selectorExists[selector] = true;
        }
    }

    // ==================== Replace ====================
    function _replaceFacet(DiamondStorage storage ds, address _facetAddress, bytes4[] memory _selectors) internal {
        require(_facetAddress != address(0), "Diamond: facet address zero");

        bool isNewFacet = ds.facetToSelectors[_facetAddress].length == 0;
        if (isNewFacet) {
            ds.facetAddresses.push(_facetAddress);
        }

        for (uint256 i = 0; i < _selectors.length; i++) {
            bytes4 selector = _selectors[i];
            address oldFacet = ds.selectorToFacet[selector];
            require(oldFacet != address(0), "Diamond: select not found for replace");

            // 更新路由
            ds.selectorToFacet[selector] = _facetAddress;
            ds.facetToSelectors[_facetAddress].push(selector);

            // 从旧 Facet 的 selector 列表中移除
            _removeSelectorFromFacet(ds, oldFacet, selector);
        }
    }

    // ==================== Remove ====================
    function _removeFacet(DiamondStorage storage ds, address _facetAddress, bytes4[] memory selectors) internal {
        for(uint256 i = 0; i < selectors.length; i++) {
            bytes4 selector = selectors[i];
            require(ds.selectorToFacet[selector] != address(0), "Diamond: selector not found for remove");

            delete ds.selectorToFacet[selector];
            delete ds.selectorExists[selector];

            _removeSelectorFromFacet(ds, _facetAddress, selector);
        }
    }

    // ==================== 辅助函数 ====================
    function _removeSelectorFromFacet(DiamondStorage storage ds, address _facetAddress, bytes4 _selector) internal {
        bytes4[] storage selectors = ds.facetToSelectors[_facetAddress];
        for(uint256 i = 0; i < selectors.length; i++) {
            if (selectors[i] == _selector) {
                // 用最后一个元素替换当前位置，然后 pop（O(1) 删除）
                selectors[i] = selectors[selectors.length - 1];
                selectors.pop();
                break;
            }
        }
    }

    /// @notice 执行初始化 delegatecall
    function _initializeDiamondCut(address _init, bytes memory _calldata) internal {
        require(_init != address(0), "Diamond: init address is zero");
        require(_calldata.length > 0, "Diamond: calldata is empty");

        (bool success, ) = _init.delegatecall(_calldata);
        require(success, "Diamond: init failed");
    }

    // ==================== Loupe 查询（在库里直接实现） ====================
    function facets() internal view returns (IDiamondLoupe.Facet[] memory) {
        DiamondStorage storage ds = diamondStorage();
        uint256 count = ds.facetAddresses.length;

        IDiamondLoupe.Facet[] memory result = new IDiamondLoupe.Facet[](count);
        for (uint256 i = 0; i < count; i++) {
            address facetAddr = ds.facetAddresses[i];
            result[i] = IDiamondLoupe.Facet({
                facetAddress: facetAddr,
                functionSelectors: ds.facetToSelectors[facetAddr]
            });
        }
        return result;
    }

    function facetAddresses() internal view returns (address[] memory) {
        return diamondStorage().facetAddresses;
    }

    function facetFunctionSelectors(address _facetAddress) internal view returns (bytes4[] memory) {
        return diamondStorage().facetToSelectors[_facetAddress];
    }

    function facetAddress(bytes4 _functionSelector) internal view returns (address) {
        return diamondStorage().selectorToFacet[_functionSelector];
    }

    // ==================== 所有权 ====================
    modifier onlyOwner {
        require(diamondStorage().owner == msg.sender, "Diamond: not owner");
        _;
    }

    function transferOwnership(address _newOwner) internal {
        require(_newOwner != address(0), "Diamond: zero address");
        diamondStorage().owner = _newOwner;
    }

    function owner() internal view returns (address) {
        return diamondStorage().owner;
    }
}