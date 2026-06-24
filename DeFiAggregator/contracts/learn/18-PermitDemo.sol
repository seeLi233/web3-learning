// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * 学习目标: 理解 EIP-2612 Permit 的底层原理
 * 这是一个"教学版"的 permit 实现，不用于生产环境
 */
contract PermitDemo {
    // ===== EIP-712 相关常量 =====
    // 域名类型哈希 — 固定值，EIP-712 标准定义
    bytes32 private constant _EIP712_DOMAIN_TYPEHASH = keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)");

    // Permit 类型哈希 — 定义了签名的"结构体"
    bytes32 private constant _PERMIT_TYPEHASH = keccak256("Permit(address owner,address spender,uint256 value,uint256 nonce,uint256 deadline)");

    // ===== 状态变量 =====
    mapping (address => uint256) private _nonces;
    mapping (address => mapping (address => uint256)) private _allowances;

    bytes32 public DOMAIN_SEPARATOR;

    constructor(string memory name) {
        // 构造函数中计算一次域名分隔符，存入不可变变量
        DOMAIN_SEPARATOR = keccak256(abi.encode(
            _EIP712_DOMAIN_TYPEHASH,
            keccak256(bytes(name)),   // 代币名称
            keccak256(bytes("1")),        // 版本号
            block.chainid,               // 链 ID (防跨链重放)
            address(this)                 // 合约地址 (防跨合约重放)
        ));
    }

    // ===== 核心函数: permit =====
    function permit(
        address owner,
        address spender,
        uint256 value,
        uint256 deadline,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) external {
        // ① 检查过期时间
        require(block.timestamp <= deadline, "PERMIT: expired");

        // ② 获取并递增 nonce（防重放的核心！）
        uint256 nonce = _nonces[owner];
        _nonces[owner] += 1; // 递增 nonce, 旧签名立刻失效

        // ③ 构造结构化数据哈希
        bytes32 structHash = keccak256(abi.encode(_PERMIT_TYPEHASH, owner, spender, value, nonce, deadline));

        // ④ 计算 EIP-712 最终签名哈希
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", DOMAIN_SEPARATOR, structHash));
        // "\x19\x01" 是 EIP-191 前缀：
        //   \x19 = 告诉钱包"这数据是给以太坊签名的"
        //   \x01 = 版本号，表示后面跟的是结构化数据

        // ⑤ 用 ecrecover 从签名恢复地址
        address signer = ecrecover(digest, v, r, s);
        require(signer != address(0), "PERMIT: invalid signature");
        require(signer == owner, "PERMIT: unauthorized");

        // ⑥ 设置授权
        _allowances[owner][spender] = value;
    }

    // ===== 辅助函数 =====
    function nonces(address owner) external view returns (uint256) {
        return _nonces[owner];
    }

    function allowances(address owner, address spender) external view returns (uint256) {
        return _allowances[owner][spender];
    }

    // ===== 讲解：签名是怎么在链下生成的 =====
    // JavaScript (ethers.js v6):
    //
    // const domain = {
    //   name: 'DeFiToken',
    //   version: '1',
    //   chainId: 31337,           // Hardhat 本地网络 ID
    //   verifyingContract: tokenAddress
    // };
    //
    // const types = {
    //   Permit: [
    //     { name: 'owner', type: 'address' },
    //     { name: 'spender', type: 'address' },
    //     { name: 'value', type: 'uint256' },
    //     { name: 'nonce', type: 'uint256' },
    //     { name: 'deadline', type: 'uint256' }
    //   ]
    // };
    //
    // const message = {
    //   owner: alice.address,
    //   spender: spender.address,
    //   value: ethers.parseEther('100'),
    //   nonce: await token.nonces(alice.address),
    //   deadline: Math.floor(Date.now()/1000) + 3600  // 1小时后过期
    // };
    //
    // const signature = await alice.signTypedData(domain, types, message);
    // const { v, r, s } = ethers.Signature.from(signature);
    //
    // await token.permit(
    //   message.owner,
    //   message.spender,
    //   message.value,
    //   message.deadline,
    //   v, r, s
    // );
}