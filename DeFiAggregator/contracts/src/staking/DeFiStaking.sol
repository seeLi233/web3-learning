// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title DeFiStaking
 * @notice 基于 Synthetix StakingRewards 模型的质押合约
 * @dev 核心思想: 累积 rewardPerToken（全局），用户 claim 时结算差额
 *
 * 数学公式:
 *   rewardPerToken = Σ (Δt × rewardRate × 1e18 / totalSupply)
 *   earned(user)   = balance(user) × (rewardPerToken - userSnapshot) / 1e18 + pending
 *
 * 安全要点:
 *   1. updateReward modifier 确保每次操作前结算该用户的奖励
 *   2. lastTimeRewardApplicable() 防止奖励周期结束后继续累积
 *   3. totalSupply == 0 时不更新 rewardPerToken（避免除以零）
 *   4. SafeERC20 防止假代币攻击
 *   5. ReentrancyGuard 防止重入攻击
 */
contract DeFiStaking is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ 不可变变量 ============
    IERC20 public immutable stakingToken; // 质押代币 (如 LP Token)
    IERC20 public immutable rewardsToken; // 奖励代币 (如治理代币)

    // ============ 状态变量 ============
    uint256 public totalSupply;           // 总质押量
    mapping (address => uint256) public balanceOf; // 每个用户的质押量

    // --- 奖励分发相关 ---
    uint256 public rewardRate;            // 每秒发放的奖励数量
    uint256 public periodFinish;          // 奖励发放结束时间
    uint256 public lastUpdateTime;        // 上次更新 rewardPerToken 的时间
    uint256 public rewardPerTokenStored;  // 累积的每代币奖励量

    // --- 用户快照 ---
    mapping (address => uint256) public userRewardPerTokenPaid; // 用户上次结算时的 rewardPerToken
    mapping (address => uint256) public rewards;                // 用户待领取的奖励

    // ============ 锁仓相关 (可选增强) ============
    mapping (address => uint256) public lockUntil; // 用户锁仓到期时间

    // ============ 常量 ============
    uint256 public constant PRECISION = 1e18;      // 精度因子
    uint256 public constant MAX_LOCK_DAYS = 365;   // 最大锁仓天数
    uint256 public constant MIN_LOCK_DAYS = 7;     // 最小锁仓天数（防止粉尘攻击）

    // ============ 事件 ============
    event Staked(address indexed user, uint256 amount, uint256 lockDays);
    event Unstaked(address indexed user, uint256 amount);
    event RewardClaimed(address indexed user, uint256 reward);
    event RewardNotified(uint256 amount, uint256 rewardRate, uint256 periodFinish);
    event Recovered(address token, uint256 amount);

    // ============ 修饰器 ============

    /**
     * @notice 在每次 stake/unstake/claim 前更新该用户的奖励
     * @dev 这是整个合约最关键的修饰器!
     *      1. 先更新全局 rewardPerToken
     *      2. 再结算该用户的待领奖励
     *      3. 更新用户快照
     */
    modifier updateReward(address account) {
        // Step 1: 更新全局累积量
        rewardPerTokenStored = rewardPerToken();
        lastUpdateTime = lastTimeRewardApplicable();

        // Step 2: 如果不是零地址(零地址用于 notifyRewardAmount)
        if (account != address(0)) {
            // 结算该用户的奖励
            rewards[account] = earned(account);
            // 更新该用户的快照
            userRewardPerTokenPaid[account] = rewardPerTokenStored;
        }
        _;
    }

    // ============ 构造函数 ============

    /**
     * @param _stakingToken 质押代币地址
     * @param _rewardsToken 奖励代币地址
     */
    constructor(address _stakingToken, address _rewardsToken) Ownable(msg.sender) {
        require(_stakingToken != address(0), "Zero staking token");
        require(_rewardsToken != address(0), "Zero rewards token");
        stakingToken = IERC20(_stakingToken);
        rewardsToken = IERC20(_rewardsToken);
    }

    // ============ 只读函数 (面试重点: earned() 的逻辑) ============

    /**
     * @notice 当前有效的截止时间
     * @dev 如果奖励还在发放中，返回当前时间；
     *      如果奖励已发完，返回 periodFinish（不再累积）
     */
    function lastTimeRewardApplicable() public view returns (uint256) {
        return block.timestamp < periodFinish ? block.timestamp : periodFinish;
    }

    /**
     * @notice 当前每个质押代币累积了多少奖励
     * @dev 核心公式: 已累积 + 本轮增量 × 精度 / 总质押量
     *      如果 totalSupply == 0，不累积（避免除以零 + 避免浪费奖励）
     */
    function rewardPerToken() public view returns (uint256) {
        if (totalSupply == 0) return rewardPerTokenStored;

        return rewardPerTokenStored
            + (lastTimeRewardApplicable() - lastUpdateTime) // Δt (秒)
            * rewardRate                                    // 每秒奖励量
            * PRECISION                                     // 精度因子
            / totalSupply;                                  // 除以总质押量
    }

    /**
     * @notice 查询某用户的待领奖励
     * @dev 面试高频! 公式:
     *   earned = 当前份额 × 累积差 / 精度 + 未领取奖励
     *
     *   其中"累积差" = 全局累积 - 用户上次结算时的累积
     *   这个差值就是该用户应该获得的新奖励
     */
    function earned(address account) public view returns (uint256) {
        return balanceOf[account] * (rewardPerToken() - userRewardPerTokenPaid[account])
            / PRECISION
            + rewards[account];
    }

    /**
     * @notice 奖励还剩多久发完
     */
    function rewardDuration() external view returns (uint256) {
        if (periodFinish <= block.timestamp) return 0;
        return periodFinish - block.timestamp;
    }

    // ============ 写入函数 ============

    /**
     * @notice 质押代币
     * @param amount 质押数量
     * @param lockDays 锁仓天数 (0 = 不锁仓, 7-365 = 锁仓)
     * 
     * 流程:
     *   1. updateReward: 结算用户之前的奖励
     *   2. 转账: 用户 → 合约
     *   3. 更新状态: balance, totalSupply
     *   4. (可选) 设置锁仓
     */
    function stake(uint256 amount, uint256 lockDays) external nonReentrant updateReward(msg.sender) {
        require(amount > 0, "Cannot stake 0");

        if (lockDays > 0) {
            require(lockDays >= MIN_LOCK_DAYS, "Lock too short");
            require(lockDays <= MAX_LOCK_DAYS, "Lock too long");
            // 如果已经在锁仓中，只能延长不能缩短
            uint256 newUnlock = block.timestamp + lockDays * 1 days;
            if (lockUntil[msg.sender] > 0) {
                require(newUnlock >= lockUntil[msg.sender], "Cannot shorten lock");
            }
            lockUntil[msg.sender] = newUnlock;
        }

        balanceOf[msg.sender] += amount;
        totalSupply += amount;

        stakingToken.safeTransferFrom(msg.sender, address(this), amount);
        
        emit Staked(msg.sender, amount, lockDays);
    }

    /**
     * @notice 解除质押
     * @param amount 解除数量
     * 
     * 流程:
     *   1. updateReward: 结算奖励
     *   2. 检查余额 + 锁仓状态
     *   3. 转账: 合约 → 用户
     *   4. 更新状态
     */
    function unstake(uint256 amount) external nonReentrant updateReward(msg.sender) {
        require(amount > 0, "Cannot unstake 0");
        require(balanceOf[msg.sender] >= amount, "Insufficient balance");

        // 检查锁仓是否到期
        if (lockUntil[msg.sender] > 0) {
            require(block.timestamp >= lockUntil[msg.sender], "Tokens are locked");
        }

        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;

        stakingToken.safeTransfer(msg.sender, amount);

        emit Unstaked(msg.sender, amount);
    }

    /**
     * @notice 领取奖励
     *
     * 流程:
     *   1. updateReward: 结算奖励
     *   2. 取出当前累积的奖励
     *   3. 置零 pending rewards
     *   4. 转账奖励代币
     */
    function claimReward() public nonReentrant updateReward(msg.sender) {
        uint256 reward = rewards[msg.sender];
        require(reward > 0, "No rewards to claim");

        rewards[msg.sender] = 0;

        rewardsToken.safeTransfer(msg.sender, reward);

        emit RewardClaimed(msg.sender, reward);
    }

    /**
     * @notice Synthetix 风格的别名
     */
    function getReward() external {
        claimReward();
    }

    // ============ 管理员函数 ============

    /**
     * @notice 向质押池注入奖励
     * @param amount 奖励总量
     * @dev 只有合约 owner 可以调用
     *
     * 面试重点:
     *   1. 如果当前还有奖励没发完，先把剩余的加到新奖励里
     *   2. 重新计算 rewardRate
     *   3. 设置新的 periodFinish
     *
     * 公式:
     *   if 还有剩余: amount += 剩余量 × 剩余时间
     *   rewardRate = amount / DURATION
     *   periodFinish = now + DURATION
     */
    function notifyRewardAmount(uint256 amount) external onlyOwner nonReentrant updateReward(address(0)) {
        require(amount > 0, "Amount must be > 0");

        // 计算剩余奖励: 如果当前周期还没结束，剩余奖励 = rewardRate × 剩余时间
        if (block.timestamp < periodFinish) {
            uint256 remaining = (periodFinish - block.timestamp) * rewardRate;
            amount += remaining;
        }

        // 默认奖励发放周期: 7 天
        uint256 DURATION = 7 days;
        rewardRate = amount / DURATION;
        periodFinish = block.timestamp + DURATION;

        // 转账奖励代币到合约
        rewardsToken.safeTransferFrom(msg.sender, address(this), amount);

        // 重置更新时间（因为在 updateReward modifier 中已经更新过了，这里再设一次）
        lastUpdateTime = block.timestamp;

        emit RewardNotified(amount, rewardRate, periodFinish);
    }

    /**
     * @notice 紧急回收非质押/奖励代币（防止用户误转）
     */
    function recoverToken(address tokenAddress, uint256 amount) external onlyOwner {
        require(
            tokenAddress != address(stakingToken) && tokenAddress != address(rewardsToken),
            "Cannot recover staking or rewards token"
        );
        IERC20(tokenAddress).safeTransfer(owner(), amount);
        emit Recovered(tokenAddress, amount);
    }
}