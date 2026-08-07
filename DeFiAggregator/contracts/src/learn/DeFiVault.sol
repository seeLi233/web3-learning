// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC4626.sol";
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

// ============ 核心合约 ============

/// @title DeFiVault — ERC4626 收益聚合金库
/// @notice 用户存入 ERC20 资产获得份额代币，金库将资产投入收益策略
///         收益累积到 share price 中，早期存款者获得更多回报
/// @dev 继承 ERC4626（份额计算 + 标准接口）+ Ownable（权限控制）+ ReentrancyGuard（安全）
contract DeFiVault is ERC4626, Ownable, ReentrancyGuard {
    // ============ 库绑定 ============

    // 为什么用 using for？
    // → SafeERC20 是 IERC20 的扩展库，用 using for 后可以直接 asset.safeTransferFrom(...)
    //    而不是 SafeERC20.safeTransferFrom(asset, ...)，代码更简洁
    // 作用：为 IERC20 类型附加 safeTransfer/safeTransferFrom 方法
    using SafeERC20 for IERC20;

    // ============ 错误定义 ============

    // 为什么用自定义 error 而不是 require string？
    // → 自定义 error 比 string 省 gas（error selector 只有 4 字节，string 可长可短）
    // 作用：让前端/测试能精确捕获错误类型
    error Vault__DepositPaused();       // 存款已暂停
    error Vault__InsufficientAssets();  // 金库资产不足以满足取款
    error Vault__ZeroAddress();         // 地址参数为空
    error Vault__SlippageTooHigh();     // 滑点超出容忍度
    error Vault__NoYieldToHarvest();    // 没有可收取的收益

    // ============ 事件定义 ============

    // 为什么定义事件？
    // → 链下监听（前端/后端/数据分析）需要通过事件得知金库状态变化
    //    没有事件的话，用户只能 poll 合约状态，低效且不可靠
    event YieldHarvested(uint256 amount, uint256 newTotalAssets);       // 收益已收取
    event DepositPaused(bool paused);                                   // 存款暂停状态变更
    event PerformanceFeeSet(uint256 oldFee, uint256 newFee);            // 绩效费变更
    event YieldStrategySet(address oldStrategy, address newStrategy);   // 收益策略变更

    // ============ 状态变量 ============

    // 为什么用 uint256 而不是 uint8 存百分比？
    // → Solidity 里 uint8 和 uint256 都占 1 个 storage slot (32 字节)，但 uint8 操作后
    //    需要额外的位掩码来清除高位，反而更费 gas。且 ERC4626 的 preview 函数用 uint256。
    // 作用：基础精度 1e18，performanceFee = 0.2e18 = 20% 绩效费
    //       为什么 20%？→ 这模仿 Yearn 的费率结构（2% 管理费 + 20% 绩效费）
    uint256 public performanceFee = 0.2e18; // 20% — 从收益中抽取给金库管理方的比例

    // 为什么 active 用 bool 而不是 enum？
    // → 当前只有 2 种状态（活跃/暂停），bool 就够了。如果未来需要更多状态（如仅暂停存款/仅暂停取款），
    //    再升级为 enum。遵循 YAGNI 原则（You Ain't Gonna Need It）
    // 作用：紧急情况下暂停存款（如发现策略漏洞），但不影响取款
    bool public depositPaused = false;

    // 为什么 strategy 用 address 而不是 IStrategy 接口？
    // → 降低耦合：不同的收益策略可能来自不同协议（AAVE/Compound/Uniswap），
    //    它们没有统一的 ERC 接口。存 address 在调用时再转换更灵活。
    //    只有在 harvest() 中知道走哪个协议的接口
    // 作用：收益策略合约地址，资产转入该地址产生收益
    address public yieldStrategy;

    // ============ 构造函数 ============

    /// @param _asset 底层 ERC20 资产地址（如 USDC/WETH）
    /// @param _name 份额代币名称（如 "DeFi Vault USDC Share"）
    /// @param _symbol 份额代币符号（如 "dvUSDC"）
    /// @param _owner 金库管理员地址
    // 为什么把 owner 作为参数而不是用 msg.sender？
    // → 工厂合约部署时 msg.sender 是工厂合约地址，不是最终管理员
    //    传参让工厂合约能指定正确的 owner
    // 为什么 ERC4626 的构造函数只接收 asset？
    // → ERC4626(asset) 把 _asset 存入不可变的底层资产引用
    //    ERC20(_name, _symbol) 设置份额代币的名称和符号
    constructor(
        IERC20 _asset,
        string memory _name,
        string memory _symbol,
        address _owner
    ) ERC4626(_asset) ERC20(_name, _symbol) Ownable(_owner) {
        // 为什么不在构造函数里做更多事？
        // → 构造函数 gas 有上限（合约 creation code 有 24KB 限制），
        //    只做必要的初始化。后续配置通过 setter 函数完成
    }

    // ============ 存款函数（重写 ERC4626） ============

    // 为什么重写 _deposit 而不是 deposit？
    // → deposit() 是外部函数，内部调用 _deposit() 做转账 + mint。
    //    重写 _deposit 可以在核心逻辑前后插入自定义逻辑（暂停检查）
    //    而 deposit/mint 的外部接口和行为保持不变
    // 为什么用 nonReentrant？
    // → deposit 涉及外部代币转账（safeTransferFrom），恶意 ERC20 token 可能在 transferFrom
    //    回调里重入 deposit，导致重复 mint 份额
    /// @dev 内部存款逻辑：用户转 asset → 金库 mint shares
    /// @param caller 存款调用者
    /// @param receiver 接收份额代币的地址（可以和 caller 不同）
    /// @param assets 存入的资产数量
    /// @param shares 应铸造的份额数量
    function _deposit(
        address caller,
        address receiver,
        uint256 assets,
        uint256 shares
    ) internal override nonReentrant {
        // 检查 1：存款暂停？→ 紧急开关，策略出问题时暂停入金但不影响出金
        if (depositPaused) revert Vault__DepositPaused();

        // 检查 2：receiver 不能是零地址 → 防止份额代币被永久锁死
        if (receiver == address(0)) revert Vault__ZeroAddress();

        // ERC4626 标准 _deposit：从 caller 转 asset 到金库，给 receiver mint shares
        // 为什么用 super._deposit 而不是自己写转账逻辑？
        // → OZ 的 _deposit 已经实现了安全的转账 + 份额铸造，
        //    包括 safeTransferFrom（兼容非标准 ERC20）和 _mint（ERC20 标准）
        super._deposit(caller, receiver, assets, shares);
    }

    // ============ 取款函数（重写 ERC4626） ============

    // 为什么取款不需要 nonReentrant？
    // → _withdraw 内部调用 safeTransfer 转出资产，如果资产是恶意 ERC20 可能重入。
    //    但由于 _withdraw 会先销毁份额再转账，遵循 CEI（Check-Effect-Interaction）模式：
    //    Check: 检查余额是否足够（_withdraw 内部）
    //    Effect: 销毁 shares（_burn）
    //    Interaction: 转 asset（safeTransfer）
    //    重入后 shares 已经为 0，无法再次取款
    /// @dev 内部取款逻辑：销毁 owner 的 shares → 转 asset 给 receiver
    function _withdraw(
        address caller,
        address receiver,
        address owner,
        uint256 assets,
        uint256 shares
    ) internal override {
        // 检查：金库有足够资产吗？
        // 为什么用 totalAssets() 而不是 asset.balanceOf(this)？
        // → totalAssets() 是虚拟的——它会去查策略里锁定的资产。
        //    asset.balanceOf(this) 只看金库合约余额，看不到策略里的钱
        if (assets > totalAssets()) revert Vault__InsufficientAssets(); 

        // OZ 的 _withdraw 内部会：
        // 1. 如果 caller != owner，检查 allowance (ERC20 标准 approve 机制)
        // 2. _burn(owner, shares) — 销毁份额
        // 3. safeTransfer(receiver, assets) — 转出底层资产
        // 为什么用 OZ 的 _withdraw？
        // → 已经正确地处理了 CEI 顺序和 rounding 方向
        super._withdraw(caller, receiver, owner, assets, shares);
    }

    // ============ 带滑点保护的存款/取款 ⭐ ============

    // 为什么需要滑点保护？
    // → 用户在 mempool 里看到 share price 是 1.0，发起 deposit。
    //    但在交易被打包前，有一个大额 deposit 先执行，share price 变成 1.02。
    //    用户的交易以 1.02 执行，得到的 shares 比预期少 2%——这就是三明治攻击
    // 作用：用户指定最少接受的份额数，如果实际少于这个数就 revert
    /// @notice 存入资产并指定最少接受的份额数（滑点保护）
    /// @param assets 存入资产数量
    /// @param receiver 接收份额的地址
    /// @param minSharesOut 最少接受的份额数（如果实际份额 < minSharesOut → revert）
    /// @return shares 实际铸造的份额数
    function depositWithSlippage(
        uint256 assets,
        address receiver,
        uint256 minSharesOut
    ) public returns (uint256 shares) {
        shares = previewDeposit(assets); // 预览：存这么多 asset 能得到多少 share

        // 为什么 shares < minSharesOut 就 revert？
        // → minSharesOut 是用户设置的"最差接受条件"，保护用户免受价格波动损失
        if (shares < minSharesOut) revert Vault__SlippageTooHigh();

        deposit(assets, receiver);  // ERC4626 标准 deposit
    }

    // 为什么 redeem 也需要滑点保护？
    // → 与 deposit 类似。用户赎回时 share price 可能被夹子压低，
    //    得到的 asset 比预期少。minAssetsOut 防止这种情况
    /// @notice 赎回份额并指定最少接受的资产数（滑点保护）
    function redeemWithSlippage(
        uint256 shares,
        address receiver,
        address owner,
        uint256 minAssetsOut
    ) public returns (uint256 assets) {
        assets = previewRedeem(shares); // 预览：销毁这些 share 能得到多少 asset

        // 为什么检查 assets 而不是继续执行？
        // → 如果预览结果已经低于用户预期，直接 revert 省 gas
        if (assets < minAssetsOut) revert Vault__SlippageTooHigh();

        redeem(shares, receiver, owner); // ERC4626 标准 redeem
    }

    /// @notice 从策略合约收取收益（harvest）
    /// @dev 调用前需确认策略有未收取收益，否则 revert
    // 为什么 harvest 是公开的而不是 onlyOwner？
    // → 任何人都可以触发 harvest（如 keeper bot），这有助于收益的及时复合。
    //    但绩效费只发给 owner，所以不存在被攻击的经济激励
    // 为什么要定期 harvest？
    // → 策略里的资产在产生收益，如果不 harvest，share price 不会涨，
    //    新用户可以享受早期用户产生的收益——不fair
    function harvest() public {
        // 为什么用 totalAssets() 而不是某个单独的 storage 变量？
        // → totalAssets() 是"金库 + 策略"的实时余额，反映了收益是否已经到账。
        //    如果策略转了一些 asset 到金库但还没记录，totalAssets() 能捕获到
        uint256 currentTotal = totalAssets();

        // 为什么需要 convertedToAssets(totalSupply())？
        // → 这是理论上的"应有总资产"——按份额 × 汇率算出来的值
        //    如果 totalSupply = 0（金库为空），应有总资产 = 0，所有余额都是初始资金
        uint256 totalAssetsForShares = (totalSupply() == 0) ? 0 : convertToAssets(totalSupply());

        // 收益 = 实际总资产 - 份额对应的理论资产
        // 为什么用 > 而不是 >= ?
        // → 如果收益 > 0 才分割绩效费，收益 = 0 时不需要任何操作
        uint256 yield = currentTotal > totalAssetsForShares ? currentTotal - totalAssetsForShares : 0;

        // 如果没有收益，直接 revert，省 gas
        // 为什么 revert 而不是 return?
        // → 让调用者明确知道没有收益可收，避免浪费 gas 调了一个空操作
        if (yield == 0) revert Vault__NoYieldToHarvest();

        // 绩效费 = 收益 × performanceFee / 1e18
        // 例如：收益 100 USDC，绩效费 20% → 20 USDC 给 owner
        // 为什么在 yield 上收取而不是在 totalAssets 上？
        // → 绩效费只对"新增的收益"收，不对本金收。如果按 totalAssets 收，用户本金越存越少
        uint256 fee = (yield * performanceFee) / 1e18;

        // 为什么先转绩效费再记录？
        // → 遵循 CEI：transfer 是外部调用（Interaction），应该放在最后。
        //    但这里我们只发一个事件，没有 storage 状态更新依赖，所以顺序无影响。
        //    真正重要的是——资产已经在金库了（策略把收益转回来了），
        //    现在只是把一部分绩效费从金库转给 owner
        if (fee > 0) {
            // SafeERC20 的安全转账——兼容不返回 bool 的 ERC20（如 USDT）
            IERC20(asset()).safeTransfer(owner(), fee);
        }

        emit YieldHarvested(yield, currentTotal);
    }

    // ============ 管理员函数 ============

    /// @notice 设置收益策略地址
    // 为什么这里要验零地址？
    // → 如果 strategy 被误设为 address(0)，资产转进去就永久丢失了
    function setYieldStrategy(address _startegy) external onlyOwner {
        if (_startegy == address(0)) revert Vault__ZeroAddress();
        emit YieldStrategySet(yieldStrategy, _startegy);
        yieldStrategy = _startegy;
    }

    /// @notice 暂停/恢复存款
    // 为什么只能暂停存款不能暂停取款？
    // → DeFi 信任第一——暂停取款等于 rug。用户必须随时能取出自己的钱
    //    暂停存款只是保护新用户不在危险时进入
    function setDepositPaused(bool _paused) external onlyOwner {
        depositPaused = _paused;
        emit DepositPaused(_paused);
    }

    /// @notice 设置绩效费比例
    /// @param _fee 新的绩效费（18 位精度，0.2e18 = 20%）
    // 为什么限制在 50%？
    // → 超过 50% 的绩效费会被认为是不合理的，防止管理员恶意设置
    function setPerformanceFee(uint256 _fee) external onlyOwner {
        if (_fee > 0.5e18) revert(); // 上限 50%，保护用户
        emit PerformanceFeeSet(performanceFee, _fee);
        performanceFee = _fee;
    }

    // ============ 辅助函数 ============

    /// @notice 查询指定用户的份额对应的资产价值
    // 为什么写这个辅助函数？
    // → 前端需要显示"你的存款现在值多少钱"
    //    maxWithdraw(user) 也能做到，但这个命名更直观
    function balanceOfAssets(address user) public view returns(uint256) {
        return convertToAssets(balanceOf(user));
    }
}