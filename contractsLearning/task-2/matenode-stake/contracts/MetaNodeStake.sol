// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/Pausable.sol";
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

import "./MetaNodeToken.sol";

contract MetaNodeStake is ReentrancyGuard, AccessControl, Pausable {
    using SafeERC20 for IERC20;

    // 角色定义
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    bytes32 public constant UPGRADER_ROLE = keccak256("UPGRADER_ROLE");
    bytes32 public constant PAUSER_ROLE = keccak256("PAUSER_ROLE");

    // 功能控制暂停标志
    bool public stakePaused;
    bool public unstakePaused;
    bool public claimPaused;

    // 奖励代币
    MetaNodeToken public metaNodeToken;

    // 每个区块的基础奖励
    uint256 public rewardPerBlock;

    // 质押池结构体
    struct Pool {
        address stTokenAddress; // 质押代币的地址
        uint256 poolWeight; // 质押池的权重，影响奖励分配
        uint256 lastRewardBlock; // 最后一次计算奖励的区块号
        uint256 accMetaNodePerST; // 每个质押代币累积的 RCC 数量
        uint256 stTokenAmount; // 池中的总质押代币量
        uint256 minDepositAmount; // 最小质押金额
        uint256 unstakeLockedBlocks; // 解除质押的锁定区块数
    }

    // 用户结构体
    struct User {
        uint256 stAmount; // 用户质押的代币数量。
        uint256 finishedMetaNode; // 已分配的 MetaNode数量。
        uint256 pendingMetaNode; // 待领取的 MetaNode 数量。
        UnstakeRequest[] requests; // 解质押请求列表，每个请求包含解质押数量和解锁区块。
    }

    // 解质押请求
    struct UnstakeRequest {
        uint256 amount;
        uint256 unlockBlock;
    }

    // 质押池列表
    Pool[] public pools;

    // 用户数据映射：poolId => userAddress => User
    mapping(uint256 => mapping(address => User)) public users;

    // 事件定义
    event Staked(address indexed user, uint256 indexed pid, uint256 amount);
    event Unstaked(address indexed user, uint256 indexed pid, uint256 amount, uint256 unlockBlock);
    event RewardClaimed(address indexed user, uint256 pid, uint256 amount);
    event PoolAdded(uint256 indexed pid, address stToken, uint256 weight);
    event PoolSetUpdated(uint256 indexed pid, address stToken, uint256 weight);
    event Withdrawn(address indexed user, uint256 indexed pid, uint256 amount);
    event RewardPerBlockUpdated(uint256 newReward);

    constructor(address _metaNodeToken, uint256 _rewardPerBlock, address admin) {
        metaNodeToken = MetaNodeToken(_metaNodeToken);
        rewardPerBlock = _rewardPerBlock;

        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(ADMIN_ROLE, admin);
        _grantRole(UPGRADER_ROLE, admin);
        _grantRole(PAUSER_ROLE, admin);

        // 创建第一个质押池
        pools.push(Pool({
            stTokenAddress: address(0),
            poolWeight: 100,
            lastRewardBlock: block.number,
            accMetaNodePerST: 0,
            stTokenAmount: 0,
            minDepositAmount: 1 * 10 ** 18,
            unstakeLockedBlocks: 20160
        }));
    }

    // modifier 检查池是否存在
    modifier validPool(uint256 _pid) {
        require(_pid < pools.length, "Invalid pool ID");
        _;
    }

    // 更新奖励计算
    function updatePool(uint256 _pid) internal validPool(_pid) {
        Pool storage pool = pools[_pid];
        if(block.number <= pool.lastRewardBlock) {
            return;
        }

        if(pool.stTokenAmount == 0) {
            pool.lastRewardBlock = block.number;
            return;
        }

        // 计算区块奖励
        uint256 blockReward = (block.number - pool.lastRewardBlock) * rewardPerBlock;
        // 根据权重计算该池的奖励
        uint256 poolReward = blockReward * pool.poolWeight / totalWeight();
        // 更新累积奖励
        pool.accMetaNodePerST += poolReward * 1e18 / pool.stTokenAmount;
        pool.lastRewardBlock = block.number;
    }

    // 计算总权重
    function totalWeight() public view returns (uint256) {
        uint256 total = 0;
        for (uint256 i = 0; i < pools.length; i++) {
            total += pools[i].poolWeight;
        }
        return total;
    }

    // 质押功能
    function stake(uint256 _pid, uint256 _amount) external payable nonReentrant whenNotPaused validPool(_pid) {
        require(!stakePaused, "Staking is paused");
        Pool storage pool = pools[_pid];
        User storage user = users[_pid][msg.sender];

        require(_amount >= pool.minDepositAmount, "Amount below minimum");

        // 更新奖励
        updatePool(_pid);

        // 计算用户待领取奖励
        if(user.stAmount > 0) {
            uint256 pending = user.stAmount * pool.accMetaNodePerST / 1e18 - user.finishedMetaNode;
            user.pendingMetaNode += pending;
        }

        // 处理质押资产
        if(pool.stTokenAddress == address(0)) {
            // 原生代币
            require(msg.value == _amount, "Incorrect ETH amount");
        } else {
            // ERC20 代币
            require(msg.value == 0, "ETH not allowed for this pool");
            IERC20(pool.stTokenAddress).transferFrom(msg.sender, address(this), _amount);
        }

        // 更新用户和池数据
        user.stAmount += _amount;
        pool.stTokenAmount += _amount;
        user.finishedMetaNode = user.stAmount * pool.accMetaNodePerST / 1e18;

        emit Staked(msg.sender, _pid, _amount);
    }

    // 解除质押功能
    function unstake(uint256 _pid, uint256 _amount) external nonReentrant whenNotPaused validPool(_pid) {
        require(!unstakePaused, "Unstakeing is paused");
        Pool storage pool = pools[_pid];
        User storage user = users[_pid][msg.sender];

        require(user.stAmount >= _amount, "Insufficient staked amount");

        // 更新奖励
        updatePool(_pid);

        // 计算用户待领取奖励
        uint256 pending = user.stAmount * pool.accMetaNodePerST / 1e18 - user.finishedMetaNode;
        user.pendingMetaNode += pending;

        // 创建解质押请求
        uint256 unlockBlock = block.number + pool.unstakeLockedBlocks;
        user.requests.push(UnstakeRequest({
            amount: _amount,
            unlockBlock: unlockBlock
        }));

        // 更新用户和池数据
        user.stAmount -= _amount;
        pool.stTokenAmount -= _amount;
        user.finishedMetaNode = user.stAmount * pool.accMetaNodePerST / 1e18;

        emit Unstaked(msg.sender, _pid, _amount, unlockBlock);
    }

    // 提取已解锁的质押资产
    function withdraw(uint256 _pid, uint256 _requestIndex) external nonReentrant whenNotPaused validPool(_pid) {
        User storage user = users[_pid][msg.sender];
        Pool storage pool = pools[_pid];

        require(_requestIndex < user.requests.length, "Invalid request index");
        UnstakeRequest storage request = user.requests[_requestIndex];
        require(block.number >= request.unlockBlock, "Still locked");

        uint256 amount = request.amount;

        // 移除请求
        if (_requestIndex < user.requests.length - 1) {
            user.requests[_requestIndex] = user.requests[user.requests.length - 1];
        }
        user.requests.pop();

        // 转移资产给用户
        if (pool.stTokenAddress == address(0)) {
            // 原生代币
            (bool success, ) = msg.sender.call{ value: amount }("");
            require(success, "ETH transfer failed");
        } else {
            // ERC20 代币
            IERC20(pool.stTokenAddress).transfer(msg.sender, amount);
        }

        emit Withdrawn(msg.sender, _pid, amount);
    }

    // 领取奖励
    function claimReward(uint256 _pid) external nonReentrant whenNotPaused validPool(_pid) {
        require(!claimPaused, "Claiming is paused");
        Pool storage pool = pools[_pid];
        User storage user = users[_pid][msg.sender];

        // 更新奖励
        updatePool(_pid);

        // 计算总可领取奖励
        uint256 pending = user.stAmount * pool.accMetaNodePerST / 1e18 - user.finishedMetaNode + user.pendingMetaNode;
        require(pending > 0, "No reward to claim");

        // 重置奖励跟踪
        user.finishedMetaNode = user.stAmount * pool.accMetaNodePerST / 1e18;
        user.pendingMetaNode = 0;

        // 发放奖励
        metaNodeToken.mint(msg.sender, pending);

        emit RewardClaimed(msg.sender, _pid, pending);
    }

    // 添加新质押池 (管理员)
    function addPool(address _stTokenAddress, uint256 _poolWeight, uint256 _minDepositAmount, uint256 _unstakeLockedBlocks) external onlyRole(ADMIN_ROLE) {
        require(_poolWeight > 0, "Weight must be positive");
        require(_minDepositAmount > 0, "Minimum deposit must be positive");
        require(_unstakeLockedBlocks > 0, "Lock period must be positive");

        for(uint256 i = 0; i < pools.length; i++) {
            updatePool(i);
        }

        pools.push(Pool({
            stTokenAddress: _stTokenAddress,
            poolWeight: _poolWeight,
            lastRewardBlock: block.number,
            accMetaNodePerST: 0,
            stTokenAmount: 0,
            minDepositAmount: _minDepositAmount,
            unstakeLockedBlocks: _unstakeLockedBlocks
        }));

        emit PoolAdded(pools.length - 1, _stTokenAddress, _poolWeight);
    }

    // 更新质押池配置 (管理员)
    function updatePoolSet(uint256 _pid, uint256 _poolWeight, uint256 _minDepositAmount, uint256 _unstakeLockedBlocks) external onlyRole(ADMIN_ROLE) validPool(_pid) {
        require(_poolWeight > 0, "Weight must be positive");
        require(_minDepositAmount > 0, "Minimum deposit must be positive");
        require(_unstakeLockedBlocks > 0, "Lock period must be positive");

        Pool storage pool = pools[_pid];
        // 先更新奖励计算
        updatePool(_pid);

        pool.poolWeight = _poolWeight;
        pool.minDepositAmount = _minDepositAmount;
        pool.unstakeLockedBlocks = _unstakeLockedBlocks;

        emit PoolSetUpdated(_pid, pool.stTokenAddress, pool.poolWeight);
    }

    // 更新每个区块的奖励数量 (管理员)
    function setRewardPerBlock(uint256 _newReward) external onlyRole(ADMIN_ROLE) {
        require(_newReward > 0, "Reward must be positive");
        rewardPerBlock = _newReward;
        emit RewardPerBlockUpdated(_newReward);
    }

    // 暂停/恢复质押功能
    function setStakePaused(bool _paused) external onlyRole(PAUSER_ROLE) {
        stakePaused = _paused;
    }

    // 暂停/恢复解质押功能
    function setUnstakePaused(bool _paused) external onlyRole(PAUSER_ROLE) {
        unstakePaused = _paused;
    }

    // 暂停/恢复领奖功能
    function setClaimPaused(bool _paused) external onlyRole(PAUSER_ROLE) {
        claimPaused = _paused;
    }

    // 获取用户的解质押请求
    function getUserUnstakeRequests(uint256 _pid, address _user) external view validPool(_pid) returns (UnstakeRequest[] memory) {
        return users[_pid][_user].requests;
    }

    // 获取池数量
    function poolLength() external view returns (uint256) {
        return pools.length;
    }

    // 接收原生代币
    receive() external payable {
        // 只接受来自质押功能的原生代币
        require(msg.sender == address(this), "Direct ETH not allowed");
    }
}