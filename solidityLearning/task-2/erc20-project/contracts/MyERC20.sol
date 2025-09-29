// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// 作业 1：ERC20 代币
// 任务：参考 openzeppelin-contracts/contracts/token/ERC20/IERC20.sol实现一个简单的 ERC20 代币合约。要求：
// 合约包含以下标准 ERC20 功能：
// balanceOf：查询账户余额。
// transfer：转账。
// approve 和 transferFrom：授权和代扣转账。
// 使用 event 记录转账和授权操作。
// 提供 mint 函数，允许合约所有者增发代币。
// 提示：
// 使用 mapping 存储账户余额和授权信息。
// 使用 event 定义 Transfer 和 Approval 事件。
// 部署到sepolia 测试网，导入到自己的钱包

contract MyERC20 {
    // 代币名称
    string public name;
    // 代币符号
    string public symbol;
    // 小数位数
    uint8 public decimals = 18;
    // 总供应量
    uint256 public totalSupply;

    // 存储账户余额
    mapping(address => uint256) public balanceOf;
    // 存储授权信息
    mapping(address => mapping(address => uint256)) public allowance;

    // 合约所有者
    address public owner;

    // 转账事件
    event Transfer(address indexed from, address indexed to, uint256 value);
    // 授权事件
    event Approval(address indexed owner, address indexed spender, uint256 value);

    // 构造函数
    constructor(string memory _name, string memory _symbol, uint256 _initialSupply) {
        name = _name;
        symbol = _symbol;
        owner = msg.sender;
        // 初始供应量，考虑小数位数
        totalSupply = _initialSupply * (10 **uint256(decimals));
        // 将初始代币分配给合约部署着
        balanceOf[msg.sender] = totalSupply;
        emit Transfer(address(0), msg.sender, totalSupply);
    }

    // 转账函数
    function transfer(address to, uint256 value) public returns (bool success) {
        require(balanceOf[msg.sender] >= value, unicode"余额不足");
        require(to != address(0), unicode"不能转账到零地址");
        
        balanceOf[msg.sender] -= value;
        balanceOf[to] += value;
        emit Transfer(msg.sender, to, value);
        return true;
    }

    // 授权函数
    function approval(address spender, uint256 value) public returns (bool success) {
        require(spender != address(0), unicode"授权地址不能为零地址");

        allowance[msg.sender][spender] = value;
        emit Approval(msg.sender, spender, value);
        return true;
    }

    // 代扣转账函数
    function transferFrom(address from, address to, uint256 value) public returns (bool success) {
        require(balanceOf[from] >= value, unicode"余额不足");
        require(allowance[from][msg.sender] >= value, unicode"授权额度不足");
        require(to != address(0), unicode"不能转账到零地址");

        balanceOf[from] -= value;
        balanceOf[to] += value;
        allowance[from][msg.sender] -= value;
        emit Transfer(from, to, value);
        return true;
    }

    // 增发代币函数，仅所有者可调用
    function mint(address to, uint256 value) public {
        require(msg.sender == owner, unicode"仅所有者可增发代币");
        require(to != address(0), unicode"不能增发到零地址");

        totalSupply += value;
        balanceOf[to] += value;
        emit Transfer(address(0), to, value);
    }
}