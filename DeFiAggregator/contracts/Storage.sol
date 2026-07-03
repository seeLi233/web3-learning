// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title Storage
 * @notice 基础存储合约 — 练习值类型、引用类型、mapping、事件
 * @dev Day 1 编写，Day 14 重构
 *
 * 重构改进点:
 *   1. ✅ transfer() → call{}
 *   2. ✅ require string → 自定义 error
 *   3. ✅ 补齐 NatSpec 注释
 *   4. ✅ 统一代码风格
 *
 * @custom:exercise 这是你 Solidity 之旅的第一个合约，从它可以看到你的成长轨迹！
 */
contract Storage {
    // ============================================================
    // 状态变量
    // ============================================================

    /// @notice 存储的数字
    uint256 public storedNumber;

    /// @notice 合约锁定状态
    bool public isLocked;

    /// @notice 合约拥有者
    address public owner;

    /// @notice 存储的字符串
    string public storedString;

    /// @notice 存储的数字数组
    uint256[] public storedArray;

    /// @notice 用户余额 (address => balance)
    mapping (address => uint256) public userBalances;

    /// @notice 白名单 (address => isWhitelisted)
    mapping (address => bool) public isWhitelisted;

    // ============================================================
    // 事件
    // ============================================================

    /// @notice 数字变更事件
    /// @param changer 操作者地址
    /// @param oldValue 旧值
    /// @param newValue 新值
    event NumberChanged(address indexed changer, uint256 oldValue, uint256 newValue);

    /// @notice 字符串变更事件
    event StringChanged(address indexed changer, string oldValue, string newValue);

    /// @notice 存款事件
    event Deposited(address indexed user, uint256 amount);

    /// @notice 锁状态变更事件
    event Withdrawn(address indexed user, uint256 amount);

    /// @notice 锁状态变更事件
    event LockStatusChanged(bool locked);

    // ============================================================
    // 自定义 Error
    // ============================================================

    /// @notice 调用者不是 owner
    /// @param caller 实际调用者
    error Unauthorized(address caller);

    /// @notice 合约已锁定
    error ContractLocked();

    /// @notice 存款金额为 0
    error ZeroDeposit();

    /// @notice 余额不足
    /// @param requested 请求的金额
    /// @param available 可用余额
    error InsufficientBalance(uint256 requested, uint256 available);

    /// @notice 转账失败
    error TransferFailed();

    /// @notice 数组越界
    /// @param index 请求的索引
    /// @param length 数组长度
    error IndexOutOfBounds(uint256 index, uint256 length);

    // ============================================================
    // Modifier
    // ============================================================

    /// @notice 只有 owner 才能调用
    modifier onlyOwner() {
        if (msg.sender != owner) revert Unauthorized(msg.sender);
        _;
    }

    /// @notice 合约未锁定时才能调用
    modifier notLocked() {
        if (isLocked) revert ContractLocked();
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    /// @notice 初始化合约
    /// @dev 部署者自动成为 owner，合约初始为未锁定状态
    constructor() {
        owner = msg.sender;
        isLocked = false;
    }

    // ============================================================
    // 核心函数
    // ============================================================

    /// @notice 设置存储的数字
    /// @param _number 新数字
    function setNumber(uint256 _number) external notLocked {
        uint256 oldValue = storedNumber;
        storedNumber = _number;
        emit NumberChanged(msg.sender, oldValue, _number);
    }

    /// @notice 获取存储的数字
    /// @return 当前存储的数字
    function getNumber() external view returns (uint256) {
        return storedNumber;
    }

    /// @notice 设置存储的字符串
    /// @param _string 新字符串（calldata: 只读，最省 gas）
    function setString(string calldata _string) external {
        string memory oldValue = storedString;
        storedString = _string;
        emit StringChanged(msg.sender, oldValue, _string);
    }

    /// @notice 存入 ETH
    /// @dev msg.value 自动附加到交易中
    function deposit() external payable notLocked {
        require(msg.value > 0, "Must send ETH");
        userBalances[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    /// @notice 取出 ETH
    /// @param amount 取款金额
    /// @dev 使用 call{} 而非 transfer() —— 更安全，不限制 gas
    function withdraw(uint256 amount) external notLocked {
        if (userBalances[msg.sender] < amount) {
            revert InsufficientBalance(amount, userBalances[msg.sender]);
        }

        // CEI 模式: Checks(已完成) → Effects(先更新状态)
        userBalances[msg.sender] -= amount;

        // CEI 模式: Interactions(再外部调用)
        (bool success, ) = payable(msg.sender).call{value: amount}("");
        if (!success) revert TransferFailed();

        emit Withdrawn(msg.sender, amount);
    }

    // ============================================================
    // 查询函数
    // ============================================================

    /// @notice 获取用户余额
    /// @param user 用户地址
    /// @return 用户余额
    function getBalance(address user) external view returns (uint256) {
        return userBalances[user];
    }

    /// @notice 获取数组长度
    /// @return 数组长度
    function getArrayLength() external view returns (uint256) {
        return storedArray.length;
    }

    /// @notice 获取数组元素
    /// @param index 索引
    /// @return 元素值
    function getArrayElement(uint256 index) external view returns (uint256) {
        if (index >= storedArray.length) {
            revert IndexOutOfBounds(index, storedArray.length);
        }
        return storedArray[index];
    }

    // ============================================================
    // 管理函数
    // ============================================================

    /// @notice 添加地址到白名单
    /// @param account 目标地址
    function addToWhitelist(address account) external onlyOwner {
        isWhitelisted[account] = true;
    }

    /// @notice 添加数字到数组
    /// @param _number 要添加的数字
    function addToArray(uint256 _number) external notLocked {
        storedArray.push(_number);
    }

    /// @notice 锁定合约（禁止非 owner 操作）
    function lock() external onlyOwner {
        isLocked = true;
        emit LockStatusChanged(true);
    }

    /// @notice 解锁合约
    function unlock() external onlyOwner{
        isLocked = false;
        emit LockStatusChanged(false);
    }

    // ============================================================
    // 接收 ETH
    // ============================================================

    /// @notice 接收纯 ETH 转账（不带 data）
    receive() external payable {
        // 可以加一个事件
        if (msg.value > 0) {
            emit Deposited(msg.sender, msg.value);
        }
    }
}