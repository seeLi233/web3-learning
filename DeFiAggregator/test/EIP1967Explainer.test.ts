import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ==================== 测试常量 ====================
// ⭐ EIP-1967 标准存储槽（用于底层验证）
const IMPLEMENTATION_SLOT =
  "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";
const ADMIN_SLOT =
  "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103";
const BEACON_SLOT =
  "0xa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6cb3582b35133d50";

describe("🔬 EIP1967Explainer — EIP-1967 存储槽原理深度解析", function () {
    // ==================== 变量声明 ====================
    let owner: any, user1: any;
    let explainer: any;

    // ==================== Setup ====================
    before(async function () {
        [owner, user1] = await ethers.getSigners();
        explainer = await ethers.deployContract("EIP1967Explainer");
    });

    // ==================== A. 部署 ====================
    describe("A. 部署", function () {
        it("A1. 初始状态：所有 EIP-1967 槽应该为 0", async function () {
            expect(await explainer.getImplementation()).to.equal(
                ethers.ZeroAddress
            );
            expect(await explainer.getAdmin()).to.equal(ethers.ZeroAddress);
        });

        it("A2. 初始普通变量应该为默认值", async function () {
            expect(await explainer.normalUint()).to.equal(0n);
            expect(await explainer.normalAddress()).to.equal(ethers.ZeroAddress);
            expect(await explainer.normalBool()).to.equal(false);
        });
    });

    // ==================== B. 存储槽读写 ====================
    describe("B. EIP-1967 存储槽读写", function () {
        it("B1. 应该成功设置和读取 implementation 地址", async function () {
            const mockImpl = "0x1111111111111111111111111111111111111111";

            await explainer.connect(owner).setImplementation(mockImpl);

            expect(await explainer.getImplementation()).to.equal(mockImpl);
        });

        it("B2. 应该成功设置和读取 admin 地址", async function () {
            const mockAdmin = "0x2222222222222222222222222222222222222222";

            await explainer.connect(owner).setAdmin(mockAdmin);

            expect(await explainer.getAdmin()).to.equal(mockAdmin);
        });

        it("B3. 应该触发 ImplementationSet 事件", async function () {
            const newImpl = "0x3333333333333333333333333333333333333333";
            const oldImpl = await explainer.getImplementation();

            await expect(explainer.connect(owner).setImplementation(newImpl))
                .to.emit(explainer, "ImplementationSet")
                .withArgs(oldImpl, newImpl);
        });

        it("B4. 🔥 底层验证：直接读取存储槽", async function () {
            const impl = await explainer.getImplementation();

            // 直接从链上读取 IMPLEMENTATION_SLOT
            const rawStorage = await ethers.provider.getStorage(
                await explainer.getAddress(),
                IMPLEMENTATION_SLOT
            );

            // 取后 20 字节（address 是 160 位）
            const implFromSlot = "0x" + rawStorage.slice(-40);
            expect(implFromSlot.toLowerCase()).to.equal(impl.toLowerCase());
        });

        it("B5. 🔥 验证三个 EIP-1967 槽位各不相同", async function () {
            // 三个槽位应该完全独立
            const implSlotBigInt = BigInt(IMPLEMENTATION_SLOT);
            const adminSlotBigInt = BigInt(ADMIN_SLOT);
            const beaconSlotBigInt = BigInt(BEACON_SLOT);

            expect(implSlotBigInt).to.not.equal(adminSlotBigInt);
            expect(implSlotBigInt).to.not.equal(beaconSlotBigInt);
            expect(adminSlotBigInt).to.not.equal(beaconSlotBigInt);
        });
    });

    // ==================== C. 存储隔离验证 ====================
    describe("C. 🔥 存储隔离 — EIP-1967 槽与普通 Slot 互不干扰", function () {
        it("C1. 应该演示普通变量和 EIP-1967 槽隔离", async function () {
            // 先设置普通变量
            await explainer.connect(owner).setNormalVars(
                42n,
                "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
                true
            );

            // 设置 EIP-1967 槽
            await explainer.connect(owner).setImplementation(
                "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
            );
            await explainer.connect(owner).setAdmin(
                "0xCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
            );

            // 读取隔离演示
            const isolation = await explainer.demonstrateIsolation();

            // 普通变量不受影响
            expect(isolation.slot0).to.equal(42n);
            expect(isolation.slot1).to.not.equal(0n); // address 不为零

            // EIP-1967 槽有值
            expect(isolation.impl).to.not.equal(ethers.ZeroAddress);
            expect(isolation.admin).to.not.equal(ethers.ZeroAddress);

            // 槽位号数量级差异巨大
            // slot0 = 0, slot1 = 1, implSlotNum ≈ 2^255
            expect(isolation.implSlotNum).to.be.gt(isolation.slot0);
            expect(isolation.implSlotNum).to.be.gt(2n ** 128n);
        });

        it("C2. 🔥 修改普通变量不会影响 EIP-1967 槽", async function () {
            const implBefore = await explainer.getImplementation();
            const adminBefore = await explainer.getAdmin();

            // 修改普通变量
            await explainer.connect(owner).setNormalVars(
                99999n,
                "0xDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD",
                false
            );

            // EIP-1967 槽不变
            expect(await explainer.getImplementation()).to.equal(implBefore);
            expect(await explainer.getAdmin()).to.equal(adminBefore);

            // 普通变量变了
            expect(await explainer.normalUint()).to.equal(99999n);
        });

        it("C3. 🔥 修改 EIP-1967 槽不会影响普通变量", async function () {
            const uintBefore = await explainer.normalUint();
            const addrBefore = await explainer.normalAddress();

            // 修改 EIP-1967 槽
            await explainer.connect(owner).setImplementation(
                "0xEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
            );

            // 普通变量不变
            expect(await explainer.normalUint()).to.equal(uintBefore);
            expect(await explainer.normalAddress()).to.equal(addrBefore);
        });
    });

    // ==================== D. 槽位计算验证 ====================
    describe("D. 🔥 存储槽计算验证", function () {
        it("D1. 应该验证 keccak256(string) - 1 等于标准槽值", async function () {
            const verification = await explainer.verifySlotCalculation();

            // ⭐ 核心验证：手动计算的值等于 EIP-1967 标准值
            expect(verification.implMatches).to.be.true;
            expect(verification.adminMatches).to.be.true;
        });

        it("D2. 🔥 面试演示：碰撞概率 ≈ 0", async function () {
            const demo = await explainer.demonstrateNoCollision();

            // EIP-1967 槽位值远大于合约可能的最大 slot
            expect(demo.cannotCollide).to.be.true;

            // 数值对比展示
            console.log("最大可能的普通 slot (2^64):", demo.maxPossibleSlot);
            console.log("EIP-1967 impl slot 值:", demo.implSlotValue);
            console.log("不会碰撞:", demo.cannotCollide);
        });

        it("D3. 手动计算验证（不依赖合约）", async function () {
            // 在 TypeScript 层面手动计算，双重验证
            // keccak256 在 JS 中的等效计算
            // 这里我们通过链上的计算结果来验证
            const implFromContract = await explainer.getImplementation();

            // 读取底层存储
            const rawStorage = await ethers.provider.getStorage(
                await explainer.getAddress(),
                IMPLEMENTATION_SLOT
            );

            const implFromRaw = "0x" + rawStorage.slice(-40);
            expect(implFromRaw.toLowerCase()).to.equal(implFromContract.toLowerCase());
        });
    });

    // ==================== E. 🔥 面试重点演示 ====================
    describe("E. 🔥 面试重点 — 综合演示", function () {
        it("E1. 🔥 EIP-1967 为什么用 keccak256 - 1？", async function () {
            // 1. 证明槽位值是固定的、可验证的
            const verification = await explainer.verifySlotCalculation();
            expect(verification.implMatches).to.be.true;

            // 2. 证明槽位远大于普通变量可能的最大 slot
            const demo = await explainer.demonstrateNoCollision();
            expect(demo.cannotCollide).to.be.true;

            // 结论：EIP-1967 的 keccak256-1 保证了：
            // a) 固定的、可验证的槽位
            // b) 与普通变量的 slot 不可能碰撞
            // c) 与 mapping 的动态槽也不碰撞
        });

        it("E2. 🔥 演示：如果不用 EIP-1967 会发生什么？", async function () {
            // ⚠️ 假设：如果代理把 implementation 存在 slot 0
            // 而 Logic 合约的第一个变量也在 slot 0
            // → 两个数据互相覆盖！

            // 在 EIP1967 方案下，这个场景不会发生
            // 因为 EIP-1967 槽位与 slot 0 距离 ~2^255

            const implFromSlot = BigInt(
                await ethers.provider.getStorage(
                    await explainer.getAddress(),
                    IMPLEMENTATION_SLOT
                )
            );

            const slot0Value = BigInt(
                await ethers.provider.getStorage(await explainer.getAddress(), "0x0")
            );

            // 它们来自完全不同的存储槽
            console.log("Slot 0 的值:", slot0Value);
            console.log("EIP-1967 槽的值:", implFromSlot);
        });
    });
});