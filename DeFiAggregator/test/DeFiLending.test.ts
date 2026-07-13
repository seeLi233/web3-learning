import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ============ 精度常量 ============
const RAY = 10n ** 27n;
const WAD = 10n ** 18n;

// ============ 利率模型参数 ============
// base=2%, slope1=10%, slope2=50%, optimal=80%, reserveFactor=10%
const BASE_RATE = (RAY * 2n) / 100n;
const SLOPE_1 = (RAY * 10n) / 100n;
const SLOPE_2 = (RAY * 50n) / 100n;
const OPTIMAL = (RAY * 80n) / 100n;
const RESERVE_FACTOR = (RAY * 10n) / 100n;

// ============ 借贷风险参数 ============
const LTV = (RAY * 75n) / 100n;                    // 75%
const LIQUIDATION_THRESHOLD = (RAY * 80n) / 100n;   // 80%
const LIQUIDATION_BONUS = (RAY * 5n) / 100n;        // 5%

describe("🏦 DeFiLending — 借贷协议核心", function () {
    let lending: any;
    let rateModel: any;
    let tokenA: any; // 用作抵押品
    let tokenB: any; // 用作借款资产
    let owner: any, user1: any, user2: any;

    beforeEach(async function () {
        [owner, user1, user2] = await ethers.getSigners();

        // 1. 部署利率模型
        const RateModel = await ethers.getContractFactory("InterestRateModel");
        rateModel = await RateModel.deploy(BASE_RATE, SLOPE_1, SLOPE_2, OPTIMAL, RESERVE_FACTOR);

        // 2. 部署两个测试代币
        const MockToken = await ethers.getContractFactory("MockToken");
        tokenA = await MockToken.deploy("Token A", "TKA", 18);
        tokenB = await MockToken.deploy("Token B", "TKB", 18);

        // 3. 部署借贷协议
        const Lending = await ethers.getContractFactory("DeFiLending");
        lending = await Lending.deploy();

        // 4. 初始化两个资产池（用同一个利率模型，生产环境不同资产用不同模型）
        await lending.initReserve(
            await tokenA.getAddress(),
            await rateModel.getAddress(),
            LTV,
            LIQUIDATION_THRESHOLD,
            LIQUIDATION_BONUS,
            RESERVE_FACTOR
        );
        await lending.initReserve(
            await tokenB.getAddress(),
            await rateModel.getAddress(),
            LTV,
            LIQUIDATION_THRESHOLD,
            LIQUIDATION_BONUS,
            RESERVE_FACTOR
        );

        // 5. 给用户铸造测试代币
        await tokenA.mint(user1.address, WAD * 10000n);
        await tokenB.mint(user1.address, WAD * 10000n);
        await tokenA.mint(user2.address, WAD * 10000n);
        await tokenB.mint(user2.address, WAD * 10000n);

        // 6. 用户授权借贷合约
        await tokenA.connect(user1).approve(await lending.getAddress(), ethers.MaxUint256);
        await tokenB.connect(user1).approve(await lending.getAddress(), ethers.MaxUint256);
        await tokenA.connect(user2).approve(await lending.getAddress(), ethers.MaxUint256);
        await tokenB.connect(user2).approve(await lending.getAddress(), ethers.MaxUint256);
    });

    // ==================== 存款测试 ====================

    describe("📥 deposit — 存款", function () {
        it("成功存入资产，更新仓位和总流动性", async function () {
            const depositAmount = WAD * 1000n; // 1000 TKA

            await lending.connect(user1).deposit(await tokenA.getAddress(), depositAmount);

            // 验证合约收到代币
            expect(await tokenA.balanceOf(await lending.getAddress())).to.equal(depositAmount);

            // 验证用户仓位
            const position = await lending.getUserPosition(user1.address, await tokenA.getAddress());
            expect(position.supplyAmount).to.equal(depositAmount);

            // 验证全局状态
            const config = await lending.getReserveConfig(await tokenA.getAddress());
            expect(config.totalLiquidity).to.equal(depositAmount);
        });

        it("多次存款，利息结算正确", async function () {
            const tokenAAddr = await tokenA.getAddress();

            // 第一次存款: 1000 TKA
            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);

            // 模拟时间流逝（先要有借款产生利息，这里先验证多次存款本身）
            // 第二次存款: 500 TKA
            await lending.connect(user1).deposit(tokenAAddr, WAD * 500n);

            const position = await lending.getUserPosition(user1.address, tokenAAddr);
            // 没有借款时利率为 0，所以 supplyAmount = 1000 + 500 = 1500
            expect(position.supplyAmount).to.equal(WAD * 1500n);
        });

        it("存款金额为 0 → revert", async function () {
            await expect(
                lending.connect(user1).deposit(await tokenA.getAddress(), 0n)
            ).to.be.revertedWith("Amount must be > 0");
        });

        it("未初始化的资产 → revert", async function () {
            const randomAddr = "0x" + "1".repeat(40);
            await expect(
                lending.connect(user1).deposit(randomAddr, WAD * 100n)
            ).to.be.revertedWith("Reserve not active");
        });
    });

    // ==================== 借款测试 ====================

    describe("📤 borrow — 借款", function () {
        beforeEach(async function () {
            // user1 先存入抵押品
            await lending.connect(user1).deposit(await tokenA.getAddress(), WAD * 1000n);
        });

        it("有足够抵押品 → 成功借款", async function () {
            // user2 提供 TKB 流动性
            await lending.connect(user2).deposit(await tokenB.getAddress(), WAD * 1000n);
            // 抵押品价值: 1000 TKA, LTV=75%, 最多借 750 TKB
            const borrowAmount = WAD * 500n;

            await lending.connect(user1).borrow(await tokenB.getAddress(), borrowAmount);

            // 验证用户收到代币（borrow 从池子转给用户，用户余额增加）
            expect(await tokenB.balanceOf(user1.address)).to.equal(WAD * 10500n); // 10000 + 500

            // 验证债务记录
            const position = await lending.getUserPosition(user1.address, await tokenB.getAddress());
            expect(position.borrowAmount).to.equal(borrowAmount);
        });

        it("超过 LTV → revert", async function () {
            // user2 提供 TKB 流动性（需要先通过流动性检查才能走到抵押检查）
            await lending.connect(user2).deposit(await tokenB.getAddress(), WAD * 1000n);
            // 抵押品 1000 TKA, LTV 75%, 最多借 750 TKB
            const tooMuch = WAD * 800n;

            await expect(
                lending.connect(user1).borrow(await tokenB.getAddress(), tooMuch)
            ).to.be.revertedWith("Insufficient collateral");
        });

        it("池子流动性不足 → revert", async function () {
            // user2 只存 100 TKB，但 user1 尝试借 500 TKB
            await lending.connect(user2).deposit(await tokenB.getAddress(), WAD * 100n);

            await expect(
                lending.connect(user1).borrow(await tokenB.getAddress(), WAD * 500n)
            ).to.be.revertedWith("Insufficient liquidity");
        });

        it("无抵押品 → revert", async function () {
            // user2 提供 TKB 流动性（让 user2 的借款通过流动性检查，但卡在抵押检查）
            await lending.connect(user1).deposit(await tokenB.getAddress(), WAD * 100n);
            // user2 没有存款，直接尝试借款
            await expect(
                lending.connect(user2).borrow(await tokenB.getAddress(), WAD * 100n)
            ).to.be.revertedWith("Insufficient collateral");
        });
    });

    // ==================== 还款测试 ====================

    describe("💰 repay — 还款", function () {
        beforeEach(async function () {
            // user2 提供恰好 500 TKB 流动性（方便验证 totalLiquidity 变化）
            await lending.connect(user2).deposit(await tokenB.getAddress(), WAD * 500n);
            // user1 存抵押品 + 借款
            await lending.connect(user1).deposit(await tokenA.getAddress(), WAD * 1000n);
            await lending.connect(user1).borrow(await tokenB.getAddress(), WAD * 500n);
        });

        it("部分还款成功", async function () {
            const tokenBAddr = await tokenB.getAddress();
            const repayAmount = WAD * 200n;

            // 还款前记录仓位
            const posBefore = await lending.getUserPosition(user1.address, tokenBAddr);

            await lending.connect(user1).repay(tokenBAddr, repayAmount);

            // 还款后 borrowAmount 减少（允许微量利息导致 ±1 wei 误差）
            const posAfter = await lending.getUserPosition(user1.address, tokenBAddr);
            expect(posAfter.borrowAmount).to.be.lt(posBefore.borrowAmount);
            // 还款 200，剩余约 300（含微量利息，放宽容忍度）
            expect(posAfter.borrowAmount).to.be.lte(WAD * 301n);
            expect(posAfter.borrowAmount).to.be.gte(WAD * 299n);

            // 验证流动性恢复：原始 500 - 借出 500 + 还款 200 = 200
            const config = await lending.getReserveConfig(tokenBAddr);
            expect(config.totalLiquidity).to.equal(WAD * 200n);
        });

        it("全额还款成功", async function () {
            const tokenBAddr = await tokenB.getAddress();

            // 还款金额远大于债务，自动按实际债务扣除
            await lending.connect(user1).repay(tokenBAddr, WAD * 10000n);

            const position = await lending.getUserPosition(user1.address, tokenBAddr);
            expect(position.borrowAmount).to.equal(0n);
        });

        it("无债务 → revert", async function () {
            const tokenBAddr = await tokenB.getAddress();
            await expect(
                lending.connect(user2).repay(tokenBAddr, WAD * 100n)
            ).to.be.revertedWith("No debt to repay");
        });

         it("多还款自动退还", async function () {
            const tokenBAddr = await tokenB.getAddress();
            const beforeBalance = await tokenB.balanceOf(user1.address);

            // 还款金额远大于债务，自动按实际债务扣除
            await lending.connect(user1).repay(tokenBAddr, WAD * 10000n);

            // 债务清零
            const position = await lending.getUserPosition(user1.address, tokenBAddr);
            expect(position.borrowAmount).to.equal(0n);

            // 只扣了实际债务（约 500 + 微量利息），余额减少正确
            const afterBalance = await tokenB.balanceOf(user1.address);
            const paid = beforeBalance - afterBalance;
            expect(paid).to.be.gte(WAD * 500n);
            expect(paid).to.be.lt(WAD * 501n);
        });
    });

    // ==================== 取款测试 ====================

    describe("📤 redeem — 取款", function () {
        it("无借款时可以自由取款", async function () {
            const tokenAAddr = await tokenA.getAddress();
            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);

            await lending.connect(user1).redeem(tokenAAddr, WAD * 500n);

            const position = await lending.getUserPosition(user1.address, tokenAAddr);
            expect(position.supplyAmount).to.equal(WAD * 500n);
        });

        it("有借款时取款会触发健康因子检查", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // 存 1000 TKA，借 500 TKB（需要先有 TKB 流动性）
            await lending.connect(user2).deposit(tokenBAddr, WAD * 1000n);
            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);
            await lending.connect(user1).borrow(tokenBAddr, WAD * 500n);

            // 试图取走太多抵押品 → revert
            await expect(
                lending.connect(user1).redeem(tokenAAddr, WAD * 900n)
            ).to.be.revertedWith("Health factor < 1 after redeem");
        });

        it("取款金额超过余额 → revert", async function () {
            const tokenAAddr = await tokenA.getAddress();
            await lending.connect(user1).deposit(tokenAAddr, WAD * 100n);

            await expect(
                lending.connect(user1).redeem(tokenAAddr, WAD * 200n)
            ).to.be.revertedWith("Insufficient balance");
        });
    });

    // ==================== 健康因子测试 ====================

    describe("❤️ Health Factor — 健康因子", function () {
        it("无借款时 HF = max", async function () {
            await lending.connect(user1).deposit(await tokenA.getAddress(), WAD * 1000n);
            const hf = await lending.getHealthFactor(user1.address);
            expect(hf).to.equal(ethers.MaxUint256);
        });

        it("有抵押有借款 → HF ≥ 1", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            await lending.connect(user2).deposit(tokenBAddr, WAD * 1000n);
            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);
            await lending.connect(user1).borrow(tokenBAddr, WAD * 500n);

            const hf = await lending.getHealthFactor(user1.address);
            // 抵押价值: 1000 × 0.80 = 800, 债务: 500, HF = 800/500 = 1.6
            expect(hf).to.equal((RAY * 160n) / 100n); // 1.6 RAY
        });
    });

    // ==================== 资产列表测试 ====================

    describe("📋 Reserves — 资产管理", function () {
        it("返回所有已注册资产", async function () {
            const list = await lending.getReserveList();
            expect(list.length).to.equal(2);
            expect(list[0]).to.equal(await tokenA.getAddress());
            expect(list[1]).to.equal(await tokenB.getAddress());
        });

        it("onlyOwner 可以设置资产状态", async function () {
            await lending.setReserveStatus(await tokenA.getAddress(), true, true, false);
            const config = await lending.getReserveConfig(await tokenA.getAddress());
            expect(config.isFrozen).to.be.true;
            expect(config.canBorrow).to.be.false;
        });

        it("非 owner 设置状态 → revert", async function () {
            await expect(
                lending.connect(user1).setReserveStatus(await tokenA.getAddress(), true, false, true)
            ).to.revert(ethers);
        });
    });

    // ==================== 利息累计测试 ====================

    describe("📈 Interest Accrual — 利息累计", function () {
        it("有借款时 liquidityIndex 随时间增长", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // user1 存抵押品 + 借款（user2 提供 TKB 流动性）
            await lending.connect(user2).deposit(tokenBAddr, WAD * 10000n);
            await lending.connect(user1).deposit(tokenAAddr, WAD * 10000n);
            await lending.connect(user1).borrow(tokenBAddr, WAD * 5000n);

            const configBefore = await lending.getReserveConfig(tokenBAddr);
            const indexBefore = configBefore.liquidityIndex;

            // 快进 30 天
            await ethers.provider.send("evm_increaseTime", [30 * 24 * 3600]);
            await ethers.provider.send("evm_mine");

            // 触发一次操作来更新指数（必须是借款资产 TKB，不是抵押品 TKA）
            // 注意：user2 已无 TKB，用 user1（它还有 TKB 余额）
            await lending.connect(user1).deposit(tokenBAddr, WAD * 100n);

            const configAfter = await lending.getReserveConfig(tokenBAddr);
            const indexAfter = configAfter.liquidityIndex;

            // 指数应该增长了
            expect(indexAfter).to.be.gt(indexBefore);
        });

        it("无借款时指数不变", async function () {
            const tokenAAddr = await tokenA.getAddress();

            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);

            const configBefore = await lending.getReserveConfig(tokenAAddr);
            const indexBefore = configBefore.liquidityIndex;

            // 快进 30 天（但没有借款，所以没有利息）
            await ethers.provider.send("evm_increaseTime", [30 * 24 * 3600]);
            await ethers.provider.send("evm_mine");

            // 再存一笔触发更新
            await lending.connect(user2).deposit(tokenAAddr, WAD * 100n);

            const configAfter = await lending.getReserveConfig(tokenAAddr);
            expect(configAfter.liquidityIndex).to.equal(indexBefore);
        });
    });

    // ==================== 清算测试 ====================

    describe("🔪 Liquidation — 清算机制", function () {
        beforeEach(async function () {
            // 构建清算场景：user1 接近最大借款额度，利息累积后 HF < 1
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // user2 提供 TKB 流动性（借出池），金额匹配 outer beforeEach 铸造量
            await lending.connect(user2).deposit(tokenBAddr, WAD * 1000n);

            // user1 存抵押品
            await lending.connect(user1).deposit(tokenAAddr, WAD * 1000n);

            // user1 借到接近上限：抵押价值 = 1000 × 0.80 = 800，借 750（HF ≈ 1.067）
            // U = 750/1000 = 75%, borrowRate = 2%+75%×10% = 9.5%, 365天后债务~821，HF < 1
            await lending.connect(user1).borrow(tokenBAddr, WAD * 750n);

            // 快进 365 天，让利息把债务推到超过抵押价值
            await ethers.provider.send("evm_increaseTime", [365 * 24 * 3600]);
            await ethers.provider.send("evm_mine");

            // 触发利率更新（任意操作）
            await lending.connect(user2).deposit(tokenBAddr, 1n);

            // 清算者准备还款资金（TKB）—— 约一半债务 ~410，铸造 2000 足够
            await tokenB.mint(owner.address, WAD * 2000n);
            await tokenB.connect(owner).approve(await lending.getAddress(), ethers.MaxUint256);
        });

        it("正常清算：HF < 1 时清算者还债拿走抵押品", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // 确认 user1 已不健康
            const hfBefore = await lending.getHealthFactor(user1.address);
            console.log("HF before liquidation:", ethers.formatUnits(hfBefore, 27));
            expect(hfBefore).to.be.lt(RAY); // HF < 1

            // 记录清算前余额
            const liquidatorBalanceBefore = await tokenA.balanceOf(owner.address);
            const user1CollateralBefore = (await lending.getUserPosition(user1.address, tokenAAddr)).supplyAmount;

            // 获取用户债务
            const [, userDebt] = await lending.getUserBalances(user1.address, tokenBAddr);
            console.log("User debt:", ethers.formatUnits(userDebt, 18));

            // 清算者替 user1 还一半债务
            const repayAmount = userDebt / 2n;

            // 执行清算
            const tx = await lending.connect(owner).liquidate(
                user1.address,
                tokenBAddr,    // 债务资产
                tokenAAddr,    // 抵押品资产
                repayAmount
            );

            // 验证事件（HF 值在 tx 内会因 _updateIndexes 微调，不校验精确值）
            await expect(tx)
                .to.emit(lending, "Liquidated");

            // 清算者获得抵押品（含 5% 奖励）
            const liquidatorBalanceAfter = await tokenA.balanceOf(owner.address);
            const gained = liquidatorBalanceAfter - liquidatorBalanceBefore;
            const expectedGain = repayAmount + (repayAmount * LIQUIDATION_BONUS) / RAY;
            // 允许微小精度误差
            expect(gained).to.be.closeTo(expectedGain, 10n);

            // user1 抵押品减少
            const user1CollateralAfter = (await lending.getUserPosition(user1.address, tokenAAddr)).supplyAmount;
            expect(user1CollateralAfter).to.be.lt(user1CollateralBefore);

            // HF 清算后提升
            const hfAfter = await lending.getHealthFactor(user1.address);
            expect(hfAfter).to.be.gt(hfBefore);
        });

        it("正常清算：清算后 HF 提升", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

             const hfBefore = await lending.getHealthFactor(user1.address);

             const [, userDebt] = await lending.getUserBalances(user1.address, tokenBAddr);
             await lending.connect(owner).liquidate(
                user1.address,
                tokenBAddr,
                tokenAAddr,
                userDebt / 2n
            );

            const hfAfter = await lending.getHealthFactor(user1.address);
            console.log("HF after liquidation:", ethers.formatUnits(hfAfter, 27));
            expect(hfAfter).to.be.gte(hfBefore);
        });

        it("HF ≥ 1 时清算 → revert", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // 让 user2 同时有抵押品和少量债务，但 HF > 1（健康）
            // user2 存入 TKA 作为抵押品
            await lending.connect(user2).deposit(tokenAAddr, WAD * 2000n);
            // user2 借少量 TKB（池子有 user1 借后剩余 + 触发操作的 1 wei ≈ 251 TKB）
            await lending.connect(user2).borrow(tokenBAddr, WAD * 100n);

            await expect(
                lending.connect(owner).liquidate(
                    user2.address,
                    tokenBAddr,    // user2 有 TKB 债务
                    tokenAAddr,    // user2 有 TKA 抵押品
                    WAD * 100n
                )
            ).to.be.revertedWith("Position is healthy");
        });

        it("不能清算自己", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            await expect(
                lending.connect(user1).liquidate(
                    user1.address,
                    tokenBAddr,
                    tokenAAddr,
                    WAD * 100n
                )
            ).to.be.revertedWith("Cannot liquidate yourself");
        });

        it("清算债务超过 closeFactor(50%) → 只清算 50%", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            const [, userDebt] = await lending.getUserBalances(user1.address, tokenBAddr);

            // 尝试清算 100% 债务
            await lending.connect(owner).liquidate(
                user1.address,
                tokenBAddr,
                tokenAAddr,
                userDebt  // 尝试还全部
            );

            // 验证只清算了一半
            const [, remainingDebt] = await lending.getUserBalances(user1.address, tokenBAddr);
            console.log("Initial debt:", ethers.formatUnits(userDebt, 18));
            console.log("Remaining debt:", ethers.formatUnits(remainingDebt, 18));
            // 剩余债务应该约为原来的一半（允许微小利息差异）
            expect(remainingDebt).to.be.closeTo(userDebt / 2n, WAD / 10n);
        });

        it("closeFactor 限制单次最多清算 50% 债务", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // 记录清算前 user1 的抵押品（含利息）
            const collatBefore = await lending.getUserCollateral(user1.address, tokenAAddr);

            const [, userDebt] = await lending.getUserBalances(user1.address, tokenBAddr);

            // 尝试清算 100% 债务（实际只会清算 50%）
            await lending.connect(owner).liquidate(
                user1.address,
                tokenBAddr,
                tokenAAddr,
                userDebt
            );

            // 清算后 user1 抵押品减少，但因 closeFactor 限制仍余约一半
            const collatAfter = await lending.getUserCollateral(user1.address, tokenAAddr);
            console.log("Collateral before:", ethers.formatUnits(collatBefore, 18));
            console.log("Collateral after:", ethers.formatUnits(collatAfter, 18));
            expect(collatAfter).to.be.lt(collatBefore); // 确实减少了
            expect(collatAfter).to.be.gt(0n);           // 但没被全部拿走

            // 验证剩余债务约一半
            const [, remainingDebt] = await lending.getUserBalances(user1.address, tokenBAddr);
            expect(remainingDebt).to.be.closeTo(userDebt / 2n, WAD / 10n);
        });

        it("无债务用户 → revert", async function () {
            const tokenAAddr = await tokenA.getAddress();
            const tokenBAddr = await tokenB.getAddress();

            // user2 存了 TokenA 但没有借款
            await lending.connect(user2).deposit(tokenAAddr, WAD * 1000n);

            await expect(
                lending.connect(owner).liquidate(
                    user2.address,
                    tokenBAddr,  // user2 没有 TKB 债务
                    tokenAAddr,
                    WAD * 100n
                )
            ).to.be.revertedWith("No debt to liquidate");
        });
    });

});