// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title Variables
 * @notice 学习变量作用域
 */

contract Variables {
    // 状态变量（State Variables）
    // 存储在区块链上，所有函数都可以访问
    uint256 public stateCounter = 0;
    address public immutable owner; // immutable: 只能在构造函数中设置
    uint256 public constant MAX_SUPPLY = 1_000_000; // constant: 编译时确定

    // 构造函数
    constructor() {
        owner = msg.sender; // 设置 immutable 变量
    }

    // 函数中的局部变量
    function calculate(uint256 a, uint256 b) external pure returns (uint256) {
        // 局部变量：只在函数内部有效
        uint256 sum = a + b;
        uint256 product = a * b;
        uint256 result = sum + product;

        return result;
    }

    // 全局变量
    function getGlobalVars() external payable returns (
        address sender,     // msg.sender: 调用者地址
        uint256 value,      // msg.value: 发送的 ETH(wei)
        uint256 timestamp,  // block.timestamp: 当前区块时间戳
        uint256 blockNum,   // block.number: 当前区块号
        uint256 gasLeft     // gasleft(): 剩余 gas
    ) {
        sender = msg.sender;
        value = msg.value;
        timestamp = block.timestamp;
        blockNum = block.number;
        gasLeft = gasleft();
    }

    // 数据位置：storage/ memory/ calldata
    // storage: 存储在区块链上（状态变量）
    // memory: 临时存储，函数结束后销毁
    // calldata: 只读，用于外部函数参数

    string public storeName;

    function setName(string calldata newName) external {
        // calldata -> storage: 需要显式复制
        storeName = newName;
    }

    function getNmae() external view returns (string memory) {
        // storage -> memory: 返回时需要复制到 memory
        return storeName;
    }

    // 值传递 vs 引用传递
    function incrementCounter() external {
        // 直接修改状态变量
        stateCounter++;
    }

    function demoMemory() external pure returns (uint256[] memory) {
        // memory 数组: 函数结束后销毁
        uint256[] memory tempArray = new uint256[](3);

        tempArray[0] = 1;
        tempArray[1] = 2;
        tempArray[2] = 3;

        return tempArray;
    }
}