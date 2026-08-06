import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ==================== 精度常量 ====================

// ethers.parseEther("1") = 10^18，所有金额都用这个单位保持一致
// 为什么定义为函数而不是常量？
// → parseEther 是 ethers 的函数，需要在 ethers 初始化后才能调用，
//   所以作为工具函数放在测试里而非顶层常量
const ONE_TOKEN = (n: number | string) => {
    // 注意：这里是 token 不是 ETH，但用同样的 18 位精度
    return BigInt(n) * 10n ** 18n;
};

// ==================== 测试套件 ====================

describe("🔒 DeFiVesting — 代币归属合约（线性释放 + 悬崖释放 + 可撤销）", function () {
    // ========== 变量声明 ==========
    // 为什么全部声明为 let 并用 any 类型？
    // → Hardhat 合约实例的类型很复杂（BaseContract + 自定义接口），
    //   any 避免 TypeScript 编译问题。这是测试代码的惯例。
    let vesting: any;        // DeFiVesting 实例
    let factory: any;        // VestingFactory 实例
    let token: any;          // 测试用的 ERC20 代币（MockERC20）
    let owner: any;          // 合约 owner（部署者）
    let beneficiary1: any;   // 受益人 1 — 线性释放场景
    let beneficiary2: any;   // 受益人 2 — 悬崖释放场景
    let beneficiary3: any;   // 受益人 3 — 可撤销场景
    let outsider: any;       // 无关地址 — 用于权限测试

    // ========== 参数常量 ==========
    // 为什么把时间参数定义为变量而不是常量？
    // → 时间参数依赖部署时的 block.timestamp，需要动态计算。
    //   定义为顶层变量，在 beforeEach 中赋值。
    let START_TIME: number;
    let CLIFF_6M: number;    // 6 个月悬崖（测试中用秒模拟月）
    let END_24M: number;     // 24 个月结束
    let END_12M: number;     // 12 个月结束（公募）

    // 简化版：测试中用秒代表月（1 月 = 60 秒），加速测试
    const ONE_MONTH = 60;  // 测试中 60 秒 = 1 个月

    // ==================== 部署组 ====================
    describe("A. 部署", function () {
        // ----- A1: 部署 MockERC20 代币 -----
        it("A1. 应该成功部署 MockERC20 代币和 Vesting 合约", async function () {
            [owner, beneficiary1, beneficiary2, beneficiary3, outsider] = await ethers.getSigners();

            // 部署测试代币
            // 为什么用 MockERC20 而不是真实代币？
            // → 测试需要完全控制代币行为（mint 给 owner，approve 给 vesting），
            //   真实代币不可控。MockERC20 提供 mint + approve 能力。
            //   注意：确保项目的 MockERC20 存在，如果不存在需要用 OpenZeppelin 的 ERC20
            const MockToken = await ethers.getContractFactory("MockERC20");
            token = await MockToken.deploy();

            // 为什么 mint 100 万代币给 owner？
            // → owner 需要足够的代币给所有受益人生成归属计划
            await token.mint(owner.address, ONE_TOKEN(1000000));

            // 部署 Vesting 合约
            const VestingFactory = await ethers.getContractFactory("DeFiVesting");
            vesting = await VestingFactory.deploy(owner.address, await token.getAddress());

            // 验证构造函数参数正确存储
            // 为什么用 await vesting.owner() 而不是 vesting.owner()？
            // → Solidity 的 public 变量自动生成 getter，但 ethers 调用任何合约方法
            //   都需要 await（即使是 view 函数，因为需要 RPC 调用）
            expect(await vesting.owner()).to.equal(owner.address);
            expect(await vesting.token()).to.equal(await token.getAddress());
        });

        // ----- A2: 部署 Factory -----
        it("A2. 应该成功部署 VestingFactory", async function () {
            [owner] = await ethers.getSigners();

            const FactoryFactory = await ethers.getContractFactory("VestingFactory");
            factory = await FactoryFactory.deploy(owner.address);

            expect(await factory.owner()).to.equal(owner.address);
            expect(await factory.getVestingCount()).to.equal(0n);
        });
    });

    // ==================== 时间辅助函数 ====================

    // 为什么封装时间操控为辅助函数？
    // → 测试中频繁使用 evm_increaseTime + evm_mine，封装后代码更清晰。
    //   面试时如果被问"怎么测试 Vesting 的时间逻辑"，能说出这两步操作。
    async function increaseTime(seconds: number) {
        // evm_increaseTime：偏移区块链的时间戳，但不挖矿（不产生新区块）
        await ethers.provider.send("evm_increaseTime", [seconds]);
        // evm_mine：强制挖一个新区块，让时间戳偏移生效
        // 为什么必须两个步骤？
        // → evm_increaseTime 只改了"下一区块的时间戳"，当前 block.timestamp 不变。
        //   evm_mine 创建新区块后才真正更新 block.timestamp。
        //   常见错误：只调 increaseTime 不调 mine → block.timestamp 不变 → 测试失败
        await ethers.provider.send("evm_mine", []);
    }

    // 为什么需要一个独立的 setup 函数而不是全放 beforeEach？
    // → 不同测试组需要不同的 Vesting 配置（线性/悬崖/可撤销），
    //   beforeEach 只能有一种配置。独立函数按需调用更灵活。
    async function setupVesting() {
        [owner, beneficiary1, beneficiary2, beneficiary3, outsider] =
            await ethers.getSigners();

        // 部署 MockERC20
        const MockToken = await ethers.getContractFactory("MockERC20");
        token = await MockToken.deploy();
        await token.mint(owner.address, ONE_TOKEN(1000000));

        // 部署 Vesting
        const VestingFactory = await ethers.getContractFactory("DeFiVesting");
        vesting = await VestingFactory.deploy(owner.address, await token.getAddress());

        // 为什么直接把所有代币 approve 给 Vesting？
        // → createSchedule 内部用 transferFrom 从 owner 拉取代币，
        //   需要 owner 先 approve。approve max 一次性省后续 gas。
        await token.approve(await vesting.getAddress(), ethers.MaxUint256);

        // 设置时间基准
        // 为什么用当前 block.timestamp 作为 START_TIME？
        // → 归属计划的 startTime 应该是"现在"或未来某个时间。
        //   从当前时间开始最容易计算：过 30 秒 = 过半个月
        const block = await ethers.provider.getBlock("latest");
        START_TIME = block!.timestamp + 10; // 10 秒后开始（给部署留缓冲）
        CLIFF_6M = START_TIME + ONE_MONTH * 6;
        END_24M = START_TIME + ONE_MONTH * 24;
        END_12M = START_TIME + ONE_MONTH * 12;
    }

    // ==================== 线性释放测试组 ====================

    describe("B. 线性释放（Linear Vesting）", function () {
        // beforeEach：每个测试前重新部署一套干净的合约 + 创建计划
        beforeEach(async function () {
            await setupVesting();

            // 给 beneficiary1 创建线性释放计划：1000 代币，无悬崖，24 个月
            // cliff = startTime — 表示无悬崖，从第一天开始线性释放
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(1000),    // 总归属 1000 tokens
                START_TIME,         // 开始时间
                START_TIME,         // 悬崖 = 开始时间（无悬崖）
                END_24M,            // 结束时间 = 24 个月后
                false               // 不可撤销（线性释放通常是不可撤销的）
            );
        });

        // ----- B1: 开始时刻释放为 0 -----
        it("B1. 开始时刻 → 可释放量为 0（时间还没流逝）", async function () {
            // 刚创建 plan，时间还没前进
            const releasable = await vesting.getReleasableAmount(beneficiary1.address);
            // 为什么是 >= 0 而不是 === 0？
            // → 如果 createSchedule 和执行之间正好过了几秒（在一个新区块），
            //   可释放量可能非常小（几秒的线性释放），所以用 >= 检查
            expect(releasable).to.equal(0n);
        });

        // ----- B2: 6 个月后释放 25% -----
        it("B2. 6 个月后 → 可释放约 25%（= 250 tokens）", async function () {
            // 快进 6 个月
            await increaseTime(ONE_MONTH * 6);

            const releasable = await vesting.getReleasableAmount(beneficiary1.address);

            // 实际流逝时间 = 360 秒（increaseTime）- 10 秒（START_TIME 缓冲）≈ 350 秒
            // 加上交易开销约 1-2 秒，所以 elapsed ≈ 351 秒
            // 1000 * 350/1440 ≈ 243 tokens，1000 * 351/1440 ≈ 243.75 tokens
            const block = await ethers.provider.getBlock("latest");
            const actualElapsed = BigInt(block!.timestamp - START_TIME);
            const duration = BigInt(ONE_MONTH * 24);
            const expected = (ONE_TOKEN(1000) * actualElapsed) / duration;
            // 误差范围：±1 token（约 ±10^18 wei）
            const tolerance = ONE_TOKEN(1);
            expect(releasable).to.be.closeTo(expected, tolerance);
        });

        // ----- B3: 12 个月后释放约 50% + release 后 updated = 0 -----
        it("B3. 12 个月后释放 50% → release 后再次查询为 0", async function () {
            await increaseTime(ONE_MONTH * 12);

            // 执行 release
            // 为什么 beneficiary1 自己调用 release？
            // → release 函数任何人都可以调（gas 友好设计），受益人自己调是最自然的场景
            await vesting.connect(beneficiary1).release(beneficiary1.address);

            // 释放后，可释放量应该归零（因为刚才全释放了）
            const afterRelease = await vesting.getReleasableAmount(beneficiary1.address);
            expect(afterRelease).to.equal(0n);

            // 验证受益人的代币余额增加了（约 500 tokens）
            const balance = await token.balanceOf(beneficiary1.address);

            const block = await ethers.provider.getBlock("latest");
            const actualElapsed = BigInt(block!.timestamp - START_TIME);
            const duration = BigInt(ONE_MONTH * 24);
            const expected = (ONE_TOKEN(1000) * actualElapsed) / duration;
            const tolerance = ONE_TOKEN(1);
            expect(balance).to.be.closeTo(expected, tolerance);

            // 验证 schedule 里 releasedAmount 已更新
            const schedule = await vesting.getSchedule(beneficiary1.address);
            expect(schedule.releasedAmount).to.be.closeTo(expected, tolerance);
        });

        // ----- B4: 24 个月后全部释放 -----
        it("B4. 24 个月后 → 可释放 100%（= 1000 tokens），vesting 结束", async function () {
            // 快进超过 24 个月
            await increaseTime(ONE_MONTH * 25);

            // 理论上应该是 100%（超过 endTime 不再增长）
            const releasable = await vesting.getReleasableAmount(beneficiary1.address);
            // 因为 elapsed 被 min(now, endTime) 截断，超过 endTime 后不再增长
            // 但之前如果有释放，releasedAmount 已经被减掉了
            expect(releasable).to.equal(ONE_TOKEN(1000));
        });

        // ----- B5: 连续多次 release -----
        it("B5. 分三次释放：6月→12月→24月，总释放 = 1000", async function () {
            // 第 1 次：6 个月
            await increaseTime(ONE_MONTH * 6);
            await vesting.connect(beneficiary1).release(beneficiary1.address);

            // 第 2 次：12 个月
            await increaseTime(ONE_MONTH * 6);
            await vesting.connect(beneficiary1).release(beneficiary1.address);

            // 第 3 次：确保超过 24 个月（END_24M）
            // 为什么不直接用 increaseTime(ONE_MONTH * 12)？
            // → 每次 release 交易消耗约 1 秒 + START_TIME 有 10 秒缓冲，
            //   3 次交易累积约 14-16 秒偏移，导致 720 秒不够准确到达 endTime。
            //   动态计算剩余时间保证一定超过 endTime，避免 vested < totalAmount
            const block2 = await ethers.provider.getBlock("latest");
            const remaining = END_24M - block2!.timestamp + ONE_MONTH; // 超过 endTime 至少 1 个月
            await increaseTime(remaining);
            await vesting.connect(beneficiary1).release(beneficiary1.address);

            // 最终余额 = 1000 tokens（超过 endTime 后归属 100%）
            const totalReleased = await token.balanceOf(beneficiary1.address);
            const tolerance = ONE_TOKEN(2); // 累积误差
            expect(totalReleased).to.be.closeTo(ONE_TOKEN(1000), tolerance);
        });
    });

    // ==================== 悬崖释放测试组 ====================
    describe("C. 悬崖释放（Cliff Vesting）", function () {
        beforeEach(async function () {
            await setupVesting();

            // 给 beneficiary2 创建悬崖释放计划：
            // 1000 代币，6 个月悬崖，24 个月线性
            await vesting.createSchedule(
                beneficiary2.address,
                ONE_TOKEN(1000),
                START_TIME,
                CLIFF_6M,           // 悬崖 6 个月
                END_24M,
                false
            );
        });

        // ----- C1: 悬崖前 → 0 -----
        it("C1. 悬崖前（第 5 个月）→ 可释放量为 0", async function () {
            // 快进 5 个月（不到 6 个月悬崖）
            await increaseTime(ONE_MONTH * 5);

            const releasable = await vesting.getReleasableAmount(beneficiary2.address);
            // 悬崖前：即使是线性释放，cliff 前也不释放任何代币
            expect(releasable).to.equal(0n);
        });

        // ----- C2: 悬崖日当天 → 释放 25%（补齐前 6 个月的线性释放）-----
        it("C2. 🔥 悬崖结束当天 → 一次性释放前 6 个月的累积（25%）", async function () {
            // 为什么这是一道高频面试题？
            // → 悬崖日的行为是面试最爱抠的细节：
            //   悬崖结束时一次性释放所有累积归属量（前 6 个月的 25%），
            //   不是从悬崖后才开始线性释放！

            // 为什么不直接用 increaseTime(ONE_MONTH * 6)？
            // → CLIFF_6M = START_TIME + 360 = (T + 10) + 360 = T + 370
            //   但 increaseTime(360) 只把 now 推到 T + ~363（beforeEach 消耗约 3 秒）
            //   363 < 370 → 还在悬崖前，releasable = 0！
            //   必须动态计算到悬崖的时间，确保 now >= CLIFF_6M
            const block0 = await ethers.provider.getBlock("latest");
            const toCliff = CLIFF_6M - block0!.timestamp + 1; // +1 确保恰好越过悬崖
            await increaseTime(toCliff > 0 ? toCliff : 1);

            const releasable = await vesting.getReleasableAmount(beneficiary2.address);
            const block = await ethers.provider.getBlock("latest");
            const actualElapsed = BigInt(block!.timestamp - START_TIME);
            const duration = BigInt(ONE_MONTH * 24);
            // 6/24 = 25% = 250 tokens（一次释放前 6 个月的累积）
            const expected = (ONE_TOKEN(1000) * actualElapsed) / duration;
            const tolerance = ONE_TOKEN(1);
            expect(releasable).to.be.closeTo(expected, tolerance);

            // 执行 release
            await vesting.connect(beneficiary2).release(beneficiary2.address);

            // 释放后马上再查 → 应该为 0（刚释放完）
            const afterRelease = await vesting.getReleasableAmount(beneficiary2.address);
            expect(afterRelease).to.equal(0n);
        });

        // ----- C3: 悬崖前一刻 vs 后一秒的边界 -----
        it("C3. 🔥 悬崖前 1 秒 ≠ 悬崖后 1 秒 — 面试最爱抠的边界", async function () {
            // 快进到悬崖前 1 秒
            // 为什么是 CLIFF_6M - START_TIME - 1？
            // → increaseTime 加的是"从当前时间往后"的秒数，
            //   START_TIME 是未来的时间（当前时间 + 10），CLIFF_6M = START_TIME + 6*60
            //   从当前时间到 cliff 前 1 秒 = CLIFF_6M - START_TIME - 1 + (START_TIME - now)
            //   简化：直接加到 cliff 前 1 秒
            const block = await ethers.provider.getBlock("latest");
            const toCliffMinus1 = CLIFF_6M - block!.timestamp - 1;
            await increaseTime(toCliffMinus1);

            const beforeCliff = await vesting.getReleasableAmount(beneficiary2.address);
            expect(beforeCliff).to.equal(0n); // 悬崖前：0

            // 快进 2 秒，刚好到悬崖后 1 秒
            await increaseTime(2);

            const afterCliff = await vesting.getReleasableAmount(beneficiary2.address);
            expect(afterCliff).to.be.gt(0n); // 悬崖后：> 0
        });

        // ----- C4: 悬崖后正常线性释放 -----
        it("C4. 悬崖后第 12 个月 → 约 50% 可释放", async function () {
            await increaseTime(ONE_MONTH * 12);

            const releasable = await vesting.getReleasableAmount(beneficiary2.address);
            const block = await ethers.provider.getBlock("latest");
            const actualElapsed = BigInt(block!.timestamp - START_TIME);
            const duration = BigInt(ONE_MONTH * 24);
            // 12/24 = 50%
            const expected = (ONE_TOKEN(1000) * actualElapsed) / duration;
            const tolerance = ONE_TOKEN(1);
            expect(releasable).to.be.closeTo(expected, tolerance);
        });
    });

    // ==================== 撤销测试组 ====================

    describe("D. 撤销（Revoke）", function () {
        beforeEach(async function () {
            await setupVesting();

            // 给 beneficiary3 创建可撤销的悬崖计划
            await vesting.createSchedule(
                beneficiary3.address,
                ONE_TOKEN(1000),
                START_TIME,
                CLIFF_6M,       // 6 个月悬崖
                END_24M,
                true            // 可撤销！
            );
        });

        // ----- D1: 不可撤销的计划不能被撤销 -----
        it("D1. 不可撤销计划 → revoke 应该 revert", async function () {
            // beneficiary1 在上面的测试中有不可撤销的计划
            // 但这里我们在 beforeEach 中只创建了 beneficiary3 的计划，
            // 所以需要先创建一个不可撤销的计划
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(500),
                START_TIME,
                START_TIME,     // 无悬崖
                END_12M,
                false           // 不可撤销
            );

            // 尝试撤销不可撤销的计划 → 应该 revert
            await expect(
                vesting.revoke(beneficiary1.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__NotRevocable");
        });

        // ----- D2: 悬崖前撤销 → 全部代币返还 owner -----
        it("D2. 🔥 悬崖前撤销 → 未释放代币全部返还 owner（代币一分没给受益人）", async function () {
            // 只过 1 个月，在悬崖前
            await increaseTime(ONE_MONTH * 1);

            // 记录撤销前 owner 的代币余额
            const ownerBalanceBefore = await token.balanceOf(owner.address);

            // 执行撤销
            await vesting.revoke(beneficiary3.address);

            // owner 余额增加 = 1000 tokens（全部返还，因为悬崖前一毛钱都没释放）
            const ownerBalanceAfter = await token.balanceOf(owner.address);
            const diff = ownerBalanceAfter - ownerBalanceBefore;
            expect(diff).to.equal(ONE_TOKEN(1000));

            // 撤销后，受益人不能再 release
            await expect(
                vesting.connect(beneficiary3).release(beneficiary3.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__NothingToRelease");

            // schedule.revoked = true
            const schedule = await vesting.getSchedule(beneficiary3.address);
            expect(schedule.revoked).to.equal(true);
        });

        // ----- D3: 悬崖后撤销 → 已归属的归受益人，未归属的返还 owner -----
        it("D3. 🔥 悬崖后第 12 个月撤销 → 50% 归受益人（已释放+可释放=归属），50% 返还 owner", async function () {
            // 快进 12 个月（过了悬崖 6 个月 + 6 个月线性）
            await increaseTime(ONE_MONTH * 12);

            // 先让受益人提取一次（把已归属的领走）
            await vesting.connect(beneficiary3).release(beneficiary3.address);
            const beneficiaryBalanceBefore = await token.balanceOf(beneficiary3.address);

            // 记录 owner 余额
            const ownerBalanceBefore = await token.balanceOf(owner.address);

            // 撤销
            await vesting.revoke(beneficiary3.address);

            // 受益人的余额不变（已释放的拿不回来）
            const beneficiaryBalanceAfter = await token.balanceOf(beneficiary3.address);
            expect(beneficiaryBalanceAfter).to.equal(beneficiaryBalanceBefore);

            // owner 收到约 50% 的返还
            const ownerBalanceAfter = await token.balanceOf(owner.address);
            const refund = ownerBalanceAfter - ownerBalanceBefore;
            // 返还应该接近 500 tokens
            const expected = ONE_TOKEN(500);
            const tolerance = ONE_TOKEN(5);
            expect(refund).to.be.closeTo(expected, tolerance);
        });

        // ----- D4: 非 owner 不能撤销 -----
        it("D4. 非 owner 调用 revoke → revert", async function () {
            // outsider 不是 owner
            await expect(
                vesting.connect(outsider).revoke(beneficiary3.address)
            ).to.be.revert(ethers); // Ownable 的 onlyOwner 会 revert
        });

        // ----- D5: 重复撤销 -----
        it("D5. 重复撤销同一个计划 → revert", async function () {
            await vesting.revoke(beneficiary3.address);

            // 第二次撤销同一个
            await expect(
                vesting.revoke(beneficiary3.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__AlreadyRevoked");
        });
    });

    // ==================== 批量操作测试组 ====================

    describe("E. 批量操作（Batch Release + Batch Revoke）", function () {
        beforeEach(async function () {
            await setupVesting();

            // 创建 3 个受益人计划
            // beneficiary1: 无悬崖 + 不可撤销
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(500),
                START_TIME,
                START_TIME,
                END_12M,
                false
            );

            // beneficiary2: 悬崖 + 可撤销
            await vesting.createSchedule(
                beneficiary2.address,
                ONE_TOKEN(800),
                START_TIME,
                CLIFF_6M,
                END_24M,
                true
            );

            // beneficiary3: 无悬崖 + 可撤销
            await vesting.createSchedule(
                beneficiary3.address,
                ONE_TOKEN(300),
                START_TIME,
                START_TIME,
                END_12M,
                true
            );
        });

        // ----- E1: 批量释放 -----
        it("E1. 批量释放 — 12 个月后所有受益人的可释放量都应该被提取", async function () {
            await increaseTime(ONE_MONTH * 12);

            // 批量释放 3 个受益人
            await vesting.batchRelease([
                beneficiary1.address,
                beneficiary2.address,
                beneficiary3.address,
            ]);

            // 所有人都应该有代币到账
            const bal1 = await token.balanceOf(beneficiary1.address);
            const bal2 = await token.balanceOf(beneficiary2.address);
            const bal3 = await token.balanceOf(beneficiary3.address);

            expect(bal1).to.be.gt(0n);
            expect(bal2).to.be.gt(0n);
            expect(bal3).to.be.gt(0n);

            // 每个人释放后，可释放量应该归零
            expect(await vesting.getReleasableAmount(beneficiary1.address)).to.equal(0n);
            expect(await vesting.getReleasableAmount(beneficiary2.address)).to.equal(0n);
            expect(await vesting.getReleasableAmount(beneficiary3.address)).to.equal(0n);
        });

        // ----- E2: 批量释放含空数组 -----
        it("E2. 批量释放空数组 → 不 revert，不释放任何代币", async function () {
            await increaseTime(ONE_MONTH * 12);

            // 传空数组 — 应该是 no-op，不 revert
            await vesting.batchRelease([]);

            // 确认没人收到代币（因为没人调 release）
            const bal1 = await token.balanceOf(beneficiary1.address);
            expect(bal1).to.equal(0n);
        });

        // ----- E3: 批量释放跳过 0 地址 -----
        it("E3. 批量释放 — 数组中混入零地址，应跳过而不回滚整批", async function () {
            await increaseTime(ONE_MONTH * 12);

            // 零地址放中间，测试跳过逻辑
            await vesting.batchRelease([
                beneficiary1.address,
                "0x0000000000000000000000000000000000000000", // 零地址 → 跳过
                beneficiary3.address,
            ]);

            // beneficiary1 和 beneficiary3 应该释放了
            const bal1 = await token.balanceOf(beneficiary1.address);
            const bal3 = await token.balanceOf(beneficiary3.address);
            expect(bal1).to.be.gt(0n);
            expect(bal3).to.be.gt(0n);
        });

        // ----- E4: 批量撤销 -----
        it("E4. 批量撤销 — 撤销所有可撤销计划，不可撤销的应被跳过", async function () {
            await increaseTime(ONE_MONTH * 1);

            const ownerBalBefore = await token.balanceOf(owner.address);

            // 批量撤销所有三个
            await vesting.batchRevoke([
                beneficiary1.address,  // 不可撤销
                beneficiary2.address,  // 可撤销
                beneficiary3.address,  // 可撤销
            ]);

            // beneficiary1 不应该被撤销（不可撤销的 plan 会被跳过）
            const schedule1 = await vesting.getSchedule(beneficiary1.address);
            expect(schedule1.revoked).to.equal(false);

            // beneficiary2 和 beneficiary3 应该被撤销
            const schedule2 = await vesting.getSchedule(beneficiary2.address);
            expect(schedule2.revoked).to.equal(true);
            const schedule3 = await vesting.getSchedule(beneficiary3.address);
            expect(schedule3.revoked).to.equal(true);
        });
    });

    // ==================== 边界条件测试组 ====================

    describe("F. 边界条件与错误处理", function () {
        beforeEach(async function () {
            await setupVesting();
        });

        // ----- F1: 零地址受益人 -----
        it("F1. 零地址受益人 → revert Vesting__ZeroAddress", async function () {
            await expect(
                vesting.createSchedule(
                    "0x0000000000000000000000000000000000000000",
                    ONE_TOKEN(100),
                    START_TIME,
                    START_TIME,
                    END_12M,
                    false
                )
            ).to.be.revertedWithCustomError(vesting, "Vesting__ZeroAddress");
        });

        // ----- F2: 零金额 -----
        it("F2. 归属总量为 0 → revert Vesting__ZeroAmount", async function () {
            await expect(
                vesting.createSchedule(
                    beneficiary1.address,
                    0,
                    START_TIME,
                    START_TIME,
                    END_12M,
                    false
                )
            ).to.be.revertedWithCustomError(vesting, "Vesting__ZeroAmount");
        });

        // ----- F3: cliff < startTime -----
        it("F3. 悬崖时间早于开始时间 → revert Vesting__InvalidTimeRange", async function () {
            await expect(
                vesting.createSchedule(
                    beneficiary1.address,
                    ONE_TOKEN(100),
                    START_TIME,
                    START_TIME - 10,   // cliff 在 start 之前，非法
                    END_12M,
                    false
                )
            ).to.be.revertedWithCustomError(vesting, "Vesting__InvalidTimeRange");
        });

        // ----- F4: endTime < cliff -----
        it("F4. 结束时间早于悬崖 → revert Vesting__InvalidTimeRange", async function () {
            await expect(
                vesting.createSchedule(
                    beneficiary1.address,
                    ONE_TOKEN(100),
                    START_TIME,
                    CLIFF_6M,          // cliff = 6 个月
                    CLIFF_6M - 10,     // end 在 cliff 之前，非法
                    false
                )
            ).to.be.revertedWithCustomError(vesting, "Vesting__InvalidTimeRange");
        });

        // ----- F5: 重复计划 -----
        it("F5. 同一受益人创建两个计划 → revert Vesting__ScheduleAlreadyExists", async function () {
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(100),
                START_TIME,
                START_TIME,
                END_12M,
                false
            );

            // 同一地址再创建
            await expect(
                vesting.createSchedule(
                    beneficiary1.address,
                    ONE_TOKEN(200),
                    START_TIME,
                    START_TIME,
                    END_12M,
                    false
                )
            ).to.be.revertedWithCustomError(vesting, "Vesting__ScheduleAlreadyExists");
        });

        // ----- F6: 查询不存在的计划 -----
        it("F6. 查询不存在的计划 → revert Vesting__NoSchedule", async function () {
            await expect(
                vesting.getReleasableAmount(outsider.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__NoSchedule");
        });

        // ----- F7: 释放不存在计划的用户 -----
        it("F7. 对无计划的地址 release → revert", async function () {
            await expect(
                vesting.release(outsider.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__NoSchedule");
        });

        // ----- F8: 无代币可释放时 release -----
        it("F8. 刚创建计划时 release → revert Vesting__NothingToRelease", async function () {
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(100),
                START_TIME,
                START_TIME,
                END_12M,
                false
            );

            // 刚创建，时间还没走，可释放 = 0
            await expect(
                vesting.release(beneficiary1.address)
            ).to.be.revertedWithCustomError(vesting, "Vesting__NothingToRelease");
        });

        // ----- F9: endTime = cliff（纯悬崖，无线性） -----
        it("F9. endTime == cliff → 纯悬崖释放模式（悬崖结束 = 100% 释放）", async function () {
            // endTime == cliff：无线性阶段，悬崖结束 = 全部释放
            const PURE_CLIFF = START_TIME + ONE_MONTH * 12;
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(500),
                START_TIME,
                PURE_CLIFF,         // cliff = 12 个月
                PURE_CLIFF,         // endTime = 12 个月（悬崖 = 结束，无线性阶段）
                false
            );

            // 悬崖前
            await increaseTime(ONE_MONTH * 11);
            expect(await vesting.getReleasableAmount(beneficiary1.address)).to.equal(0n);

            // 悬崖后
            await increaseTime(ONE_MONTH * 2); // 总共过了 13 个月
            // 超过 endTime 后 100% 可释放
            expect(await vesting.getReleasableAmount(beneficiary1.address)).to.equal(ONE_TOKEN(500));
        });
    });

    // ==================== 面试题测试组 ====================

    describe("G. 🔥 面试高频考点", function () {
        beforeEach(async function () {
            await setupVesting();
        });

        // ----- G1: vesting 后时间超过 endTime 归属不会再增长 -----
        it("G1. 🔥 超过 endTime 后，可释放量被 cap 在 totalAmount，不会无限增长", async function () {
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(1000),
                START_TIME,
                START_TIME,
                END_12M,
                false
            );

            // 快进远超 endTime（100 个月）
            await increaseTime(ONE_MONTH * 100);

            const releasable = await vesting.getReleasableAmount(beneficiary1.address);
            // min(now, endTime) 确保了 capped at totalAmount
            expect(releasable).to.equal(ONE_TOKEN(1000));
        });

        // ----- G2: 悬崖设计的意义 — 团队跑路场景 -----
        it("G2. 🔥 悬崖前撤销 → owner 拿回全部代币（团队跑路保护）", async function () {
            await vesting.createSchedule(
                beneficiary1.address,
                ONE_TOKEN(10000),
                START_TIME,
                CLIFF_6M,       // 6 个月悬崖
                END_24M,
                true            // 可撤销
            );

            // 模拟：团队成员 3 个月后离职
            await increaseTime(ONE_MONTH * 3);

            // 撤销 — 100% 返还给 owner
            const ownerBefore = await token.balanceOf(owner.address);
            await vesting.revoke(beneficiary1.address);
            const ownerAfter = await token.balanceOf(owner.address);

            // owner 拿回 10000 tokens
            expect(ownerAfter - ownerBefore).to.equal(ONE_TOKEN(10000));

            // 离职成员余额 = 0（因为悬崖前还没释放过）
            const quitterBalance = await token.balanceOf(beneficiary1.address);
            expect(quitterBalance).to.equal(0n);
        });
    });

    // ==================== 工厂测试 ====================

    describe("H. VestingFactory", function () {
        beforeEach(async function () {
            [owner] = await ethers.getSigners();

            // 部署代币
            const MockToken = await ethers.getContractFactory("MockERC20");
            token = await MockToken.deploy();
            await token.mint(owner.address, ONE_TOKEN(1000000));

            // 部署工厂
            const FactoryFactory = await ethers.getContractFactory("VestingFactory");
            factory = await FactoryFactory.deploy(owner.address);
        });

        // ----- H1: 通过工厂创建 Vesting 合约 -----
        it("H1. createVesting → 成功部署新 Vesting 合约，事件正确", async function () {
            // 为什么 salt 用 1？
            // → salt 是 CREATE2 的参数，只要是唯一值即可。实际项目用 keccak256(teamName) 或递增 ID
            const salt = 1;

            // 注意：createVesting 内部会用 transferFrom，需要先 approve 给 VestingFactory？
            // 不对！createVesting 部署的是新 Vesting 合约，代币 approve 需要给新 Vesting 合约。
            // 在测试中我们先部署，然后手动 approve 和创建 schedule。
            // 工厂只负责部署 Vesting 合约，不负责创建 schedule。
            const tx = await factory.createVesting(await token.getAddress(), salt);
            const receipt = await tx.wait();

            // 工厂应该记录了 1 个 Vesting 合约
            expect(await factory.getVestingCount()).to.equal(1n);

            // 获取部署的 Vesting 地址
            const vestingAddr = await factory.getVestingAt(0);
            expect(vestingAddr).to.not.equal("0x0000000000000000000000000000000000000000");

            // 用这个地址创建 Vesting 实例
            const VestingContract = await ethers.getContractFactory("DeFiVesting");
            const deployedVesting = VestingContract.attach(vestingAddr);

            // 验证 Vesting 的 owner 是工厂的 owner
            expect(await deployedVesting.owner()).to.equal(owner.address);
            expect(await deployedVesting.token()).to.equal(await token.getAddress());
        });

        // ----- H2: 预测地址 -----
        it("H2. 🔥 predictVestingAddress → 预测的地址与部署后的实际地址一致（CREATE2 特性）", async function () {
            const salt = 42;
            const tokenAddr = await token.getAddress();

            // 先预测
            const predicted = await factory.predictVestingAddress(tokenAddr, salt);

            // 再部署
            await factory.createVesting(tokenAddr, salt);
            const actual = await factory.getVestingAt(0);

            // CREATE2：预测地址 = 实际地址 ✅
            expect(predicted).to.equal(actual);
        });

        // ----- H3: 无效代币地址 -----
        it("H3. 传入 EOA 地址作为代币 → revert VestingFactory__InvalidToken", async function () {
            // outsider 是一个 EOA，不是合约，code.length = 0
            await expect(
                factory.createVesting(outsider.address, 999)
            ).to.be.revertedWithCustomError(factory, "VestingFactory__InvalidToken");
        });

        // ----- H4: 多个 Vesting -----
        it("H4. 创建多个 Vesting → getAllVestingContracts 返回所有", async function () {
            const tokenAddr = await token.getAddress();

            // 创建 3 个 Vesting（用不同 salt）
            await factory.createVesting(tokenAddr, 1);
            await factory.createVesting(tokenAddr, 2);
            await factory.createVesting(tokenAddr, 3);

            expect(await factory.getVestingCount()).to.equal(3n);

            const all = await factory.getAllVestingContracts();
            expect(all.length).to.equal(3);
            // 3 个地址应该都不相同（不同 salt → 不同 CREATE2 地址）
            expect(all[0]).to.not.equal(all[1]);
            expect(all[1]).to.not.equal(all[2]);
        });
    });
});