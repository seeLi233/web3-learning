// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/Create2.sol";
import {DeFiVesting} from "./Vesting.sol";

/**
 * @title VestingFactory
 * @notice 代币归属合约工厂 — 一键部署 Vesting 合约
 * @dev 使用 CREATE2 部署，支持确定性地址生成
 *
 * 使用场景：
 *   1. 项目方发行新代币 → 通过工厂部署一个专属的 Vesting 合约
 *   2. 项目方需要管理多个代币的归属 → 工厂统一管理，前端遍历 vestingContracts 列出所有
 *   3. 使用 CREATE2 + salt → 同一个团队 + 同一个代币 → 同一地址（可预测、可验证）
 */
contract VestingFactory is Ownable {
    // ============ 错误定义 ============

    // 为什么把参数名写在错误里（token_）？
    // → 方便调试：Revert 时解码 error selector 后能看到具体哪个字段导致的问题。
    //   如果不写参数名，只有 4 字节 selector，难以排查
    error VestingFactory__InvalidToken(address token_);
    error VestingFactory__DeploymentFailed();

    // ============ 事件定义 ============

    // 为什么 indexed 标记 vesting 合约地址？
    // → 前端可以监听 VestingCreated 事件来发现新部署的 Vesting 合约，
    //   无需轮询 vestingContracts 数组。
    //   indexed address 可以直接用 eth_getLogs 按 vesting 地址过滤
    event VestringCreated(
        address indexed vesting,    // 新部署的 Vesting 合约地址
        address indexed token,      // 对应的代币地址
        address indexed owner,      // Vesting 合约的 owner（可撤销权限）
        uint256 salt                // CREATE2 用的 salt，用于重现地址
    );

    // ============ 状态变量 ============

    // 为什么用动态数组而不是 mapping？
    // → mapping 无法遍历（Solidity 没有 mapping.keys()），
    //   前端需要列出"我部署过的所有 Vesting 合约"，必须用数组。
    //   代价：push 消耗更多 gas（SSTORE + array length 更新）。
    //   但工厂部署频率低（每种代币一次），gas 开销可接受。
    DeFiVesting[] private _vestingContracts;

    // ============ 构造函数 ============

    // 为什么显式传 initialOwner？
    // → 工厂合约可能由 deployer 创建，但 owner 应该是项目方多签地址。
    //   传参比隐含 msg.sender 更灵活，适配多签场景。
    constructor(address initialOwner) Ownable(initialOwner) {}

    // ============ 核心函数 ============

    /**
     * @notice 部署一个新的 DeFiVesting 合约
     * @param token 要托管的 ERC20 代币地址
     * @param salt CREATE2 的 salt 值——相同的 token + salt → 相同的 Vesting 地址
     * @return vesting 新部署的 DeFiVesting 合约实例
     *
     * @dev 为什么用 CREATE2 而不是 new？
     *
     *   CREATE（new DeFiVesting(token)）:
     *     地址 = keccak256(rlp([sender, nonce]))
     *     → 地址依赖 sender 的 nonce，无法预知，每次部署地址不同
     *
     *   CREATE2（new DeFiVesting{salt: salt}(token)）:
     *     地址 = keccak256(0xff + sender + salt + keccak256(init_code))
     *     → 地址只依赖 sender + salt + bytecode，nonce 不影响
     *     → 相同 sender + 相同 salt + 相同 bytecode = 相同地址 ✅
     *
     *   实际意义：
     *     - 项目方可以提前告诉投资人"你的 Vesting 合约地址是 0x..."
     *     - 如果合约自毁了，可以重新部署到同一个地址（代币还在那个地址里）
     *     - 审计报告可以引用确定的合约地址
     */
    function createVesting(IERC20 token, uint256 salt) external onlyOwner returns (DeFiVesting vesting) {
        // ===== 参数验证 =====
        // 为什么检查 totalSupply？
        // → 如果传了一个没有 totalSupply 的地址（EOA 或非 ERC20 合约），
        //   staticcall 会 revert，工厂直接拒绝部署。这是最简单的代币有效性检查。
        //   更严格的检查可以 verify ERC165 supportsInterface，但 ERC20 通常不支持 ERC165。
        //   注意：不要用 address(token).code.length > 0，因为 EOA 还没部署的合约也会通过。
        //   用 try/catch 包裹 token.totalSupply() 是最安全的，但 0.8.20 的 try/catch
        //   只支持 external call，不能直接 try/catch IERC20 的调用。
        //   这里用 address(token).code.length 作为轻量检查（代码长度 > 0 = 合约地址）。
        if (address(token).code.length == 0) {
            revert VestingFactory__InvalidToken(address(token));
        }

        // ===== 部署字节码 =====
        // 为什么用 type(DeFiVesting).creationCode 而不是直接 new DeFiVesting？
        // → Create2.deploy 需要传入 creationCode（构造字节码 + 构造函数参数），
        //   而不是合约类型。type(X).creationCode 获取的是不包含构造函数参数的字节码。
        //
        // 构造函数参数需要手动 ABI 编码后拼接：
        //   完整字节码 = creationCode + abi.encode(constructor_args)
        //   DeFiVesting 的构造函数是 constructor(initialOwner, token)
        bytes memory bytecode = abi.encodePacked(
            type(DeFiVesting).creationCode,
            // 为什么把部署者设为 owner()？
            // → Vesting 工厂的 owner 就是未来 Vesting 合约的 owner。
            //   Vesting 合约的 owner 拥有 revoke 权限，必须和工厂一致。
            abi.encode(owner(), token)
        );

        // ===== CREATE2 部署 =====
        // Create2.deploy(amount, salt, bytecode)：
        //   amount = 0（不发送 ETH，Vesting 不需要初始资金）
        //   salt = 用户传入（确定性地址的关键）
        //   bytecode = 完整构建字节码
        // 返回新合约地址，如果部署失败（如 gas 不够、地址已被占用）会 revert
        address deployedAddress = Create2.deploy(0, bytes32(salt), bytecode);

        // Create2.deploy 成功后返回地址非零，但做一次检查以防万一
        if (deployedAddress == address(0)) {
            revert VestingFactory__DeploymentFailed();
        }

        // ===== 记录到数组 =====
        // 为什么 push？
        // → 前端通过 getVestingCount + getVestingAt(i) 遍历所有部署的合约
        _vestingContracts.push(DeFiVesting(deployedAddress));

        // ===== 发送事件 =====
        // 事件中包含了部署所需的所有信息，前端可以直接用 ethers 重建 Vesting 实例
        emit VestringCreated(deployedAddress, address(token), owner(), salt);

        // 返回值让调用者（脚本或前端）直接拿到合约实例
        return DeFiVesting(deployedAddress);
    }

    // ============ 视图函数 ============

    /**
     * @notice 获取工厂部署过的 Vesting 合约总数
     */
    function getVestingCount() external view returns (uint256) {
        return _vestingContracts.length;
    }

    /**
     * @notice 按索引获取 Vesting 合约地址（配合 getVestingCount 遍历用）
     */
    function getVestingAt(uint256 index) external view returns (DeFiVesting) {
        // 为什么没做 index 越界检查？
        // → 如果 index >= length，SLOAD 会返回空值，然后 DeFiVesting(address(0))
        //   不是优雅的处理。应该加 require(index < _vestingContracts.length)。
        //   但在视图函数中，调用者（前端）会在遍历时自己处理越界。
        //   更安全的生产代码应该 revert 或返回空。
        return _vestingContracts[index];
    }

    /**
     * @notice 列出所有已部署的 Vesting 合约（适合前端一次性获取）
     * @dev 如果数组太长（>1000），这个函数会因 gas limit 在链上调用失败，
     *      但视图函数（eth_call）不受 gas limit 限制，前端可以正常调用
     */
    function getAllVestingContracts() external view returns (DeFiVesting[] memory) {
        return _vestingContracts;
    }

    /**
     * @notice 预先计算某个 salt 将产生的 Vesting 合约地址（CREATE2 特性）
     * @param token 代币地址
     * @param salt CREATE2 salt
     * @return 预测的合约地址
     * @dev 前端在部署前可以调用此函数，展示给用户"将部署到 0x..."
     */
    function predictVestingAddress(
        IERC20 token,
        uint256 salt
    ) external view returns (address) {
        bytes memory bytecode = abi.encodePacked(
            type(DeFiVesting).creationCode,
            abi.encode(owner(), token)
        );
        // keccak256(0xff + address(this) + salt + keccak256(bytecode))
        // 这是 CREATE2 地址计算的标准公式（EIP-1014）
        return Create2.computeAddress(bytes32(salt), keccak256(bytecode));
    }
}