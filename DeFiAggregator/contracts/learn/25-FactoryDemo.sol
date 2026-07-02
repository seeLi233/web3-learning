// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ============================================
// Part 1: 产品合约 — 被工厂创建
// ============================================

// 产品合约1: 最简 ERC20（手写，帮助理解底层）
contract MinimalToken {
    string public name;
    string public symbol;
    uint8 public immutable decimals;    // immutable 运行时不占 storage
    uint256 public totalSupply;
    address public immutable factory;   // 记录是谁创建了我

    mapping(address => uint256) private _balances;
    mapping(address => mapping(address => uint256)) private _allowances;

    event Transfer(address indexed from, address indexed to, uint256 value);

    // ===== 构造函数 =====
    // 每个参数为什么这么写：
    //   _name/_symbol: 代币的显示名称和代码，存入 storage
    //   _decimals: 小数位（一般是 18），用 immutable 省 gas
    //   _initialSupply: 初始发行量，全部 mint 给创建者
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _initialSupply
    ) {
        name = _name;
        symbol = _symbol;
        decimals = _decimals;   // immutable 只能在构造函数中赋值
        factory = msg.sender;     // 记录工厂地址（⚠️ 这里是工厂合约地址！）

        // 铸造初始代币给创建者
        // msg.sender 在 new 时是工厂合约地址
        // 所以我们需要一个参数来传真正创建者的地址
        //   → 这就是为什么工厂模式通常用 createToken(name, symbol, owner) 而不是 new Token(name, symbol)
    }

    function balanceOf(address account) external view returns (uint256) {
        return _balances[account];
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        // 为什么这样写：
        //   1. _balances[msg.sender] >= amount → 手动检查，clear error message
        //   2. unchecked → 0.8+ 内置溢出检查，这里不会溢出所以用 unchecked 省 gas
        //   3. _balances[msg.sender] -= amount → 先减发送者余额
        //   4. _balances[to] += amount → 再加接收者余额
        //   5. emit Transfer → 状态变更后必须 emit 事件
        require(_balances[msg.sender] >= amount, "insufficient balance");

        unchecked {
            _balances[msg.sender] -= amount;
            _balances[to] += amount;
        }

        emit Transfer(msg.sender, to, amount);
        return true;
    }

    // mint: 只有工厂能调用，用于给真正的创建者发行代币
    function mint(address to, uint256 amount) external {
        // 只有工厂可以 mint
        require(msg.sender == factory, "only factory"); // ← 工厂专属权限
        unchecked {
            totalSupply += amount;
            _balances[to] += amount;
        }
        emit Transfer(address(0), to, amount);
    }
}

// ============================================
// Part 2: 基础工厂合约
// ============================================

contract BasicTokenFactory {
    // ===== 存储：跟踪所有创建的子合约 =====
    // 为什么用数组 + mapping 组合？
    //   数组：方便遍历所有代币（可枚举）
    //   mapping：O(1) 查找某个地址是否是本工厂创建的
    address[] public allTokens;
    mapping(address => bool) public isDeployed; // O(1) 防重查

    // 每个用户的维度
    // 为什么嵌套 mapping？
    //   外层 mapping(address => array): 每个用户有一个数组
    //   这样就可以快速查"某个用户创建了哪些代币"
    mapping(address => address[]) public _userTokens;

    // ===== 事件 =====
    // 为什么要 emit 事件？
    //   链下服务（如后台/前端）可以监听这个事件
    //   不需要轮询所有区块来发现新创建的代币
    event TokenCreated(
        address indexed creator,        // indexed → 可以按创建者过滤
        address indexed tokenAddress,   // indexed → 可以按代币地址过滤
        string name,                    // 非 indexed → 存在 data 里
        string symbol, 
        uint256 timestamp               // 区块时间戳
    );

    // ===== factory 的核心函数 =====
    function createToken(
        string calldata _name,          // calldata: 外部参数，只读，最省 gas
        string calldata _symbol,
        uint8 _decimals,
        uint256 _initialSupply
    ) external returns (address tokenAddress) {
        // 第1步: 部署子合约
        // new 关键字：EVM 执行 CREATE 操作码
        //   1. 从当前合约的 nonce 计算新合约地址
        //   2. 创建新账户
        //   3. 执行 MinimalToken 的构造函数
        //   4. 把 MinimalToken 的 runtime bytecode 存入新账户
        MinimalToken token = new MinimalToken(
            _name,
            _symbol,
            _decimals,
            _initialSupply
        );

        tokenAddress = address(token);

        // 第2步: 给真正的用户 mint 代币
        // 为什么要单独 mint？
        //   因为构造函数的 msg.sender 是工厂合约，不是用户
        //   所以构造函数里不能直接给用户发代币
        token.mint(msg.sender, _initialSupply);

        // 第3步: 记录到存储
        allTokens.push(tokenAddress);
        isDeployed[tokenAddress] = true;
        _userTokens[msg.sender].push(tokenAddress);
    
        // 第4步: emit 事件
        emit TokenCreated(
            msg.sender,
            tokenAddress,
            _name,
            _symbol,
            block.timestamp // 当前区块的时间戳
        );
    }

    // ===== 查询函数 =====
    // view: 只读，不消耗 gas（外部调用时）
    function getTotalTokens() external view returns (uint256) {
        return allTokens.length;
    }

    function getUserTokens(address user) external view returns (address[] memory) {
        return _userTokens[user];
    }

    function getUserTokenCount(address user) external view returns (uint256) {
        return _userTokens[user].length;
    }
}