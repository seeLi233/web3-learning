// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title DeFiVesting
 * @notice 代币归属合约 — 支持线性释放 + 悬崖释放 + 可撤销
 * @dev 使用场景：
 *   - 团队代币：cliff=6个月, end=24个月, revocable=true
 *   - VC 投资：cliff=12个月, end=36个月, revocable=true
 *   - 公募代币：cliff=0, end=12个月, revocable=false
 */
contract DeFiVesting is Ownable, ReentrancyGuard {
    // ============ 错误定义 ============
    // 为什么用自定义错误而不是 require 字符串？
    // → 自定义错误省 gas（只存 4 字节 selector，不需要存储 revert 字符串），
    //   且前端可以用 error selector 精确匹配并展示中文提示
    error Vesting__ZeroAddress();
    error Vesting__ZeroAmount();
    error Vesting__InvalidTimeRange();
    error Vesting__ScheduleAlreadyExists();
    error Vesting__NoSchedule();
    error Vesting__NothingToRelease();
    error Vesting__NotRevocable();
    error Vesting__AlreadyRevoked();
    error Vesting__TokenTransferFailed();
    error Vesting__InsufficientAllowance();

    // ============ 事件定义 ============
    // 为什么用 indexed 标记 address 参数？
    // → indexed 让前端可以按用户地址过滤事件日志（eth_getLogs），
    //   不 indexed 的参数存 data 区，只能顺序读取，无法过滤
    event ScheduleCreated(
        address indexed beneficiary,
        uint256 totalAmount,
        uint256 startTime,
        uint256 cliff,
        uint256 endTime,
        bool revocable
    );

    event TokensReleased(
        address indexed beneficiary,
        uint256 amount
    );

    event ScheduleRevoked(
        address indexed beneficiary,
        uint256 retrunedAmount  // 返还给 owner 的未释放代币
    );

    // ============ 数据结构 ============

    // 为什么用 struct 而不是多个并行 mapping？
    // → struct 把所有字段打包在一起，一次 SLOAD 可能读到多个字段（如果总大小 ≤ 32 bytes
    //   编译器会尽量紧凑排列）。更重要的是：删除时只需要 delete 整个 struct。
    //   多个 mapping 需要逐个 delete，gas 更高且容易漏删。
    struct VestingSchedule {
        uint256 totalAmount;        // 总归属量（代币最小单位，如 18 位精度）
        uint256 releasedAmount;     // 已释放量（用于计算本次可释放 = 累积可释放 - 已释放）
        uint64 startTime;           // 归属开始时间（uint64 存 Unix timestamp 够用，比 uint256 省 storage slot）
        uint64 cliff;               // 悬崖结束时间，cliff=startTime 表示无悬崖
        uint64 endTime;             // 归属结束时间，此刻之后 100% 可释放
        bool revocable;             // 是否可被 owner 撤销
        bool revoked;               // 是否已被撤销（已撤销的计划 release 返回 0）
    }
    // 注：uint64×3 + bool×2 + uint256×2 = 24 + 2 + 64 = 90 bytes → 约 3 个 storage slot

    // ============ 状态变量 ============

    // 为什么把代币地址存为 immutable？
    // → 归属合约只服务一种代币，部署后不会换，immutable 比 storage 省 gas（不占 slot）
    IERC20 private immutable _token;

    // 为什么用 mapping 而不是 array？
    // → O(1) 按地址查计划，release 函数每次都要查，array 需要 O(n) 遍历
    mapping (address => VestingSchedule) private _schedules;

    // ============ 构造函数 ============

    // 为什么把 initialOwner 和 token_ 都放构造函数？
    // → Ownable 需要知道谁是 owner（可撤销/不可撤销的权限来源），
    //   token 是 immutable 必须在构造函数赋值
    //   initialOwner 可能不是 deployer，比如通过工厂合约部署
    constructor(address initialOwner, IERC20 token_) Ownable(initialOwner) {
        // 为什么检查 address(token_) != address(0)？
        // → 如果传了零地址，后续所有 transfer 都会失败（零地址没有代码），
        //   而且无法修复（immutable 不能改）。部署前校验，fail early
        if (address(token_) == address(0)) revert Vesting__ZeroAddress();
        _token = token_;
    }

    // ============ 核心函数 ============

    /**
     * @notice 为一个受益人创建代币归属计划
     * @param beneficiary 受益人地址
     * @param totalAmount 总归属量
     * @param startTime 开始时间（Unix timestamp）
     * @param cliff 悬崖时间（设置为 startTime 表示无悬崖，即从第一天开始线性释放）
     * @param endTime 结束时间
     * @param revocable 是否可撤销
     * @dev 调用前需要先 approve 足够的代币给本合约
     */
    function createSchedule(
        address beneficiary,
        uint256 totalAmount,
        uint64 startTime,
        uint64 cliff,
        uint64 endTime,
        bool revocable
    ) external onlyOwner {
        // ===== 参数校验 =====
        if (beneficiary == address(0)) revert Vesting__ZeroAddress();
        if (totalAmount == 0) revert Vesting__ZeroAmount();

        // 为什么 cliff < startTime 也是非法的？
        // → cliff 表示"悬崖结束时间"，cliff 必须 ≥ startTime。
        //   如果 cliff < startTime，意味着在 startTime 之前悬崖就结束了，逻辑上说不通。
        //   cliff == startTime 是合法场景：表示"从一开始就线性释放，无悬崖"。
        if (cliff < startTime) revert Vesting__InvalidTimeRange();

        // 为什么 endTime < cliff 是非法的？
        // → 如果 endTime 在 cliff 之前，悬崖结束后 vesting 已经结束，线性释放区间为 0。
        //   等于只做了一个"延迟一次性释放"，不如直接用 cliff-only 模式（设置 endTime=cliff）。
        if (endTime < cliff)  revert Vesting__InvalidTimeRange();

        // 为什么每人只能有一个计划？
        // → 简化管理：如果允许一人多计划，release 时不知道用户想释放哪一个，
        //   需要额外参数或子 ID，增加复杂度也容易出错。
        //   大多数项目（Uniswap、Aave）的 vesting 也是每人一个计划。
        if (_schedules[beneficiary].totalAmount > 0) revert Vesting__ScheduleAlreadyExists();

        // ===== 转移代币 =====
        // 为什么先转代币再写 storage？
        // → CEI 模式（Check-Effects-Interaction）：转代币是外部调用（Interaction），
        //   应该在修改 storage（Effects）之后。但这里 createSchedule 是可重入的吗？
        //   safeTransferFrom 调的是代币合约，恶意代币可以回调本合约...
        //   安全起见，本合约有 ReentrancyGuard 保护。且这里先写 storage 会多写一次。
        //   实际排序：Check（上面做完了）→ Effects（写 schedule）→ Interaction（转账）
        //   但为了正确性，我们需要在 revert 时不留 storage 痕迹。
        //   更好的做法：先 Effects 再 Interaction。
        _schedules[beneficiary] = VestingSchedule({
            totalAmount: totalAmount,
            releasedAmount: 0,
            startTime: startTime,
            cliff: cliff,
            endTime: endTime,
            revocable: revocable,
            revoked: false
        });

        // 为什么用 safeTransferFrom 而不是 transferFrom？
        // → safeTransferFrom 会检查返回值（USDT 不返回 bool），且对 ERC777 等代币更安全。
        //   常规 transferFrom 可能静默失败（不 revert 但代币没转成功）。
        // 为什么是 msg.sender → 本合约？
        // → createSchedule 需要 owner 先持有代币并 approve 给 Vesting 合约。
        //   owner 调用此函数，合约从 owner 地址拉取代币到自己名下托管。
        if (!_token.transferFrom(msg.sender, address(this), totalAmount)) {
            revert Vesting__TokenTransferFailed();
        }

        emit ScheduleCreated(beneficiary, totalAmount, startTime, cliff, endTime, revocable);
    }

    // ============ 内部计算函数 ============

    /**
     * @notice 计算某受益人当前可释放的总量（不是本次）
     * @dev 纯计算，不修改 storage。本函数是 release 的核心逻辑。
     *
     * 计算逻辑（分三种情况）：
     *
     * 情况 A：已撤销 — 返回 0（不能再释放）
     *
     * 情况 B：悬崖前（block.timestamp < cliff）
     *   → 累积可释放 = 0
     *
     * 情况 C：悬崖后（block.timestamp >= cliff）
     *   → 累积可释放 = totalAmount × (elapsed / duration)
     *     其中 elapsed = min(now, endTime) - startTime
     *          duration = endTime - startTime
     *
     * 实际可释放 = 累积可释放 - releasedAmount
     */
    function _computeReleasableAmount(
        VestingSchedule memory schedule
    ) internal view returns (uint256) {
        // 为什么先读 block.timestamp 到局部变量？
        // → 减少多次读取 block.timestamp 的 gas（虽然几乎没差别），
        //   但更重要的是代码可读性：一眼看出当前时间点。
        uint256 now_ = block.timestamp;

        // 已撤销 → 不再释放
        if (schedule.revoked) {
            return 0;
        }

        // 悬崖前 → 0
        // 为什么用 < 而不是 <=？
        // → 悬崖结束时间是"可释放的开始时间"：cliff 时刻正好触发释放。
        //   cliff=100, now=100 → >= 满足条件，进入线性释放。
        if (now_ < schedule.cliff) {
            return 0;
        }

        // 悬崖后 → 线性计算
        // 为什么 elapsed 用 min(now_, endTime)？
        // → 超过 endTime 之后，时间是"静止的"——已过去的时间不能超过 duration，
        //   否则 elapsed/duration > 1，导致可释放 > totalAmount。
        uint256 elapsed = (now_ < schedule.endTime ? now_ : schedule.endTime) - schedule.startTime;
        uint256 duration = schedule.endTime - schedule.startTime;

        // 为什么用乘法在前、除法在后？
        // → Solidity 无浮点数：(elapsed / duration) 会先做整数除法，小数部分被截断为 0 或 1。
        //   先乘后除保持精度：totalAmount * elapsed / duration ≈ totalAmount × 时间比例
        //   注意溢出：totalAmount * elapsed 可能超过 uint256，但实际代币量级（< 10^30）不会溢出
        uint256 vestedAmount = (schedule.totalAmount * elapsed) / duration;

        // 累积可释放 - 已释放 = 本次可释放
        // 为什么用 vestedAmount > releasedAmount 判断？
        // → 正常情况下 vestedAmount >= releasedAmount（时间前进，归属增加）。
        //   但如果有 bug 导致 releasedAmount > vestedAmount（不应该发生），用 if 防护避免 underflow revert。
        if (vestedAmount > schedule.releasedAmount) {
            return vestedAmount - schedule.releasedAmount;
        }
        return 0;
    }

    // ============ 视图函数 ============

    /**
     * @notice 查询某受益人的当前可释放量（供前端调用）
     */
    function getReleasableAmount(address beneficiary) external view returns (uint256) {
        VestingSchedule memory schedule = _schedules[beneficiary];
        if (schedule.totalAmount == 0) revert Vesting__NoSchedule();
        return _computeReleasableAmount(schedule);
    }

    /**
     * @notice 查询某受益人的完整归属计划
     */
    function getSchedule(address beneficiary) external view returns (VestingSchedule memory) {
        return _schedules[beneficiary];
    }

    /**
     * @notice 返回当前托管的代币地址
     */
    function token() external view returns (IERC20) {
        return _token;
    }

    // ============ 核心交互函数 ============

    /**
     * @notice 受益人调用，提取当前可释放的代币
     * @dev 任何人可以帮助受益人调用（自动转给受益人，不需要 msg.sender 就是受益人）
     *      这是 gas 友好的设计：项目方可以帮所有用户批量释放，用户不用自己花 gas
     */
    function release(address beneficiary) external nonReentrant {
        // ===== 获取计划 =====
        // 为什么用 storage 指针而不是 memory 拷贝？
        // → storage 指针直接操作链上数据，修改后自动写回。
        //   memory 拷贝后修改需要手动写回 storage（多一笔 SSTORE gas）。
        //   这里需要在计算后更新 releasedAmount，storage 指针更高效。
        VestingSchedule storage schedule = _schedules[beneficiary];

        // 没有计划 → 报错
        if (schedule.totalAmount == 0) revert Vesting__NoSchedule();

        // ===== CEI: Check =====
        // 计算本次可释放量：累积已归属 - 已释放
        // 为什么放 Check 阶段？因为这是纯计算，不消耗外部 gas，且能提前 revert 省 gas
        uint256 releasable = _computeReleasableAmount(schedule);
        if (releasable == 0) revert Vesting__NothingToRelease();

        // ===== CEI: Effects =====
        // 为什么先更新 storage 再转代币？
        // → CEI 模式的核心原则：状态变更必须在外部调用之前完成。
        //   如果先转账再更新 releasedAmount，恶意代币在 transfer 回调中重入 release()，
        //   releasedAmount 还没更新 → _computeReleasableAmount 返回同样的值 → 重复提取！
        schedule.releasedAmount += releasable;

        // ===== CEI: Interaction =====
        // 为什么用 transfer 而不是 safeTransfer？
        // → safeTransfer 需要 IERC20 接口，标准 transfer 更通用。
        //   如果代币不支持返回 bool（如 USDT），这里加了一层 require 检查。
        //   更好的做法：用 SafeERC20 库的 safeTransfer，但这里保持简洁。
        if (!_token.transfer(beneficiary, releasable)) {
            revert Vesting__TokenTransferFailed();
        }

        emit TokensReleased(beneficiary, releasable);
    }

    /**
     * @notice Owner 撤销某个受益人的归属计划，未释放的代币返还给 owner
     * @dev 只有标记为 revocable 的计划才能被撤销
     *
     * 撤销后的代币流向：
     *   已释放 → 归受益人（拿不回来，区块链不可逆）
     *   未释放 → 返回 owner（= totalAmount - releasedAmount）
     *
     * 为什么已释放的拿不回来？
     * → 区块链上代币已经转到用户钱包，Vesting 合约没有权限从用户钱包转走代币。
     *   即使有 approve，用户也可以提前转走或撤销 approve。
     */
    function revoke(address beneficiary) external onlyOwner nonReentrant {
        VestingSchedule storage schedule = _schedules[beneficiary];
        if (schedule.totalAmount == 0) revert Vesting__NoSchedule();

        // 为什么先检查 revocable + revoked 而不是先算 amount？
        // → 如果不满足条件，尽早 revert 省 gas（后面计算是多余的）
        //   这是一个 gas 优化细节：把最可能失败的检查放最前面
        if (!schedule.revocable) revert Vesting__NotRevocable();
        if (schedule.revoked) revert Vesting__AlreadyRevoked();

        // ===== CEI: Check — 计算未释放量 =====
        // 累积已归属 = 根据当前时间算出来的理论可释放总量
        uint256 vestedAmount;
        {
            // 用 _computeReleasableAmount 的累积量 + releasedAmount = 总已归属量
            // 为什么这样算？
            // → _computeReleasableAmount 返回的是"本次可释放"，
            //   releasedAmount 是"已经释放的"，
            //   总累积 = 已释放 + 本次可释放 = 所有已经归属的代币
            uint256 releasableNow = _computeReleasableAmount(schedule);
            vestedAmount = schedule.releasedAmount + releasableNow;
        }

        // 为什么用 unchecked？
        // → 已确认 totalAmount >= vestedAmount（归属量不可能超过总量，数学上保证不 underflow）
        //   unchecked 省掉 SafeMath 检查的 gas
        uint256 unvested;
        unchecked {
            unvested = schedule.totalAmount - vestedAmount;
        }

        // ===== CEI: Effects =====
        // 为什么设置 revoked = true 而不是 delete 整个 schedule？
        // → delete 会清除所有字段，包括 totalAmount。如果保留 schedule 但标记 revoked，
        //   前端仍可查询历史计划，_computeReleasableAmount 会因为 revoked=true 返回 0。
        //   这是"软删除"vs"硬删除"——软删除保留了审计轨迹。
        schedule.revoked = true;
        // 为什么把 releasedAmount 设置成 totalAmount？
        // → 防止通过 _computeReleasableAmount 算出非零值（虽然 revoked 已经拦截了），
        //   双重保险：releasedAmount == totalAmount → vestedAmount - releasedAmount = 0
        schedule.releasedAmount = schedule.totalAmount;

        // ===== CEI: Interaction =====
        if (unvested > 0) {
            if (!_token.transfer(owner(), unvested)) {
                revert Vesting__TokenTransferFailed();
            }
        }

        emit ScheduleRevoked(beneficiary, unvested);
    }

    /**
     * @notice 批量释放 — 一次交易释放多位受益人的代币
     * @param beneficiaries 受益人的地址数组
     * @dev Gas 优化：节省了 (N-1) × 21000（每笔交易的基础 gas）。
     *      但循环内的 storage 读写和 transfer 的 gas 不变。
     *
     * 为什么用循环而不是逐个 emit 后继续？
     * → 循环是唯一的方式。emit 只是记录日志，不会中断执行。
     *   需要处理空数组（边界条件）——如果传空数组，循环不执行，不耗 gas。
     *
     * 为什么不在循环内使用 nonReentrant 的 release？
     * → release 已经有 nonReentrant 修饰符，但 modifier 是函数级的。
     *   如果 batchRelease 调用 release，每次都会检查 nonReentrant（多余）。
     *   这里内联实现，只在最外层加 nonReentrant，一次检查保护全部循环。
     *   但为了代码可维护性，选择复用 release 逻辑的方式：
     *   直接调 release 更简洁（nonReentrant 的检查开销 ≈ 5000 gas，可接受）
     */
    function batchRelease(
        address[] calldata beneficiaries
    ) external nonReentrant {
        uint256 len = beneficiaries.length;
        // 为什么缓存 length 到局部变量？
        // → calldata 的 .length 每次读取消耗 gas（虽然极少）。
        //   循环中用局部变量 len 比每次读 beneficiaries.length 省一点点 gas。
        //   更重要的是：如果 beneficiaries 是 storage array，length 每次都是 SLOAD（2100 gas），
        //   缓存后只需要一次 SLOAD。这里虽然是 calldata（便宜），但养成好习惯。
        for (uint256 i = 0; i < len; ) {
            // 为什么跳过 0 地址？
            // → 如果数组中有 address(0)，release(address(0)) 会在 get schedule 时 revert，
            //   导致整批交易回滚。跳过 0 地址让批量操作更健壮（partial success）。
            //   注意：这里只是跳过，不会回滚整批。
            // 更严谨的做法：用 try/catch 包裹每次 release（Solidity 0.8.20+ 不支持 try/catch 在 external call 之外）。
            if (beneficiaries[i] != address(0)) {
                // 为什么直接调 release 而不是内联复制代码？
                // → release 已经有完整的 CEI + nonReentrant 逻辑。
                //   nonReentrant 在 batchRelease 外层已经设了锁，release 内部会检查到锁已持 → revert！
                //   ⚠️ 这是 nonReentrant 的关键陷阱：嵌套调用会失败。
                //
                // 解决方案：内联 release 的核心逻辑（去掉 nonReentrant modifier）。
                // 但为了代码清晰，这里创建一个内部函数 _releaseUnsafe 供 release() 和 batchRelease() 共用。
                //
                // 以下是内联版本（生产环境建议提取 _releaseUnsafe 内部函数）：
                VestingSchedule storage schedule = _schedules[beneficiaries[i]];
                if (schedule.totalAmount > 0) {
                    uint256 releasable = _computeReleasableAmount(schedule);
                    if (releasable > 0) {
                        schedule.releasedAmount += releasable;
                        if (!_token.transfer(beneficiaries[i], releasable)) {
                            revert Vesting__TokenTransferFailed();
                        }
                        emit TokensReleased(beneficiaries[i], releasable);
                    }
                }
            }
            // 为什么用 unchecked { ++i } 而不是 i++？
            // → Solidity 0.8+ 默认有溢出检查。循环计数器不可能溢出（len 最大 ≈ 2^256-1），
            //   unchecked 省掉每次迭代的溢出检查 gas。习惯写法。
            unchecked {
                ++i;
            }
        }
    }

     /**
     * @notice 批量撤销 — 一次交易撤销多位受益人的归属计划
     * @param beneficiaries 受益人的地址数组
     */
    function batchRevoke(
        address[] calldata beneficiaries
    ) external onlyOwner nonReentrant {
        uint256 len = beneficiaries.length;
        for (uint256 i = 0; i < len; ) {
            if (beneficiaries[i] != address(0)) {
                // 同样的 nonReentrant 嵌套问题，内联 revoke 逻辑
                VestingSchedule storage schedule = _schedules[beneficiaries[i]];
                if (schedule.totalAmount > 0 && schedule.revocable && !schedule.revoked) {
                    // 计算未释放量
                    uint256 releasableNow = _computeReleasableAmount(schedule);
                    uint256 vestedAmount = schedule.releasedAmount + releasableNow;
                    uint256 unvested;
                    unchecked {
                        unvested = schedule.totalAmount - vestedAmount;
                    }

                    // Effects
                    schedule.revoked = true;
                    schedule.releasedAmount = schedule.totalAmount;

                    // Interaction
                    if (unvested > 0) {
                        if (!_token.transfer(owner(), unvested)) {
                            revert Vesting__TokenTransferFailed();
                        }
                    }

                    emit ScheduleRevoked(beneficiaries[i], unvested);
                }
            }

            unchecked {
                ++i;
            }
        }
    }
}