// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

/**
 * @title GasComparison
 * @notice 一个用于对比 Gas 优化效果的演示合约
 * @dev 包含成对的函数：未优化版 vs 优化版，方便用 Hardhat gas reporter 对比
 *
 * 为什么需要这个合约？
 * → 面试时被问"你优化过合约 Gas 吗？"需要有具体的数据支撑。
 *    这个合约可以跑 gas reporter 生成对比报告，面试时直接引用数据。
 */
contract GasComparison {
    // ==================== 1. 变量初始化优化 ====================

    // ❌ 未优化：在声明时赋初值
    // 为什么不好？声明赋初值 = 构造函数里的 SSTORE，多花 20000 gas
    uint256 public counterV1 = 0;   // 0 是默认值，再写一遍 = 浪费 gas
    address public ownerV1 = address(0); // 同上

    // ✅ 优化：利用默认值特性，不显式赋初值
    // 为什么好？未初始化的变量 = 0 或 address(0)，不需要额外 SSTORE
    uint256 public counterV2;       // 默认就是 0
    address public ownerV2;         // 默认就是 address(0)

    // ==================== 2. Struct Packing 对比 ====================

    // ❌ 未优化：3 个 slot
    // uint8 后面跟 uint256 → 无法打包，浪费 2 个 slot
    struct UserInfoV1 {
        uint8 age;      // Slot 0: 1B + 31B 浪费
        uint256 balance;    // Slot 1: 32B
        uint8 level;        // Slot 2: 1B + 31B 浪费
        bool isActive;      // Slot 2: 1B（可以和 level 打包，但已经多用了 slot）
    }
    // 存储成本：首次写入 4 个 slot = 4 × 20000 = 80000 gas
    mapping (address => UserInfoV1) public usersV1;

    // ✅ 优化：2 个 slot
    // 把小类型放一起，uint256 单独放
    struct UserInfoV2 {
        // Slot 0: age(1B) + level(1B) + isActive(1B) = 3B ≤ 32B ✅
        uint8 age;
        uint8 level;
        bool isActive;
        // Slot 1: balance(32B) 占满
        uint256 balance;
    }
    // 存储成本：首次写入 2 个 slot = 2 × 20000 = 40000 gas
    mapping (address => UserInfoV2) public usersV2;

    // ==================== 3. calldata vs memory 对比 ====================

    // ❌ 未优化：用 memory 传递数组
    // 为什么不好？数组从 calldata 复制到 memory → 每字节都要 gas
    // 作用：接收 uint256 数组并返回总和
    function sumMemory(uint256[] memory arr) public pure returns (uint256 sum) {
        for (uint256 i = 0; i < arr.length; i++) {
            sum += arr[i];
        }
    }

    // ✅ 优化：用 calldata 传递数组
    // 为什么好？直接从 calldata 读，零拷贝
    // 作用：功能相同，但 gas 更低
    function sumCalldata(uint256[] calldata arr) public pure returns (uint256 sum) {
        for (uint256 i = 0; i < arr.length; i++) {
            sum += arr[i];
        }
    }

    // ==================== 4. unchecked 优化 ====================

    // ❌ 未优化：每次循环的 i++ 都会检查溢出（Solidity 0.8+ 默认安全）
    // 为什么浪费？循环变量 i 已知不会溢出（受 arr.length 限制），检查多余
    function sumChecked(uint256[] calldata arr) public pure returns (uint256 sum) {
        for (uint256 i = 0; i < arr.length; i++) {  // i++ 带溢出检查
            sum += arr[i];                          // sum += 带溢出检查

        }
    }

    function sumUnchecked(uint256[] calldata arr) public pure returns (uint256 sum) {
        for (uint256 i = 0; i< arr.length;) {
            sum += arr[i];
            // 为什么用 unchecked？i < arr.length 已经保证不会溢出
            unchecked {
                ++i;    // 为什么 ++i 不 i++？前置自增省临时变量
            }
        }
    }

    // ==================== 5. 自定义 Error 优化 ====================

    // ❌ 未优化：revert 带 string 信息
    // 为什么浪费？string 被编码到字节码中，越长的 string 消耗越多 gas
    // 作用：只允许 owner 调用
    modifier onlyOwnerV1() {
        require(msg.sender == ownerV2, unicode"Only owner can call this function — this is a long message");
        _;
    }

    // ✅ 优化：自定义 error
    // 为什么好？自定义 error 只存 4 字节 selector，比长 string 省数百 gas
    // 作用：功能完全相同，gas 更低
    error NotOwner(address caller);
    modifier onlyOwnerV2() {
        if (msg.sender != ownerV2) revert NotOwner(msg.sender);
        _;
    }

    // ==================== 6. 循环内 SLOAD 缓存 ====================

    uint256[] public data; // 假设这是一个很长的数组

    // ❌ 未优化：每次循环都读 storage
    // 为什么不好？每次 data.length 都触发 SLOAD（2100 gas 冷，100 gas 热）
    function getDataV1() external view returns (uint256[] memory) {
        uint256[] memory result = new uint256[](data.length);
        for (uint256 i = 0; i < data.length; i++) {   // 每次循环都读 data.length
            result[i] = data[i];                        // 每次循环都读 data[i]
        }
        return result;
    }

    // ✅ 优化：缓存到 memory
    // 为什么好？一次 SLOAD 缓存，循环内读 memory（便宜 100 倍）
    function getDataV2() external view returns (uint256[] memory) {
        uint256 len = data.length;  // 缓存长度到 memory——只读一次 storage
        uint256[] memory result = new uint256[](len);
        // 为什么用 while 不用 for？减少一次局部变量分配
        for (uint256 i = 0; i < len; ++i) {  // 用 memory 中的 len 做条件
            result[i] = data[i];
        }
        return result;
    }

    // ==================== 写入函数（用于测试对比） ====================

    // 写入未优化的 struct（供 gas 对比用）
    // 作用：给 usersV1 写一条记录，gas reporter 会记录消耗
    function setUserV1(address user, uint8 age, uint256 balance, uint8 level, bool active) external {
        usersV1[user] = UserInfoV1(age, balance, level, active);
    }

    // 写入优化后的 struct（供 gas 对比用）
    // 作用：给 usersV2 写一条记录，与 setUserV1 的 gas 对比
    function setUserV2(address user, uint8 age, uint256 balance, uint8 level, bool active) external {
        usersV2[user] = UserInfoV2(age, level, active, balance);
    }

    // 往 data 数组追加数据（供 getData 对比测试用）
    function pushData(uint256 value) external {
        data.push(value);
    }
}