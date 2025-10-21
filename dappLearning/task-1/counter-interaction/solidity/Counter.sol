// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Counter {
    // 公共计数变量 
    uint256 public count;

    // 构造函数：初始化计数为 0
    constructor() {
        count = 0;
    }

    // 增加计数
    function increment() public {
        count += 1;
    }

    // 减少计数
    function decrement() public {
        require(count >0 , "Count cannot be negative");
        count -= 1;
    }

    // 手动设置计数
    function setCount(uint256 _count) public {
        count = _count;
    }
}