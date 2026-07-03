// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title AccessControl
 * @notice 基于角色的访问控制合约
 * @dev 综合练习： modifier + event + error + 权限管理
 */
contract AccessControl {
    // 角色定义
    bytes32 public constant DEFAULT_ADMIN_ROLE = 0x00;
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    bytes32 public constant MODERATOR_ROLE = keccak256("MODERATOR_ROLE");
    bytes32 public constant USER_ROLE = keccak256("USER_ROLE");

    // 状态变量
    address public owner;
    mapping (bytes32 => mapping (address => bool)) private _role;

    // 事件
    event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender);
    event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender);
    event OwnerChanged(address indexed oldOwner, address indexed newOwner);
    event ActionPerformed(address indexed account, string action);

    // 自定义 Error
    error NotOwner(address caller);
    error NotAuthorized(address caller, bytes32 requireRole);
    error RoleAlreadyGranted(bytes32 role, address account);
    error RoleNotGranted(bytes32 role, address account);
    error CannotRevokeSelf(address account);
    error ZeroAddress();

    // 构造函数
    constructor() {
        owner = msg.sender;
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(ADMIN_ROLE, msg.sender);
    }

    // Modifier

    /**
     * @notice 只有 owner 可以调用
     */
    modifier onlyOwner {
        if (msg.sender != owner) {
            revert NotOwner(msg.sender);
        }
        _;
    }

    /**
     * @notice 只有拥有指定角色的账户可以调用
     * @param role 需要的角色
     */
    modifier onlyRole(bytes32 role) {
        if (!hasRole(role, msg.sender)) {
            revert NotAuthorized(msg.sender, role);
        }
        _;
    }

    /**
     * @notice 防止零地址
     */
    modifier nonZero(address account) {
        if (account == address(0)) {
            revert ZeroAddress();
        }
        _;
    }

    // 角色管理函数
    /**
     * @notice 授予橘色
     * @param role 角色 identifier
     * @param account 目标账户
     */
    function grantRole(bytes32 role, address account) external onlyOwner nonZero(account) {
        if (hasRole(role, account)) {
            revert RoleAlreadyGranted(role, account);
        }
        _grantRole(role, account);
    }

    /**
     * @notice 撤销角色
     * @param role 角色 identifier
     * @param account 目标账户
     */
    function revokeRole(bytes32 role, address account) external onlyOwner nonZero(account) {
        if (!hasRole(role, account)) {
            revert RoleAlreadyGranted(role, account);
        }
        if (account == msg.sender) {
            revert CannotRevokeSelf(msg.sender);
        }
        _revokeRole(role, account);
    }

    /**
     * @notice 放弃自己的角色
     * @param role 要放弃的角色
     */
    function renounceRole(bytes32 role) external {
        if (!hasRole(role, msg.sender)) {
            revert RoleNotGranted(role, msg.sender);
        }
        _revokeRole(role, msg.sender);
    }

    // 查询函数

    /**
     * @notice 检查账户是否拥有某角色
     * @param role 角色 identifier
     * @param account 要检查的账户
     * @return 是否拥有角色
     */
    function hasRole(bytes32 role, address account) public view returns (bool) {
        return _role[role][account];
    }

    // Owner 管理

    /**
     * @notice 转移 owner 权限
     * @param newOwner 新 owner
     */
    function transferOwnership(address newOwner) external onlyOwner nonZero(newOwner) {
        address oldOwner = owner;
        owner = newOwner;
        _grantRole(ADMIN_ROLE, newOwner);
        _revokeRole(ADMIN_ROLE, oldOwner);
        emit OwnerChanged(oldOwner, newOwner);
    }

    // 业务函数（受角色保护）
    function adminAction(string calldata action) external onlyRole(ADMIN_ROLE) {
        emit ActionPerformed(msg.sender, action);
    }

    /**
     * @notice 专属操作
     */
    function moderatorAction(string calldata action) external onlyRole(MODERATOR_ROLE) {
        emit ActionPerformed(msg.sender, action);
    }

    /**
     * @notice 任何注册用户可执行的操作
     */
    function userAction(string calldata action) external onlyRole(USER_ROLE) {
        emit ActionPerformed(msg.sender, action);
    }

    // 内部函数
    function _grantRole(bytes32 role, address account) private {
        _role[role][account] = true;
        emit RoleGranted(role, account, msg.sender);
    }

    function _revokeRole(bytes32 role, address account) private {
        _role[role][account] = false;
        emit RoleRevoked(role, account, msg.sender);
    }
}