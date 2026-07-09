// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

// ============ ERC-3156 标准接口 ============

/// @notice ERC-3156 闪电贷借款方必须实现的接口
interface IERC3156FlashBorrower {
    /// @notice 收到闪电贷后的回调
    /// @param initiator 发起闪电贷的地址
    /// @param token 借出的代币地址
    /// @param amount 借出数量
    /// @param fee 手续费
    /// @param data 自定义数据
    function onFlashLoan(
        address initiator,
        address token,
        uint256 amount,
        uint256 fee,
        bytes calldata data
    ) external returns (bytes32);
}

/// @title FlashLoan
/// @notice ERC-3156 闪电贷放贷方合约
/// @dev 支持多种代币的闪电贷，手续费可配置
///
/// 核心安全机制:
///   1. 原子性: 借和还在同一笔交易完成，任一失败则全部回滚
///   2. 手续费: 每笔闪电贷收取手续费（默认 0.09% = 9 bps）
///   3. 魔法值校验: onFlashLoan 必须返回正确的 bytes32，防止假回调
///   4. 余额验证: 借出后合约余额必须 >= 借出前余额 + 手续费
contract FlashLoan is Ownable, ReentrancyGuard {
    using SafeERC20 for IERC20;

    // ============ 错误定义 ============
    error FlashLoanNotRepaid();     // 闪电贷未还
    error InvalidCallbackReturn();  // 回调返回值错误
    error InsufficientBalance();    // 合约余额不足
    error TokenNotSupported();      // 不支持的代币
    error ZeroAmount();             // 借款金额为0
    error FeeTooHigh(uint256 fee, uint256 maxFee); // 手续费超过上限

    // ============ 事件 ============
    event FlashLoanExecuted(address indexed receiver, address indexed token, uint256 amount, uint256 fee);

    event FeeUpdated(address indexed token, uint256 oldFee, uint256 newFee);
    event TokenAdded(address indexed token);
    event TokenRemoved(address indexed token);

    // ============ 常量 ============
    // magic value for onFlashLoan
    bytes32 public constant CALLBACK_SUCCESS = keccak256("ERC3156FlashBorrower.onFlashLoan");
    uint256 public constant MAX_BPS = 10_000;

    // ============ 状态变量 ============
    // 默认手续费 (bps)
    uint256 public defaultFee;
    // 支持代币 → 是否启用
    mapping (address => bool) public supportedTokens;
    // 代币 → 手续费 (bps, 如 9 = 0.09%)
    mapping (address => uint256) public flashFees;      // 默认 9 bps = 0.09%
    // 代币 → 最大可借数量 (0 = 无限制)
    mapping (address => uint256) public maxFlashLoanAmounts;

    // ============ 构造函数 ============
    /// @param _initialFee 默认手续费（bps），如 9 = 0.09%
    constructor(uint256 _initialFee) Ownable(msg.sender) {
        if (_initialFee > MAX_BPS) revert FeeTooHigh(_initialFee, MAX_BPS);
        defaultFee = _initialFee;
    }

    // ============ 核心函数: flashLoan ============

    /// @notice 执行闪电贷 ⚡
    /// @param receiver 借款方合约（必须实现 IERC3156FlashBorrower）
    /// @param token 借出的代币地址
    /// @param amount 借出数量
    /// @param data 自定义数据（传给 receiver.onFlashLoan）
    /// @return success 是否成功
    ///
    /// 执行流程:
    ///   1. 检查合约余额是否足够
    ///   2. 计算手续费
    ///   3. 转出代币给 receiver
    ///   4. 调用 receiver.onFlashLoan() 执行借款方逻辑
    ///   5. 验证魔法值返回值
    ///   6. 验证余额: balanceAfter >= balanceBefore + fee
    ///   7. 如果任一失败 → 整个交易 revert
    function flashLoan(IERC3156FlashBorrower receiver, address token, uint256 amount, bytes calldata data) external nonReentrant returns (bool success) {
        // 1. 安全检查
        if (!supportedTokens[token]) revert TokenNotSupported();
        if (amount == 0) revert ZeroAmount();

        uint256 maxLoan = maxFlashLoanAmounts[token];
        if (maxLoan > 0 && amount > maxLoan) revert InsufficientBalance();

        // 2. 记录借出前余额
        uint256 balanceBefore = IERC20(token).balanceOf(address(this));

        // 3. 检查合约余额是否足够
        if (balanceBefore < amount) revert InsufficientBalance();

        // 4. 计算手续费
        uint256 fee = flashFee(token, amount);

        // 5. ⚡ 转出代币 — 钱离开合约了！
        IERC20(token).safeTransfer(address(receiver), amount);

        // 6. 🔄 调用借款方回调 — 在这里执行借款方的套利/清算逻辑
        bytes32 result = IERC3156FlashBorrower(receiver).onFlashLoan(
            msg.sender,  // initiator — 谁发起的
            token,
            amount,
            fee,
            data        // 传递自定义数据
        );

        // 7. ✅ 验证回调返回值 — 防止假回调
        if (result != CALLBACK_SUCCESS) revert InvalidCallbackReturn();

        // 8. 🔒 关键检查: 钱还回来了吗？
        uint256 balanceAfter = IERC20(token).balanceOf(address(this));
        if (balanceAfter < balanceBefore + fee) revert FlashLoanNotRepaid();

        // 9. 如果还多了，多余的钱留在合约（对放贷方有利）
        //    也可以在后续版本中退还多余部分

        emit FlashLoanExecuted(address(receiver), token, amount, fee);
        return true;
    }

    // ============ 查询函数 ============

    /// @notice 查询最大可借数量
    function maxFlashLoan(address token) external view returns (uint256) {
        if (!supportedTokens[token]) return 0;
        uint256 max = maxFlashLoanAmounts[token];
        if (max > 0) return max;
        // 如果没设上限，按 ERC-3156 规范返回 type(uint256).max
        return type(uint256).max;
    }

    /// @notice 查询手续费
    function flashFee(address token, uint256 amount) public view returns (uint256) {
        // 手续费 = 借款额 * 费率(bps) / 10000
        return (amount * flashFees[token]) / MAX_BPS;
    }

    // ============ 管理函数（onlyOwner）============

    /// @notice 充值代币到闪电贷池
    function deposit(address token, uint256 amount) external onlyOwner {
        IERC20(token).safeTransferFrom(msg.sender, address(this), amount);
    }

    /// @notice 提取代币
    function withdraw(address token, uint256 amount) external onlyOwner {
        IERC20(token).safeTransfer(msg.sender, amount);
    }

    /// @notice 添加支持的代币
    function addSupportedToken(address token, uint256 fee, uint256 maxLoan) external onlyOwner {
        if (fee > MAX_BPS) revert FeeTooHigh(fee, MAX_BPS);
        supportedTokens[token] = true;
        flashFees[token] = fee;
        maxFlashLoanAmounts[token] = maxLoan;
        emit TokenAdded(token);
    }

    /// @notice 移除支持的代币
    function removeSupportToken(address token) external onlyOwner {
        supportedTokens[token] = false;
        emit TokenRemoved(token);
    }

    /// @notice 更新手续费
    function updateFee(address token, uint256 newFee) external onlyOwner {
        if (newFee > MAX_BPS) revert FeeTooHigh(newFee, MAX_BPS);  // 手续费不能超过 100%
        uint256 oldFee = flashFees[token];
        flashFees[token] = newFee;
        emit FeeUpdated(token, oldFee, newFee);
    }

    // ============ 紧急救援 ============

    /// @notice 紧急提取（防止资金被锁）
    function rescueToken(address token) external onlyOwner {
        uint256 balance = IERC20(token).balanceOf(address(this));
        IERC20(token).safeTransfer(msg.sender, balance);
    }
}