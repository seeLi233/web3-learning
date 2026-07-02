// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ============================================
// 极简"钱包"合约 — 用于演示 CREATE2 部署
// ============================================
contract MinimalWallet {
    address public immutable owner; // 钱包的主人

    // 构造函数：设置 owner
    // 注意：如果是 CREATE2 部署，constructor 参数不同 → 不同地址
    constructor(address _owner) {
        owner = _owner;
    }

    // 只允许 owner 操作
    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    // 接收 ETH
    receive() external payable {}

    // owner 可以提取 ETH
    function withdraw(uint256 amount) external onlyOwner {
        (bool ok, ) = owner.call{value: amount}("");
        require(ok, "transfer failed");
    }

    // 查看余额
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}

// ============================================
// CREATE2 演示合约
// ============================================
contract Create2Demo {
    // 事件：记录部署情况
    event Deployed(address indexed addr, bytes32 salt);

    // ========== 方式 1: CREATE (传统) ==========
    function deployWithCreate(address _owner) external returns (address) {
        // new = CREATE 操作码
        // 地址 = keccak256(address(this), nonce)
        // 每次调用地址都不同（nonce 递增）
        MinimalWallet wallet = new MinimalWallet(_owner);
        return address(wallet);
    }

    // ========== 方式 2: CREATE2 ==========
    function deployWithCreate2(address _owner, bytes32 _salt) public returns (address deployed) {
        // 第1步: 构建 creation bytecode
        // creation bytecode = 合约字节码 + 构造函数参数（ABI 编码）
        bytes memory bytecode = abi.encodePacked(
            type(MinimalWallet).creationCode, // 合约的创建时字节码
            abi.encode(_owner)              // 构造函数
        );

        // 第2步: 用 CREATE2 部署
        // 为什么用 assembly？
        //   因为 Solidity 不直接提供 CREATE2 的高级语法
        //   assembly 是 EVM 的底层操作码
        assembly {
            // create2(v, p, n, s)
            //   v: 发送的 ETH (wei) — 这里是 0
            //   p: 字节码在内存中的起始位置
            //   n: 字节码长度
            //   s: salt（32 bytes）
            // 返回值：新合约地址（失败返回 0）
            deployed := create2(
                0,                      // 不发送 ETH
                add(bytecode, 0x20),    // 跳过前 32 字节（Solidity 的 bytes 布局：前 32 字节 = 长度）
                mload(bytecode),        // 读取长度
                _salt                   // salt
            )
        }

        require(deployed != address(0), "Create2: deployment failed");
        emit Deployed(deployed, _salt);
    }

    // ========== 预测 CREATE2 地址（部署前就能算出来！）==========
    function predictAddress(address _owner, bytes32 _salt) public view returns (address) {
        // 第1步: 构建跟部署时完全一样的字节码
        bytes memory bytecode = abi.encodePacked(
            type(MinimalWallet).creationCode,
            abi.encode(_owner)
        );

        // 第2步: 计算 hash
        // CREATE2 地址公式:
        //   address = keccak256(0xff + deployer + salt + keccak256(bytecode))[12:]  ← 取后 20 字节
        bytes32 hash = keccak256(
            abi.encodePacked(
                bytes1(0xff),          // 固定前缀
                address(this),         // 部署者地址
                _salt,                 // salt
                keccak256(bytecode)    // bytecode 的哈希
            )
        );

        // 第3步: hash 是 32 字节，取后 20 字节就是地址
        // uint256(hash) → 把 bytes32 转成 uint256
        // uint160(...) → 截断到 20 字节（160 bits）
        // address(...) → 转成地址类型
        return address(uint160(uint256(hash)));
    }

    // ========== 验证：部署后地址是否跟预测一致 ==========
    function deployAndVerify(address _owner, bytes32 _salt) external returns (address deployed, address predicted, bool matches) {
        predicted = predictAddress(_owner, _salt);
        deployed = deployWithCreate2(_owner, _salt);
        matches = (deployed == predicted);
        // matches 应该总是 true！
    }

    // ========== 工具：生成伪随机 salt ==========
    function generateSalt(uint256 seed) external pure returns (bytes32) {
        return keccak256(abi.encodePacked(seed));
    }
    constructor() {
        
    }
}