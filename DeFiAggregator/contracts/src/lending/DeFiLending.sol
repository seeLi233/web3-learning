// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "./IInterestRateModel.sol";

/// @title DeFiLending — 去中心化借贷协议核心
/// @notice 支持多种资产作为抵押品和借款资产
///
/// @dev 核心设计思路:
///   1. 利率指数模型 — 通过全局累计指数实现高效利息分配
///   2. 超额抵押 — 每笔借款必须有足够抵押品
///   3. 健康因子 — 实时监控用户仓位安全
///
///   利率计算公式 (参考 Day 19):
///     U = totalDebt / (totalLiquidity + totalDebt)
///     borrowRate = f(U)  ← InterestRateModel 拐点模型
///     supplyRate = borrowRate × U × (1 - reserveFactor)
///
///   指数累计:
///     liquidityIndex_new = liquidityIndex × (1 + supplyRate × Δt)
///     borrowIndex_new    = borrowIndex    × (1 + borrowRate × Δt)
///
///   用户余额计算:
///     userBalance = userAmount × (currentIndex / userIndex)
contract DeFiLending is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    constructor() Ownable(msg.sender) {}

    // ============ 精度常量 ============
    uint256 public constant RAY = 1e27;     // 基础精度
    uint256 public constant SECONDS_PER_YEAR = 365 days;    // 年化时间基准

    // ============ 资产配置 ============
    struct ReserveConfiguration {
        // --- 资产状态 ---
        bool isActive;          // 是否启用该资产
        bool isFrozen;          // 是否冻结（紧急暂停）
        bool canBorrow;         // 是否允许借款
        bool canBeCollateral;   // 是否可作为抵押品

        // --- 风险参数 (RAY 精度) ---
        uint256 ltv;                    // 贷款价值比 Loan-to-Value
        uint256 liquidationThreshold;   // 清算阈值（通常 > LTV）
        uint256 liquidationBonus;       // 清算奖励（清算者获得的额外折扣）

        // --- 利率模型 ---
        IInterestRateModel interestRateModel;   // 该资产的利率模型（可不同资产用不同模型！）
        uint256 reserveFactor;                  // 储备因子 (RAY)，平台从利息中的抽成

        // --- 利率累计指数 (RAY 精度, 初始值 = RAY) ---
        uint256 liquidityIndex;         // 存款利息累计指数
        uint256 variableBorrowIndex;    // 借款利息累计指数
        uint256 lastUpdateTimestamp;    // 上次更新时间戳

        // --- 资金池状态 ---
        uint256 totalLiquidity;         // 总存款 (token 原生单位)
        uint256 totalVariableDebt;      // 总债务 (token 原生单位)
    }

    /// @notice 用户在单个资产上的仓位
    struct UserPosition {
        uint256 supplyAmount;       // 存款数量（原始值，不含利息）
        uint256 supplyIndex;        // 存款时的 liquidityIndex（用于计算利息）
        uint256 borrowAmount;       // 借款数量（原始值，不含利息）
        uint256 borrowIndex;        // 款时的 variableBorrowIndex（用于计算利息）
    }

    // ============ 状态变量 ============
    // asset => ReserveConfiguration
    mapping (address => ReserveConfiguration) internal _reserves;

    // user => asset => UserPosition
    mapping (address => mapping(address => UserPosition)) internal _positions;

    // 所有已注册的资产列表
    address[] internal _reserveList;

    // 资产地址 => 在 _reserveList 中的索引
    mapping (address => uint256) internal _reserveIndex;

    // ============ 事件 ============
    event ReserveInitialized(address indexed asset, uint256 ltv, uint256 liquidationThreshold);
    event Deposited(address indexed user, address indexed asset, uint256 amount, uint256 shares);
    event Redeemed(address indexed user, address indexed asset, uint256 amount, uint256 shares);
    event Borrowed(address indexed user, address indexed asset, uint256 amount);
    event Repaid(address indexed user, address indexed asset, uint256 amount);
    event ReserveUpdated(address indexed asset, bool isActive, bool isFrozen);

    // ============ 初始化函数 ============

    /// @notice 初始化一个新资产（只有 Owner 可以添加）
    /// @param asset 资产代币地址
    /// @param interestRateModel 该资产的利率模型地址
    /// @param ltv 贷款价值比 (RAY)，如 0.75 RAY = 75%
    /// @param liquidationThreshold 清算阈值 (RAY)，如 0.80 RAY = 80%
    /// @param liquidationBonus 清算奖励 (RAY)，如 0.05 RAY = 5%
    /// @param reserveFactor 储备因子 (RAY)，如 0.10 RAY = 10%
    function initReserve(
        address asset,
        IInterestRateModel interestRateModel,
        uint256 ltv,
        uint256 liquidationThreshold,
        uint256 liquidationBonus,
        uint256 reserveFactor
    ) external onlyOwner {
        require(!_reserves[asset].isActive, "Already initialized");
        require(ltv <= RAY && ltv > 0, "Invalid LTV");
        require(liquidationThreshold >= ltv && liquidationThreshold <= RAY, "Invalid liquidation threshold");
        require(liquidationBonus <= RAY, "Invalid liquidation bonus");
        require(reserveFactor <= RAY, "Invalid reserve factor");

        ReserveConfiguration storage config = _reserves[asset];
        config.isActive = true;
        config.canBorrow = true;
        config.canBeCollateral = true;
        config.ltv = ltv;
        config.liquidationThreshold = liquidationThreshold;
        config.liquidationBonus = liquidationBonus;
        config.interestRateModel = interestRateModel;
        config.reserveFactor = reserveFactor;

        // 初始化指数为 1.0 (RAY)
        config.liquidityIndex = RAY;
        config.variableBorrowIndex = RAY;
        config.lastUpdateTimestamp = block.timestamp;

        // 加入资产列表
        _reserveIndex[asset] = _reserveList.length;
        _reserveList.push(asset);

        emit ReserveInitialized(asset, ltv, liquidationThreshold);
    }

    // ============ 查询函数 ============

    /// @notice 获取所有已注册的资产列表
    function getReserveList() external view returns (address[] memory) {
        return _reserveList;
    }

    /// @notice 获取单个资产的完整配置
    function getReserveConfig(address asset) external view returns (ReserveConfiguration memory) {
        return _reserves[asset];
    }

    /// @notice 获取用户在某个资产上的仓位
    function getUserPosition(address user, address asset) external view returns (UserPosition memory) {
        return _positions[user][asset];
    }

    // ============ 利率更新（利息累计） ============

    /// @notice 更新某个资产池的利率指数
    /// @dev 每次 deposit/redeem/borrow/repay 前都必须调用
    /// @param asset 资产地址
    /// @return 更新后的 liquidityIndex 和 variableBorrowIndex
    function _updateIndexes(address asset) internal returns (uint256, uint256) {
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive, "Reserve not active");

        // 如果上一次更新时间就是当前区块，不需要重复更新
        if (config.lastUpdateTimestamp == block.timestamp) {
            return (config.liquidityIndex, config.variableBorrowIndex);
        }

        // Step 1: 调用利率模型获取当前利率
        (uint256 borrowRate, uint256 supplyRate) = config.interestRateModel.getRates(
            // cash = totalLiquidity, borrows = totalVariableDebt, reserves = 0
            config.totalLiquidity,
            config.totalVariableDebt,
            0
        );

        // Step 2: 计算时间差
        uint256 timeDelta = block.timestamp - config.lastUpdateTimestamp;

        // Step 3: 只有有借款时才更新（没借款时利息为 0，跳过以节省 Gas）
        if (config.totalVariableDebt > 0 && timeDelta > 0) {
            // 把年化利率转为时间段利率，使用线性近似
            // indexGrowth = rate × Δt / YEAR
            // newIndex = oldIndex + oldIndex × indexGrowth
            //          = oldIndex × (1 + rate × Δt / YEAR)

            uint256 oldBorrowIndex = config.variableBorrowIndex;
            uint256 oldLiquidityIndex = config.liquidityIndex;

            uint256 borrowRatePerSecond = (borrowRate * timeDelta) / SECONDS_PER_YEAR;
            config.variableBorrowIndex = oldBorrowIndex + (oldBorrowIndex * borrowRatePerSecond) / RAY;

            uint256 supplyRatePerSecond = (supplyRate * timeDelta) / SECONDS_PER_YEAR;
            config.liquidityIndex = oldLiquidityIndex + (oldLiquidityIndex * supplyRatePerSecond) / RAY;

            // 同步更新总债务和总存款到新指数（否则 repay 时会 overflow）
            config.totalVariableDebt = (config.totalVariableDebt * config.variableBorrowIndex) / oldBorrowIndex;
            config.totalLiquidity = (config.totalLiquidity * config.liquidityIndex) / oldLiquidityIndex;
        }

        // Step 4: 更新时间戳
        config.lastUpdateTimestamp = block.timestamp;

        return (config.liquidityIndex, config.variableBorrowIndex);
    }

    /// @notice 计算用户在某个资产上的当前实际余额（含利息）
    /// @param user 用户地址
    /// @param asset 资产地址
    /// @return supplyBalance 存款实际余额（本金+利息）
    /// @return borrowBalance 借款实际余额（本金+利息）
    function getUserBalances(address user, address asset) public view returns (uint256 supplyBalance, uint256 borrowBalance) {
        ReserveConfiguration storage config = _reserves[asset];
        UserPosition storage position = _positions[user][asset];

        if (position.supplyAmount > 0) {
            // supplyBalance = supplyAmount × (currentIndex / userIndex)
            supplyBalance = (position.supplyAmount * config.liquidityIndex) / position.supplyIndex;
        }

        if (position.borrowAmount > 0) {
            // borrowBalance = borrowAmount × (currentIndex / userIndex)
            borrowBalance = (position.borrowAmount * config.variableBorrowIndex) / position.borrowIndex;
        }
    }

    // ============ 存款 ============

    // 1. 存款的"份额"机制
    // 用户 A 在 index=1.0 时存入 1000 USDC
    //     supplyAmount = 1000, supplyIndex = 1.0

    // 一年后 index=1.15（涨了 15%）
    // 用户 A 再存入 500 USDC

    // 步骤:
    // ① 先结算旧存款利息: currentBalance = 1000 × 1.15/1.0 = 1150
    // ② supplyAmount = 1150 + 500 = 1650
    // ③ supplyIndex = 1.15

    // 用户总余额 = 1650 × 1.15/1.15 = 1650 ✓

    /// @notice 存入资产到借贷池，赚取利息
    /// @param asset 资产地址
    /// @param amount 存入数量
    function deposit(address asset, uint256 amount) external nonReentrant {
        require(amount > 0, "Amount must be > 0");
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive, "Reserve not active");
        require(!config.isFrozen, "Reserve frozen");

        // Step 1: 更新利率指数（必须先更新，保证利息计算准确）
        _updateIndexes(asset);

        // Step 2: 计算用户的"份额"（存入时除以当前指数，取款时再乘以新指数）
        UserPosition storage position = _positions[msg.sender][asset];

        // 如果用户已经有存款，先把利息结算到 supplyAmount
        if (position.supplyAmount > 0) {
            uint256 currentBalance = (position.supplyAmount * config.liquidityIndex) / position.supplyIndex;
            position.supplyAmount = currentBalance;
        } else {
            // 第一次存款，把旧的债务相关数据清零（防止残留数据）
            position.supplyAmount = 0;
        }

        // Step 3: 新增存款
        position.supplyAmount += amount;
        position.supplyIndex = config.liquidityIndex;   // 记录当前的指数

        // Step 4: 更新全局状态
        config.totalLiquidity += amount;

        // Step 5: 转账
        IERC20(asset).safeTransferFrom(msg.sender, address(this), amount);

        emit Deposited(msg.sender, asset, amount, amount);
    }

    // ============ 取款 ============

    // 2. 取款时检查健康因子
    // — 这是最容易忘记的！用户取出抵押品后，剩余抵押品可能不足以支撑借款，必须先检查。
    // 3. safeTransferFrom vs safeTransfer
    // — deposit 用 safeTransferFrom（从用户转给合约）
    // — redeem 用 safeTransfer（从合约转给用户）
    // — safe 前缀是 OpenZeppelin 的 SafeERC20 库，防止某些不规范的 ERC20 代币（如 USDT 的 transfer 不返回 bool）

    /// @notice 从借贷池中取回存款+利息
    /// @param asset 资产地址
    /// @param amount 取款数量（包含本金+利息的总额）
    function redeem(address asset, uint256 amount) external nonReentrant {
        require(amount > 0, "Amount must be > 0");
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive, "Reserve not active");

        // Step 1: 更新利率指数
        _updateIndexes(asset);

        // Step 2: 计算用户当前实际余额
        UserPosition storage position = _positions[msg.sender][asset];
        uint256 currentBalance = (position.supplyAmount * config.liquidityIndex) / position.supplyIndex;
        require(currentBalance >= amount, "Insufficient balance");

        // Step 3: 检查取款后是否仍满足借款的抵押要求（检查用户所有债务，不只当前资产）
        (, uint256 totalDebt) = _getUserPositionValue(msg.sender);
        if (totalDebt > 0) {
            // 计算取款后的健康因子
            uint256 newSupplyBalance = currentBalance - amount;
            require(_checkHealthFactor(msg.sender, asset, newSupplyBalance), "Health factor < 1 after redeem");
        }

        // Step 4: 更新用户的份额（按比例减少）
        if (currentBalance == amount) {
            // 全部取走
            position.supplyAmount = 0;
            position.supplyIndex = RAY;
        } else {
            // 部分取走，更新 supplyAmount 为新剩余量
            position.supplyAmount = currentBalance - amount;
            position.supplyIndex = config.liquidityIndex;
        }

        // Step 5: 更新全局状态
        config.totalLiquidity -= amount;

        // Step 6: 转账
        IERC20(asset).safeTransfer(msg.sender, amount);

        emit Redeemed(msg.sender, asset, amount, amount);
    }

    // ============ 借款 ============

    // 1. 借款的"两步利息结算"
    // 用户之前借了 1000，borrowIndex=1.0
    // 现在 borrowIndex=1.05
    // 再次借款 500 之前：
    // ① currentDebt = 1000 × 1.05/1.0 = 1050 (先结算旧利息)
    // ② borrowAmount = 1050 + 500 = 1550
    // ③ borrowIndex = 1.05

    // 用户总欠款 = 1550 × 1.05/1.05 = 1550 ✓

    /// @notice 以抵押品为担保借出资产
    /// @param asset 要借出的资产地址
    /// @param amount 借款数量
    function borrow(address asset, uint256 amount) external nonReentrant {
        require(amount > 0, "Amount must be > 0");
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive, "Reserve not active");
        require(!config.isFrozen, "Reserve frozen");
        require(config.canBorrow, "Borrowing disabled");
        require(config.totalLiquidity >= amount, "Insufficient liquidity");

        // Step 1: 更新利率指数
        _updateIndexes(asset);

        // Step 2: 更新用户的借款余额（先结算旧利息）
        UserPosition storage position = _positions[msg.sender][asset];
        if (position.borrowAmount > 0) {
            uint256 currentDebt = (position.borrowAmount * config.variableBorrowIndex) / position.borrowIndex;
            position.borrowAmount = currentDebt;
        }

        // Step 3: 新增借款
        position.borrowAmount += amount;
        position.borrowIndex = config.variableBorrowIndex;

        // Step 4: 检查抵押是否充足（遍历用户所有抵押品）
        require(_isPositionHealthy(msg.sender), "Insufficient collateral");

        // Step 5: 更新全局状态
        config.totalLiquidity -= amount;
        config.totalVariableDebt += amount;

        // Step 6: 转账
        IERC20(asset).safeTransfer(msg.sender, amount);

        emit Borrowed(msg.sender, asset, amount);
    }

    // ============ 还款 ============

    /// @notice 归还借款+利息
    /// @param asset 资产地址
    /// @param amount 还款数量
    function repay(address asset, uint256 amount) external nonReentrant {
        require(amount > 0, "Amount must be > 0");
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive, "Reserve not active");

        // Step 1: 更新利率指数
        _updateIndexes(asset);

        // Step 2: 计算当前实际债务
        UserPosition storage position = _positions[msg.sender][asset];
        require(position.borrowAmount > 0, "No debt to repay");
        uint256 currentDebt = (position.borrowAmount * config.variableBorrowIndex) / position.borrowIndex;

        // Step 3: 还款金额不能超过债务
        uint256 repayAmount = amount > currentDebt ? currentDebt : amount;

        // Step 4: 更新用户的债务
        uint256 newDebt = currentDebt - repayAmount;
        if (newDebt == 0) {
            position.borrowAmount = 0;
            position.borrowIndex = RAY;
        } else {
            position.borrowAmount = newDebt;
            position.borrowIndex = config.variableBorrowIndex;
        }

        // Step 5: 更新全局状态
        config.totalLiquidity += repayAmount;
        config.totalVariableDebt -= repayAmount;

        // Step 6: 转账（只扣除实际还款金额，safeTransferFrom 不会多扣）
        IERC20(asset).safeTransferFrom(msg.sender, address(this), repayAmount);

        emit Repaid(msg.sender, asset, amount);
    }

    // ============ 健康因子计算 ⭐⭐⭐ ============

    // 2. 还款的"多退"设计
    // solidity
    // uint256 repayAmount = amount > currentDebt ? currentDebt : amount;
    // 2. — 如果你欠 1000 但转了 1200，只收 1000，退 200。防止用户多付。
    // 3. 健康因子计算中为什么用 liquidationThreshold 而不是 LTV？
    // — LTV 决定"最多能借多少"，liquidationThreshold 决定"什么时候清算"。
    // — 这两个值的差就是"安全缓冲区"（通常 2-5%）。
    // — 例：ETH LTV=80%, LT=82.5%。你借到 LTV 上限 80% 时，ETH 跌 3.04% 就触发清算。
    // 4. ⭐ 最终面试重点 — 健康因子公式
    // HF = Σ(supplyBalance_i × price_i × liquidationThreshold_i) / Σ(borrowBalance_j × price_j)

    /// @notice 计算用户的健康因子
    /// @param user 用户地址
    /// @return healthFactor 健康因子 (RAY 精度)
    ///          > RAY: 安全 | = RAY: 临界 | < RAY: 可清算
    function getHealthFactor(address user) public view returns (uint256 healthFactor) {
        (uint256 totalCollateralValue, uint256 totalDebtValue) = _getUserPositionValue(user);

        if (totalDebtValue == 0) {
            // 无借款 → 完全健康
            return type(uint256).max;
        }

        // HF = collateralValue × liquidationThreshold / debtValue
        // 注意: 这里简化处理，假设所有资产价格 = 1（因为没有集成预言机）
        // 真实协议中: collateralValue = Σ(supplyAmount × price × liquidationThreshold)
        healthFactor = (totalCollateralValue * RAY) / totalDebtValue;
    }

    /// @notice 检查用户仓位是否健康
    function _isPositionHealthy(address user) internal view returns (bool) {
        return getHealthFactor(user) > RAY;
    }

    /// @notice 检查取款后健康因子（针对单个资产取款）
    function _checkHealthFactor(
        address user,
        address redeemingAsset,
        uint256 newSupplyBalance
    ) internal view returns (bool) {
        // 简化版本：计算取款后用户的总抵押价值和总债务，判断 HF >= 1
        // 注意：这里需要预估取款后该资产的价值变化
        // 真实协议会更复杂，需要考虑每个资产的 LTV 和 liquidationThreshold 不同

        uint256 totalCollateralValue = 0;
        uint256 totalDebtValue = 0;

        // 遍历所有资产
        for (uint256 i = 0; i < _reserveList.length; i++) {
            address assetAddr = _reserveList[i];
            ReserveConfiguration storage cfg = _reserves[assetAddr];
            UserPosition storage pos = _positions[user][assetAddr];

            // 计算该资产的抵押价值
            uint256 supplyBalance;
            if (assetAddr == redeemingAsset) {
                supplyBalance = newSupplyBalance; // 使用取款后的新余额
            } else if (pos.supplyAmount > 0) {
                supplyBalance = (pos.supplyAmount * cfg.liquidityIndex) / pos.supplyIndex;
            }

            if (supplyBalance > 0 && cfg.canBeCollateral) {
                // 抵押价值 = 存款余额 × 清算阈值
                // （用清算阈值而不是 LTV，因为清算发生在 liquidationThreshold）
                totalCollateralValue += (supplyBalance * cfg.liquidationThreshold) / RAY;
            }

            // 计算该资产的债务价值
            uint256 borrowBalance;
            if (pos.borrowAmount > 0) {
                borrowBalance = (pos.borrowAmount * cfg.variableBorrowIndex) / pos.borrowIndex;
                totalDebtValue += borrowBalance;
            }
        }

        if (totalDebtValue == 0) return true;
        // 两个值都是原生单位（WAD），直接比较即可（抵押价值已除以 RAY 归一化）
        return totalCollateralValue >= totalDebtValue;
    }

    /// @notice 获取用户的总抵押价值和总债务
    function _getUserPositionValue(address user) internal view returns (uint256 totalCollateral, uint256 totalDebt) {
        for(uint256 i = 0; i < _reserveList.length; i++) {
            address assetAddr = _reserveList[i];
            ReserveConfiguration storage cfg = _reserves[assetAddr];
            UserPosition storage pos = _positions[user][assetAddr];

            if (pos.supplyAmount > 0 && cfg.canBeCollateral) {
                uint256 supplyBalance = (pos.supplyAmount * cfg.liquidityIndex) / pos.supplyIndex;
                totalCollateral += (supplyBalance * cfg.liquidationThreshold) / RAY;
            }

            if (pos.borrowAmount > 0) {
                uint256 borrowBalance = (pos.borrowAmount * cfg.variableBorrowIndex) / pos.borrowIndex;
                totalDebt += borrowBalance;
            }
        }
    }

    // ============ 紧急管理 ============

    /// @notice 设置资产状态（冻结/解冻，暂停/恢复借款）
    function setReserveStatus(address asset, bool isActive, bool isFrozen, bool canBorrow) external onlyOwner {
        ReserveConfiguration storage config = _reserves[asset];
        require(config.isActive || isActive, "Reserve not initialized");
        config.isActive = isActive;
        config.isFrozen = isFrozen;
        config.canBorrow = canBorrow;
        emit ReserveUpdated(asset, isActive, isFrozen);
    }
}