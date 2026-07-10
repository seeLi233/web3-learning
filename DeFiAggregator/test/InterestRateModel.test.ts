import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

// ============ 精度常量 ============
const RAY = 10n ** 27n;
const WAD = 10n ** 18n;

// ============ 测试参数 (以 RAY 表示) ============
// baseRate = 2%, slope1 = 10%, slope2 = 50%, optimal = 80%, reserveFactor = 10%
const BASE_RATE = (RAY * 2n) / 100n;          // 0.02 RAY
const SLOPE_1 = (RAY * 10n) / 100n;            // 0.10 RAY
const SLOPE_2 = (RAY * 50n) / 100n;            // 0.50 RAY
const OPTIMAL = (RAY * 80n) / 100n;            // 0.80 RAY
const RESERVE_FACTOR = (RAY * 10n) / 100n;     // 0.10 RAY

describe("📊 InterestRateModel — 可变利率模型", function () {
    let model: any;

    beforeEach(async function () {
       model = await ethers.deployContract("InterestRateModel", [BASE_RATE, SLOPE_1, SLOPE_2, OPTIMAL, RESERVE_FACTOR]); 
    });

    // ==================== 利用率测试 ====================

    describe("getUtilizationRate", function () {
        it("borrows == 0 → U = 0%", async () => {
            const u = await model.getUtilizationRate(WAD * 100n, 0n, 0n);
            expect(u).to.equal(0n);
        });

        it("cash=20, borrows=80 → U = 80%", async () => {
            // 总流动性 = 20 + 80 = 100，U = 80/100 = 80%
            const u = await model.getUtilizationRate(WAD * 20n, WAD * 80n, 0n);
            // 期望: 0.80 RAY
            expect(u).to.equal((RAY * 80n) / 100n);
        });

        it("cash=5, borrows=95 → U = 95%", async () => {
            const u = await model.getUtilizationRate(WAD * 5n, WAD * 95n, 0n);
            // 期望: 0.95 RAY
            expect(u).to.equal((RAY * 95n) / 100n);
        });

        it("cash=0, borrows=100 → U = 100% (极端情况)", async () => {
            const u = await model.getUtilizationRate(0n, WAD * 100n, 0n);
            expect(u).to.equal(RAY); // 100%
        });

        it("cash=0, borrows=0 → U = 0% (空池)", async () => {
            const u = await model.getUtilizationRate(0n, 0n, 0n);
            expect(u).to.equal(0n);
        });
    });

    // ==================== 借款利率测试 ====================

    describe("borrowRate — 拐点计算", function () {
        it("U=0% → borrowRate = baseRate = 2%", async () => {
            const [borrowRate] = await model.getRates(WAD * 100n, 0n, 0n);
            expect(borrowRate).to.equal(BASE_RATE);
        });

        it("U=50% (低于拐点) → borrowRate = 2% + 50%×10% = 7%", async () => {
            const [borrowRate] = await model.getRates(WAD * 50n, WAD * 50n, 0n);
            // 期望: 0.07 RAY
            expect(borrowRate).to.equal((RAY * 7n) / 100n);
        });

        it("U=80% (刚好拐点) → borrowRate = 2% + 80%×10% = 10%", async () => {
            const [borrowRate] = await model.getRates(WAD * 20n, WAD * 80n, 0n);
            // 期望: 0.10 RAY
            expect(borrowRate).to.equal((RAY * 10n) / 100n);
        });

        it("U=95% (超过拐点) → borrowRate = 2% + 8% + 15%×50% = 17.5%", async () => {
            const [borrowRate] = await model.getRates(WAD * 5n, WAD * 95n, 0n);
            // 期望: 0.175 RAY
            expect(borrowRate).to.equal((RAY * 175n) / 1000n);
        });

        it("U=100% → 最高利率 = 2% + 8% + 20%×50% = 20%", async () => {
            const [borrowRate] = await model.getRates(0n, WAD * 100n, 0n);
            // 期望: 0.20 RAY
            expect(borrowRate).to.equal((RAY * 20n) / 100n);
        });
    });

    // ==================== 存款利率测试 ====================

    describe("supplyRate — 存款利率", function () {
        it("U=50%, borrowRate=7%, reserveFactor=10% → supplyRate = 7%×50%×90% = 3.15%", async () => {
            const [, supplyRate] = await model.getRates(WAD * 50n, WAD * 50n, 0n);
            // 期望: 0.0315 RAY
            const expected = (RAY * 315n) / 10000n;
            expect(supplyRate).to.equal(expected);
        });

        it("U=80%, borrowRate=10% → supplyRate = 10%×80%×90% = 7.2%", async () => {
            const [, supplyRate] = await model.getRates(WAD * 20n, WAD * 80n, 0n);
            // 期望: 0.072 RAY
            const expected = (RAY * 72n) / 1000n;
            expect(supplyRate).to.equal(expected);
        });

        it("U=95%, borrowRate=17.5% → supplyRate = 17.5%×95%×90% ≈ 14.96%", async () => {
            const [, supplyRate] = await model.getRates(WAD * 5n, WAD * 95n, 0n);
            // 手动计算: 17.5% × 95% × 90% = 14.9625%
            // 实际会有一点精度误差，用近似断言
            const expected = (RAY * 149625n) / 1000000n; // 0.149625 RAY
            // 误差在 1 RAY 以内（即 1e-27 精度）
            const diff = supplyRate > expected ? supplyRate - expected : expected - supplyRate;
            expect(diff).to.be.lte(RAY / 1000000n); // 允许极小误差
        });
    });

    // ==================== 模拟测试 ====================

    describe("simulateBorrow / simulateRepay", function () {
        it("借出前 vs 借出后 — 利率应上升", async () => {
            // 当前: cash=50, borrows=50 (U=50%)
            const [beforeRate] = await model.getRates(WAD * 50n, WAD * 50n, 0n);

            // 借出 30 → cash=20, borrows=80 (U=80%)
            const [afterRate] = await model.simulateBorrow(WAD * 50n, WAD * 50n, WAD * 30n);

            // U 从 50% → 80%，利率应该上升
            expect(afterRate).to.be.gt(beforeRate);
        });

        it("还款后 — 利率应下降", async function() {
            // 当前: cash=20, borrows=80 (U=80%)
            const [beforeRate] = await model.getRates(WAD * 20n, WAD * 80n, 0n);

            // 还 40 → cash=60, borrows=40 (U=40%)
            const [afterRate] = await model.simulateRepay(WAD * 20n, WAD * 80n, WAD * 40n);

            // U 从 80% → 40%，利率应该下降
            expect(afterRate).to.be.lt(beforeRate);
        });

        it("还清所有借款 → 利率回到 baseRate", async function() {
            const [afterRate] = await model.simulateRepay(WAD * 20n, WAD * 80n, WAD * 80n);
            // 还款后 borrows=0，U=0%
            expect(afterRate).to.equal(BASE_RATE);
        });
    });

    // ==================== 边界条件 ====================

    describe("边界条件", function () {
        it("borrows 远超 cash — 不应 revert，应合理计算", async () => {
            // 如果 borrows 远超 cash（实际协议中不应发生，但合约要健壮）
            // U = 200/(10+200) ≈ 95.2%，利率应大于 baseRate
            const [borrowRate] = await model.getRates(WAD * 10n, WAD * 200n, 0n);
            expect(borrowRate).to.be.gt(BASE_RATE);
        });

        it("极大值 — cash 和 borrows 为 type(uint128).max", async () => {
            const big = 2n ** 128n - 1n;
            // U = big / (big + big) = 50%，利率应为 7%
            const [borrowRate] = await model.getRates(big, big, 0n);
            expect(borrowRate).to.equal((RAY * 7n) / 100n);
        });

        it("reserveFactor=100% 极端情况", async () => {
            const extremeModel = await ethers.deployContract("InterestRateModel", [
                BASE_RATE,
                SLOPE_1,
                SLOPE_2,
                OPTIMAL,
                RAY // reserveFactor = 100%
            ]);
            const [, supplyRate] = await extremeModel.getRates(WAD * 50n, WAD * 50n, 0n);
            // reserveFactor=100% → 存款利率应为 0
            expect(supplyRate).to.equal(0n);
        });
    });
});

describe("📊 StableRateModel — 稳定利率模型", function () {
    const STABLE_PREMIUM = (RAY * 2n) / 100n;        // 2% 溢价
    const REBALANCE_THRESHOLD = (RAY * 95n) / 100n;  // 95% 阈值
    const STABLE_CAP = (RAY * 30n) / 100n;           // 30% 上限

    let stableModel: any; // StableRateModel

    beforeEach(async function () {
        stableModel = await ethers.deployContract("StableRateModel", [
            BASE_RATE,
            SLOPE_1,
            SLOPE_2,
            OPTIMAL,
            RESERVE_FACTOR,
            STABLE_PREMIUM,
            REBALANCE_THRESHOLD,
            STABLE_CAP
        ]);
    });

    it("稳定利率 = 可变利率 + 溢价", async () => {
        // U=50% → 可变利率=7%，稳定利率=7%+2%=9%
        const stableRate = await stableModel.getStableRate(WAD * 50n, WAD * 50n, 0n);
        const expected = (RAY * 9n) / 100n; // 9%
        expect(stableRate).to.equal(expected);
    });

    it("稳定利率不超过 cap", async () => {
        // U=100% → 可变利率=20%，+2%溢价=22%，未超过30%上限，但也不应无限增长
        const stableRate = await stableModel.getStableRate(0n, WAD * 100n, 0n);
        expect(stableRate).to.be.lte(STABLE_CAP);
    });

    it("利用率 < 95% 时不需要 rebalance", async () => {
        const [needsRebalance] = await stableModel.checkRebalance(
            WAD * 50n,  // cash=50
            WAD * 50n,  // borrows=50 → U=50%
            (RAY * 9n) / 100n  // 锁定在 9%
        );
        expect(needsRebalance).to.be.false;
    });

    it("利用率 < 95% 时不需要 rebalance", async () => {
        const [needsRebalance] = await stableModel.checkRebalance(
            WAD * 50n,  // cash=50
            WAD * 50n,  // borrows=50 → U=50%
            (RAY * 9n) / 100n  // 锁定在 9%
        );
        expect(needsRebalance).to.be.false;
    });

    it("利用率 > 95% 且新利率 > 锁定利率时触发 rebalance", async () => {
        const [needsRebalance, newRate] = await stableModel.checkRebalance(
            WAD * 4n,    // cash=4
            WAD * 96n,   // borrows=96 → U=96% > 95% threshold
            (RAY * 5n) / 100n  // 锁定在 5%（远低于当前）
        );
        expect(needsRebalance).to.be.true;
        expect(newRate).to.be.gt((RAY * 5n) / 100n);
    });

    it("getAllRates 返回所有利率信息", async () => {
        const [varRate, stableRate, supplyRate, utilRate] =
            await stableModel.getAllRates(WAD * 50n, WAD * 50n, 0n);

        expect(varRate).to.equal((RAY * 7n) / 100n);     // 7%
        expect(stableRate).to.equal((RAY * 9n) / 100n);   // 9%
        expect(utilRate).to.equal(RAY / 2n);               // 50%
        // supplyRate 也应该是合理值
        expect(supplyRate).to.be.gt(0n);
    });
});