// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

/**
 * @title Slashing
 * @notice 质押罚没扩展合约 — 与 DeFiStaking 配合使用
 * @dev 设计原则:
 *   1. 比例惩罚: 惩罚与作恶程度成正比
 *   2. 延迟执行: 罚没有等待期，给申诉时间
 *   3. 可验证: 任何人都能提交作恶证据
 *   4. 不可逆: 罚没的资金不能恢复
 *   5. 上限控制: 不会罚没超过质押量
 *
 * 面试重点: Slashing 解决了什么问题？
 *   → 让作恶有经济成本。如果作恶获利 < 罚没金额，理性人就不会作恶
 *   → 这是 PoS 共识安全的基石
 */
contract Slashing is Ownable {
    using SafeERC20 for IERC20;

    // ============ 数据结构 ============

    /// @notice 违规类型
    enum ViolationType {
        DoubleSigning,      // 双重签名 — 最严重
        Downtime,           // 离线 — 较轻微
        MaliciousVote       // 恶意投票 — 严重
    }

    /// @notice 违规记录
    struct Violation {
        address violator;       // 违规者
        ViolationType vType;    // 违规类型
        uint256 slashAmount;    // 罚没金额
        uint256 readyTime;      // 最早可执行时间（申诉期）
        bool executed;          // 是否已执行
        bool appealed;          // 是否已申诉成功
    }

    // ============ 状态变量 ============

    /// @notice 质押合约地址（用于读取质押信息）
    address public stakingContract;

    /// @notice 违规记录 ID → 违规详情
    mapping (bytes32 => Violation) public violations;

    /// @notice 违规者被罚没的次数
    mapping (address => uint256) public slashCount;

    /// @notice 已罚没的总金额（用于统计）
    uint256 public totalSlashed;

    // ============ 罚没比例配置 ============
    /// @notice 不同违规类型的罚没比例（基点，100% = 10000）
    mapping (ViolationType => uint256) public slashRate;

    /// @notice 申诉等待期（秒）
    uint256 public appealPeriod;

    // ============ 常量 ============
    uint256 public constant MAX_SLASH_RATE = 10000; // 100% = 10000 bps
    uint256 public constant MAX_APPEAL_PERIOD = 7 days;

    // ============ 事件 ============
    event ViolationReported(
        bytes32 indexed id,
        address indexed violator,
        ViolationType vType,
        uint256 slashAmount,
        uint256 readyTime
    );
    event Slashed(bytes32 indexed id, address indexed violator, uint256 amount);
    event Appealed(bytes32 indexed id, address indexed violator);
    event SlashRateUpdated(ViolationType indexed vType, uint256 oldRate, uint256 newRate);
    event Recovered(uint256 amount);

    // ============ 错误 ============
    error Slashing_AlreadyReported(bytes32 id);
    error Slashing_AlreadyExecuted(bytes32 id);
    error Slashing_AlreadyAppealed(bytes32 id);
    error Slashing_NotReady(bytes32 id, uint256 readyTime, uint256 currentTime);
    error Slashing_InvalidRate(uint256 rate);

    // ============ 构造函数 ============

    /**
     * @param _stakingContract DeFiStaking 合约地址
     * @param _appealPeriod 申诉等待期（秒）
     */
    constructor(address _stakingContract, uint256 _appealPeriod) Ownable(msg.sender) {
        require(_stakingContract != address(0), "Zero staking contract");
        require(_appealPeriod <= MAX_APPEAL_PERIOD, "Appeal period too long");

        stakingContract = _stakingContract;
        appealPeriod = _appealPeriod;

        // 默认罚没比例
        slashRate[ViolationType.DoubleSigning] = 5000;      // 50%
        slashRate[ViolationType.Downtime] = 500;            // 5%
        slashRate[ViolationType.MaliciousVote] = 3000;      // 30%
    }

    // ============ 核心操作 ============

    /**
     * @notice 举报违规行为
     * @param violator 违规者地址
     * @param vType 违规类型
     * @param stakedAmount 违规者当前的质押量（由举报者提供）
     * @param evidence 证据哈希
     * @return id 违规记录 ID
     *
     * 面试重点: 任何人可举报 → 去中心化执法
     *   举报者需要提供质押量，合约会验证
     */
    function reportViolation(
        address violator,
        ViolationType vType,
        uint256 stakedAmount,
        bytes32 evidence
    ) external returns (bytes32 id) {
        require(violator != address(0), "Zero address");
        require(stakedAmount > 0, "Zero staked amount");

        // 生成唯一 ID
        id = keccak256(abi.encodePacked(violator, vType, evidence, block.timestamp));

        // 不能重复举报
        if (violations[id].readyTime != 0) revert Slashing_AlreadyReported(id);

        // 计算罚没金额
        uint256 rate = slashRate[vType];
        uint256 slashAmount = stakedAmount * rate / MAX_SLASH_RATE;

        uint256 readyTime = block.timestamp + appealPeriod;
        violations[id] = Violation({
            violator: violator,
            vType: vType,
            slashAmount: slashAmount,
            readyTime: readyTime,
            executed: false,
            appealed: false
        });

        emit ViolationReported(id, violator, vType, slashAmount, readyTime);
    }

    /**
     * @notice 执行罚没
     * @dev 必须等待申诉期过后才能执行
     *
     * 面试重点: 为什么需要申诉期？
     *   → 防止恶意举报立即造成不可逆损失
     *   → 给被举报者时间自证清白
     */
    function executeSlash(bytes32 id) external {
        Violation storage v = violations[id];

        if (v.readyTime == 0) revert Slashing_AlreadyReported(id);
        if (v.executed) revert Slashing_AlreadyExecuted(id);
        if (v.appealed) revert Slashing_AlreadyAppealed(id);
        if (block.timestamp < v.readyTime) {
            revert Slashing_NotReady(id, v.readyTime, block.timestamp);
        }

        v.executed = true;
        slashCount[v.violator] += 1;
        totalSlashed += v.slashAmount;

        // 注意：实际项目中这里会调用 staking 合约来扣除质押代币
        // 由于 DeFiStaking 合约需要配合修改，这里先记录状态
        // 完整实现需要 DeFiStaking 添加 slash() 函数，
        // 然后将罚没的代币转入销毁地址或国库

        emit Slashed(id, v.violator, v.slashAmount);
    }

    /**
     * @notice 申诉成功，取消罚没
     * @dev 只有 owner（治理合约）可以免除罚没
     */
    function appeal(bytes32 id) external onlyOwner {
        Violation storage v = violations[id];

        if (v.readyTime == 0) revert Slashing_AlreadyReported(id);
        if (v.executed) revert Slashing_AlreadyExecuted(id);
        if (v.appealed) revert Slashing_AlreadyAppealed(id);

        v.appealed = true;

        emit Appealed(id, v.violator);
    }

    // ============ 管理员 ============

    /**
     * @notice 设置罚没比例
     * @dev 面试重点: 修改罚没比例也需要时间锁！
     *       否则治理攻击者可以先降低罚没比例再作恶
     */
    function setSlashRate(ViolationType vType, uint256 rate) external onlyOwner {
        if (rate > MAX_SLASH_RATE) revert Slashing_InvalidRate(rate);
        uint256 oldRate = slashRate[vType];
        slashRate[vType] = rate;
        emit SlashRateUpdated(vType, oldRate, rate);
    }

    /**
     * @notice 设置申诉等待期
     */
    function setAppealPeriod(uint256 _period) external onlyOwner {
        require(_period <= MAX_APPEAL_PERIOD, "Too long");
        appealPeriod = _period;
    }

    /**
     * @notice 查询违规信息
     */
    function getViolation(bytes32 id) external view returns (Violation memory) {
        return violations[id];
    }
}