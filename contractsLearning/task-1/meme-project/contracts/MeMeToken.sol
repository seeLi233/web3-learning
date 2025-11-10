// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@uniswap/v2-core/contracts/interfaces/IUniswapV2Pair.sol";
import "@uniswap/v2-core/contracts/interfaces/IUniswapV2Factory.sol";
import "@uniswap/v2-periphery/contracts/interfaces/IUniswapV2Router02.sol";

/**
 * @title ShibaLikeToken
 * @dev 基于ERC20的Meme代币合约，实现SHIB风格的交易税和交易限制功能
 * 包含税费分配机制和防市场操纵的交易限制
 */
contract MeMeToken is ERC20, Ownable {
    // 税费相关参数
    uint256 public taxRate; // 总交易税率 (百分比，例如10表示10%)
    uint256 public treasuryPercent; // 国库分配比例 (占总税费的百分比)
    uint256 public burnPercent; // 销毁分配比例 (占总税费的百分比)
    uint256 public marketingPercent; // 营销地址分配比例 (占总税费的百分比)

    // 分配地址
    address public treasuryAddress;
    address public marketingAddress;
    address public constant BURN_ADDRESS = address(0x0); // 销毁地址

    // 交易限制参数
    uint256 public maxTxPercent; // 单笔最大交易比例 (占总供应量的百分比)
    uint256 public maxDailyTx; // 每日最大交易次数

    // 交易记录跟踪
    mapping(address => uint256) public lastTxDay; // 记录地址最后交易的天数
    mapping(address => uint256) public dailyTxCount; // 记录地址当日交易次数

    // 豁免地址 (不受交易限制和税费影响)
    mapping(address => bool) public isExempt;

    // 流动性相关参数
    IUniswapV2Router02 public uniswapRouter; // Uniswap 路由合约
    address public uniswapPair; // 本代币与 WETH 的交易对
    address public WETH;

    event LiquidityAdded(address indexed provider, uint256 tokenAmount, uint256 wethAmount, uint256 lpTokens);
    event LiquidityRemoved(address indexed provider, uint256 tokenAmount, uint256 wethAmount, uint256 lpTokens);

    /**
     * @dev 构造函数初始化代币基本信息和初始参数
     * @param name 代币名称
     * @param symbol 代币符号
     * @param totalSupply_ 总供应量
     */
    constructor(
        string memory name,
        string memory symbol,
        uint256 totalSupply_,
        address routerAddress // 初始化是传入 uniswap 路由地址
    ) ERC20(name, symbol) Ownable(msg.sender) {
        // 初始供应量 (假设精度为18位)
        _mint(msg.sender, totalSupply_);

        // 初始税费设置 (总税率10%，分配比例：国库5%，销毁3%，营销2%)
        taxRate = 10;
        treasuryPercent = 50; // 占总税费的50% (即总交易的5%)
        burnPercent = 30; // 占总税费的30% (即总交易的3%)
        marketingPercent = 20; // 占总税费的20% (即总交易的2%)

        // 初始地址设置 (部署者需后续更新实际地址)
        treasuryAddress = msg.sender;
        marketingAddress = msg.sender;

        // 初始交易限制 (单笔最大5%总供应量，每日最多10笔)
        maxTxPercent = 5;
        maxDailyTx = 10;

        // 豁免部署者地址
        isExempt[msg.sender] = true;

        // 流动性池初始化
        uniswapRouter = IUniswapV2Router02(routerAddress);
        // 创建本代币与 WETH 的交易对 (仅首次使用)
        uniswapPair = IUniswapV2Factory(uniswapRouter.factory()).createPair(address(this), WETH);
        // 将流动性池地址加入豁免列表 (避免对流动性交易征税)
        isExempt[uniswapPair] = true;
    }

    /**
     * @dev 添加流动性 (本代币 + WETH)
     * @param tokenAmount 要添加的本代币数量
     * @param minWeth 最小可接受的 WETH 数量 (防止滑点)
     * @param deadline 交易截止时间戳
     */
    function addLiquidity(
        uint256 tokenAmount,
        uint256 minWeth,
        uint256 deadline
    ) external payable returns (uint256 lpTokens) {
        // 1.批准路由合约使用本代币
        _approve(address(this), address(uniswapRouter), tokenAmount);

        // 2.调用路由添加流动性 (本代币 + ETH, 自动转为 WETH)
        (uint256 tokenAdded, uint256 wethAdded, uint256 lpReceived) = uniswapRouter.addLiquidityETH { value : msg.value } (
            address(this), 
            tokenAmount, 
            tokenAmount, // 最小代币数量 (可根据需求调整滑点)
            minWeth, 
            msg.sender, // LP 代币接受地址
            deadline
        );

        // 3.退还多余的本代币 (如果有)
        uint256 remainingTokens = tokenAmount - tokenAdded;
        if(remainingTokens > 0) {
            _transfer(address(this), msg.sender, remainingTokens);
        }

        emit LiquidityAdded(msg.sender, tokenAdded, wethAdded, lpReceived);
        return lpReceived;
    }

    /**
     * @dev 移除流动性 (本代币 + WETH)
     * @param lpAmount 要销毁的 LP 代币数量
     * @param minToken 最小可接受的本代币数量
     * @param minWeth 最小可接受的 WETH 数量
     * @param deadline 交易截止时间戳
     */
    function removeLiquidity(
        uint256 lpAmount,
        uint256 minToken,
        uint256 minWeth,
        uint256 deadline
    ) external returns (uint256 tokenReceived, uint256 wethReceived) {
        // 1. 获取交易对合约
        IUniswapV2Pair pair = IUniswapV2Pair(uniswapPair);

        // 2.批准路由合约使用 LP 代币
        pair.transferFrom(msg.sender, address(this), lpAmount);
        pair.approve(address(uniswapRouter), lpAmount);

        // 3.调用路由移除流动性
        (uint256 tokenOut, uint256 wethOut) = uniswapRouter.removeLiquidityETH(
            address(this), 
            lpAmount, 
            minToken, 
            minWeth, 
            msg.sender, // 代币接受地址 
            deadline
        );

        emit LiquidityRemoved(msg.sender, tokenOut, wethOut, lpAmount);
        return (tokenOut, wethOut);
    }

    /**
     * @dev 更新 Uniswap 路由地址 (仅 owner)
     * @param newRouter 新的路由合约地址
     */
    function setUniswapRouter(address newRouter) external onlyOwner {
        require(newRouter != address(0), "Invalid router address");
        uniswapRouter = IUniswapV2Router02(newRouter);
    }

    /**
     * @dev 检查交易限制
     * @param sender 发送者地址
     * @param amount 交易金额
     */
    function _checkTxLimits(address sender, uint256 amount) internal {
        // 豁免地址不受限制
        if (isExempt[sender]) return;

        // 检查单笔交易额度限制
        uint256 maxTxAmount = (totalSupply() * maxTxPercent) / 100;
        require(amount <= maxTxAmount, "Transaction exceeds max amount");

        // 检查每日交易次数限制
        uint256 currentDay = block.timestamp / 86400; // 每天86400秒
        if (lastTxDay[sender] != currentDay) {
            // 新的一天，重置交易次数
            dailyTxCount[sender] = 1;
            lastTxDay[sender] = currentDay;
        } else {
            // 同一天，检查次数限制
            require(dailyTxCount[sender] < maxDailyTx,"Exceeded daily transaction limit");
            dailyTxCount[sender]++;
        }
    }

    /**
     * @dev 计算税费并分配
     * @param amount 交易金额
     * @return 扣除税费后的实际转账金额
     */
    function _calculateAndDistributeTax(uint256 amount) internal returns (uint256) {
        // 豁免地址无税费
        if (isExempt[msg.sender]) return amount;

        // 计算总税费
        uint256 tax = (amount * taxRate) / 100;
        uint256 transferAmount = amount - tax;

        // 计算各部分分配金额
        uint256 treasuryAmount = (tax * treasuryPercent) / 100;
        uint256 burnAmount = (tax * burnPercent) / 100;
        uint256 marketingAmount = tax - treasuryAmount - burnAmount;

        // 分配税费
        if (treasuryAmount > 0 && treasuryAddress != address(0)) {
            _transfer(msg.sender, treasuryAddress, treasuryAmount);
        }
        if (burnAmount > 0) {
            _burn(msg.sender, burnAmount);
        }
        if (marketingAmount > 0 && marketingAddress != address(0)) {
            _transfer(msg.sender, marketingAddress, marketingAmount);
        }

        return transferAmount;
    }

    /**
     * @dev 重写transfer函数，添加税费和交易限制
     */
    function transfer(address to, uint256 amount) public override returns (bool) {
        // 检查交易限制
        _checkTxLimits(msg.sender, amount);
        
        // 计算税费并分配
        uint256 transferAmount = _calculateAndDistributeTax(amount);
        
        // 执行实际转账
        _transfer(_msgSender(), to, transferAmount);
        return true;
    }

    /**
     * @dev 重写transferFrom函数，添加税费和交易限制
     */
    function transferFrom(
        address from,
        address to,
        uint256 amount
    ) public override returns (bool) {
        // 检查交易限制
        _checkTxLimits(from, amount);
        
        // 计算税费并分配
        uint256 transferAmount = _calculateAndDistributeTax(amount);

        // 扣减授权额度
        uint256 currentAllowance = allowance(from, _msgSender());
        require(currentAllowance >= amount, "ERC20: insufficient allowance");
        unchecked {
            _approve(from, _msgSender(), currentAllowance - amount);
        }

        // 执行实际转账
        _transfer(from, to, transferAmount);
        return true;
    }

    /**
     * @dev 管理函数：更新税率 (仅所有者)
     * @param newTaxRate 新税率 (百分比)
     */
    function setTaxRate(uint256 newTaxRate) external onlyOwner {
        require(newTaxRate <= 30, "Tax rate too high (max 30%)");
        taxRate = newTaxRate;
    }

    /**
     * @dev 管理函数：更新税费分配比例 (仅所有者)
     * @param newTreasuryPercent 国库分配比例
     * @param newBurnPercent 销毁分配比例
     * @param newMarketingPercent 营销分配比例
     */
    function setDistributionPercent(
        uint256 newTreasuryPercent,
        uint256 newBurnPercent,
        uint256 newMarketingPercent
    ) external onlyOwner {
        require(
            newTreasuryPercent + newBurnPercent + newMarketingPercent == 100,
            "Distribution must sum to 100%"
        );
        treasuryPercent = newTreasuryPercent;
        burnPercent = newBurnPercent;
        marketingPercent = newMarketingPercent;
    }

    /**
     * @dev 管理函数：更新国库地址 (仅所有者)
     * @param newTreasuryAddress 新国库地址
     */
    function setTreasuryAddress(address newTreasuryAddress) external onlyOwner {
        require(newTreasuryAddress != address(0), "Invalid address");
        treasuryAddress = newTreasuryAddress;
    }

    /**
     * @dev 管理函数：更新营销地址 (仅所有者)
     * @param newMarketingAddress 新营销地址
     */
    function setMarketingAddress(address newMarketingAddress) external onlyOwner {
        require(newMarketingAddress != address(0), "Invalid address");
        marketingAddress = newMarketingAddress;
    }

    /**
     * @dev 管理函数：更新单笔最大交易比例 (仅所有者)
     * @param newMaxTxPercent 新的最大交易比例
     */
    function setMaxTxPercent(uint256 newMaxTxPercent) external onlyOwner {
        require(newMaxTxPercent <= 10, "Max tx too high (max 10%)");
        maxTxPercent = newMaxTxPercent;
    }

    /**
     * @dev 管理函数：更新每日最大交易次数 (仅所有者)
     * @param newMaxDailyTx 新的每日最大交易次数
     */
    function setMaxDailyTx(uint256 newMaxDailyTx) external onlyOwner {
        require(newMaxDailyTx > 0, "Max daily tx must be positive");
        maxDailyTx = newMaxDailyTx;
    }

    /**
     * @dev 管理函数：添加豁免地址 (仅所有者)
     * @param addr 要豁免的地址
     */
    function addExemptAddress(address addr) external onlyOwner {
        isExempt[addr] = true;
    }

    /**
     * @dev 管理函数：移除豁免地址 (仅所有者)
     * @param addr 要移除豁免的地址
     */
    function removeExemptAddress(address addr) external onlyOwner {
        isExempt[addr] = false;
    }
}