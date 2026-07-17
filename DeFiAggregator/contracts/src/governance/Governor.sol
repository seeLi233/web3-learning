// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TimelockController.sol";
import "./Delegation.sol";

/**
 * @title Governor
 * @notice Compound Governor Bravo 风格的 DAO 治理合约
 * @dev 整合 Delegation（投票权重） + Timelock（延迟执行）
 *
 * 提案生命周期:
 *   propose() → [Pending] → [Active: 投票] → [Succeeded/Defeated]
 *   → queue() → [Queued: 等待 Timelock] → execute() → [Executed]
 *
 * 面试重点: Governor 为什么不做实际执行？
 *   → 分离关注点: Governor 负责"决策"(投票)，Timelock 负责"执行"(安全延迟)
 *   → 如果 Governor 直接执行，恶意提案通过后立刻生效 → 闪电贷攻击
 */
contract Governor {
    // ============ 提案状态枚举 ============
    // 面试: 能说出 7 个状态吗？
    enum ProposalState {
        Pending,    // 0: 刚创建，等待 votingDelay 到期
        Active,     // 1: 投票中
        Canceled,   // 2: 被取消
        Defeated,   // 3: 投票未通过（反对多 或 未达 Quorum）
        Succeeded,  // 4: 投票通过
        Queued,     // 5: 已加入 Timelock 队列
        Expired,    // 6: Queued 后超时未执行（grave period 到期）
        Executed    // 7: 已执行
    }

    // ============ 投票类型 ============
    // 面试: 为什么 Against=0 而不是 1？
    //   → 默认值 0 意味着"不投票"和"反对"在数值上一致
    //   → 但实际上 castVote 时必须显式传入，不会混淆
    uint8 public constant VOTE_AGAINST = 0;
    uint8 public constant VOTE_FOR = 1;
    uint8 public constant VOTE_ABSTAIN = 2;

    // ============ 提案结构体 ============
    struct Proposal {
        uint256 id;                             // 提案 ID
        address proposer;                       // 提案人
        address[] targets;                      // 目标合约地址
        uint256[] values;                       // 发送 ETH 数量
        bytes[] calldatas;                      // 调用的 calldata
        uint256 startBlock;                     // 投票开始区块
        uint256 endBlock;                       // 投票结束区块
        uint256 forVotes;                       // 赞成票
        uint256 againstVotes;                   // 反对票
        uint256 abstainVotes;                   // 弃权票
        bool canceled;                          // 是否被取消
        bool executed;                          // 是否已执行
        mapping (address => Receipt) receipts;  // 投票收据
    }

    // 投票收据 — 记录每个地址的投票
    struct Receipt {
        bool hasVoted;      // 是否已投票
        uint8 support;      // 投票类型: 0=反对, 1=赞成, 2=弃权
        uint256 votes;      // 投了多少票
    }

    // ============ 配置参数（不可变） ============
    /// @notice 创建提案所需的最低投票权
    uint256 public proposalThreshold;

    /// @notice 提案创建后等待多少区块开始投票
    uint256 public votingDelay;

    /// @notice 投票持续多少区块
    uint256 public votingPeriod;

    /// @notice 法定人数
    uint256 public quorum;

    /// @notice 提案通过后，在 Timelock 中过期的时间（秒）
    /// @dev 防止旧提案堆积在 Timelock 中
    uint256 public constant GRACE_PERIOD = 14 days;

    // ============ 外部依赖 ============
    TimelockController public timelock;
    Delegation public delegation;

    // ============ 事件 ============
    event ProposalCreated(
        uint256 indexed id,
        address indexed proposer,
        address[] targets,
        uint256[] values,
        string[] signatures,
        bytes[] calldatas,
        uint256 startBlock,
        uint256 endBlock,
        string description
    );

    event VoteCast(
        address indexed voter,
        uint256 indexed proposalId,
        uint8 support,
        uint256 votes,
        string reason
    );

    event ProposalCanceled(uint256 indexed id);
    event ProposalQueued(uint256 indexed id, uint256 eta);  // eta = estimated time of arrival
    event ProposalExecuted(uint256 indexed id);

    // ============ 错误 ============
    error Governor__AlreadyVoted(address voter, uint256 proposalId);
    error Governor__InvalidProposalId(uint256 proposalId);
    error Governor__NotActive(uint256 proposalId);
    error Governor__NotSucceeded(uint256 proposalId);
    error Governor__NotQueued(uint256 proposalId);
    error Governor__TimelockNotReady(uint256 proposalId, uint256 eta);
    error Governor__ProposalExpired(uint256 proposalId);
    error Governor__BelowThreshold(address proposer, uint256 votes, uint256 threshold);
    error Governor__NotProposer();
    error Governor__VotingNotEnded();

    // ============ 提案存储 ============
    mapping (uint256 => Proposal) private _proposals;
    uint256 public proposalCount;

    // ============ 构造函数 ============
    constructor(
        address _timelock,
        address _delegation,
        uint256 _votingDelay,
        uint256 _votingPeriod,
        uint256 _proposalThreshold,
        uint256 _quorum
    ) {
        require(_timelock != address(0), "Governor: zero timelock");
        require(_delegation != address(0), "Governor: zero delegation");
        require(_votingPeriod > 0, "Governor: voting period zero");

        timelock = TimelockController(_timelock);
        delegation = Delegation(_delegation);
        votingDelay = _votingDelay;
        votingPeriod = _votingPeriod;
        proposalThreshold = _proposalThreshold;
        quorum = _quorum;
    }

    // ============ 提案创建 ============

    /**
     * @notice 创建新提案
     * @param targets    目标合约地址数组（一次提案可以执行多个操作）
     * @param values     每个调用发送的 ETH 数量
     * @param calldatas  每个调用的 calldata（函数选择器 + ABI 编码参数）
     * @param description 提案描述（标题 + 详细说明的 IPFS 链接）
     * @return proposalId 提案 ID
     *
     * ⭐ 面试重点: 为什么 targets/values/calldatas 是数组？
     *   → 一个提案可以包含多个链上操作（批量执行）
     *   → 例如: 提案 #42 = [给 A 转 100 ETH, 修改参数 X 为 5, 升级合约 Y]
     *   → 这些操作必须原子执行（全部成功或全部失败）
     *
     * ⭐ 面试重点: description 不存原文只存 hash 的原因？
     *   → Gas 优化: description 可以很长，只存 hash 到链上
     *   → 原文存在 event 的 log 中（更便宜）
     *
     * 防止垃圾提案的三层防护:
     *   1. proposalThreshold: 必须持有足够投票权
     *   2. votingDelay: 提案创建后有等待期
     *   3. 投票阶段: 没通过的提案自动失败
     */
    function propose(
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        string memory description
    ) public returns (uint256) {
        // 检查1: 提案人必须有足够投票权重
        uint256 proposalVotes = delegation.getVotes(msg.sender);
        if (proposalVotes < proposalThreshold) {
            revert Governor__BelowThreshold(msg.sender, proposalVotes, proposalThreshold);
        }

        // 检查2: 数组长度必须一致
        require(targets.length == values.length, "Governor: length mismatch");
        require(targets.length == calldatas.length, "Governor: length mismatch");
        require(targets.length > 0, "Governor: empty proposal");
        require(bytes(description).length > 0, "Governor: empty description");

        // 生成唯一提案 ID
        // = keccak256(targets, values, calldatas, keccak256(description))
        uint256 proposalId = _hashProposal(targets, values, calldatas, keccak256(bytes(description)));

        // 防止重复提案
        require(_proposals[proposalId].endBlock == 0, "Governor: proposal exists");

        // 计算投票时间窗口
        uint256 startBlock = block.number + votingDelay;
        uint256 endBlock = startBlock + votingPeriod;

        proposalCount++;

        // 存储提案（注意: mapping 字段 receipts 会自动初始化）
        Proposal storage proposal = _proposals[proposalId];
        proposal.id = proposalId;
        proposal.proposer = msg.sender;
        proposal.targets = targets;
        proposal.values = values;
        proposal.calldatas = calldatas;
        proposal.startBlock = startBlock;
        proposal.endBlock = endBlock;

        emit ProposalCreated(
            proposalId, msg.sender, targets, values,
            _getSigntures(calldatas),   // 从 calldata 提取函数签名（可读性）
            calldatas, startBlock, endBlock, description
        );

        return proposalId;
    }

    /**
     * @notice 从 calldata 数组提取函数选择器（仅用于 event 的可读性）
     */
    function _getSigntures(bytes[] memory calldatas) internal pure returns (string[] memory signatures) {
        signatures = new string[](calldatas.length);
        for(uint256 i = 0; i < calldatas.length; i++) {
            if (calldatas[i].length >= 4) {
                bytes4 selector;
                // 提取前 4 字节
                // 为什么这样写？

                // calldatas 是一个 bytes[] memory 数组，在 EVM 内存中的布局是：

                // calldatas 地址 → [数组长度 N]
                // calldatas + 32 → [calldatas[0] 的指针]
                // calldatas + 64 → [calldatas[1] 的指针]
                // calldatas + 96 → [calldatas[2] 的指针]
                // ...

                // 所以 calldatas[i] 的指针位置 = calldatas + 32 + i * 32（跳过长度 + i 个元素指针）。

                // 然后这个指针指向的 bytes 内存布局是：

                // 指针位置 → [bytes 长度 L]
                // 指针 + 32 → [实际数据开始，前 4 字节就是 selector]

                // 所以 mload(add(elemPtr, 32)) 读出前 32 字节数据（低 4 字节就是函数选择器）。
                assembly {
                    // calldatas 是 bytes[] memory 类型
                    // 内存布局: [array_length][elem0_ptr][elem1_ptr]...
                    // 每个元素指针指向: [bytes_length][data...]
                    let elemPtr := mload(add(calldatas, add(32, mul(i, 32))))
                    selector := mload(add(elemPtr, 32))
                }
                signatures[i] = _selectorToString(selector);
            }
        }
    }

    /**
     * @notice bytes4 → string (例如 0xa9059cbb → "transfer(address,uint256)")
     * @dev 这只是演示，生产环境用签名数据库
     */
    function _selectorToString(bytes4 selector) internal pure returns (string memory) {
        bytes memory result = new bytes(10);
        result[0] = "0";
        result[1] = "x";
        bytes4ToHex(selector, result, 2);
        return string(result); 
    }

    function bytes4ToHex(bytes4 b, bytes memory result, uint256 offset) internal pure {
        for (uint256 i = 0; i < 4; i++) {
            uint8 byteVal = uint8(b[i]);
            uint8 high = byteVal / 16;
            uint8 low = byteVal % 16;
            result[offset + i * 2] = high < 10 ? bytes1(uint8(bytes1("0")) + high): bytes1(uint8(bytes1("a")) + high - 10);
            result[offset + i * 2 + 1] = low < 10 ? bytes1(uint8(bytes1("0")) + low) : bytes1(uint8(bytes1("a")) + low - 10);
        }
    }

    /**
     * @notice 生成提案 ID 的 hash
     */
    function _hashProposal(
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 description 
    ) internal pure returns (uint256) {
        return uint256(keccak256(abi.encode(targets, values, calldatas, description)));
    }

    /**
     * @notice 公开的 hashProposal 函数，方便链下计算
     */
    function hashProposal(
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas,
        bytes32 description
    ) public pure returns (uint256) {
        return _hashProposal(targets, values, calldatas, description);
    }

    // ============ 投票 ============

    /**
     * @notice 对提案投票
     * @param proposalId 提案 ID
     * @param support    投票类型: 0=反对, 1=赞成, 2=弃权
     *
     * ⭐ 面试重点: 投票权从哪里来？
     *   → delegation.getPastVotes(voter, proposal.startBlock)
     *   → 使用 startBlock 时的投票权重（历史快照）
     *   → 防止"先投票，再买票，再投票"的双重投票攻击
     *
     * ⭐ 为什么用 getPastVotes 而不是 getVotes？
     *   → 投票权重必须锚定在投票开始时的区块
     *   → 如果投票期间可以转账改变投票权，同一批代币可能投多次
     */
    function castVote(uint256 proposalId, uint8 support) public {
        _castVote(msg.sender, proposalId, support, "");
    }

    /**
     * @notice 投票并附理由
     * @param reason 投票理由（记录在 event 中，不上链存储）
     *
     * 面试: reason 为什么不存 storage？
     *   → 字符串存储极贵（每个字节都消耗 Gas）
     *   → 事件 log 足够（链下可查询，永久存储）
     */
    function castVoteWithReason(uint256 proposalId, uint8 support, string memory reason) public {
        _castVote(msg.sender, proposalId, support, reason);
    }

    /**
     * @notice 内部投票逻辑
     */
    function _castVote(
        address voter,
        uint256 proposalId,
        uint8 support,
        string memory reason
    )  internal {
        // 检查1: 提案存在且处于 Active 状态
        ProposalState state = stateOf(proposalId);
        if (state != ProposalState.Active) revert Governor__NotActive(proposalId);
    
        // 检查2: 投票类型合法
        require(support <= 2, "Governor: invalid vote type");

        Proposal storage proposal = _proposals[proposalId];

        // 检查3: 不能重复投票
        Receipt storage recepit = proposal.receipts[voter];
        if (recepit.hasVoted) revert Governor__AlreadyVoted(voter, proposalId);

        // 获取投票开始时的历史投票权重
        // ⚡ 这是治理安全的核心: 用 past votes 防止闪贷攻击
        uint256 votes = delegation.getPastVotes(voter, proposal.startBlock);
        require(votes > 0, "Governor: no votes");

        // 记录投票收据
        recepit.hasVoted = true;
        recepit.support = support;
        recepit.votes = votes;

        // 计票
        _countVote(proposal, support, votes);

        emit VoteCast(voter, proposalId, support, votes, reason);
    }

    /**
     * @notice 计票 — 根据投票类型累加到对应计数器
     * @dev 面试重点: Abstain 只影响 Quorum 不影响胜负
     *   For > Against → 提案通过（前提: 总票数 ≥ Quorum）
     */
    function _countVote(Proposal storage proposal, uint8 support, uint256 votes) internal {
        if (support == VOTE_FOR) {
            proposal.forVotes += votes;
        } else if (support == VOTE_AGAINST) {
            proposal.againstVotes += votes;
        } else {
            proposal.abstainVotes += votes;
        }
    }

    // ============ 状态查询 ============

    /**
     * @notice 查询提案当前状态
     * @dev ⭐⭐⭐ 面试必问！这是 Governor 最核心的逻辑
     *
     * 状态转换图:
     *   [创建时] → Pending (block < startBlock)
     *   [投票期] → Active  (startBlock ≤ block ≤ endBlock)
     *   [投票结束] → 根据结果判断:
     *     canceled=true    → Canceled
     *     for ≤ against    → Defeated
     *     quorum 未达标    → Defeated
     *     quorum 达标 + for > against → Succeeded
     *   [已 queued] → Queued
     *   [Queued + 超时] → Expired
     *   [已 execute] → Executed
     */
    function stateOf(uint256 proposalId) public view returns (ProposalState) {
        Proposal storage proposal = _proposals[proposalId];
        if (proposal.endBlock == 0) revert Governor__InvalidProposalId(proposalId);

        // 已执行 → 终极状态
        if (proposal.executed) return ProposalState.Executed;

        // 已取消 → 不可逆
        if (proposal.canceled) return ProposalState.Canceled;

        // 还没到投票开始时间
        if (block.number < proposal.startBlock) return ProposalState.Pending;

        // 在投票期间内
        if (block.number <= proposal.endBlock) return ProposalState.Active;

        // 投票结束，检查是否已 Queued
        // 注意: Queued 状态通过存储标记判断（queue 函数设置）
        // 这里用 Timelock 的状态查询
        bytes32 timelockId = _getTimelockId(proposal);
        if (timelockId != bytes32(0)) {
            if (timelock.isOperationDone(timelockId)) return ProposalState.Executed;

            // ⭐ 新增: 检查是否过期
            if (timelock.isOperationExpired(timelockId)) return ProposalState.Expired;

            if (timelock.isOperationScheduled(timelockId)) return ProposalState.Queued;
        }

        // Queued 之后超时未执行 — Expired
        // （简易判断: 如果已经过了 Timelock readyTime + GRACE_PERIOD）

        // 投票结束，判断结果
        // 法定人数检查
        uint256 totalVotes = proposal.forVotes + proposal.againstVotes + proposal.abstainVotes;
        if (totalVotes < quorum) return ProposalState.Defeated;

        // 赞成 vs 反对
        if (proposal.forVotes <= proposal.againstVotes) return ProposalState.Defeated;

        // 投票通过！
        return ProposalState.Succeeded;
    }

    // ============ 排队与执行 ============

    /**
     * @notice 投票通过后，将提案加入 Timelock 队列
     * @dev 任何人都可以调用 queue()
     *
     * ⭐ 面试重点: 为什么任何人都能 queue？
     *   → 去中心化: 不需要依赖特定角色
     *   → 如果只有特定地址能 queue，该地址作恶/丢失 → 所有通过的提案都执行不了
     *
     * queue() 做的事情:
     *   1. 验证提案状态 = Succeeded
     *   2. 把提案的 targets/values/calldatas 提交给 Timelock.schedule()
     *   3. Timelock 计算 readyTime = now + minDelay
     *   4. 等待 minDelay 后才能 execute()
     */
    function queue(uint256 proposalId) public {
        ProposalState state = stateOf(proposalId);
        if(state != ProposalState.Succeeded) revert Governor__NotSucceeded(proposalId);

        Proposal storage proposal = _proposals[proposalId];

        // 计算 Timelock 中的 eta (estimated time of arrival)
        uint256 eta = block.timestamp + timelock.minDelay();

        // 把提案操作加入 Timelock
        // 注意: 需要 Governor 有 Timelock 的 PROPOSER_ROLE
        // for(uint256 i = 0; i < proposal.targets.length; i++) {
        //     timelock.schedule(
        //         proposal.targets[i],
        //         proposal.values[i],
        //         proposal.calldatas[i],
        //         bytes32(0),         // predecessor: 无前置操作
        //         bytes32(0)          // salt: 用默认值
        //     );
        // }
        // ✅ 改进: 用 scheduleBatch 一次性排队所有操作
        timelock.scheduleBatch(
            proposal.targets,
            proposal.values,
            proposal.calldatas,
            bytes32(0),         // predecessor: 无前置依赖
            bytes32(0),         // salt: 用默认值
            timelock.minDelay() // delay
        );

        emit ProposalQueued(proposalId, eta);
    }

    /**
     * @notice 执行已通过且 Timelock 到期的提案
     *
     * 检查清单:
     *   1. 提案状态 = Succeeded 或 Queued
     *   2. Timelock 等待时间已到
     *   3. 未超 Grace Period
     */
    function execute(uint256 proposalId) public payable {
        ProposalState state = stateOf(proposalId);

        // 必须先 queue() 进入 Queued 状态才能 execute()
        if (state != ProposalState.Queued) {
            revert Governor__NotQueued(proposalId);
        }

        Proposal storage proposal = _proposals[proposalId];

        // 调用 Timelock.execute() 执行每个操作
        // for(uint256 i = 0; i < proposal.targets.length; i++) {
        //     timelock.execute(
        //         proposal.targets[i],
        //         proposal.values[i],
        //         proposal.calldatas[i],
        //         bytes32(0),
        //         bytes32(0)
        //     );
        // }
        // ✅ 改进: 用 executeBatch 原子执行所有操作
        timelock.executeBatch(
            proposal.targets,
            proposal.values,
            proposal.calldatas,
            bytes32(0),     // predecessor
            bytes32(0)      // salt
        );

        // CEI: 外部调用成功后再标记为已执行
        proposal.executed = true;

        emit ProposalExecuted(proposalId);
    }

    /**
     * @notice 提案人可以取消自己的提案（投票结束前）
     * @dev 面试: 为什么只有提案人能取消？
     *   → 防止恶意取消别人的提案
     *   → 但投票通过后不能取消（已经进入执行流程）
     */
    function cancel(uint256 proposalId) public {
        Proposal storage proposal = _proposals[proposalId];
        if (proposal.endBlock == 0) revert Governor__InvalidProposalId(proposalId);
        if (msg.sender != proposal.proposer) revert Governor__NotProposer();

        ProposalState state = stateOf(proposalId);
        // 只能在投票结束前取消
        if (state != ProposalState.Pending && state != ProposalState.Active) {
            revert("Governor: cannot cancel");
        }

        proposal.canceled = true;

        emit ProposalCanceled(proposalId);
    }

    /**
     * @notice 生成提案对应的 Timelock 操作 ID
     */
    function _getTimelockId(Proposal storage proposal) internal view returns (bytes32) {
        // 取第一个操作来检查 Timelock 状态
        if (proposal.targets.length == 0) return bytes32(0);
        return timelock.hashOperationBatch(
            proposal.targets,
            proposal.values,
            proposal.calldatas,
            bytes32(0),     // predecessor
            bytes32(0)      // salt
        );
    }

    // ============ 查询函数 ============

    /**
     * @notice 获取提案基本信息
     */
    function getProposal(uint256 proposalId) public view returns (
        address proposer,
        uint256 startBlock,
        uint256 endBlock,
        uint256 forVotes,
        uint256 againstVotes,
        uint256 abstainVotes,
        ProposalState state
    ) {
        Proposal storage p = _proposals[proposalId];
        if (p.endBlock == 0) revert Governor__InvalidProposalId(proposalId);
        return (
            p.proposer,
            p.startBlock,
            p.endBlock,
            p.forVotes,
            p.againstVotes,
            p.abstainVotes,
            stateOf(proposalId)
        );     
    }

    /**
     * @notice 获取提案的预计执行时间（ETA）
     * @dev 前端用这个展示倒计时
     * @return eta 秒级 Unix 时间戳，0 表示未排队
     */
    function getProposalEta(uint256 proposalId) public view returns (uint256) {
        Proposal storage proposal = _proposals[proposalId];
        if (proposal.endBlock == 0) revert Governor__InvalidProposalId(proposalId);

        bytes32 timelockId = _getTimelockId(proposal);
        if (timelockId == 0) return 0;

        // 如果操作已执行，返回 0
        if (timelock.isOperationDone(timelockId)) return 0;

        // 如果已过期，返回 0
        if (timelock.isOperationExpired(timelockId)) return 0;

        // try-catch 处理未排队的情况
        try timelock.getTimestamp(timelockId) returns (uint256 timestamp) {
            return timestamp;
        } catch {
            return 0;
        }
    }

    /**
     * @notice 获取某个地址在某个提案的投票收据
     */
    function getReceipt(uint256 proposalId, address voter) public view returns (
        bool hasVoted,
        uint8 support,
        uint256 votes
    ) {
      Receipt storage receipt = _proposals[proposalId].receipts[voter];
      return (receipt.hasVoted, receipt.support, receipt.votes);  
    }

    /**
     * @notice 获取提案的操作列表（用于 queue/execute 的链下准备）
     */
    function getActions(uint256 proposalId) public view returns (
        address[] memory targets,
        uint256[] memory values,
        bytes[] memory calldatas0
    ) {
        Proposal storage p = _proposals[proposalId];
        if (p.endBlock == 0) revert Governor__InvalidProposalId(proposalId);
        return (p.targets, p.values, p.calldatas);
    }
}