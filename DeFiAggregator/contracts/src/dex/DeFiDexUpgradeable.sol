// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

// UUPS + Initializable — 升级合约使用 upgradeable 版本（带 init gap）
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";

// v5 中接口和 ReentrancyGuard 不需要专属 upgradeable 版本
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/// @title DeFiDexUpgradeable — 可升级版 DEX
/// @notice 基于原 DeFiDex 的逻辑，重构为可升级架构
/// @dev 使用 UUPS 代理模式 + Initializable
contract DeFiDexUpgradeable is Initializable, UUPSUpgradeable, OwnableUpgradeable, ReentrancyGuard, PausableUpgradeable {
    // ==================== 存储变量（绝对不能改变顺序！） ====================
    // slot 0: Initializable 的状态变量（_initialized + _initializing）
    // slot 1~N: OwnableUpgradeable, ReentrancyGuardUpgradeable, PausableUpgradeable 的变量
    // 然后是我们的业务变量：

    // 费率相关
    uint256 public feeRate;                         // 手续费率（基点，100 = 1%）
    uint256 public constant MAX_FEE_RATE = 1000;    // 最大费率 10%
    uint256 public totalFeeCollected;               // 累计手续费

    // 流动性跟踪
    mapping (address => mapping (address => uint256)) private _liquidity;
    mapping (address => address[]) public tokenPairs;

    // 白名单
    mapping (address => bool) public whitelistedTokens;

    // 暂停状态（注意：PausableUpgradeable 已提供 _pause/_unpause）

    // ==================== 事件 ====================
    event LiquidityAdded(address indexed provider, address indexed token, uint256 amount);
    event LiquidityRemoved(address indexed provider, address indexed token, uint256 amount);
    event Swapped(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut);
    event FeeCollected(address indexed token, uint256 amount);
    event TokenWhitelistUpdated(address indexed token, bool status);
    event FeesWithdrawn(address indexed to, uint256 amount);

    // ==================== 初始化器 ====================

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    /// @notice 初始化可升级 DEX
    /// @param _feeRate 初始费率（基点）
    function initialize(uint256 _feeRate) public initializer {
        require(_feeRate <= MAX_FEE_RATE, "Fee to high");

        __Ownable_init(msg.sender);
        __Pausable_init();

        feeRate = _feeRate;
    }

    // ==================== 业务逻辑（保留原 DeFiDex 功能）====================

    /// @notice 添加流动性
    function addLiquidity(address token, uint256 amount) external nonReentrant whenNotPaused {
        require(whitelistedTokens[token], "Token not whitelisted");
        require(amount > 0, "Amount must be > 0");

        IERC20(token).transferFrom(msg.sender, address(this), amount);
        _liquidity[msg.sender][token] += amount;

        // 记录交易对
        if (_liquidity[msg.sender][token] == amount) {
            tokenPairs[msg.sender].push(token);
        }

        emit LiquidityAdded(msg.sender, token, amount);
    }

    /// @notice 移除流动性
    function removeLiquidity(
        address token,
        uint256 amount
    ) external nonReentrant {
        require(_liquidity[msg.sender][token] >= amount, "Insufficient liquidity");

        _liquidity[msg.sender][token] -= amount;
        IERC20(token).transfer(msg.sender, amount);

        emit LiquidityRemoved(msg.sender, token, amount);
    }

    /// @notice 交换代币
    function swap(
        address tokenIn,
        address tokenOut,
        uint256 amountIn
    ) external nonReentrant whenNotPaused returns (uint256 amountOut) {
        require(whitelistedTokens[tokenIn], "TokenIn not whitelisted");
        require(whitelistedTokens[tokenOut], "TokenOut not whitelisted");
        require(amountIn > 0, "Amount must be > 0");
        require(tokenIn != tokenOut, "Same token");

        // 计算手续费
        uint256 fee = (amountIn * feeRate) / 10000;
        uint256 amountAfterFee = amountIn - fee;
        totalFeeCollected += fee;

        // 转移代币
        IERC20(tokenIn).transferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenOut).transfer(msg.sender, amountAfterFee);

        amountOut = amountAfterFee;

        emit Swapped(msg.sender, tokenIn, tokenOut, amountIn, amountOut);
        if (fee > 0) {
            emit FeeCollected(tokenIn, fee);
        }
    }

    // ==================== 管理功能 ====================

    /// @notice 设置费率
    function setFeeRate(uint256 _feeRate) external onlyOwner {
        require(_feeRate <= MAX_FEE_RATE, "Fee too high");
        feeRate = _feeRate;
    }

    /// @notice 更新代币白名单
    function updateWhitelist(address token, bool status) external onlyOwner {
        require(token != address(0), "Zero address");
        whitelistedTokens[token] = status;
        emit TokenWhitelistUpdated(token, status);
    }

    /// @notice 提取手续费
    function withdrawFees(address to) external onlyOwner {
        require(to != address(0), "Zero address");
        uint256 amount = totalFeeCollected;
        totalFeeCollected = 0;
        // 简化：用 ETH 代替（实际应该用具体代币）
        payable(to).transfer(amount);
        emit FeesWithdrawn(to, amount);
    }

    /// @notice 暂停/恢复
    function pause() external onlyOwner { _pause(); }
    function unpause() external onlyOwner { _unpause(); }

    // ==================== 查询函数 ====================

    /// @notice 查询流动性
    function getLiquidity(address provider, address token)
        external
        view
        returns (uint256)
    {
        return _liquidity[provider][token];
    }

    /// @notice 查询用户的代币对列表
    function getTokenPairs(address provider)
        external
        view
        returns (address[] memory)
    {
        return tokenPairs[provider];
    }

    /// @notice 获取合约版本（constant 可以安全修改，不在存储中）
    function VERSION() external pure returns (string memory) {
        return "1.0.0";
    }

    // ==================== UUPS 升级授权 ====================

    /// @notice 只有 owner 可以升级
    function _authorizeUpgrade(
        address newImplementation
    ) internal override onlyOwner {}
}