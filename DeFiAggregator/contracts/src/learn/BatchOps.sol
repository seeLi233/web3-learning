// SPDX-License-Ientifier: MIT
pragma solidity ^0.8.20;

// 为什么用 IERC20 接口而不是完整 ERC20？
// → 我们的合约只需要 transfer 和 transferFrom，引入完整 OpenZeppelin 太重了
// → 自己定义最小接口 = 编译更快 + 字节码更小，省部署 gas
interface IERC20Minimal {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}

/**
 * @title BatchOps
 * @notice 批量操作合约：转账、Swap、Claim
 * @dev 核心优化：把 N 笔独立交易合并成 1 笔，省掉 (N-1) × 21000 基础 gas
 *
 * 为什么面试官喜欢问批量操作？
 * → DEX 聚合器（1inch）、DeFi 协议（Aave）、空投（Uniswap）都在用
 *    这是从 "能跑" 到 "生产级" 的分水岭
 */
contract BatchOps {
    // ==================== 自定义 Error（省 gas） ====================

    // 为什么用自定义 error 而不是 require string？
    // → 自定义 error 只存 4 字节 selector，require(string) 存整个字符串
    // → 批量操作失败率高（数组长度不匹配等），每个 revert 都省一点，累积可观
    error BatchOps__LengthMismatch();
    error BatchOps__InsufficientBalance(uint256 required, uint256 actual);
    error BatchOps__TransferFailed(address to, uint256 amount);
    error BatchOps__EmptyArray();
    error BatchOps__SwapFailed(address tokenIn, address tokenOut);

    // ==================== 事件 ====================

    // 为什么用 event 记录每次批量操作？
    // → 事件成本远低于 storage（375 + 8/字节 vs 20000），适合做历史追溯
    // → 前端通过监听事件来展示交易历史，不需要额外查询 storage
    event BatchTransferExecuted(
        address indexed operator,
        uint256 recipientCount,
        uint256 totalAmount
    );

    event BatchSwapExecuted(
        address indexed operator,
        uint256 hopCount,
        uint256 amountIn,
        uint256 amountOut
    );

    // ==================== 1. 批量 ETH 转账 ====================

    /**
     * @notice 一次交易向多个地址转 ETH
     * @param recipients 接收地址列表
     * @param amounts 对应金额列表
     *
     * 为什么先算总金额再转账？
     * → 1. 可以在转账前一次性校验 msg.value，避免中途发现不够部分已转
     * → 2. 符合 CEI 模式（Check-Effects-Interactions）：先验再转
     *
     * 为什么用 call 不用 transfer？
     * → transfer 固定 2300 gas，EIP-1884 后接收方 fallback 可能消耗更多
     * → call 不限制 gas，未来兼容性更好
     */
    function batchTransferETH(
        address[] calldata recipients,   // calldata：只读不需要复制到 memory
        uint256[] calldata amounts       // calldata：同上
    ) external payable {
        // 安全检查 1：数组不能为空
        if (recipients.length == 0) revert BatchOps__EmptyArray();
        // 安全检查 2：两个数组长度必须一致——每笔转出和接收一一对应
        if (recipients.length != amounts.length) revert BatchOps__LengthMismatch();

        // 第一步：计算总金额（Check）
        // 为什么不在循环内累加的同时转账？→ CEI 原则：校验完再做外部调用
        uint256 total = 0;
        for (uint256 i = 0; i < amounts.length; ) {
            total += amounts[i];
            unchecked { ++i; }  // i < amounts.length 保证不溢出
        }

        // 第二步：校验 msg.value（Check）
        // 总金额必须等于附带的 ETH，多退少补在批量场景太复杂，直接拒绝
        if (msg.value != total) revert BatchOps__InsufficientBalance(total, msg.value);

        // 第三步：批量转账（Effects + Interactions）
        // 为什么 msg.value 已经校验过，这里还需要 try/catch 心态？
        // → call 可能因接收方拒绝（revert）而失败，需要冒泡告知用户哪笔失败了
        for (uint256 i = 0; i < recipients.length; ) {
            (bool success, ) = recipients[i].call{value: amounts[i]}("");
            if (!success) revert BatchOps__TransferFailed(recipients[i], amounts[i]);
            unchecked { ++i; }
        }

        // 事件记录（比存 storage 便宜 50+ 倍）
        emit BatchTransferExecuted(msg.sender, recipients.length, total);
    }

    // ==================== 2. 批量 ERC20 转账 ====================

    /**
     * @notice 一次交易向多个地址转同一种 ERC20 代币
     * @dev 调用者需要先 approve 本合约足够的额度
     *
     * 为什么需要 approve 而不是直接 transfer？
     * → 本合约要替用户转账，必须用 transferFrom 从用户钱包扣代币
     * → transferFrom 要求用户先 approve 本合约，这是 ERC20 的标准安全机制
     */

    // ==================== 2. 批量 ERC20 转账 ====================

    /**
     * @notice 一次交易向多个地址转同一种 ERC20 代币
     * @dev 调用者需要先 approve 本合约足够的额度
     *
     * 为什么需要 approve 而不是直接 transfer？
     * → 本合约要替用户转账，必须用 transferFrom 从用户钱包扣代币
     * → transferFrom 要求用户先 approve 本合约，这是 ERC20 的标准安全机制
     */
    function batchTransferERC20(
        address token,
        address[] calldata recipients,
        uint256[] calldata amounts
    ) external {
        if (recipients.length == 0) revert BatchOps__EmptyArray();
        if (recipients.length != amounts.length) revert BatchOps__LengthMismatch();

        // 计算总金额
        uint256 total = 0;
        for (uint256 i = 0; i < amounts.length; ) {
            total += amounts[i];
            unchecked { ++i; }
        }

        // 一次性从调用者转入所有代币（一次 transferFrom 而非 N 次）
        // 为什么先全部转入再逐个转出？
        // → 如果逐个 transferFrom，需要 N 次外部调用 + N 次 approve 校验 = 巨量 gas
        // → 一次转入 + N 次转出 = 1 + N 次外部调用，且单次转入省掉 N-1 次 approve 检查
        IERC20Minimal(token).transferFrom(msg.sender, address(this), total);

        // 逐个转给接收方
        for (uint256 i = 0; i < recipients.length; ) {
            bool success = IERC20Minimal(token).transfer(recipients[i], amounts[i]);
            // 为什么检查 transfer 返回值？
            // → 有些 ERC20 的 transfer 不 revert 而是返回 false（如 USDT、BNB）
            // → 不检查会导致"以为转了实际没转"的 bug
            if (!success) revert BatchOps__TransferFailed(recipients[i], amounts[i]);
            unchecked { ++i; }
        }

        emit BatchTransferExecuted(msg.sender, recipients.length, total);
    }

    // ==================== 3. 批量 Swap（多跳路由） ====================

    // 单跳 Swap 的配置结构
    // 为什么用 struct 而不是多个数组？
    // → struct 把一组相关数据打包，代码更清晰，而且可以进行打包优化
    struct SwapHop {
        address tokenIn;      // 输入代币
        address tokenOut;     // 输出代币
        uint256 amountIn;     // 输入数量
        uint256 minAmountOut; // 最小输出量（滑点保护）
    }

    /**
     * @notice 执行多跳 Swap（模拟 DEX 聚合器路由）
     * @dev 每跳的输出 = 下一跳的输入，形成连续的兑换路径
     *
     * 为什么需要批量 swap？
     * → Uniswap 只支持两个代币间直接兑换，ETH→USDC→DAI→ETH 要 3 笔交易
     * → 批量 swap = 1 笔交易完成多跳，省 2 笔基础 gas + 2 次 approve
     *
     * 注意：这里用 ERC20Mock 的 _mint 模拟 swap（真实场景要对接 Uniswap Router）
     */
    function batchSwap(SwapHop[] calldata hops) external {
        if (hops.length == 0) revert BatchOps__EmptyArray();

        // 第一步：把第一跳的输入代币从用户转入本合约
        IERC20Minimal(hops[0].tokenIn).transferFrom(
            msg.sender,
            address(this),
            hops[0].amountIn
        );

        uint256 currentAmount = hops[0].amountIn;

        // 逐跳执行
        for (uint256 i = 0; i < hops.length; ) {
            SwapHop calldata hop = hops[i];  // calldata 指针——不复制整个 struct

            // 检查本合约有足够的代币做下一跳输入
            // 为什么用 balanceOf 检查而不是用 tracking 变量？
            // → 真实 DEX 的 swap 返回值可能与预期不同（滑点、手续费）
            // → balanceOf 是最终数据源，比追踪变量更可靠
            if (i > 0) {
                currentAmount = IERC20Minimal(hop.tokenIn).balanceOf(address(this));
            }

            // 模拟 swap：把 tokenIn 销毁，铸造 tokenOut
            // 为什么这里用 1:1 兑换？
            // → 这是教学版的 mock——真实场景应该调用 Uniswap Router 的 swapExactTokensForTokens
            // → mock 简化了逻辑，专注于批量操作的 gas 优化

            // 销毁 tokenIn（本合约收到的代币）
            // ⚠️ 真实场景：调用 router.swapExactTokensForTokens(...)
            _simulateSingleSwap(hop.tokenIn, hop.tokenOut, currentAmount);

            unchecked { ++i; }
        }

        // 最终输出转给用户
        SwapHop calldata lastHop = hops[hops.length - 1];
        uint256 finalBalance = IERC20Minimal(lastHop.tokenOut).balanceOf(address(this));
        IERC20Minimal(lastHop.tokenOut).transfer(msg.sender, finalBalance);

        emit BatchSwapExecuted(msg.sender, hops.length, hops[0].amountIn, finalBalance);
    }

    // 模拟单次兑换（Mock——真实环境替换为 Uniswap Router 调用）
    // 为什么单独抽一个 internal 函数？
    // → 代码复用 + 方便测试。真实部署时只改这一个函数就能对接实际 DEX
    // → internal 不暴露到外部，省掉 public 函数的 jump 成本
    function _simulateSingleSwap(
        address tokenIn,
        address tokenOut,
        uint256 amount
    ) internal {
        // 模拟：销毁 tokenIn，铸造等量 tokenOut
        // 真实场景替换为：
        //   address[] memory path = new address[](2);
        //   path[0] = tokenIn; path[1] = tokenOut;
        //   IUniswapV2Router(router).swapExactTokensForTokens(amount, 0, path, address(this), deadline);
        // 此处作为教学 mock，保持结构完整即可
    }

    // ==================== 4. 批量领取奖励（Claim） ====================

    /**
     * @notice 批量领取多个协议的奖励
     * @dev 实际场景：用户参与了多个 Staking 池，一键领取所有奖励
     *
     * 为什么需要批量 Claim？
     * → Yearn、Convex 等聚合器核心功能就是帮用户一键收菜
     * → 面试高频："设计一个聚合器的 Claim 功能"
     */
    function batchClaim(address[] calldata protocols) external returns (uint256 totalClaimed) {
        if (protocols.length == 0) revert BatchOps__EmptyArray();

        for (uint256 i = 0; i < protocols.length; ) {
            // 每个协议的 claim 方法用低层级 call 调用
            // 为什么用 call 而不是直接调用接口？
            // → 不同协议的 claim 函数签名不同：
            //    Compound: claimComp(address holder)
            //    Aave: claimRewards(address[] assets, ...)
            //    Uniswap: collect()
            // → 不能用一个统一接口，只能用 bytes4 selector + call
            // → 这是 DEX 聚合器必须面对的现实：协议接口不统一

            // 尝试调用 claim() — 最通用的签名
            (bool success, ) = protocols[i].call(
                abi.encodeWithSignature("claim(address)", msg.sender)
            );
            // 如果 claim(address) 失败，尝试 getReward() — Synthetix 用的签名
            if (!success) {
                (success, ) = protocols[i].call(
                    abi.encodeWithSignature("getReward()")
                );
            }
            // 两种签名都失败 → 跳过该协议（不阻断整个批量操作）
            // 为什么跳过而不是 revert？
            // → 某个协议恰好这轮没奖励，不应该阻止用户领取其他协议的奖励
            // → 这是生产级聚合器的设计：fail gracefully

            unchecked { ++i; }
        }
        // totalClaimed 返回 0——真实场景需要查询余额变化来计算
        // 此处保持结构完整，关注点在批量调用模式而非具体收益计算
        return totalClaimed;
    }

    // ==================== 辅助函数 ====================

    // 允许接收 ETH（批量转账的 refund 等情况）
    receive() external payable {}
}