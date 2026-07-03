// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title Voting
 * @notice 一个完整的投票合约，综合运用 mapping + struct + array + 可迭代模式
 * @dev Day 4 实战项目
 *
 * 功能：
 *  - 主席创建多个提案
 *  - 投票人在指定时间窗口内投票（每人一票）
 *  - 投票人可将投票权委托给其他人
 *  - 可查询所有提案、计票结果
 *
 * 知识点覆盖：
 *  - mapping 底层存储（keccak256）
 *  - struct 存储布局（打包规则）
 *  - 可迭代 mapping 模式
 *  - struct + mapping + array 组合
 *  - modifier 时间窗口控制
 *  - 自定义 error
 *  - 事件记录
 */

contract Voting {
    // ============================================================
    // Type Declarations
    // ============================================================

    /// @notice 选民结构体
    /// @dev 存储布局分析：
    ///   - weight (uint256, 32B) → slot 0 (独占，满32字节)
    ///   - voted  (bool, 1B)    ─┐
    ///   - vote   (uint8, 1B)   ─┤ slot 1 (打包)
    ///   - delegate (address,20B)─┘
    struct Voter {
        uint256 weight;     // 投票权重（默认 1，可被委托增加）
        bool voted;         // 是否已投票
        uint8 vote;         // 投给了哪个提案（索引）
        address delegate;   // 委托给了谁
    }

    /// @notice 提案结构体
    /// @dev 存储布局分析：
    ///   - name (bytes32, 32B)    → slot 0 (独占)
    ///   - voteCount (uint256,32B) → slot 1 (独占)
    struct Proposal {
        bytes32 name;       // 提案名称（用 bytes32 比 string 省 gas，因为定长)
        uint256 voteCount;  // 得票数
    }

    // ============================================================
    // State Variables
    // ============================================================

    /// @notice 主席（创建提案的人）
    address public chairperson;

    /// @notice 提案列表（用于遍历所有提案 —— 可迭代 mapping 的 key 列表角色）
    Proposal[] public proposals;

    /// @notice 地址 → 选民信息
    /// @dev mapping 的 slot 存空值，实际数据按 keccak256(addr, slot) 分布
    mapping (address => Voter) public voters;

    /// @notice 是否是已注册选民（可迭代 mapping 的去重 mapping）
    mapping (address => bool) public isRegistered;

    /// @notice 已注册选民列表（可迭代 mapping 的 key 列表）
    address[] private voterList;

    /// @notice 投票截止时间
    uint256 public votingDeadline;

    // ============================================================
    // Events
    // ============================================================

    event ProposalCreated(uint256 indexed proposalId, bytes32 name);
    event Voted(address indexed voter, uint256 indexed proposalId, uint256 weight);
    event Delegated(address indexed from, address indexed to);
    event VoterRegister(address indexed voter, uint256 weight);

    // ============================================================
    // Errors
    // ============================================================
    error NotChairperson(address caller);
    error AlreadyVoted(address voter);
    error AlreadyRegistered(address voter);
    error VotingNotStarted();
    error VotingEnded();
    error VotingStillActive();
    error InvalidProposal(uint256 proposalId);
    error SelfDelegationNotAllowed();
    error CannotVoteTwice();
    error NotRegistered(address voter);

    // ============================================================
    // Modifiers
    // ============================================================

    modifier onlyChairperson() {
        if (msg.sender != chairperson) revert NotChairperson(msg.sender);
        _;
    }

    modifier votingActive() {
        if (votingDeadline == 0) revert VotingNotStarted();
        if (block.timestamp > votingDeadline) revert VotingEnded();
        _;
    }

    modifier votingEnded() {
        if (votingDeadline == 0) revert VotingNotStarted();
        if (block.timestamp <= votingDeadline) revert VotingStillActive();
        _;
    }

    // ============================================================
    // Constructor
    // ============================================================

    /// @notice 初始化投票合约
    /// @param proposalNames 提案名称列表
    /// @param durationMinutes 投票持续时间（分钟）
    constructor(bytes32[] memory proposalNames, uint256 durationMinutes) {
        chairperson = msg.sender;

        // 创建所有提案
        for (uint256 i = 0; i < proposalNames.length; i++) {
            proposals.push(Proposal({
                name: proposalNames[i],
                voteCount: 0
            }));
            emit ProposalCreated(i, proposalNames[i]);
        }

        // 设置投票截止时间
        votingDeadline = block.timestamp + (durationMinutes * 1 minutes);

        // 主席自动注册为选民
        _registerVoter(msg.sender, 1);
    }

    // ============================================================
    // Voter Management (可迭代 mapping 模式)
    // ============================================================

    /// @notice 注册选民（only chairperson）
    /// @param voter 选民地址
    function giveRightToVote(address voter) external onlyChairperson votingActive {
        _registerVoter(voter, 1);
    }

    /// @notice 注册选民（only chairperson）
    /// @param voter 选民地址
    function _registerVoter(address voter, uint256 weight) private {
        // 可迭代 mapping：去重检查
        if (isRegistered[voter]) revert AlreadyRegistered(voter);

        // 可迭代 mapping： 记录 key
        isRegistered[voter] = true;
        voterList.push(voter);

        // 设置选民数据
        voters[voter] = Voter({
            weight: weight,
            voted: false,
            vote: 0,
            delegate: address(0)
        });

        emit VoterRegister(voter, weight);
    }

    // ============================================================
    // Voting Logic
    // ============================================================

    /// @notice 投票
    /// @param proposalId 提案索引
    function vote(uint256 proposalId) external votingActive {
        Voter storage sender = voters[msg.sender];

        // 检查是否已注册
        if (!isRegistered[msg.sender]) revert NotRegistered(msg.sender);
        // 检查是否已投票
        if (sender.voted) revert AlreadyVoted(msg.sender);
        // 检查提案是否有效
        if (proposalId >= proposals.length) revert InvalidProposal(proposalId);

        // CEI 模式：先更新状态
        sender.voted = true;
        sender.vote = uint8(proposalId);

        // 再修改其他状态
        proposals[proposalId].voteCount += sender.weight;

        emit Voted(msg.sender, proposalId, sender.weight);
    }

    // ============================================================
    // Delegation Logic
    // ============================================================

    /// @notice 将投票权委托给另一个选民
    /// @param to 受托人地址
    function delegate(address to) external votingActive {
        Voter storage sender = voters[msg.sender];
    
        // 检查是否已注册
        if (!isRegistered[msg.sender]) revert NotRegistered(msg.sender);
        // 不能委托自己
        if (to == msg.sender) revert SelfDelegationNotAllowed();
        // 不能重复投票
        if (sender.voted) revert AlreadyVoted(msg.sender);

        // 跟随委托人，找到最终委托人
        // 这就是可迭代 mapping 的一个实际应用场景--虽然这里遍历的是委托链 
        address currentDelegate = to;
        // 防止循环委托： 最多检查 voteList 长度次
        uint256 maxLoop = voterList.length;
        while (voters[currentDelegate].delegate != address(0) && maxLoop > 0) {
            currentDelegate = voters[currentDelegate].delegate;
            // 防止循环委托
            if (currentDelegate == msg.sender) revert("Circular delegation");
            unchecked {maxLoop--;}
        }

        // 检查最终委托人是否已注册
        if (!isRegistered[currentDelegate]) revert NotRegistered(currentDelegate);
    
        // 设置委托
        sender.voted = true;
        sender.delegate = currentDelegate;

        Voter storage delegateTo = voters[currentDelegate];

        if (delegateTo.voted) {
            // 如果委托人已经投过票，直接给他的提案加权重
            proposals[delegateTo.vote].voteCount += sender.weight;
        } else {
            // 如果委托人还没投票，增加他的权重
            delegateTo.weight += sender.weight;
        }

        emit Delegated(msg.sender, currentDelegate);
    }

    // ============================================================
    // Query Functions (遍历所有提案 —— 可迭代 mapping 的关键价值)
    // ============================================================

    /// @notice 获取所有提案
    function getProposals() external view returns (Proposal[] memory) {
        return proposals;
    }

    /// @notice 获取提案数量
    function proposalCount() external view returns (uint256) {
        return proposals.length;
    }

    /// @notice 获取所有已注册选民（可迭代 mapping 的价值体现）
    function getVoterList() external view returns (address[] memory) {
        return voterList;
    }

    /// @notice 获取选民数量
    function voterCount() external view returns (uint256) {
        return voterList.length;
    }

    // ============================================================
    // Result Calculation
    // ============================================================

    /// @notice 获取获胜提案的名称（投票结束后才能调用）
    /// @return winnerName_ 获胜提案的名称
    function winnerName() external view votingEnded returns (bytes32 winnerName_) {
        uint256 winnerVoteCount = 0;

        // 遍历所有提案找最高票数
        // 如果提案数量非常大，这个函数会消耗很多 gas
        // 实际生产中可以在投票时间同步跟新 winningProposalId
        for (uint256 i = 0; i < proposals.length; i++) {
            if (proposals[i].voteCount > winnerVoteCount) {
                winnerVoteCount = proposals[i].voteCount;
                winnerName_ = proposals[i].name;
            }
        }
    }

    /// @notice 获取获胜提案的完整信息
    function winningProposal() external view votingEnded returns (uint256 proposalId_, bytes32 name_, uint256 voteCount_) {
        uint256 winnerVoteCount = 0;

        // 遍历所有提案找最高票数
        // 如果提案数量非常大，这个函数会消耗很多 gas
        // 实际生产中可以在投票时间同步跟新 winningProposalId
        for (uint256 i = 0; i < proposals.length; i++) {
            if (proposals[i].voteCount > winnerVoteCount) {
                winnerVoteCount = proposals[i].voteCount;
                proposalId_ = i;
                name_ = proposals[i].name;
                voteCount_ = proposals[i].voteCount;
            }
        }
    }
}