// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title Storage
 * @notice 第一个练习合约： 基础存储功能
 * @dev 学习类型、引用类型、mapping、event
 */
contract Storage {
    // 状态变量

    // 值类型
    uint256 public storedNumber;
    bool public isLocked;
    address public owner;

    // 引用类型
    string public storedString;
    uint256[] public storedArray;

    // Mapping
    mapping (address => uint256) public userBalances;
    mapping (address => bool) public isWhitelisted;

    // 事件
    event NumberChanged(address indexed changer, uint256 oldValue, uint256 newValue);
    event StringChanged(address indexed changer, string oldValue, string newValue);
    event Deposited(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);

    // 修饰符
    modifier onlyOwner() {
        require(msg.sender == owner, "Not owner");
        _;
    }

    modifier notLocked() {
        require(!isLocked, "Contract is locked");
        _;
    }

    // 构造函数
    constructor() {
        owner = msg.sender;
        isLocked = false;
    }

    // 函数
    /**
     * @notice 设置存储的数字
     * @param _number 新的数字
     */
    function setNumber(uint256 _number) external notLocked {
        uint256 oldValue = storedNumber;
        storedNumber = _number;
        emit NumberChanged(msg.sender, oldValue, _number);
    }

    /**
     * @notice 获取存储数字
     * @return 存储的数字
     */
    function getNumber() external view returns (uint256) {
        return storedNumber;
    }

    /**
     * @notice 设置存储的字符串
     * @param _string 新的字符串
     */
    function setString(string calldata _string) external {
        string memory oldValue = storedString;
        storedString = _string;
        emit StringChanged(msg.sender, oldValue, _string);
    }

    /**
     * @notice 存入 ETH
     */
    function deposit() external payable notLocked {
        require(msg.value > 0, "Must send ETH");
        userBalances[msg.sender] += msg.value;
        emit Deposited(msg.sender, msg.value);
    }

    /**
     * @notice 取出 ETH
     */
    function withdraw(uint256 amount) external notLocked {
        require(userBalances[msg.sender] >= amount, "Insufficient balance");
        userBalances[msg.sender] -= amount;
        payable(msg.sender).transfer(amount);
        emit Withdrawn(msg.sender, amount);
    }

    /**
     * @notice 获取用户余额
     * @param user 用户地址
     * @return 余额
     */
    function getBalance(address user) external view returns (uint256) {
        return userBalances[user];
    }

    /**
     * @notice 添加到白名单
     * @param account 账户地址
     */
    function addToWhitelist(address account) external onlyOwner {
        isWhitelisted[account] = true;
    }

    /**
     * @notice 添加数字到数字
     * @param _number 数字
     */
    function addToArray(uint256 _number) external notLocked {
        storedArray.push(_number);
    }

    /**
     * @notice 获取数组的长度
     * @return 数组长度
     */
    function getArrayLength() external view returns (uint256) {
        return storedArray.length;
    }

    /**
     * @notice 获取数组元素
     * @param index 索引
     * @return 元素值
     */
    function getArrayElement(uint256 index) external view returns (uint256) {
        require(index < storedArray.length, "Index out of bounds");
        return storedArray[index];
    }

    /**
     * @notice 锁定合约
     */
    function lock() external onlyOwner {
        isLocked = true;
    }

    /**
     * @notice 解锁合约
     */
    function unlock() external onlyOwner{
        isLocked = false;
    }

    /**
     * @notice 接收 ETH (fallback)
     */
    receive() external payable {}
}