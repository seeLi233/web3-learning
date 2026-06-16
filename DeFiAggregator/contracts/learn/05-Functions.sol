// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

contract Functions {
    uint256 public data;

    // 1. view 函数 -- 只读状态
    function getData() public view returns (uint256) {
        return data;
    }

    // 2. pure 函数 -- 不读不写状态
    function add(uint256 a, uint256 b) public pure returns (uint256) {
        return a + b;
    }

    // 3. payable 函数 -- 可接受 ETH
    function deposit() public payable {
        // msg.value 是发送 ETH 数量 (wei)
        require(msg.value > 0, "Must send ETH");
    }

    // 4. 带返回值的函数
    // 方式 A：命名返回值
    function getBalance() public view returns (uint256 balance) {
        balance = address(this).balance; // 当前合约的 ETH 余额
    }

    // 方式 B：多返回值
    function getInfo() public view returns (address, uint256, uint256) {
        return (msg.sender, data, address(this).balance);
    }

    // 方式 C:解构命名返回值
    function getInfoName() public view returns (
        address sender,
        uint256 storedData,
        uint256 contractBalance
    ) {
        sender = msg.sender;
        storedData = data;
        contractBalance = address(this).balance;
    }

    // 5. fallback -- 调用不存在的函数时
    fallback() external payable {
        // 可以记录日志或者拒绝
    }

    // 6. receive -- 纯 ETH 转账时触发（无 calldata）
    receive() external payable {
        // 单纯的 ETH 转账进入这里
    }

    // 7. 外部可见性
    // external: 只能外部调用，但内部可用 this.func() 调用
    function externalFunc() external pure returns (string memory) {
        return "called externally";
    }

    // public：内外部都能直接调用
    function publicFunc() public view returns (string memory) {
        return this.externalFunc(); // public 内部可以调用
    }
}