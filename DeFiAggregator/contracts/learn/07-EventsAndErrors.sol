// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

// 事件部分
contract EventDemo {
    // 事件声明
    // indexed 参数 -> topics (最多 3 个)
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    // 无 indexed -> 所有参数都在 data 里
    event Log(string message, uint256 timestamp);

    // 事件的 topics 结构
    // topic[0] = keccak256("Transfer(address, address, uint256)") 事件签名
    // topic[1] = from  (第一个 indexed)
    // topic[2] = to    (第二个 indexed)
    // data     = value (非 indexed， ABI 编码)

    mapping (address => uint256) public balances;
    mapping (address => mapping (address => uint256)) public allowance;

    function transfer(address to, uint256 value) public {
        balances[msg.sender] -= value;
        balances[to] += value;
        emit Transfer(msg.sender, to, value);
    }

    function approve(address spender, uint256 value) public {
        allowance[msg.sender][spender] = value;
        emit Approval(msg.sender, spender, value);
    }
}

// 错误处理演示
contract ErrorDemo {
    address public owner;
    uint256 public balance;

    // 自定义 Error（0.8.4+, 最省 gas）
    error Unauthorized(address caller, bytes32 requireRole);
    error InsufficientBalance(uint256 requested, uint256 available);
    error TransferFailed(address to, uint256 amount);

    constructor() {
        owner = msg.sender;
    }

    // 1. require -- 验证外部输入
    function setBalance(uint256 amount) public {
        // require(条件，错误信息)
        require(amount > 0, "Amount must be positive");
        require(msg.sender == owner, "Only owner can set");
        balance = amount;
    }

    // 2. revert -- 复杂条件中断
    function withdraw(uint256 amount) public {
        // 方式 A: revert + 字符串
        if (amount > balance) {
            revert("Insufficient balance");
        }

        // 方式 B：revert + 自定义 error (更省 gas)
        if (msg.sender != owner) {
            revert Unauthorized(msg.sender, 0x0);
        }

        balance -= amount;
        (bool success, ) = msg.sender.call{value: amount}("");
        if (!success) {
            revert TransferFailed(msg.sender, amount);
        }
    }

    // 3. assert -- 检查内部不变量
    uint256 public totalSupply;
    uint256 public constant MAX_SUPPLY = 1_000_000;

    function mint(uint256 amount) public onlyOwner {
        totalSupply += amount;
        // assert: 这种条件永远不为 false
        // 如果为 false -> 合约有 bug
        assert(totalSupply <= MAX_SUPPLY);

    }

    modifier onlyOwner() {
        if (msg.sender != owner) {
            revert Unauthorized(msg.sender, bytes32("OWNER"));
        }
        _;
    }
}