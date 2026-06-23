// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * 16-EventDeepDive.sol
 * 深入学习 EVM 事件 / 日志的底层机制
 *
 * 今天要搞懂的核心问题：
 * 1. topics 和 data 到底有什么区别？
 * 2. indexed 参数为什么最多 3 个？
 * 3. 事件的 gas 成本怎么算？
 * 4. 事件存在哪？合约内部能读吗？
 */

contract EventDeepDive {
    // ============ 1. 基础事件 ============
    // indexed → topic（可检索，最多 3 个）
    // 非 indexed → data（不可检索，但无数量限制）
    event  BasicEvent(
        address indexed sender,   // → topic[1]，前端可按地址过滤
        uint256 indexed id,       // → topic[2]，前端可按 ID 过滤
        string message,           // → data，只能读取后解码
        uint256 timestamp         // → data
    );

    function emitBasic(uint256 id, string calldata message)  external {
        emit BasicEvent(msg.sender, id, message, block.timestamp);
    }

    // ============ 2. 最多 4 个 topic 的演示 ============
    // topic[0] = 事件签名 hash（自动）
    // topic[1] = 第 1 个 indexed
    // topic[2] = 第 2 个 indexed
    // topic[3] = 第 3 个 indexed
    // 如果我写 4 个 indexed 会怎样？→ 编译报错！
    event MaxTopicsEvent(
        address indexed a,   // topic[1]
        address indexed b,   // topic[2]
        address indexed c    // topic[3]
        // address indexed d // ❌ 编译错误！最多 3 个 indexed
    );

    // ============ 3. 匿名事件（anonymous）============
    // 匿名事件不把事件签名作为 topic[0]
    // 好处：可以多出 1 个 indexed 位置（共 4 个 indexed）
    // 坏处：无法通过事件签名过滤（前端不好查）
    event AnonymousEvent(
        address indexed a,
        address indexed b,
        address indexed c,
        address indexed d
    ) anonymous ;

    function emitAnonymous(address a, address b, address c, address d) external {
        emit AnonymousEvent(a, b, c, d);
    }

    // ============ 4. 复杂数据结构的事件 ============
    struct Order {
        uint256 id;
        address user;
        uint256 amount;
        string product;
    }

    event OrderCreated(
        uint256 indexed orderId,        // topic: 方便按 Id 检索
        address indexed user,           // topic: 方便按用户检索
        uint256 amount,                 // data: 不需要检索
        string porduct                  // data: 动态类型，必须放data
    );

    function createOrder(uint256 id, uint256 amount, string calldata product) external {
        emit OrderCreated(id, msg.sender, amount, product);
    }

    // ============ 5. 事件的 gas 成本实验 ============
    event LightEvent(uint256 indexed value);       // 1 topic + 0 data
    event HeavyEvent(
        uint256 indexed v1,
        address indexed v2,
        bytes32 indexed v3,    // 3 indexed → 4 个 topic 全满
        string data1,
        string data2,
        uint256[] values       // 大量 data
    );

    function emitLight(uint256 value) external {
        emit LightEvent(value);
    }

    function emitHeavy(
        uint256 v1,
        address v2,
        bytes32 v3,
        string calldata d1,
        string calldata d2,
        uint256[] calldata values
    ) external {
        emit HeavyEvent(v1, v2, v3, d1, d2, values);
    }

    // ============ 6. 事件签名计算 ============
    // 这个函数可以帮你验证事件签名的 keccak256 值
    function getEventSignature(string calldata eventSignature)
        external
        pure
        returns (bytes32)
    {
        return keccak256(bytes(eventSignature));
        // 例如输入 "Transfer(address,address,uint256)"
        // 返回 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
    }

    constructor() {
        
    }
}