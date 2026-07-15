// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title Timelock
 * @notice DAO 治理时间锁 — 提案执行前强制等待期
 * @dev 简化版 TimelockController，支持单个操作队列
 *
 * 核心安全模型:
 *   提案通过后 → queue() 排队 → 等待 minDelay → execute() 执行
 *   等待期间任何人都可以 cancel() 取消
 *
 * 面试重点: 为什么 Timelock 是 DAO 安全的基石？
 *   → 给社区反应时间，防止恶意提案立即生效
 *   → 让闪电贷治理攻击失效（借的钱等不到 minDelay 到期就要还）
 */
contract Timelock is AccessControl {
    // ============ 角色定义 ============
    bytes32 public constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
    bytes32 public constant EXECUTOR_ROLE = keccak256("EXECUTOR_ROLE");
    bytes32 public constant CANCELLER_ROLE = keccak256("CANCELLER_ROLE");

    // ============ 配置 ============
    /// @notice 最小延迟时间（秒）
    uint256 public minDelay;

    // ============ 操作结构 ============

    /// @notice 记录每个排队操作的参数，用于执行和验证
    struct Operation {
        address target;         // 目标合约地址
        uint256 value;          // 发送的 ETH 数量
        bytes data;             // 调用的 calldata
        bytes32 predecessor;    // 前置操作 ID（支持链式执行）
        bytes32 salt;           // 盐值（用于区分相同参数的不同操作）
        uint256 readyTime;      // 最早可执行时间 = 排队时间 + minDelay
        bool done;              // 是否已执行
    }

    // ============ 状态 ============
    mapping (bytes32 => Operation) private _operations;

    // ============ 事件 ============
    event CallScheduled(
        bytes32 indexed id,
        address indexed target,
        uint256 value,
        bytes data,
        bytes32 predecessor,
        bytes32 salt,
        uint256 readyTime
    );
    event CallExecuted(bytes32 indexed id, address indexed target, uint256 value, bytes data);
    event Cancelled(bytes32 indexed id);
    event MinDelayChanged(uint256 oldDuration, uint256 newDuration);

    // ============ 错误 ============
    error Timelock_NotReady(bytes32 id, uint256 readyTime, uint256 currentTime);
    error Timelock_AlreadyDone(bytes32 id);
    error Timelock_OperationNotScheduled(bytes32 id);
    error Timelock_InvalidTarget(address target);
    error Timelock_MinDelayTooLarge(uint256 delay, uint256 maxDelay);

    uint256 public constant MAX_DELAY = 30 days;

    // ============ 构造函数 ============

    /**
     * @param _minDelay 最小延迟（秒），建议至少 2 days
     * @param _proposers 可以提交操作的地址列表
     * @param _executors 可以执行操作的地址列表
     * @param _cancellers 可以取消操作的地址列表（通常是多签或紧急 DAO）
     */
    constructor(
        uint256 _minDelay,
        address[] memory _proposers,
        address[] memory _executors,
        address[] memory _cancellers
    ) {
        require(_minDelay <= MAX_DELAY, "Delay exceeds max");
        minDelay = _minDelay;

        // 设置角色
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);

        for (uint256 i = 0; i < _proposers.length; i++) {
            _grantRole(PROPOSER_ROLE, _proposers[i]);
        }

        for (uint256 i = 0; i < _executors.length; i++) {
            _grantRole(EXECUTOR_ROLE, _executors[i]);
        }

        for (uint256 i = 0; i < _cancellers.length; i++) {
            _grantRole(CANCELLER_ROLE, _cancellers[i]);
        }
    }

    // ============ 核心操作 ============

    /**
     * @notice 生成操作 ID
     * @dev 面试重点: ID 的生成规则
     *   keccak256(target, value, data, predecessor, salt)
     *   五个参数中任何一个不同都会产生不同的 ID
     *   这保证了每个操作有唯一标识
     */
    function hashOperation(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) public pure returns (bytes32) {
        return keccak256(abi.encode(target, value, data, predecessor, salt));
    }

    /**
     * @notice 将操作加入时间锁队列
     * @dev 只有 PROPOSER_ROLE 可以调用
     *      排队后必须等待 minDelay 才能执行
     *
     * 面试重点: readyTime 的计算
     *   readyTime = block.timestamp + minDelay
     *   这个时间戳是执行时的硬性检查条件
     */
    function schedule(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) external onlyRole(PROPOSER_ROLE) returns (bytes32 id) {
        require(target != address(0), "Timelock: zero target");

        id = hashOperation(target, value, data, predecessor, salt);

        // 防止重复排队
        require(_operations[id].readyTime == 0, "Timelock: already scheduled");

        uint256 readyTime = block.timestamp + minDelay;

        _operations[id] = Operation({
            target: target,
            value: value,
            data: data,
            predecessor: predecessor,
            salt: salt,
            readyTime: readyTime,
            done: false
        });

        emit CallScheduled(id, target, value, data, predecessor, salt, readyTime);
    }

    /**
     * @notice 执行已排队且等待时间到期的操作
     * @dev 检查清单:
     *   1. 操作必须已排队
     *   2. readyTime 必须已到
     *   3. 前置操作必须已完成（或为空）
     *   4. 操作未被执行过
     *   5. 调用者必须是 Executor
     */
    function execute(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) external payable onlyRole(EXECUTOR_ROLE) {
        bytes32 id = hashOperation(target, value, data, predecessor, salt);

        Operation storage op = _operations[id];

        // 检查1: 操作必须已排队
        if (op.readyTime == 0) revert Timelock_OperationNotScheduled(id);

        // 检查2: 时间到了吗？
        if (block.timestamp < op.readyTime) {
            revert Timelock_NotReady(id, op.readyTime, block.timestamp);
        }

        // 检查3: 前置操作完成了吗？
        if (predecessor != bytes32(0) && !_operations[predecessor].done) {
            revert("Timelock: predecessor not done");
        }

        // 检查4: 不能重复执行
        if (op.done) revert Timelock_AlreadyDone(id);

        op.done = true;

        // 执行外部调用
        (bool success, ) = target.call{value:value}(data);
        require(success, "Timelock: call failed");

        emit CallExecuted(id, target, value, data);
    }

    /**
     * @notice 取消排队中的操作
     * @dev 只有 CANCELLER_ROLE 可以取消
     *      已执行的操作不能取消
     *
     * 面试重点: 谁应该有 CANCELLER_ROLE？
     *   → 多签钱包或紧急 DAO（Emergency DAO）
     *   → 不能是单个 EOA，否则中心化风险
     */
    function cancel(bytes32 id) external onlyRole(CANCELLER_ROLE) {
        if (_operations[id].readyTime == 0) revert Timelock_OperationNotScheduled(id);
        if (_operations[id].done) revert Timelock_AlreadyDone(id);

        delete _operations[id];

        emit Cancelled(id);
    }

    // ============ 查询函数 ============

    /**
     * @notice 查询某个操作是否已排队
     */
    function isOperationSchedule(bytes32 id) external view returns (bool) {
        return _operations[id].readyTime > 0 && !_operations[id].done;
    }

    /**
     * @notice 查询某个操作是否已执行
     */
    function isOperationDone(bytes32 id) external view returns (bool) {
        return _operations[id].done;
    }

    /**
     * @notice 获取操作详情
     */
    function getOperation(bytes32 id) external view returns (
        address target,
        uint256 value,
        bytes memory data,
        bytes32 predecessor,
        bytes32 salt,
        uint256 readyTime,
        bool done
    ) {
       Operation storage op = _operations[id];
       return (op.target, op.value, op.data, op.predecessor, op.salt, op.readyTime, op.done);
    }

    // ============ 管理员 ============

    /**
     * @notice 修改最小延迟
     * @dev 面试重点: 修改 minDelay 也需要等待旧的 minDelay 才能生效
     *       因为修改 minDelay 本身就是一个需要排队执行的操作！
     */
    function updateMinDelay(uint256 _newDelay) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(_newDelay <= MAX_DELAY, "Delay exceeds max");
        uint256 oldDelay = minDelay;
        minDelay = _newDelay;
        emit MinDelayChanged(oldDelay, _newDelay);
    }
}