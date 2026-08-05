// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {MerkleProof} from "./MerkleProof.sol";

/**
 * @title DeFiLaunchpad
 * @notice 代币发售合约 — 支持 MerkleProof 白名单 + 多阶段销售 + 募资退款
 *
 * 状态机流转：
 *   Pending → Whitelist → Public → Ended → Success/Refunding
 *
 * 为什么继承 ReentrancyGuard？
 *   → refund() 和 claimTokens() 都涉及 ETH/Token 转出，必须防重入
 *
 * 为什么继承 Ownable？
 *   → 管理员需要控制阶段切换、设置白名单根、提取资金
 *   → Ownable 提供 onlyOwner modifier，通用的访问控制方案
 */
contract DeFiLaunchpad is ReentrancyGuard, Ownable {
    // 为什么用 SafeERC20 而不是直接调 IERC20？
    // → 某些 ERC20 代币（如 USDT）transfer 不返回 bool，直接调会 revert
    // → SafeERC20 包装了安全调用，兼容所有 ERC20 实现
    using SafeERC20 for IERC20;

    // ============ 枚举：销售阶段 ============

    // 为什么用 enum 而不是 uint8 常量？
    // → Solidity 编译器会自动做范围检查（不会出现 stage=99 这种非法值）
    // → 代码可读性远超数字：require(stage == SaleStage.Public) 一目了然
    enum SaleStage {
        Pending,    // 0 — 等待开始（管理员设置参数）
        Whitelist,  // 1 — 白名单轮（需 MerkleProof，低价格）
        Public,     // 2 — 公开轮（任何人可买，价格更高）
        Ended,      // 3 — 销售结束（到达硬顶或手动结束）
        Success,    // 4 — 达到软顶，项目方可提取资金
        Refunding   // 5 — 未达软顶，用户可退款
    }

    // ============ 白名单数据结构 ============

    // 为什么白名单信息用 struct？
    // → 每个白名单地址除了"是否在名单中"，还有"分配额度"
    // → struct 把相关信息打包在一起，比分散的 mapping 更清晰
    struct WhitelistInfo {
        bytes32 root;           // Merkle Root（链上只存 32 字节！）
        uint256 price;          // 白名单轮价格（ETH per token，18 位精度）
        uint256 maxAllocation;  // 单人最大购买额（ETH 计价）
    }

    // ============ 状态变量 ============

    // --- 销售代币 ---
    // 为什么要存 saleToken 而不是 hardcode？
    // → 部署时传入，同一份合约代码可以发售任意 ERC20 代币，可复用
    IERC20 public immutable saleToken;

    // --- 阶段控制 ---
    SaleStage public currentStage;

    // --- 白名单 ---
    WhitelistInfo public whitelistInfo;

    // --- 公开轮 ---
    // 为什么公开轮价格通常比白名单高？
    // → 白名单承担了早期风险（不知道会不会成团），所以给折扣
    // → 公开轮是确定性购买，价格溢价
    uint256 public publicPrice;

    // --- 资金目标 ---
    // hardCap：最多筹多少 ETH（到了立刻停止）
    // 为什么是 ETH 上限而不是代币数量上限？
    // → ETH 是计价单位，"筹 100 ETH"比"卖 100 万代币"更直观
    // → 而且代币价格可能不同（白名单 vs 公开），ETH 上限是统一约束
    uint256 public immutable hardCap;

    // softCap：最低筹多少 ETH（达不到就退款）
    // 为什么不是 immutable？→ 理论上 softCap 一旦设定就不该变，这里用 immutable
    uint256 public immutable softCap;

    // --- 资金追踪 ---
    // totalRaised：已筹 ETH 总量（白名单轮 + 公开轮）
    // 为什么单独维护一个变量而不是每次遍历 contributions？
    // → 遍历 mapping 是 O(n)，gas 爆炸
    // → 每次购买累加 totalRaised，O(1) 就能判断是否到 hardCap
    uint256 public totalRaised;

    // contributions：每个用户的贡献（ETH）
    // 为什么用 mapping 存？
    // → O(1) 查询，退款时需要知道每人该退多少
    mapping(address => uint256) public contributions;

    // whitelistPurchased：白名单轮中每个地址已购买的 ETH 数量
    // 为什么和白名单轮分开 tracking？
    // → 白名单有个人上限（maxAllocation），需要精确追踪每人买了多少
    mapping(address => uint256) public whitelistPurchased;

    // --- 退款标记 ---
    // 为什么需要 refunded 标记？
    // → contributions 在 refund() 中会清零，但清零后无法区分"没参与"和"已退款"
    // → refunded 标记用于 UI 展示和事件记录
    mapping(address => bool) public refunded;

    // ============ 事件 ============

    // 为什么每个操作都要发事件？
    // → 链下服务（Go 后端、The Graph）通过监听事件同步数据
    // → 前端查询购买记录不需要调合约，直接查询事件即可
    // → indexed 参数可被高效过滤（最多 3 个 indexed）
    event StageChanged(SaleStage indexed oldStage, SaleStage indexed newStage);
    event WhitelistSet(bytes32 indexed root, uint256 price, uint256 maxAllocation);
    event TokensPurchased(
        address indexed buyer,
        SaleStage indexed stage,
        uint256 ethAmount,
        uint256 tokenAmount
    );
    event Refunded(address indexed buyer, uint256 amount);
    event FundsWithdrawn(address indexed owner, uint256 amount);

    // ============ 自定义错误（Gas 优化） ============
    // 为什么用自定义 error 而不是 require("string")？
    // → require 的字符串参数会编码进字节码，增大了部署 gas
    // → 自定义 error 只存 4 字节 selector + ABI 编码参数，部署和调用都更省 gas
    // → 而且前端可以用 error selector 做精确的错误匹配
    error InvalidStage(SaleStage current, SaleStage expected);
    error NotWhitelisted(address user);
    error ExceedsAllocation(uint256 requested, uint256 max);
    error ExceedsHardCap(uint256 requested, uint256 remaining);
    error NoContribution();
    error TransferFailed();
    error ZeroAddress();
    error ZeroAmount();

    // ============ 构造函数 ============

    /**
     * @param _saleToken  待发售的 ERC20 代币地址
     * @param _hardCap    硬顶（ETH，含 18 位小数）
     * @param _softCap    软顶（ETH，含 18 位小数）
     *
     * 为什么 saleToken 用 immutable？
     * → 代币地址部署后永不改变，immutable 比 storage 省 2100+ gas 每次读取
     *
     * 为什么检查和传 Ownable(msg.sender)？
     * → Ownable 构造函数把 msg.sender 设为 owner
     */
    constructor(
        IERC20 _saleToken,
        uint256 _hardCap,
        uint256 _softCap
    ) Ownable(msg.sender) {
        // 为什么在构造函数校验参数？→ fail early 原则：部署时发现参数错误立刻回滚，不等到运行时
        if (address(_saleToken) == address(0)) revert ZeroAddress();
        if (_hardCap == 0) revert ZeroAmount();
        // 软顶不能超过硬顶——逻辑上说不通：最低门槛不能高于最高上限
        if (_softCap > _hardCap) revert("Soft cap exceeds hard cap");

        saleToken = _saleToken;
        hardCap = _hardCap;
        softCap = _softCap;
        // 初始阶段：Pending，等管理员设置白名单后启动
        currentStage = SaleStage.Pending;
    }

    // ============ 修饰器 ============

    /**
     * @notice 校验当前阶段
     * @dev 为什么把阶段校验抽成 modifier？→ 多个函数共用同一校验逻辑，避免代码重复
     */
    modifier onlyAtStage(SaleStage _stage) {
        if (currentStage != _stage) revert InvalidStage(currentStage, _stage);
        _;
    }

    // ============ 阶段管理（仅 Owner） ============

    /**
     * @notice 设置白名单参数并启动白名单轮
     * @param _root          Merkle Root（链下根据白名单地址生成）
     * @param _price         白名单轮价格
     * @param _maxAllocation 单人最大购买额
     *
     * 为什么设置参数和启动放在同一个函数？
     * → 原子操作：要么全部设置成功，要么全部失败
     * → 防止出现"设置了 root 但忘了切换阶段"的中间状态
     */
    function setWhitelistAndStart(
        bytes32 _root,
        uint256 _price,
        uint256 _maxAllocation
    ) external onlyOwner onlyAtStage(SaleStage.Pending) {
        if(_root == bytes32(0)) revert("Invalid root");
        if (_price == 0) revert ZeroAmount();
        if (_maxAllocation == 0) revert ZeroAmount();

        whitelistInfo = WhitelistInfo({
            root: _root,
            price: _price,
            maxAllocation: _maxAllocation
        });

        currentStage = SaleStage.Whitelist;
        emit WhitelistSet(_root, _price, _maxAllocation);
        emit StageChanged(SaleStage.Pending, SaleStage.Whitelist);
    }

    /**
     * @notice 启动公开轮
     * @param _price 公开轮价格（通常高于白名单轮）
     *
     * 为什么不在构造函数里设置公开轮价格？
     * → 价格可能需要根据白名单轮的认购情况动态调整
     * → 在启动公开轮时才确定价格更灵活
     */
    function startPublicSale(
        uint256 _price
    ) external onlyOwner onlyAtStage(SaleStage.Whitelist) {
        if (_price == 0) revert ZeroAmount();
        publicPrice = _price;
        currentStage = SaleStage.Public;
        emit StageChanged(SaleStage.Whitelist, SaleStage.Public);
    }

    /**
     * @notice 结束销售（手动或自动触发）
     *
     * 为什么需要手动结束？
     * → 硬顶被触达时可以在购买函数中自动结束
     * → 但管理员也可能需要提前结束（如：时间到期）
     *
     * 为什么结束时自动判断成功/退款？
     * → 根据 totalRaised 是否 >= softCap 自动分流
     * → 减少管理员操作步骤，降低出错可能
     */
    function endSale() external onlyOwner {
        // 只允许在白名单轮或公开轮中结束
        if (currentStage != SaleStage.Whitelist && currentStage != SaleStage.Public) revert InvalidStage(currentStage, SaleStage.Whitelist);

        // 为什么先存 oldStage？
        // → emit StageChanged 需要 old → new，currentStage 马上要变了
        // → 不存的话 emit 里的 old 就是已经更新后的值 — 事件参数就错了
        SaleStage oldStage = currentStage;

        if (totalRaised >= softCap) {
            currentStage = SaleStage.Success;
        } else {
            currentStage = SaleStage.Refunding;
        }
        emit StageChanged(oldStage, currentStage);
    }

    // ============ 白名单购买 ============

    /**
     * @notice 白名单轮购买
     * @param proof      Merkle Proof（链下根据用户地址生成）
     * @param allocation 白名单分配额度（ETH）
     *
     * 为什么 proof 在 calldata？
     * → proof 数组只读不写，calldata 直接从交易数据读取，不复制到 memory，省 gas
     *
     * 为什么 allocation 由用户传入而不是链上存？
     * → 为了证明"这个地址有 X ETH 的配额"，需要把 (address, allocation) 打包进叶子节点
     * → 用户传入 allocation，MerkleProof 验证它确实属于这个地址
     * → 实际项目中 allocation 是链下存证的，链上只验证
     */
    function buyWhitelist(
        bytes32[] calldata proof,
        uint256 allocation
    ) external payable nonReentrant onlyAtStage(SaleStage.Whitelist) {
        // ========== Checks ==========

        // 1. 验证白名单：用 MerkleProof 证明 msg.sender 在白名单中
        // 为什么叶子节点 = keccak256(abi.encodePacked(sender, allocation))？
        // → 把地址和配额打包在一起 hash，既证明"在名单里"又证明"配额是对的"
        // → 没有 allocation 的话，任何人都可以冒用别人的地址抢购
        bytes32 leaf = keccak256(abi.encodePacked(msg.sender, allocation));
        if (!MerkleProof.verify(proof, whitelistInfo.root, leaf)) {
            revert NotWhitelisted(msg.sender);
        }

        uint256 amount = msg.value;

        // 2. 个人上限检查：本次购买 + 已购买 <= 分配额度
        // 为什么用 whitelistPurchased 记录而不是直接读 contributions？
        // → 白名单和公开轮的贡献混在一个 contributions 里，无法区分
        // → whitelistPurchased 专门记录白名单轮的购买
        uint256 newUserTotal = whitelistPurchased[msg.sender] + amount;
        if (newUserTotal > allocation) {
            revert ExceedsAllocation(amount, allocation - whitelistPurchased[msg.sender]);
        }

        // 3. 硬顶检查：已筹 + 本次购买 <= 硬顶
        uint256 newTotal = totalRaised + amount;
        if (newTotal > hardCap) {
            revert ExceedsHardCap(amount, hardCap - totalRaised);
        }

        // 4. 不能买 0
        if (amount == 0) revert ZeroAmount();

        // ========== Effects ==========
        // 为什么 Effects 在 Interactions 之前？→ CEI 模式！防止重入攻击
        // 先更新所有状态变量，再做外部调用
        whitelistPurchased[msg.sender] = newUserTotal;
        contributions[msg.sender] += amount;
        totalRaised = newTotal;

        // ========== Interactions ==========
        // 为什么纯 ETH 购买不需要 transfer？
        // → ETH 已经通过 msg.value 发送到合约了，不存在外部调用
        // → 但如果这里转代币给买家，需要 SafeERC20.safeTransfer

        // 自动结束：达到硬顶立刻停售
        if (newTotal == hardCap) {
            SaleStage oldStage = currentStage;
            if (newTotal >= softCap) {
                currentStage = SaleStage.Success;
            } else {
                currentStage = SaleStage.Refunding;
            }
            emit StageChanged(oldStage, currentStage);
        }

        // 计算应得代币数量并记录
        // 为什么 tokenAmount = ETH / price？
        // → 如果 price = 0.01 ETH/token，那 1 ETH = 100 token
        // → tokenAmount = amount * 1e18 / price（price 和 amount 都是 18 位精度）
        uint256 tokenAmount = amount * 1e18 / whitelistInfo.price;
        emit TokensPurchased(msg.sender, SaleStage.Whitelist, amount, tokenAmount);
    }

    // ============ 公开轮购买 ============

    /**
     * @notice 公开轮购买（任何人可参与）
     *
     * 为什么公开购买逻辑比白名单简单很多？
     * → 不需要 MerkleProof 验证，不需要个人上限
     * → 只需要硬顶检查就够了
     * → 这就是"公开轮"的含义——无门槛
     */
    function buyPublic() external payable nonReentrant onlyAtStage(SaleStage.Public) {
        uint256 amount = msg.value;

        // ========== Checks ==========

        // 硬顶检查
        uint256 newTotal = totalRaised + amount;
        if (newTotal > hardCap) {
            revert ExceedsHardCap(amount, hardCap - totalRaised);
        }

        if (amount == 0) revert ZeroAmount();

        // ========== Effects ==========

        contributions[msg.sender] += amount;
        totalRaised = newTotal;

        // ========== Interactions ==========
        // （同上，纯 ETH 无外部调用）

        // 达到硬顶自动结束
        if (newTotal == hardCap) {
            if (newTotal >= softCap) {
                currentStage = SaleStage.Success;
                emit StageChanged(SaleStage.Public, SaleStage.Success);
            } else {
                currentStage = SaleStage.Refunding;
                emit StageChanged(SaleStage.Public, SaleStage.Refunding);
            }
        }

        uint256 tokenAmount = (amount * 1e18) / publicPrice;
        emit TokensPurchased(msg.sender, SaleStage.Public, amount, tokenAmount);
    }

    // ============ 退款 ============

    /**
     * @notice 退款（仅在未达软顶时可用）
     *
     * 为什么这是最需要防重入的函数？
     * → 涉及 ETH 转出（msg.sender.call{value}），是最危险的交互
     * → 攻击者可以在 fallback/receive 中重新调用 refund() 反复取钱
     *
     * CEI 模式详解：
     *   Checks:   阶段 = Refunding + 用户有贡献 + 还没退过
     *   Effects:  先清零 contributions（即使重入，第二层进来 contribution=0，不执行后续）
     *   Interactions: 最后才转账
     */
    function refund() external nonReentrant onlyAtStage(SaleStage.Refunding) {
        uint256 amount = contributions[msg.sender];

        // ========== Checks ==========
        if (amount == 0) revert NoContribution();

        // ========== Effects ==========
        // 为什么先清零再转账？
        // → 如果攻击者合约的 receive() 重入 refund()，contributions 已经是 0
        // → require(amount > 0) 会拒绝第二次调用
        // → 这是 CEI 模式的精髓！状态先变，攻击面就消失了
        contributions[msg.sender] = 0;
        refunded[msg.sender] = true;

        // ========== Interactions ==========
        // 为什么用 .call{value} 而不是 .transfer？
        // → .transfer 只给 2300 gas，如果用户是合约钱包（需要更多 gas），transfer 会失败
        // → .call 发送所有剩余 gas，兼容合约钱包
        // → 但 .call 需要手动检查返回值
        (bool success, ) = msg.sender.call{value:amount}("");
        if (!success) revert TransferFailed();

        emit Refunded(msg.sender, amount);
    }

    // ============ 资金提取（仅 Owner，达软顶后） ============

    /**
     * @notice 项目方提取已筹 ETH（仅在达软顶后可用）
     *
     * 为什么需要这个函数？
     * → 合约收到 ETH 后不能自动转给项目方（没有自动转账机制）
     * → 需要项目方主动调用提取
     *
     * 为什么用 nonReentrant？
     * → 虽然只转给 owner，但如果 owner 是多签合约，它也可能被攻击
     * → 加 nonReentrant 是最佳实践
     */
    function withdrawFunds() external onlyOwner nonReentrant onlyAtStage(SaleStage.Success) {
        uint256 balance = address(this).balance;
        if (balance == 0) revert ZeroAmount();

        // 为什么不需要 Effects 更新状态变量？
        // → balance 是 address(this).balance，转账后自动减少
        // → totalRaised 保持不变，用于统计（已筹金额和历史记录）

        (bool success, ) = msg.sender.call{value: balance}("");
        if (!success) revert TransferFailed();

        emit FundsWithdrawn(msg.sender, balance);
    }

    /**
     * @notice 项目方提取未售出的代币（或全部代币，根据业务逻辑）
     * @dev 销售结束后，合约里剩余的代币可以退回给项目方
     */
    function withdrawUnsoldTokens() external onlyOwner {
        // 允许在成功或退款后提取剩余代币
        if (currentStage != SaleStage.Success && currentStage != SaleStage.Refunding) revert("Sale not ended");

        uint256 balance = saleToken.balanceOf(address(this));
        if (balance == 0) revert ZeroAmount();

        // SafeERC20.safeTransfer —— 兼容所有 ERC20 实现
        saleToken.safeTransfer(owner(), balance);
    }

    // ============ 查询函数 ============

    /**
     * @notice 查询当前阶段剩余可筹额度
     * @return 剩余 ETH 额度
     */
    function remainingCap() external view returns (uint256) {
        if (totalRaised >= hardCap) return 0;
        return hardCap - totalRaised;
    }

    /**
     * @notice 检查地址是否已到软顶
     */
    function isSoftCapReached() external view returns (bool) {
        return totalRaised >= softCap;
    }
}