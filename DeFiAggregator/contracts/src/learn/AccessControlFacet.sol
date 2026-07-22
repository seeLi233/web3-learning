// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title AccessControlFacet — 权限管理 Facet
/// @notice 使用 Diamond Storage 管理角色
library AccessControlStorage {
    bytes32 internal constant STORAGE_POSITION = keccak256("diamond.standard.access.control.storage");

    struct Layout {
        mapping (bytes32 => mapping (address => bool)) roles;
    }

    function layout() internal pure returns (Layout storage s) {
        bytes32 position = STORAGE_POSITION;
        assembly {
            s.slot := position
        }
    }
}

contract AccessControlFacet {
    event RoleGranted(bytes32 indexed role, address indexed account);
    event RoleRevoked(bytes32 indexed role, address indexed account);

    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");

    /// @notice 授予角色
    function grantRole(bytes32 role, address account) external {
        AccessControlStorage.layout().roles[role][account] = true;
        emit RoleGranted(role, account);
    }
    
    /// @notice 撤销角色
    function revokeRole(bytes32 role, address account) external {
        AccessControlStorage.layout().roles[role][account] = false;
        emit RoleRevoked(role, account);
    }

    /// @notice 检查账户是否有某角色
    function hasRole(bytes32 role, address account) external view returns (bool) {
        return AccessControlStorage.layout().roles[role][account];
    }
}