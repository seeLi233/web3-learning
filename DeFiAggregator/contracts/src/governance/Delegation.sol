// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title Delegation
 * @notice Compound 风格的投票权委托合约
 * @dev 核心机制:
 *   1. 持币者可以把投票权委托给自己 → 直接投票
 *   2. 持币者可以把投票权委托给别人 → 代表投票
 *   3. 使用 Checkpoint 记录历史投票权重 → 防止双重投票
 *
 * 面试高频: 为什么用 Checkpoint？
 *   → 防止"投票后转账再用同一余额投票"的攻击
 *   → Checkpoint 把投票权重锚定在过去的区块
 *
 * 使用方式:
 *   继承此合约到你的 ERC20 代币合约
 *   在 _beforeTokenTransfer / _afterTokenTransfer 中调用 _moveDelegates
 */
abstract contract Delegation {
    // ============ 数据结构 ============

    /// @notice Checkpoint: 记录某个区块时的投票权重
    /// @dev 面试重点: uint224 的选择
    ///   fromBlock 用 uint32 足够（~136年）
    ///   votes 用 uint224（2^224 ≈ 2.7×10^67，远超总供应量）
    ///   两个合起来刚好 256 bits → 一个 storage slot → 省 Gas！
    struct Checkpoint {
        uint32 fromBlock;
        uint224 votes;
    }

    // ============ 状态变量 ============

    /// @notice 每个地址的投票权重
    mapping (address => uint256) private _votingPower;

    /// @notice 每个地址把票委托给谁
    mapping (address => address) private _delegates;

    /// @notice 每个地址的 Checkpoint 数组
    mapping (address => Checkpoint[]) private _checkpoints;

    /// @notice 总投票权重（等于总质押量）
    uint256 private _totalVotingPower;

    // ============ 事件 ============
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);
    event DelegateVotesChanged(address indexed delegate, uint256 previousBalance, uint256 newBalance);

    // ============ 公开查询 ============

    /**
     * @notice 获取某个地址的投票权重（当前区块）
     */
    function getVotes(address account) public view returns (uint256) {
        // 用当前区块号查最近的 checkpoint
        uint256 pos = _checkpoints[account].length;
        if (pos == 0) return 0;
        return _checkpoints[account][pos - 1].votes;
    }

    /**
     * @notice 获取某个地址在特定区块的投票权重（历史查询）
     * @dev 使用二分查找定位 blockNumber 对应的 checkpoint
     *
     * 面试重点: 为什么需要二分查找？
     *   → Checkpoint 数组可能很长（每次转账产生一个）
     *   → 线性查找 O(n) 太贵，二分查找 O(log n)
     */
    function getPastVotes(address account, uint256 blockNumber) public view returns (uint256) {
        require(blockNumber < block.number, "Delegation: not yet determined");

        Checkpoint[] storage ckpts = _checkpoints[account];
        uint256 len = ckpts.length;

        // 空数组 → 返回 0
        if (len == 0) return 0;

        // 请求的区块比最后 checkpoint 还新 → 返回最新值
        if (ckpts[len - 1].fromBlock <= blockNumber) return ckpts[len - 1].votes;

        // 请求的区块比最早 checkpoint 还老 → 返回 0
        if (ckpts[0].fromBlock > blockNumber) return 0;

        // 二分查找：找到 ≤ blockNumber 的最大 fromBlock
        uint256 low = 0;
        uint256 high = len - 1;

        while (low < high) {
            uint256 mid = (low + high + 1) / 2; // 向上取整，避免死循环
            if (ckpts[mid].fromBlock <= blockNumber) {
                low = mid;
            } else {
                high = mid - 1;
            }
        }

        return ckpts[low].votes;
    }

    /**
     * @notice 获取某个地址把票委托给了谁
     */
    function delegates(address account) public view returns (address) {
        address d = _delegates[account];
        // 如果从未设置过，默认委托给自己
        return d == address(0) ? account : d;
    }

    /**
     * @notice 获取总投票权重
     */
    function totalVotingPower() public view returns (uint256) {
        return _totalVotingPower;
    }

    /**
     * @notice 获取某个地址的 checkpoint 数量
     */
    function numCheckpoints(address account) public view returns (uint256) {
        return _checkpoints[account].length;
    }

    // ============ 写操作 ============

    /**
     * @notice 把投票权委托给另一个地址
     * @param delegatee 被委托的地址
     *
     * 面试重点: delegate 的幂等性
     *   如果已经委托给 A，再次调用 delegate(A) 不会重复计算投票权
     *   因为内部会先减去旧的再添加新的
     */
    function delegate(address delegatee) public {
        _delegate(msg.sender, delegatee);
    }

    /**
     * @notice 恢复自委托 — 将投票权收回自己
     */
    function delegateToSelf() external {
        _delegate(msg.sender, msg.sender);
    }

    // ============ 内部函数（子合约调用） ============

    /**
     * @notice 内部委托逻辑
     * @dev 核心流程:
     *   1. 获取当前委托目标
     *   2. 获取委托者的投票权重
     *   3. 从旧目标减去投票权重
     *   4. 给新目标加上投票权重
     *   5. 更新委托关系
     *
     * 面试重点: 为什么需要 _moveDelegates 而不是直接修改？
     *   → 因为投票权重是累积的：一个代表可能被多人委托
     *   → 每个委托者的权重变化都会影响代表的投票权重
     */
    function _delegate(address delegator, address delegatee) internal {
        // 不能委托给自己... 等等，其实可以
        // Compound 的实现中，自己默认就是自己的代表

        address currentDelegate = delegates(delegator);
        uint256 delegatorBalance = _votingPower[delegator];

        // 更新委托关系
        _delegates[delegator] = delegatee;

        emit DelegateChanged(delegator, currentDelegate, delegatee);

        // 从旧代表移走投票权
        _moveDelegates(currentDelegate, delegatee, delegatorBalance);
    }

    /**
     * @notice 在转账时更新投票权重
     * @dev 必须由子合约在转账钩子中调用！
     *
     * 流程:
     *   from: 减去 amount 的投票权 → 写入 checkpoint
     *   to:   加上 amount 的投票权 → 写入 checkpoint
     *
     * 注意: 不是从 from 和 to 的余额中增减！
     *       是从 from 的 delegate 和 to 的 delegate 中增减！
     *
     * 面试重点: 为什么从代表余额增减？
     *   → 因为实际投票的是代表，不是持币者本人
     *   → 持币者 A 委托给 B，A 转账时应该是 B 的投票权减少
     */
    function _afterTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal virtual {
        // mint (from == 0): 给 to 的代表加投票权
        if (from == address(0)) {
            _totalVotingPower += amount;
            _votingPower[to] += amount;
            _moveDelegates(address(0), delegates(to), amount);
        }
        // burn (to == 0): 从 from 的代表减投票权
        else if (to == address(0)) {
            _totalVotingPower -= amount;
            _votingPower[from] -= amount;
            _moveDelegates(delegates(from), address(0), amount);
        }
        // transfer (from → to)
        else {
            _votingPower[from] -= amount;
            _votingPower[to] += amount;
            // 注意: 这里是从 from 的代表移到 to 的代表
            _moveDelegates(delegates(from), delegates(to), amount);
        }
    }

    /**
     * @notice 在两个代表之间移动投票权重
     * @dev 面试高频! 理解这个函数就理解了委托投票的核心
     *
     * 调用场景:
     *   [场景1] 转账:    _moveDelegates(转出方的代表, 接收方的代表, amount)
     *   [场景2] 委托变更: _moveDelegates(旧代表, 新代表, 委托者的余额)
     *   [场景3] 铸币:    _moveDelegates(address(0), 接收方的代表, amount)
     *   [场景4] 销毁:    _moveDelegates(转出方的代表, address(0), amount)
     *
     * 为什么用 _writeCheckpoint 而不是直接修改？
     *   → 因为需要记录历史，供 getPastVotes 查询
     */
    function _moveDelegates(
        address srcRep,
        address dstRep,
        uint256 amount
    ) internal {
        // 从旧代表减去投票权重
        if (srcRep != dstRep && amount > 0) {
            if (srcRep != address(0)) {
                uint256 oldWeight = getVotes(srcRep);
                uint256 newWeight = oldWeight - amount;
                _writeCheckpoint(srcRep, oldWeight, newWeight);
            }
            // 给新代表加上投票权重
            if (dstRep != address(0)) {
                uint256 oldWeight = getVotes(dstRep);
                uint256 newWeight = oldWeight + amount;
                _writeCheckpoint(dstRep, oldWeight, newWeight);
            }
        }
    }

    /**
     * @notice 写入 checkpoint
     * @dev Gas 优化:
     *   如果当前区块已经有 checkpoint（同一个区块内多次操作）
     *   则覆盖最后一个 checkpoint，而不是新增一个
     *
     * 面试重点: 为什么可以在同一区块覆盖？
     *   → 因为在同一个区块内的所有操作，外部看到的状态是一样的
     *   → 只需要保存最终结果，中间过程不重要
     *   → 这样可以大幅减少存储写入（每次新增 checkpoint 都是 SSTORE，很贵）
     */
    function _writeCheckpoint(
        address delegatee,
        uint256 oldWeight,
        uint256 newWeight
    )  internal {
        uint32 blockNumber = uint32(block.number);
        Checkpoint[] storage ckpts = _checkpoints[delegatee];
        uint256 len = ckpts.length;

        // Gas 优化: 同一区块覆盖最后一个 checkpoint
        if (len > 0 && ckpts[len - 1].fromBlock == blockNumber) {
            ckpts[len - 1].votes = uint224(newWeight);
        } else {
            ckpts.push(Checkpoint({
                fromBlock: blockNumber,
                votes: uint224(newWeight)
            }));
        }

        emit DelegateVotesChanged(delegatee, oldWeight, newWeight);
    }

    
    constructor() {
        
    }
}