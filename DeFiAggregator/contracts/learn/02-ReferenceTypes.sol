// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title ReferenceTypes
 * @notice 学习 Solidity 引用类型
 */
contract ReferenceTypes {
    // 字符串
    string public name ="DeFiAggregator";
    string public symbol = "DFA";

    // 字节数组(动态)
    bytes public dynamicBytes;

    // 数组
    // 固定长度数组
    uint256[3] public fixedArray = [1,2,3];

    // 动态数组
    uint256[] public dynamicArray;
    address[] public whitelist;

    // 结构体
    struct User {
        address addr;
        uint256 balance;
        uint256 timestamp;
        bool isActive;
    }

    mapping (address => User) public users;

    // 函数演示
    function addUser(uint256 _balance)  external {
        users[msg.sender] = User({
            addr: msg.sender,
            balance: _balance,
            timestamp: block.timestamp,
            isActive: true
        });

        // 添加到动态数组
        whitelist.push(msg.sender);
    }

    function getArrayLength() external view returns (uint256) {
        return whitelist.length;
    }

    function removeLastUser() external {
        require(whitelist.length > 0, "Array is empty");
        whitelist.pop();
    }
}