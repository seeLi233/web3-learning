// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * 学习目标: 理解 ERC20Votes 的 Checkpoint（快照）机制
 *
 * 核心问题: 如何防止"投票后转账再投票"的重复投票攻击？
 * 答案: Checkpoint — 每次余额变动都记录快照，投票时用历史快照的值
 */
contract VotesDemo {

    // ===== Checkpoint 数据结构 =====
    struct Checkpoint {
        uint32 fromBlock;       // 从哪个区块开始这个余额生效
        uint224 votes;          // 在这个区块的投票权重
        // uint32 + uint224 = 256 bits = 正好一个 slot! Gas 最佳
    }

    // ===== 状态变量 =====
    mapping (address => uint256) private _balances;
    mapping (address => Checkpoint[]) private _checkpoints;
    uint256 private _totalSupply;

    // ===== 事件 =====
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);

    // ===== 核心: 写入快照 =====
    function _writeCheckpoint(address account, uint256 newVotes) internal {
        Checkpoint[] storage ckpts = _checkpoints[account];

        // 如果新权重和最近一次快照相同，不重复写入（省 gas）
        uint256 len = ckpts.length;
        if (len > 0 && ckpts[len - 1].fromBlock == block.number) {
            // 同一区块内更新：直接修改最后一个 checkpoint
            ckpts[len - 1].votes = uint224(newVotes);
        } else {
            // 新区块：push 新的 checkpoint
            ckpts.push(Checkpoint({
                fromBlock: uint32(block.number),
                votes: uint224(newVotes)
            }));
        }
    }

    // ===== 核心: 读取历史快照（二分查找）=====
    function getPastVotes(address account, uint256 blockNumber) public view returns (uint256) {
        require(blockNumber < block.number, "Votes: block not yet mined");

        Checkpoint[] storage ckpts = _checkpoints[account];
        uint256 len = ckpts.length;

        // 没有 checkpoint 或 查询的区块在第一个 checkpoint 之前
        if (len == 0 || blockNumber < ckpts[0].fromBlock) {
            return 0;
        }

        // 查询的区块在最后一个 checkpoint 之后 → 返回最新值
        if (blockNumber >= ckpts[len - 1].fromBlock) {
            return ckpts[len - 1].votes;
        }

        // 二分查找：找到 blockNumber 之前最近的那个 checkpoint
        uint256 low = 0;
        uint256 high = len - 1;

        while(low < high) {
            uint mid = (low + high + 1) / 2; // 向上取整, 避免死循环
            if (ckpts[mid].fromBlock <= blockNumber) {
                low = mid;
            } else {
                high = mid - 1;
            }
        }

        return ckpts[low].votes;
    }

    // ===== 演示: 委托投票 =====
    // 委托映射：delegator → delegatee
    mapping (address => address) private _delegates;

    function delegate(address delegatee) external {
        address oldDelegate = _delegates[msg.sender];
        uint256 delegatorBalance = _balances[msg.sender];

        _delegates[msg.sender] = delegatee;

        emit DelegateChanged(msg.sender, oldDelegate, delegatee);

        // 从旧委托人减去投票权重
        if (oldDelegate != address(0)) {
            uint256 oldVotes = _balances[oldDelegate];
            _writeCheckpoint(oldDelegate, oldVotes - delegatorBalance);
        }

        // 给新委托人加上投票权重
        if (delegatee != address(0)) {
            uint256 newVotes = _balances[delegatee] + delegatorBalance;
            _writeCheckpoint(delegatee, newVotes);
        }
    }
}

// ===== 图解: Checkpoint 如何工作 =====
//
// Alice 的余额变化:
//   区块 100: Alice mint 100 DTF  → _checkpoints[Alice] = [{100, 100}]
//   区块 110: Alice 转走 50 DTF   → _checkpoints[Alice] = [{100, 100}, {110, 50}]
//   区块 120: Alice 转走 30 DTF   → _checkpoints[Alice] = [{100, 100}, {110, 50}, {120, 20}]
//
// 查询 Alice 在区块 105 的投票权重:
//   getPastVotes(Alice, 105)
//   → 二分查找: ckpts[0].fromBlock=100 ≤ 105 < ckpts[1].fromBlock=110
//   → 返回 ckpts[0].votes = 100 DTF  ← 用的不是当前余额！
//
// 这样即使 Alice 在投票后立刻转账，也无法重复投票！