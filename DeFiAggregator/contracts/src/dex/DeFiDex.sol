// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title DeFiDex
 * @notice AMM 自动做市商核心合约 — 恒定乘积 (x*y=k) 模型
 * @dev 基于 Uniswap V2 设计，实现 swap / addLiquidity / removeLiquidity
 *
 * 核心公式:
 *   x * y = k
 *   其中 x = reserve0 (token0 储备量), y = reserve1 (token1 储备量)
 *
 * 手续费: 0.3% (每笔 swap 留 0.3% 在池子里分给 LP)
 */
contract DeFiDex is ERC20, Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    // ============ 自定义 Error（比 revert string 省 gas）============
    error InsufficientLiquidity();
    error InsufficientInputAmount();
    error InsufficientOutputAmount();
    error InvalidAddress();
    error IdenticalAddresses();
    error Overflow();
    error TransferFailed();
    error DeadlineExpired();
    error KConstantViolation(); // x*y=k 被破坏（严重 bug）

    // ============ 事件 ============
    event LiquidityAdded(
        address indexed provider,
        uint256 amount0,
        uint256 amount1,
        uint256 liquidity
    );

    event LiquidityRemoved(
        address indexed provider,
        uint256 amount0,
        uint256 amount1,
        uint256 liquidity
    );

    event Swap(
        address indexed user,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOut
    );

    // ============ 状态变量 ============
    IERC20 public immutable token0; // 交易对代币 A（如 ETH 的 WETH）
    IERC20 public immutable token1; // 交易对代币 B（如 USDT）

    uint256 public reserve0;    // 池子中 token0 的数量
    uint256 public reserve1;    // 池子中 token1 的数量

    // 手续费常量: 0.3%
    // 公式: amountIn * 997 / 1000 = 扣除 0.3% 后的实际入账
    uint256 public constant FEE_NUMERATOR = 997;
    uint256 public constant FEE_DONOMINATOR = 1000;

    // 最小流动性（防止粉尘攻击，初始 LP 会被锁一部分）
    uint256 public constant MINIMUM_LIQUIDITY = 1000;

    // ============ 手续费追踪 ============
    uint256 public accumulatedFee0; // token0 的累积手续费
    uint256 public accumulatedFee1; // token1 的累积手续费

    // ============ TWAP 预言机 ============
    uint256 public price0CumulativeLast;    // token0 价格累计 (UQ112x112)
    uint256 public price1CumulativeLast;    // token1 价格累计
    uint32 public blockTimestampLast;       // 上次更新时间

    // ============ 构造函数 ============
    /// @param _token0 交易对代币 A 的地址
    /// @param _token1 交易对代币 B 的地址
    /// @notice token0 地址必须 < token1 地址（排序保证唯一性）
    constructor(
        address _token0,
        address _token1
    ) ERC20("DeFiDex LP Token", "DLP") Ownable(msg.sender) {
        // 安全检查: 两个代币地址不能相同
        if (_token0 == _token1) revert IdenticalAddresses();
        // 安全检查: 地址不能是零地址
        if (_token0 == address(0) || _token1 == address(0))
            revert InvalidAddress();

        // 按地址排序，保证每个交易对只有一个池子
        // 这样 (tokenA, tokenB) 和 (tokenB, tokenA) 不会创建两个池子
        if (_token0 < _token1) {
            token0 = IERC20(_token0);
            token1 = IERC20(_token1);
        } else {
            token0 = IERC20(_token1);
            token1 = IERC20(_token0);
        }
    }

    // ============ Swap 核心逻辑 ============

    /// @notice 用 tokenIn 兑换 tokenOut
    /// @param amountIn 用户输入的数量
    /// @param minAmountOut 最小输出数量（滑点保护）
    /// @param tokenInAddr 输入代币地址
    /// @param tokenOutAddr 输出代币地址
    /// @param deadline 交易截止时间（Unix timestamp）
    /// @return amountOut 实际输出数量
    function swap(uint256 amountIn, uint256 minAmountOut, address tokenInAddr, address tokenOutAddr, uint256 deadline) external nonReentrant returns (uint256 amountOut) {
        // 1. 安全检查
        if (block.timestamp > deadline) revert DeadlineExpired();
        if (amountIn == 0) revert InsufficientInputAmount();

        // 2. 确认输入/输出代币
        bool isToken0In = tokenInAddr == address(token0);
        bool isToken0Out = tokenOutAddr == address(token0);

        // 输入和输出不能是同一种代币
        if (!(isToken0In || isToken0Out) || (isToken0In && isToken0Out)) {
            revert InvalidAddress();
        }

        // 3. 读当前储备量
        (uint256 reserveIn, uint256 reserveOut) = isToken0In ? (reserve0, reserve1) : (reserve1, reserve0);

        // 4. 计算输出数量（扣除 0.3% 手续费）
        // swap 公式: amountOut = (reserveOut * amountInWithFee) / (reserveIn + amountInWithFee)
        // 其中 amountInWithFee = amountIn * 997 (即扣了 0.3% 的费用)
        // 已知: 用户要换出 Δy 个 tokenOut

        // 池子里原来有: x 个 tokenIn, y 个 tokenOut
        // 池子里会新增: Δx * 0.997 个 tokenIn（扣 0.3% 手续费）
        // 池子里会减少: Δy 个 tokenOut

        // 恒定乘积:
        //     (x + Δx * 0.997) * (y - Δy) = x * y

        // 解出 Δy:
        //     y - Δy = x * y / (x + Δx * 0.997)
        //     Δy = y - x * y / (x + Δx * 0.997)
        //     Δy = (y * (x + Δx * 0.997) - x * y) / (x + Δx * 0.997)
        //     Δy = (y * Δx * 0.997) / (x + Δx * 0.997)

        // 这就是代码中的:
        //     amountOut = (reserveOut * amountIn * 997) / (reserveIn * 1000 + amountIn * 997)
        uint256 amountInWithFee = amountIn * FEE_NUMERATOR;
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * FEE_DONOMINATOR + amountInWithFee;

        amountOut = numerator / denominator;

        // 5. 滑点保护: 实际输出不能小于用户预期的最小值
        if (amountOut < minAmountOut) revert InsufficientOutputAmount();

        // 6. 计算手续费（在更新 reserve 之前计算）
        uint256 fee = amountIn - (amountIn * FEE_NUMERATOR / FEE_DONOMINATOR);

        // 7. 更新储备量 + 预言机
        uint256 newReserve0 = reserve0;
        uint256 newReserve1 = reserve1;

        if (isToken0In) {
            reserve0 += amountIn;
            reserve1 -= amountOut;
            accumulatedFee0 += fee; // 手续费留在池子里
        } else {
            reserve1 += amountIn;
            reserve0 -= amountOut;
            accumulatedFee1 += fee;
        }

        _update(newReserve0, newReserve1);

        // 8. 转出代币给用户
        IERC20(tokenInAddr).safeTransferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenOutAddr).safeTransfer(msg.sender, amountOut);

        // 不需要再检查 k 值 — _update 已经在更新 reserve 后，手续费会增加 k

        emit Swap(msg.sender, tokenInAddr, tokenOutAddr, amountIn, amountOut);
    }

    /// @notice 查询给定输入能换到多少输出（只读，不执行交易）
    /// @param amountIn 输入数量
    /// @param tokenIn 输入代币地址
    /// @param tokenOut 输出代币地址
    /// @return amountOut 预期输出数量
    function getAmountOut(uint256 amountIn, address tokenIn, address tokenOut) public view returns (uint256 amountOut) {
        if (amountIn == 0) revert InsufficientInputAmount();

        (uint256 reserveIn, uint256 reserveOut) = _getReserves(tokenIn, tokenOut);

        uint256 amountInWithFee = amountIn * FEE_NUMERATOR;
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * FEE_DONOMINATOR + amountInWithFee;

        amountOut = numerator / denominator;
    }

    /// @notice 查询给定输出需要多少输入（反向计算）
    /// @param amountOut 期望输出数量
    /// @param tokenIn 输入代币地址
    /// @param tokenOut 输出代币地址
    /// @return amountIn 需要的输入数量
    function getAmountIn(uint256 amountOut, address tokenIn, address tokenOut) public view returns (uint256 amountIn) {
        if (amountOut == 0) revert InsufficientOutputAmount();

        (uint256 reserveIn, uint256 reserveOut) = _getReserves(tokenIn, tokenOut);

        // 反向公式: amountIn = (reserveIn * amountOut * 1000) / ((reserveOut - amountOut) * 997) + 1
        // +1 是因为整数除法向下取整，需要确保足够支付
        uint256 numerator = reserveIn * amountOut * FEE_DONOMINATOR;
        uint256 denominator = (reserveOut - amountOut) * FEE_NUMERATOR;
        amountIn = numerator / denominator + 1;
    }

    /// @dev 内部函数：根据输入/输出代币返回对应的储备量
    function _getReserves(address tokenIn, address tokenOut) private view returns (uint256 reserveIn, uint256 reserveOut) {
        if (tokenIn == address(token0) && tokenOut == address(token1)) {
            return (reserve0, reserve1);
        } else if (tokenIn == address(token1) && tokenOut == address(token0)) {
            return (reserve1, reserve0);
        } else {
            revert InvalidAddress();
        }
    }

    /// @dev 更新储备量 + 价格累加器（TWAP 预言机）
    function _update(uint256 _reserve0, uint256 _reserve1) private {
        // 使用 uint32 因为 block.timestamp 在 2106 年之前不会溢出
        uint32 blockTimestamp = uint32(block.timestamp % 2 ** 32);
        uint32 timeElapsed;

        unchecked {
            // 处理 timestamp 溢出换行（罕见）
            timeElapsed = blockTimestamp - blockTimestampLast;
        }

        if (timeElapsed > 0 && reserve0 != 0 && reserve1 != 0) {
            // ⭐ 价格累加 = 现货价格 × 时间间隔
            // price0 = reserve1 / reserve0 (1 token0 = ? token1)
            // 乘以 2**112 精度防止精度丢失（UQ112x112 定点数）
            price0CumulativeLast += (reserve1 * (2 ** 112) / reserve0) * timeElapsed;
            price1CumulativeLast += (reserve0 * (2 ** 112) / reserve1) * timeElapsed;
        }

        blockTimestampLast = blockTimestamp;
        reserve0 = _reserve0;
        reserve1 = _reserve1;
    }

    // ============ 流动性管理 ============

    // 假设池子已有 100 ETH + 300,000 USDT，总共 mint 了 100 个 LP 代币

    // 你想存 10 ETH + 30,000 USDT（正好是 10%）

    // 你的 LP = 10% × 100 = 10 个 LP 代币

    // 如果比例不对：
    //     你存 10 ETH + 25,000 USDT
    //     → 系统按 (10 * 300,000 / 100) = 30,000 USDT 的最优值
    //     → 你只需要存 10 ETH + 30,000 USDT
    //     → 25,000 不够！
    //     → 反过来算: (25,000 * 100 / 300,000) = 8.33 ETH
    //     → 你存 8.33 ETH + 25,000 USDT
    //     → LP = 8.33 个

    /// @notice 添加流动性，获得 LP 代币
    /// @param amount0Desired 想存入 token0 的数量
    /// @param amount1Desired 想存入 token1 的数量
    /// @param amount0Min 最少接受 token0 数量（滑点保护）
    /// @param amount1Min 最少接受 token1 数量（滑点保护）
    /// @param deadline 截止时间
    /// @return amount0 实际存入 token0 数量
    /// @return amount1 实际存入 token1 数量
    /// @return liquidity mint 的 LP 代币数量
    function addLiquidity(uint256 amount0Desired, uint256 amount1Desired, uint256 amount0Min, uint256 amount1Min, uint256 deadline) external nonReentrant returns (uint256 amount0, uint256 amount1, uint256 liquidity) {
        if (block.timestamp > deadline) revert DeadlineExpired();

        // ====== 情况 1: 池子为空，第一次添加流动性 ======
        if (totalSupply() == 0) {
            // 直接用传入的数量，不需要按比例
            amount0 = amount0Desired;
            amount1 = amount1Desired;

            // 初始 LP 代币 = sqrt(x * y) - MINIMUM_LIQUIDITY
            // 减去 MINIMUM_LIQUIDITY 是为了永久锁定一部分，防止粉尘攻击
            liquidity = _sqrt(amount0 * amount1) - MINIMUM_LIQUIDITY;

            // 永久锁定 MINIMUM_LIQUIDITY 个 LP 代币到零地址
            _mint(address(0xdead), MINIMUM_LIQUIDITY);
        }
        // ====== 情况 2: 池子不为空，按比例添加 ======
        else {
            // 按当前池子的比例，计算最优存入量
            // reserve0 / reserve1 = amount0 / amount1 的比例关系
            uint256 amount1Optimal = (amount0Desired * reserve1) / reserve0;

            if (amount1Optimal <= amount1Desired) {
                // 按 amount0Desired 的比例来，调整 amount1
                if (amount1Optimal < amount1Min)
                    revert InsufficientLiquidity();
                amount0 = amount0Desired;
                amount1 = amount1Optimal;
            } else {
                // 按 amount1Desired 的比例来，调整 amount0
                uint256 amount0Optimal = (amount1Desired * reserve0) / reserve1;
                if (amount0Optimal < amount0Min)
                    revert InsufficientLiquidity();
                amount0 = amount0Optimal;
                amount1 = amount1Desired;
            }

            // mint LP 代币 = min(amount0/totalReserve0, amount1/totalReserve1) * totalSupply
            // 取最小值保证不被稀释
            uint256 liquidity0 = (amount0 * totalSupply()) / reserve0;
            uint256 liquidity1 = (amount1 * totalSupply()) / reserve1;
            liquidity = liquidity0 < liquidity1 ? liquidity0 : liquidity1;

            if (liquidity == 0) revert InsufficientLiquidity();
        }

        // 安全检查
        if (amount0 < amount0Min || amount1 < amount1Min)
            revert InsufficientLiquidity();

        // CEI: 先更新储备量
        _update(reserve0 + amount0, reserve1 + amount1);

        // mint LP 代币给用户
        _mint(msg.sender, liquidity);

        // 再转移用户的代币到合约（用 safeTransferFrom）
        token0.safeTransferFrom(msg.sender, address(this), amount0);
        token1.safeTransferFrom(msg.sender, address(this), amount1);

        emit LiquidityAdded(msg.sender, amount0, amount1, liquidity);
    }
    
    /// @notice 移除流动性，销毁 LP 代币，拿回两种代币
    /// @param liquidity 要销毁的 LP 代币数量
    /// @param amount0Min 最少接受的 token0 数量
    /// @param amount1Min 最少接受的 token1 数量
    /// @param deadline 截止时间
    /// @return amount0 拿回的 token0 数量
    /// @return amount1 拿回的 token1 数量
    function removeLiquidity(uint256 liquidity, uint256 amount0Min, uint256 amount1Min, uint256 deadline) external nonReentrant returns (uint256 amount0, uint256 amount1) {
        if (block.timestamp > deadline) revert DeadlineExpired();
        if (liquidity == 0) revert InsufficientLiquidity();

        // 按 LP 份额计算应得的代币数量
        // amountX = (liquidity * reserveX) / totalSupply
        amount0 = (liquidity * reserve0) / totalSupply();
        amount1 = (liquidity * reserve1) / totalSupply();

        if (amount0 < amount0Min || amount1 < amount1Min)
            revert InsufficientLiquidity();

        // CEI: 先更新储备量
        _update(reserve0 - amount0, reserve1 - amount1);

        // 销毁 LP 代币
        _burn(msg.sender, liquidity);

        // 转移代币给用户
        token0.safeTransfer(msg.sender, amount0);
        token1.safeTransfer(msg.sender, amount1);

        emit LiquidityRemoved(msg.sender, amount0, amount1, liquidity);
    }

    /// @notice 单边移除流动性 — 销毁 LP 全部换成一种代币
    /// @param liquidity 要销毁的 LP 数量
    /// @param tokenOut 想要输出的代币地址
    /// @param minAmountOut 最小输出数量（滑点保护）
    /// @param deadline 截止时间
    /// @return amountOut 实际输出的代币数量
    function removeLiquiditySingle(uint256 liquidity, address tokenOut, uint256 minAmountOut, uint256 deadline) external nonReentrant returns (uint256 amountOut) {
        if (block.timestamp > deadline) revert DeadlineExpired();
        if (liquidity == 0) revert InsufficientLiquidity();
        if (tokenOut != address(token0) && tokenOut != address(token1))
            revert InvalidAddress();
        
        // Step 1: 计算两种代币的份额
        uint256 amount0 = (liquidity * reserve0) / totalSupply();
        uint256 amount1 = (liquidity * reserve1) / totalSupply();

        // Step 2: 销毁 LP 代币 + 更新 reserve
        _burn(msg.sender, liquidity);

        if (tokenOut == address(token0)) {
            // Step 3: 把 token1 部分 swap 成 token0
            // 注意：这里的 swap 在 pool 内部发生，
            // reserve 在下一步更新之前需要先扣除 amount0 和 amount1
            uint256 newReserve0 = reserve0 - amount0;
            uint256 newReserve1 = reserve1 - amount1;

            // 用 amount1 swap token0（在扣除后的池子里换）
            // swap 公式: amountOut = (reserveOut * amountIn * 997) / (reserveIn * 1000 + amountIn * 997)
            uint256 swapOut = (newReserve0 * amount0 * FEE_NUMERATOR) / (newReserve1 * FEE_DONOMINATOR + amount1 * FEE_NUMERATOR);

            // ⭐ 修复: 防止 swapOut 超过池子中的 token0 余额
            if (swapOut > newReserve0) {
                swapOut = newReserve0;
            }

            // 总输出 = 你应得的 token0 + swap 换来的 token0
            amountOut = amount0 + swapOut;

            // 更新 reserve（token1 进了池子，token0 减少了 amount0 + swapOut）
            _update(newReserve0 - swapOut, newReserve1 + amount1);

            // 更新手续费（swap 部分产生了手续费）
            uint256 swapFee = amount1 - (amount1 * FEE_NUMERATOR / FEE_DONOMINATOR);
            accumulatedFee1 += swapFee;
        } else {
            // tokenOut == token1: 把 token0 部分 swap 成 token1
            uint256 newReserve0 = reserve0 - amount0;
            uint256 newReserve1 = reserve1 - amount1;

            uint256 swapOut = (newReserve1 * amount0 * FEE_NUMERATOR) / (newReserve0 * FEE_DONOMINATOR + amount0 * FEE_NUMERATOR);

            // ⭐ 修复
            if (swapOut > newReserve1) {
                swapOut = newReserve1;
            }

            amountOut = amount1 + swapOut;
            _update(newReserve0 + amount0, newReserve1 - swapOut);

            uint256 swapFee = amount0 - (amount0 * FEE_NUMERATOR / FEE_DONOMINATOR);
            accumulatedFee0 += swapFee;
        }

        // 滑点保护
        if (amountOut < minAmountOut) revert InsufficientOutputAmount();

        // 转出代币
        IERC20(tokenOut).safeTransfer(msg.sender, amountOut);

        emit LiquidityRemoved(msg.sender, 
            tokenOut == address(token0) ? amount0 : amountOut - amount1,
            tokenOut == address(token1) ? amount1 : amountOut - amount0,
            liquidity);
    }

    /// @notice 查询某个地址的 LP 份额信息
    /// @param account 用户地址
    /// @return liquidity LP 代币余额
    /// @return share0 按份额对应的 token0 数量
    /// @return share1 按份额对应的 token1 数量
    /// @return sharePercent 份额百分比（精度 1e18）
    function getLPShare(address account) external view returns (uint256 liquidity, uint256 share0, uint256 share1, uint256 sharePercent) {
        liquidity = balanceOf(account);
        uint256 totalLp = totalSupply();

        if (totalLp == 0) return (0, 0, 0, 0);

        share0 = (liquidity * reserve0) / totalLp;
        share1 = (liquidity * reserve1) / totalLp;
        sharePercent = (liquidity * 1e18) / totalLp;    // 如 0.25 = 250000000000000000
    }

    /// @notice 查询一段时间内的 TWAP 价格
    /// @param priceCumulativeStart 开始时刻的累计价格
    /// @param priceCumulativeEnd 结束时刻的累计价格
    /// @param timeElapsed 时间间隔（秒）
    /// @return twap TWAP 价格（精度 2**112）
    function computeTWAP(uint256 priceCumulativeStart, uint256 priceCumulativeEnd, uint32 timeElapsed) public pure returns (uint256 twap) {
        if (timeElapsed == 0) return 0;
        // TWAP = (priceCumulativeEnd - priceCumulativeStart) / timeElapsed
        twap = (priceCumulativeEnd - priceCumulativeStart) / timeElapsed;
    }

    // ============ 辅助函数 ============

    /// @notice 获取当前池子储备量
    function getReserves() external view returns (uint256 _reserve0, uint256 _reserve1) {
        return (reserve0, reserve1);
    }

    /// @notice 获取当前现货价格
    /// @return price0 token0 相对于 token1 的价格（token0/token1）
    /// @return price1 token1 相对于 token0 的价格（token1/token0）
    function getPrices() external view returns (uint256 price0, uint256 price1) {
        if (reserve0 == 0 || reserve1 == 0) return (0, 0);
        // price0 = reserve1 / reserve0 (1 token0 = ? token1)
        // 注意: Solidity 不支持浮点数，这里返回的是按 1e18 精度放大的值
        price0 = (reserve1 * 1e18) / reserve0;
        price1 = (reserve0 * 1e18) / reserve1;
    }

    /// @notice 计算恒定乘积 k = x * y
    function getK() external view returns (uint256) {
        return reserve0 * reserve1;
    }

    /// @dev 整数平方根（Babylonian method 巴比伦方法）
    function _sqrt(uint256 y) internal pure returns (uint256 z) {
        if (y > 3) {
            z = y;
            uint256 x = y / 2 + 1;
            // 迭代逼近，通常 3-4 次就收敛
            while (x < z) {
                z = x;
                x = (y / x + x) / 2;
            }
        } else if (y != 0) {
            z = 1;
        }
    }

    /// @notice 检查池子是否为空
    function isEmpty() external view returns (bool) {
        return totalSupply() == 0;
    }

}