// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title ValueTypes
 * @author 
 * @notice 学习 solidity 值类型
 */
contract Name {
    // 布尔值类型
    bool public isActive = true;
    bool public isPaused = false;

    // 整数类型
    // 无符号整数 （0 到 2^256-1）
    uint256 public totalSupply = 1_000_000;
    uint8 public samllNumber = 255;
    uint128 public mediumNumber;

    // 有符号整数（-2^127 到 2^127-1）
    int256 public temperature = -10;
    int8 public smallInt = 127; // -128 到 127

    // 地址类型
    // 普通地址 （20 字节）
    address public owner = msg.sender;

    // payable 地址（可以接受 ETH）
    address payable public treasury = payable(msg.sender);

    // 定长字节数组
    bytes1 public singleByte = 0xff;
    bytes32 public hash = keccak256("hello");

    // 枚举类型
    enum Status {
        Pending, // 0
        Active,  // 1
        Paused,  // 2
        Closed   // 3
    }
    Status public currentStatus = Status.Pending;

    // 函数演示
    function activate() external {
        isActive = true;
        currentStatus = Status.Active;
    }

    function pause() external {
        isActive = false;
        currentStatus = Status.Paused;
    }

    constructor() {
        
    }
}