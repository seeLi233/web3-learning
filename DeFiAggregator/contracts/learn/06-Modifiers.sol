// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

contract Modifers {
    address public owner;
    bool public paused;

    constructor() {
        owner = msg.sender;
    }

    // 1. 基础 modifier （无参数）
    modifier onlyOwner() {
        require(msg.sender == owner, "Not Owner");
        _;
    }

    // 2. 带参数的 modifer
    modifier onlyAfter(uint256 _time) {
        // 必须是下划线开头的参数名，避免与状态变量冲突
        require(block.timestamp >= _time, "Too early");
        _;
    }

    // 3. 组合多个 modifier
    modifier whenNoPaused() {
        require(!paused, "Paused");
        _;
    }

    // 函数可以同时使用多个 modifier
    function importantAction() public onlyOwner whenNoPaused {
        // 只有 owner 且未暂停才能执行
        // modifier 按声明顺序执行
    }

    // 4. modifier 中的前置和后置逻辑
    uint256 public actionCount;

    modifier countAction() {
        // 前置函数: 函数执行前
        _;
        // 后置逻辑：函数执行后
    }

    function doSomething() public countAction {
        // 执行完 actionCount 自动 +1
    }

    // 5. 带条件的 modifier
    modifier onlyIf(bool _condition, string memory _error) {
        require(_condition, _error);
        _;
    }

    function conditionalAction(bool allowed) public onlyIf(allowed, "Not allowed") {
        // 只有 allowed == true 才能执行
    }

    // 控制函数
    function pause() external onlyOwner {
        paused = true;
    }

    function unpause() external onlyOwner {
        paused = false;
    }
}