// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/interfaces/IERC4626.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/Create2.sol";
import "./DeFiVault.sol";

// ============ 工厂合约 ============

/// @title VaultFactory — 收益金库工厂
/// @notice 一键部署标准化的 ERC4626 金库，追踪所有已部署的金库
/// @dev 使用 Ownable 管理协议参数（部署费、协议费分成），追踪金库列表
contract VaultFactory is Ownable {
    // ============ 错误定义 ============

    // 为什么把错误分门别类？
    // → 前端调用工厂时，不同的 revert 原因需要不同的 UI 反馈：
    //    Factory__DeploymentPaused → 显示"金库部署已暂停"
    //    Factory__FeeTooLow → 显示"部署费不足，请补充"
    error Factory__DeploymentPaused();          // 金库部署功能已暂停
    error Factory__FeeTooLow();                 // 部署费不足
    error Factory__ZeroAddress();               // 零地址参数
    error Factory__VaultAlreadyExists();        // 该名称的金库已存在（防重名）
    error Facotry__FeeTransferFailed();         // 部署费转账失败

    // ============ 事件定义 ============

    // 为什么部署事件记录这些字段？
    // → 前端需要：vault address（跳转详情页）、asset（筛选"所有 USDC 金库"）
    //    name（展示金库名称）、deployer（谁创建的）、fee（统计协议收入）
    event VaultDeployed(
        address indexed vault,      // indexed → 可按金库地址过滤
        address indexed asset,      // indexed → 可按底层资产过滤
        string name,                // 金库名称
        string symbol,              // 份额代币符号
        address indexed deployer,   // indexed → 可按部署者过滤
        uint256 fee                 // 部署费（协议收入）
    );

    // 为什么用单独的事件而不是在 VaultDeployed 里塞更多字段？
    // → 事件字段越多 gas 越贵。把不常查询的字段拆分到独立事件
    //    前端按需监听——一般只看 VaultDeployed，需要详情才查 VaultParamsSet
    event VaultParamsSet(
        address indexed vault,
        uint256 performanceFee,     // 绩效费比例
        address yieldStrategy       // 初始收益策略
    );

    // ============ 状态变量 ============

    // 为什么用 mapping 按名字查地址？
    // → 防止同名的金库让用户混淆（如两个 "USDC High Yield Vault"）。
    //    映射 name → address(true = 已占用)
    mapping(string => bool) public vaultNameExists;

    // 为什么用 array 存所有金库地址？
    // → 前端需要展示"所有已部署的金库列表"，array 遍历比 mapping 遍历更简单
    //    但 array 的 push 有 gas 成本——如果预期金库数量大，改用 The Graph 索引更好
    address[] public allVaults;

    // 为什么用 mapping 单独追踪每个资产的第一个金库？
    // → 这是一个增值功能——前端可以快速查询"USDC 相关的金库有哪些"
    //    通过 first + next 链表结构实现，每新增一个 USDC 金库就插入链表头部
    mapping(address => address) public firstVaultByAsset;   // asset → 该资产的第一个金库
    mapping(address => address) public nextVault;           // 当前金库 → 同资产的下一个金库（链表）

    // 为什么部署费用 0.01 ETH？
    // → 纯学习目的，设一个象征性的费用，预防垃圾部署。
    //    生产环境中通常不收部署费（更多金库 = 更多 TVL = 更多绩效费）
    uint256 public deployFee = 0.01 ether;

    // 为什么暂停开关在这里也用？
    // → 和金库里一样——如果发现 DeFiVault 有漏洞，暂停新部署同时通知已部署金库暂停存款
    bool public deploymentPaused;

    // ============ 构造函数 ============

    constructor() Ownable(msg.sender) {
        // Factory 的 owner 就是部署者自己
        // 什么都不用做——状态变量都有默认值
    }

    // ============ 核心功能：部署金库 ⭐ ============

    /// @notice 部署一个新的收益金库
    /// @param _asset 底层 ERC20 资产地址
    /// @param _name 金库名称（唯一，如 "USDC High Yield Vault"）
    /// @param _symbol 份额代币符号（如 "dvUSDC"）
    /// @param _yieldStrategy 初始收益策略地址（可以后续修改）
    /// @return vault 新部署的金库地址
    // 为什么返回 vault 地址？
    // → 调用者（前端/脚本）部署后需要立即知道地址，用于后续交互或事件监听
    function deployVault(
        IERC20 _asset,
        string memory _name,
        string memory _symbol,
        address _yieldStrategy
    ) external payable returns (address vault) {
        // ========== 检查阶段 (Checks) ==========

        // 检查 1：部署没被暂停
        if (deploymentPaused) revert Factory__DeploymentPaused();

        // 检查 2：部署费够了
        // 为什么用 msg.value < deployFee 而不是 msg.value == deployFee？
        // → 多付一点没关系（用户可以多给），但少付不行
        if (msg.value < deployFee) revert Factory__FeeTooLow();

        // 检查 3：参数不为空
        // 为什么每个参数都要检查？
        // → address(0) 的 asset 会让所有 deposit 都 revert（无法转账）
        //    name 为空会让金库在 Etherscan 上不可读
        if(address(_asset) == address(0)) revert Factory__ZeroAddress();
        if(_yieldStrategy == address(0)) revert Factory__ZeroAddress();
        if(bytes(_name).length == 0 || bytes(_symbol).length == 0) revert Factory__ZeroAddress();

        // 检查 4：名称唯一
        if(vaultNameExists[_name]) revert Factory__VaultAlreadyExists();

        // ========== 状态变更 (Effects) ==========

        // 为什么先改状态再部署？
        // → CEI 模式：先更新 Factory 内部状态，再执行外部调用（new DeFiVault）
        //    如果 new DeFiVault 失败，整个交易回滚，mapping 也会回退
        vaultNameExists[_name] = true;

        // ========== 外部交互 (Interactions) ==========

        // 为什么先让 Factory 成为临时 owner？
        // → Factory 需要在部署后调用 setYieldStrategy（onlyOwner 函数）
        //    但如果直接设 msg.sender 为 owner，Factory 就没权限了
        //    策略：Factory 临时当 owner → 配置参数 → 移交所有权给实际部署者
        vault = address(
            new DeFiVault(
                _asset,     // 底层资产
                _name,      // 金库名称
                _symbol,    // 份额符号
                address(this) // Factory 临时做 owner（稍后会移交）
            )
        );

        // 存储追踪
        allVaults.push(vault);

        // ========== 链表维护 ==========

        // 为什么用链表而不是 mapping(address => address[])？
        // → Solidity 不支持 mapping 嵌套动态数组（gas 太高且实现复杂）
        //    链表是 gas 最低的链上索引方案：每个元素只多存 20 字节
        // 头插法：新金库 → 旧头 → ...
        // 例如：已有 USDC 金库列表 [A → B]（firstVaultByAsset[USDC] = A, nextVault[A] = B）
        //      新增 C 后：C → A → B（firstVaultByAsset[USDC] = C, nextVault[C] = A）
        if (firstVaultByAsset[address(_asset)] == address(0)) {
            // 该资产的第一个金库——直接记录为链表头
            firstVaultByAsset[address(_asset)] = vault;
            // nextVault[vault] = address(0) 已经是默认值，不需要显式设置
        } else {
            // 头插法：新金库成为新的链表头
            // 新节点的 next = 旧头
            nextVault[vault] = firstVaultByAsset[address(_asset)];
            // 链表头 = 新节点
            firstVaultByAsset[address(_asset)] = vault;
        }

        // ========== 退款 ==========

        // 如果用户多付了部署费，退回多余部分
        // 为什么用 > 而不是 >= ？
        // → 如果正好付够，不需要退；多付了才退
        if (msg.value > deployFee) {
            // 为什么用 call{value: refund} 而不是 transfer？
            // → transfer 有 2300 gas 限制，如果接收方是合约且 fallback 逻辑复杂会失败
            //    call 不限制 gas，更安全兼容
            (bool refunded, ) = msg.sender.call{value: msg.value - deployFee}("");
            // 退款失败不阻塞部署——但这里其实可以吞掉错误，因为部署已经成功了
            // 不过最好还是检查一下
            if (!refunded) revert Facotry__FeeTransferFailed();
        }

        emit VaultDeployed(vault, address(_asset), _name, _symbol, msg.sender, deployFee);
        emit VaultParamsSet(vault, 0.2e18, _yieldStrategy);

        // 部署后立即设置收益策略（如果指定了非零地址）
        // 为什么在这里设置而不是在构造函数里？
        // → DeFiVault 的构造函数不接收 yieldStrategy 参数（简化构造函数）
        //    部署后通过 setYieldStrategy 设置更灵活
        //    注意：此时 Factory 还是 vault 的临时 owner，所以 onlyOwner 能通过
        if(_yieldStrategy != address(0)) {
            DeFiVault(vault).setYieldStrategy(_yieldStrategy);
        }

        // 移交金库所有权给实际部署者
        // 为什么最后移交？
        // → 所有需要 onlyOwner 权限的配置都已完成（setYieldStrategy）
        //    移交后 msg.sender 拥有金库的完整管理权
        DeFiVault(vault).transferOwnership(msg.sender);
    }

    // ============ 查询函数 ============

    /// @notice 获取所有已部署的金库数量
    // 为什么用函数包装而不是直接读取 public array？
    // → 虽然 Solidity 的 public array 会自动生成 getter，
    //    但 getter 需要传 index，前端需要先知道 length 才能遍历
    function getVaultCount() external view returns (uint256) {
        return allVaults.length;
    }

    /// @notice 获取所有金库地址（用于前端一次性拉取）
    // 为什么返回整个 array？
    // → 前端列出"所有金库"，一次调用拿到全部地址，不需要 N 次 RPC 请求
    function getAllVaults() external view returns (address[] memory) {
        return allVaults;
    }

    /// @notice 获取某个资产的所有金库地址（分页版本）
    // 为什么需要分页？
    // → 如果某资产有 1000 个金库，不加 limit 返回整个列表 gas 可能超 block limit
    //    分页遍历是最安全的做法
    /// @param _asset 底层资产地址
    /// @param _limit 返回的最大数量（0 = 无限制）
    /// @return vaults 该资产的金库地址数组
    function getVaultsByAsset(
        address _asset,
        uint256 _limit
    ) external view returns (address[] memory vaults) {
        // 第一步：遍历链表统计数量
        uint256 count = 0;
        address current = firstVaultByAsset[_asset];

        // 为什么用临时变量 current 而不是直接修改 firstVaultByAsset？
        // → firstVaultByAsset 是 storage 引用，直接修改会改变 mapping 值（虽然 view 函数不允许）
        //    用临时变量更安全，且不会混淆
        while (current != address(0)) {
            count++;
            current = nextVault[current]; // 移到下一个节点
        }

        // 第二步：分配结果数组
        // 为什么先统计数量再分配数组？
        // → Solidity 不支持动态 push 到 memory array（memory array 长度必须编译时确定或在分配时指定）
        //    所以需要先知道数量，再一次性分配正确大小的数组
        uint256 resultSize = (_limit == 0 || _limit > count) ? count : _limit;
        vaults = new address[](resultSize);

        // 第三步：填充数组
        current = firstVaultByAsset[_asset];    // 重置到链表头
        for (uint256 i = 0; i < resultSize; i++) {
            vaults[i] = current;
            current = nextVault[current]; // 遍历链表
        }
    }

    // ============ 管理员函数 ============

    /// @notice 设置部署费
    // 为什么设置上限?
    // → 不设上限的话，owner 可以恶意把费用设为 100 ETH，锁死所有部署
    function setDeployFee(uint256 _fee) external onlyOwner {
        if (_fee > 1 ether) revert();   // 限 1 ETH
        deployFee = _fee;
    }

    /// @notice 暂停/恢复新金库部署
    function setDeploymentPaused(bool _paused) external onlyOwner {
        deploymentPaused = _paused;
    }

    /// @notice 提取协议收入（部署费的 ETH）
    // 为什么需要这个函数？
    // → 所有 deployVault 的部署费都在 Factory 合约里，需要转给 owner
    function withdrawFees() external onlyOwner {
        // 为什么用 call 而不是 transfer？
        // → transfer 只有 2300 gas，如果 owner 是合约可能失败
        (bool sent, ) = owner().call{value: address(this).balance}("");
        if (!sent) revert Facotry__FeeTransferFailed();
    }

    // ============ 接收 ETH ============

    // 为什么需要 receive()？
    // → 如果 owner 或其他合约误转 ETH 到 Factory，没有 receive() 会直接 revert
    //    这在 DeFi 组合时很常见（如另一个合约把 Factory 当作收款方）
    receive() external payable {}
}