// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title TimelockController
 * @notice 升级版时间锁 — 批量操作 + Grace Period + ETA
 * @dev 参考 OZ TimelockController 核心逻辑，教学简化版
 *
 * 相比 Day 23 简版 Timelock 的改进:
 *   1. ✅ 批量操作: scheduleBatch() / executeBatch()
 *   2. ✅ Grace Period: 操作在 readyTime + GRACE_PERIOD 后过期
 *   3. ✅ ETA 查询: getTimestamp(id) 精确返回 readyTime
 *   4. ✅ 操作过期检查: isOperationExpired(id)
 *   5. ✅ 完整的角色体系
 *
 * ⭐ 面试重点:
 *   - 为什么批量操作要原子执行？
 *   - Grace Period 的作用是什么？
 *   - 谁应该有 CANCELLER_ROLE？
 */
contract TimelockController is AccessControl {
    // ============ 角色定义 ============
    bytes32 public constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
    bytes32 public constant EXECUTOR_ROLE = keccak256("EXECUTOR_ROLE");
    bytes32 public constant CANCELLER_ROLE = keccak256("CANCELLER_ROLE");

    // ============ 配置 ============
    /// @notice 最小延迟（秒）
    uint256 public minDelay;

    /// @notice 宽限期（秒）— readyTime 之后多久过期
    /// @dev 14 days 是行业常用值（Compound、Uniswap 都用这个）
    uint256 public constant GRACE_PERIOD = 14 days;

    /// @notice minDelay 的最大值（防止设太大导致协议无法治理）
    uint256 public constant MAX_DELAY = 30 days;

    // ============ 存储 ============
    /// @notice 操作 ID → readyTime（最早可执行时间戳）
    mapping (bytes32 => uint256) private _timestamps;

    /// @notice 操作 ID → 是否已执行
    mapping (bytes32 => bool) private _done;

    // ============ 事件 ============
    event CallScheduled(
        bytes32 indexed id,
        uint256 indexed index,
        address target,
        uint256 value,
        bytes data,
        bytes32 predecessor,
        uint256 delay
    );

    event CallExecuted(
        bytes32 indexed id,
        uint256 indexed index,
        address target,
        uint256 value,
        bytes data
    );

    event Cancelled(bytes32 indexed id);
    event MinDelayChange(uint256 oldDuration, uint256 newDuration);

    // ============ 错误 ============
    error Timelock__OperationNotScheduled(bytes32 id);
    error Timelock__OperationAlreadyScheduled(bytes32 id);
    error Timelock__OperationExpired(bytes32 id, uint256 readyTime, uint256 deadline);
    error Timelock__NotReady(bytes32 id, uint256 readyTime, uint256 currentTime);
    error Timelock__AlreadyDone(bytes32 id);
    error Timelock__PredecessorNotDone(bytes32 predecessor);
    error Timelock__MinDelayTooLarge(uint256 delay, uint256 maxDelay);
    error Timelock__ZeroTarget();

    // ============ 修饰器 ============
    modifier onlySelf {
        require(msg.sender == address(this), "Timelock: only self");
        _;
    }

    // ============ 构造函数 ============
    /**
     * @param _minDelay   最小延迟（秒），建议 >= 2 days
     * @param _proposers  初始 PROPOSER_ROLE 地址（通常是 Governor 合约）
     * @param _executors  初始 EXECUTOR_ROLE 地址（可以设为 address(0) 表示任何人）
     * @param _cancellers 初始 CANCELLER_ROLE 地址（通常是多签或 Guardian）
     *
     * 面试: 为什么 ADMIN_ROLE 通常给 Timelock 自己？
     *   → 防止单点控制: 修改 minDelay 需要走 Timelock 流程
     *   → 如果 ADMIN 是 EOA → 这个人可以随时改 minDelay → 形同虚设
     */
    constructor(uint256 _minDelay, address[] memory _proposers, address[] memory _executors, address[] memory _cancellers) {
        require(_minDelay <= MAX_DELAY, "Delay exceeds max");
        minDelay = _minDelay;

        // DEFAULT_ADMIN_ROLE 给 Timelock 自身和部署者
        // Timelock 自身持有 ADMIN: 修改角色需要走完整的提案-投票-Timelock 流程
        // 部署者持有 ADMIN: 初始设置角色后应 renounce
        _grantRole(DEFAULT_ADMIN_ROLE, address(this));
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);

        for (uint256 i = 0; i < _proposers.length; i++) {
            _grantRole(PROPOSER_ROLE, _proposers[i]);
        }

        for (uint256 i = 0; i < _executors.length; i++) {
            _grantRole(EXECUTOR_ROLE, _executors[i]);
        }

        for(uint256 i = 0; i < _cancellers.length; i++) {
            _grantRole(CANCELLER_ROLE, _cancellers[i]);
        }
    }

    // ============ 核心操作 ============

    /**
     * @notice 生成单个操作 ID
     * @dev ID = keccak256(target, value, data, predecessor, salt)
     *
     * 五个参数:
     *   target     - 目标合约
     *   value      - 发送的 ETH
     *   data       - calldata
     *   predecessor- 前置操作 ID（bytes32(0) 表示无前置）
     *   salt       - 盐值（区分相同参数的不同操作）
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
     * @notice 生成批量操作 ID
     * @dev 批量 ID = keccak256(targets, values, payloads, predecessor, salt)
     *      所有操作打包成一个整体
     */
    function hashOperationBatch(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata payloads,
        bytes32 predecessor,
        bytes32 salt
    ) public pure returns (bytes32) {
        return keccak256(abi.encode(targets, values, payloads, predecessor, salt));
    }

    // ============ 单个操作排队 ============

    /**
     * @notice 单个操作加入时间锁队列
     */
    function schedule(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt,
        uint256 delay
    ) external onlyRole(PROPOSER_ROLE) {
        if (target == address(0)) revert Timelock__ZeroTarget();

        bytes32 id = hashOperation(target, value, data, predecessor, salt);
        _schedule(id, delay);

        emit CallScheduled(id, 0, target, value, data, predecessor, delay);
    }

    // ============ 批量操作排队 ⭐ ============

    /**
     * @notice 批量操作加入时间锁队列
     * @dev ⭐ 面试重点: 批量操作的核心价值
     *
     * 为什么需要批量 schedule？
     *   1. Gas 优化: 一次交易代替多次
     *   2. 原子性: 整个批次的 readyTime 一致
     *   3. 逻辑一致性: 一个提案 = 一个批次操作 = 一个 ID
     *
     * 参数验证:
     *   - targets/values/payloads 长度必须一致
     *   - predecessor 如果不为空，对应操作必须已执行
     */
    function scheduleBatch(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata payloads,
        bytes32 predecessor,
        bytes32 salt,
        uint256 delay
    ) external onlyRole(PROPOSER_ROLE) {
        require(targets.length == values.length, "Timelock: length mismatch");
        require(targets.length == payloads.length, "Timelock: length mismatch");
        require(targets.length > 0, "Timelock: empty batch");

        bytes32 id = hashOperationBatch(targets, values, payloads, predecessor, salt);
        _schedule(id, delay);

        // 为批次中每个操作单独发事件（方便链下索引）
        for (uint256 i = 0; i < targets.length; i++) {
            emit CallScheduled(id, i, targets[i], values[i], payloads[i], predecessor, delay);
        }
    }

    /**
     * @notice 内部排队逻辑
     */
    function _schedule(bytes32 id, uint256 delay) internal {
        if (_timestamps[id] != 0) revert Timelock__OperationAlreadyScheduled(id);

        // readyTime = 当前时间 + delay
        // delay 必须 >= minDelay（也可以设更大的值）
        if (delay < minDelay) {
            delay = minDelay;
        }

        _timestamps[id] = block.timestamp + delay;
    }

    // ============ 单个操作执行 ============

    /**
     * @notice 执行已排队的单个操作
     * @dev 三阶段检查:
     *   1. 时间到了吗？（readyTime ≤ now）
     *   2. 过期了吗？（now ≤ readyTime + GRACE_PERIOD）
     *   3. 前置操作完成了吗？（predecessor is done or bytes32(0)）
     */
    function execute(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) external payable onlyRole(EXECUTOR_ROLE) {
        bytes32 id = hashOperation(target, value, data,predecessor,salt);
        _beforeCall(id, predecessor);
        _call(id, 0, target, value, data);
        _afterCall(id);
    }

    // ============ 批量操作执行 ⭐ ============

    /**
     * @notice 批量执行已排队的所有操作
     * @dev ⭐ 面试重点: 批量执行中的"部分失败"问题
     *
     * 如果批次中第 2 个操作失败了怎么办？
     *   → 整个 executeBatch() 回滚（revert）
     *   → EVM 保证原子性: 第 1 个操作的效果也会被撤销
     *   → 这是 Solidity 的默认行为，不需要额外处理
     *
     * 但如果第 2 个操作是被调用合约内部 revert 呢？
     *   → 同样会导致 executeBatch() 整体 revert
     *   → 这就是原子执行保证
     */
    function executeBatch(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata payloads,
        bytes32 predecessor,
        bytes32 salt
    ) external payable onlyRole(EXECUTOR_ROLE) {
        require(targets.length == values.length, "Timelock: length mismatch");
        require(targets.length == payloads.length, "Timelock: length mismatch");
        require(targets.length > 0, "Timelock: empty batch");

        bytes32 id = hashOperationBatch(targets, values, payloads, predecessor, salt);
        _beforeCall(id, predecessor);

        // 逐个执行批次中的操作
        // EVM 保证: 任何一个失败 → 全部回滚
        for (uint256 i = 0; i < targets.length; i++) {
            _call(id, i, targets[i], values[i], payloads[i]);
        }

        _afterCall(id);
    }

    /**
     * @notice 执行前检查（所有 execute 的公共逻辑）
     */
    function _beforeCall(bytes32 id, bytes32 predecessor) internal view {
        uint256 timestamp = _timestamps[id];

        // 检查1: 已排队了吗？
        if (timestamp == 0) revert Timelock__OperationNotScheduled(id);

        // 检查2: 时间到了吗？
        if (block.timestamp < timestamp) {
            revert Timelock__NotReady(id, timestamp, block.timestamp);
        }

        // 检查3: 过期了吗？ ⭐ Grace Period 核心
        if (block.timestamp > timestamp + GRACE_PERIOD) {
            revert Timelock__OperationExpired(id, timestamp, timestamp + GRACE_PERIOD);
        }

        // 检查4: 前置操作完成了吗？
        if (predecessor != bytes32(0)) {
            if (!_done[predecessor]) {
                revert Timelock__PredecessorNotDone(predecessor);
            }
        }
    }

    /**
     * @notice 底层外部调用
     */
    function _call(bytes32 id, uint256 index, address target, uint256 value, bytes calldata data) internal {
        // solhint-disable-next-line avoid-low-level-calls
        (bool success, ) = target.call{value:value}(data);
        require(success, "Timelock: call failed");

        emit CallExecuted(id, index, target, value, data);
    }

    /**
     * @notice 执行后标记
     */
    function _afterCall(bytes32 id) internal {
        _done[id] = true;
        // 释放存储（Gas Refund)
        delete _timestamps[id];
    }

    // ============ 取消操作 ============

    /**
     * @notice 取消已排队但未执行的操作
     * @dev 只有 CANCELLER_ROLE 可以取消
     *
     * ⭐ 面试重点: 紧急取消的使用场景
     *   1. 提案排队期间发现了安全漏洞
     *   2. 市场剧烈波动导致提案不再合适
     *   3. 前置操作失败导致后续操作永远无法执行
     */
    function cancel(bytes32 id) external onlyRole(CANCELLER_ROLE) {
        if (_timestamps[id] == 0) revert Timelock__OperationNotScheduled(id);
        if (_done[id]) revert Timelock__AlreadyDone(id);

        delete _timestamps[id];

        emit Cancelled(id);
    }

    // ============ 修改延迟（需要过 Timelock） ⭐ ============

    /**
     * @notice 修改最小延迟
     * @dev ⭐ 面试重点: 为什么修改 minDelay 要等旧的 minDelay 生效？
     *
     * 场景推演:
     *   minDelay = 2 days, 攻击者获得 ADMIN 权限
     *   如果可以直接改 minDelay = 0:
     *     → 攻击者提交恶意提案 → 立即执行 → 第一笔钱被转走
     *
     *   如果必须先排队等待 2 days:
     *     → 攻击者提交修改 minDelay 的提案
     *     → 社区有 2 天时间发现并阻止
     *     → 即使通过了，也要再等 2 天才生效
     *     → 给了充足的反应时间
     *
     * 因此: updateMinDelay 只能由 Timelock 自身调用
     *       也就是必须走完整的提案-投票-排队-执行流程
     */
    function updateMinDelay(uint256 newDelay) external onlySelf {
        require(newDelay <= MAX_DELAY, "Delay exceeds max");
        uint256 oldDaly = minDelay;
        minDelay = newDelay;
        emit MinDelayChange(oldDaly, newDelay);
    }

    // ============ 查询函数 ============

    /**
     * @notice 获取操作的 readyTime
     * @dev ⭐ ETA 查询 — 前端用这个显示倒计时
     */
    function getTimestamp(bytes32 id) external view returns (uint256) {
        uint256 timestamp = _timestamps[id];
        if (timestamp == 0) revert Timelock__OperationNotScheduled(id);
        return timestamp;
    }

    /**
     * @notice 操作是否已排队
     */
    function isOperationScheduled(bytes32 id) external view returns (bool) {
        return _timestamps[id] > 0 && !_done[id];
    }

    /**
     * @notice 操作是否已执行
     */
    function isOperationDone(bytes32 id) external view returns (bool) {
        return _done[id];
    }

    /**
     * @notice 操作是否已过期 ⭐ Grace Period 查询
     */
    function isOperationExpired(bytes32 id) external view returns (bool) {
        uint256 timestamp = _timestamps[id];
        if (timestamp == 0) return false;
        if (_done[id]) return false;
        return block.timestamp > timestamp + GRACE_PERIOD;
    }

    /**
     * @notice 操作是否已就绪（可以执行）
     */
    function isOperationReady(bytes32 id) external view returns (bool) {
        uint256 timestamp = _timestamps[id];
        if (timestamp == 0) return false;
        if (_done[id]) return false;
        return block.timestamp >= timestamp && block.timestamp <= timestamp + GRACE_PERIOD;
    }
}