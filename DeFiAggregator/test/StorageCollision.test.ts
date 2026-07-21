import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ==================== 辅助函数：直接读原始存储槽 ====================
/// @notice 直接读取 addr 的 storage slot，返回 BigInt
/// @dev 绕过合约 ABI，直接读底层存储——这才是存储碰撞的正确验证方式
async function getStorageAt(addr: string, slot: number | bigint): Promise<bigint> {
    const slotHex = "0x" + slot.toString(16);
    const raw = await ethers.provider.getStorage(addr, slotHex);
    return BigInt(raw);
}

describe("💥 StorageCollision — 存储布局冲突灾难演示", function () {
    // ==================== 变量声明 ====================
    let owner: any, user1: any;
    let v1: any;
    let v2Broken: any;
    let v2Correct: any;

    // ==================== A. 部署 V1 ====================
    describe("A. 部署 V1 并初始化数据", function () {
        it("A1. 部署 V1 并写入数据", async function () {
            [owner, user1] = await ethers.getSigners();

            v1 = await ethers.deployContract("StorageV1");

            // 写入数据
            await v1.connect(owner).init(100n, 200n);

            // 验证分层存储（V1 自身 getter）
            expect(await v1.valueA()).to.equal(100n);  // slot 0
            expect(await v1.valueB()).to.equal(200n);  // slot 1
            expect(await v1.owner()).to.equal(owner.address); // slot 2
        });

        it("A2. 底层验证：slot 0/1/2 的值", async function () {
            const v1Addr = await v1.getAddress();

            const slot0 = await getStorageAt(v1Addr, 0n);
            const slot1 = await getStorageAt(v1Addr, 1n);
            const slot2 = await getStorageAt(v1Addr, 2n);

            // V1: slot 0 = 100, slot 1 = 200, slot 2 = owner(uint160)
            expect(slot0).to.equal(100n);
            expect(slot1).to.equal(200n);

            // slot 2 存的是 address → 取低 160 位
            const ownerFromSlot = "0x" + slot2.toString(16).slice(-40);
            expect(ownerFromSlot).to.equal(owner.address.toLowerCase());
        });
    });

    // ==================== B. 🔥 存储冲突演示 ====================
    // ⚠️ 关键：用原始 getStorage 直接读槽，绕过合约 ABI
    // 因为 getContractAt 只换 ABI，实际执行仍是原地址的代码
    // 正确做法：读原始槽 → 手动按 V2Broken 的布局"解码"
    describe("B. 🔥 灾难现场：V2Broken 视角下的 V1 存储", function () {
        let v1Addr: string;

        // 存储布局对照表：
        // ┌──────────┬─────────────┬──────────────────┐
        // │   Slot   │ V1 存了什么  │ V2Broken 认为是什么 │
        // ├──────────┼─────────────┼──────────────────┤
        // │    0     │  valueA=100 │     owner        │
        // │    1     │  valueB=200 │     valueA       │
        // │    2     │   owner地址  │     valueB       │
        // └──────────┴─────────────┴──────────────────┘

        before(async function () {
            v1Addr = await v1.getAddress();
        });

        it("B1. 部署 Broken V2（变量顺序被打乱）", async function () {
            v2Broken = await ethers.deployContract("StorageV2Broken");
            // 单独部署即可，不需要 init——我们是用它的"视角"去解读 V1 的存储
        });

        it("B2. 🔥 V2Broken 的 owner() 读到的是 valueA=100 的地址表示", async function () {
            // 原始 slot 0 = valueA = 100
            const rawSlot0 = await getStorageAt(v1Addr, 0n);

            // 在 V2Broken 的布局中，slot 0 = owner
            // → 如果 V2Broken 的逻辑读 slot 0 作为 address：
            // 取低 160 位，就是 100 转成 address → 0x000...064
            const brokenOwner = "0x" + rawSlot0.toString(16).padStart(40, "0");

            console.log("💥 Broken V2 视角的 'owner':", brokenOwner);
            console.log("   真实的 owner:", owner.address);
            console.log("   原因: slot 0 = valueA(100) → V2Broken 当 owner 读 → " + brokenOwner);

            expect(brokenOwner).to.not.equal(owner.address.toLowerCase());
            expect(rawSlot0).to.equal(100n); // slot 0 存的就是 valueA, 不是 owner 地址
        });

        it("B3. 🔥 V2Broken 的 valueA() 读到的是 valueB=200", async function () {
            // 原始 slot 1 = valueB = 200
            const rawSlot1 = await getStorageAt(v1Addr, 1n);

            // 在 V2Broken 的布局中，slot 1 = valueA
            // → V2Broken 视角的 valueA = 200（实际是 valueB）
            console.log("💥 Broken V2 视角的 'valueA':", rawSlot1);
            console.log("   预期的 valueA: 100");
            console.log("   实际返回的是 valueB=200（slot 1）");

            expect(rawSlot1).to.equal(200n);   // slot 1 = valueB
            expect(rawSlot1).to.not.equal(100n); // ≠ valueA
        });

        it("B4. 🔥 V2Broken 的 valueB() 读到的是 owner 地址的 uint256", async function () {
            // 原始 slot 2 = owner 地址
            const rawSlot2 = await getStorageAt(v1Addr, 2n);

            // 在 V2Broken 的布局中，slot 2 = valueB
            // → V2Broken 视角的 valueB = owner 地址的 uint256 表示
            const ownerAsUint = BigInt(owner.address);

            console.log("💥 Broken V2 视角的 'valueB':", rawSlot2);
            console.log("   预期的 valueB: 200");
            console.log("   实际等于 owner 地址的 uint256:", ownerAsUint);

            expect(rawSlot2).to.equal(ownerAsUint);
            expect(rawSlot2).to.not.equal(200n);
        });

        it("B5. 🔥 总结：数据完全错乱！", async function () {
            const rawSlot0 = await getStorageAt(v1Addr, 0n);
            const rawSlot1 = await getStorageAt(v1Addr, 1n);
            const rawSlot2 = await getStorageAt(v1Addr, 2n);

            const brokenOwner = "0x" + rawSlot0.toString(16).padStart(40, "0");
            const brokenValueA = rawSlot1;
            const brokenValueB = rawSlot2;

            console.log("");
            console.log("📊 存储碰撞灾难对照表：");
            console.log("┌─────────┬──────────────┬─────────────────────────┐");
            console.log("│  变量   │    期望值      │  V2Broken 实际读到的       │");
            console.log("├─────────┼──────────────┼─────────────────────────┤");
            console.log("│ owner   │ 部署者地址     │ " + brokenOwner.slice(0, 20) + "...  │");
            console.log("│ valueA  │ 100          │ " + brokenValueA + "                    │");
            console.log("│ valueB  │ 200          │ " + brokenValueB + "   │");
            console.log("└─────────┴──────────────┴─────────────────────────┘");
            console.log("💀 全部错乱！这就是变量顺序不一致的后果");

            // B2/B3/B4 已分别验证，这里集中总结
        });
    });

    // ==================== C. ✅ 正确做法对比 ====================
    describe("C. ✅ 正确做法：变量顺序不变", function () {
        it("C1. 部署 Correct V2（变量顺序与 V1 一致）", async function () {
            v2Correct = await ethers.deployContract("StorageV2Correct");
        });

        it("C2. ✅ 用原始存储 + V2Correct 布局验证数据正确", async function () {
            const v1Addr = await v1.getAddress();

            // 直接读原始槽 → 两个布局一致，无需"翻译"
            const slot0 = await getStorageAt(v1Addr, 0n);
            const slot1 = await getStorageAt(v1Addr, 1n);
            const slot2 = await getStorageAt(v1Addr, 2n);

            // V2Correct 的 slot 布局 = V1
            // slot 0 = valueA, slot 1 = valueB, slot 2 = owner
            const correctValueA = slot0;
            const correctValueB = slot1;
            const correctOwner = "0x" + slot2.toString(16).slice(-40);

            console.log("✅ Correct V2 视角：");
            console.log("   valueA:", correctValueA, "(期望: 100)");
            console.log("   valueB:", correctValueB, "(期望: 200)");
            console.log("   slot2→owner:", correctOwner);

            expect(correctValueA).to.equal(100n);
            expect(correctValueB).to.equal(200n);
            expect(correctOwner).to.equal(owner.address.toLowerCase());
        });

        it("C3. ✅ 新变量 description（slot 3）应该为默认值（0）", async function () {
            const v1Addr = await v1.getAddress();

            // V1 没有 slot 3 → 未初始化，返回 0x00...00
            // V2Correct 在 slot 3 追加了 description (string)
            // string 空字符串 → storage slot 为 0
            const rawSlot3 = await getStorageAt(v1Addr, 3n);
            expect(rawSlot3).to.equal(0n);

            console.log("✅ slot 3 (description) = 0，追加新变量安全");
        });
    });

    // ==================== D. 🔥 面试重点 ====================
    describe("D. 🔥 面试重点 — 存储布局铁律", function () {
        it("D1. 🔥 铁律 1：删除变量导致 slot 错位", async function () {
            console.log("铁律 1: 不能删除已有变量");
            console.log("  V1: slot 0=A, slot 1=B, slot 2=owner");
            console.log("  V2(删A): slot 0=B(原是slot 1), slot 1=owner(原是slot 2)");
            console.log("  → B 读到 A 的值，owner 读到 B 的值 → 全错位");
        });

        it("D2. 🔥 铁律 2：改变变量类型", async function () {
            console.log("铁律 2: 不能改变变量类型");
            console.log("  uint256→uint128+uint128: 打包规则变化 → 后续 slot 偏移");
            console.log("  address→address payable: 可以，ABI 编码相同");
        });

        it("D3. 🔥 铁律 3：重排变量顺序 → B 组已验证灾难", async function () {
            // 验证数据确实错乱了
            const v1Addr = await v1.getAddress();
            const rawSlot0 = await getStorageAt(v1Addr, 0n);
            const rawSlot1 = await getStorageAt(v1Addr, 1n);
            const ownerAsUint = BigInt(owner.address);

            // B 组演示的核心：V2Broken 视角全乱
            expect(rawSlot0).to.not.equal(ownerAsUint); // slot 0 ≠ owner 的 uint256
            expect(rawSlot1).to.not.equal(100n);         // slot 1 ≠ valueA
            expect(rawSlot1).to.equal(200n);              // slot 1 = valueB (证实了重排的灾难)
            console.log("铁律 3: 已验证 — slot 1 = " + rawSlot1 + " ≠ valueA(100)");
        });

        it("D4. 🔥 铁律 4：新变量只能追加在末尾（已验证安全）", async function () {
            const v1Addr = await v1.getAddress();
            const slot3 = await getStorageAt(v1Addr, 3n);
            expect(slot3).to.equal(0n);
            console.log("铁律 4: 已验证 — slot 3 = 0，追加 description 无冲突");
        });
    });
});
