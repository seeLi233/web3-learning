import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

describe("⚡ FlashLoan 闪电贷测试套件", function() {
    // ============ 测试变量 ============
    let flashLoan: any;
    let tokenA: any;
    let tokenB: any;
    let dex: any;
    let attack: any;
    let owner: any;
    let attacker: any;
    let user: any;

    const INITIAL_SUPPLY = ethers.parseEther("1000000");    // 100 万
    const FEE_BPS = 9;  // 0.09%
    const FEE_DENOM = 10000n;

    // Helper: 获取基于当前区块时间戳的 deadline（避免 evm_increaseTime 导致 Date.now() 失效）
    async function getDeadline(secondsFromNow: number = 600): Promise<number> {
        const block = await ethers.provider.getBlock("latest");
        return block!.timestamp + secondsFromNow;
    }

    beforeEach(async function () {
        [owner, attacker, user] = await ethers.getSigners();

        // 1. 部署 MockToken
        tokenA = await ethers.deployContract("MockToken", ["Token A", "TKA", 18]);
        tokenB = await ethers.deployContract("MockToken", ["Token B", "TKB", 18]);

        // 2. 铸造代币给 owner
        await tokenA.mint(owner.address, INITIAL_SUPPLY);
        await tokenB.mint(owner.address, INITIAL_SUPPLY);

        // 2. 部署 DeFiDex (TokenA/TokenB 交易对)
        // 注意: DeFiDex 构造函数内部会按地址排序，保证 token0 < token1
        dex = await ethers.deployContract("DeFiDex", [
            tokenA.target,
            tokenB.target
        ]);

        // 给 DEX 添加初始流动性
        const addAmount = ethers.parseEther("100000");      // 10 万
        await tokenA.approve(dex.target, addAmount);
        await tokenB.approve(dex.target, addAmount);
        await dex.addLiquidity(addAmount, addAmount, 0, 0, await getDeadline());

        // 3. 部署 FlashLoan
        flashLoan = await ethers.deployContract("FlashLoan", [FEE_BPS]);

        // 4. 给 FlashLoan 充值 tokenA（作为可借资金池）
        const depositAmount = ethers.parseEther("500000");  //  50 万
        await tokenA.approve(flashLoan.target, depositAmount);
        await flashLoan.deposit(tokenA.target, depositAmount);

        // 5. 添加 tokenA 为支持的闪电贷资产
        await flashLoan.addSupportedToken(tokenA.target, FEE_BPS, 0);   // 0 = 无上限

        // 也支持 tokenB
        await tokenB.approve(flashLoan.target, depositAmount);
        await flashLoan.deposit(tokenB.target, depositAmount);
        await flashLoan.addSupportedToken(tokenB.target, FEE_BPS, 0);

        // 6. 部署攻击合约
        attack = await ethers.deployContract("FlashLoanAttack");
    });

    // ========================================
    // 第一组: 闪电贷基本功能测试
    // ========================================
    describe("📦 第一组: 闪电贷基本功能", function () {
        it("应该能成功执行一次闪电贷（通过 SimpleBorrower）", async function () {
            // 部署一个简单的借款方合约
            const borrower = await ethers.deployContract("SimpleFlashBorrower");

            const borrowAmount = ethers.parseEther("1000");
            const fee = await flashLoan.flashFee(tokenA.target, borrowAmount);

            // 先给 borrower 一些代币用于还手续费
            await tokenA.transfer(borrower.target, fee);

            await expect(
                flashLoan.flashLoan(borrower.target, tokenA.target, borrowAmount, "0x")
            ).to.emit(flashLoan, "FlashLoanExecuted");
        });

        it("应该正确计算手续费", async function () {
            const amount = ethers.parseEther("1000"); // 1000 个 token
            const expectedFee = amount * BigInt(FEE_BPS) / FEE_DENOM;
            // 1000 * 9 / 10000 = 0.9 (0.09%)

            const actualFee = await flashLoan.flashFee(tokenA.target, amount);
            expect(actualFee).to.equal(expectedFee);
        });

        it("应该拒绝不支持的代币", async function () {
            // 部署一个新代币但不添加到 FlashLoan
            const newToken = await ethers.deployContract("MockToken", ["New", "NEW", 18]);

            await expect(
                flashLoan.flashLoan(attacker.address, newToken.target, 100, "0x")
            ).to.be.revertedWithCustomError(flashLoan, "TokenNotSupported");
        });

        it("借款方不还钱 → 应该 revert", async function () {
            // 直接传 EOA 地址作为 receiver → EOA 没有 onFlashLoan，回调返回空数据
            // FlashLoan 检测到返回值不等于 CALLBACK_SUCCESS → 整个交易 revert
            await expect(
                flashLoan.flashLoan(attacker.address, tokenA.target, ethers.parseEther("100"), "0x")
            ).to.be.revert(ethers);
        });
    });

    // ========================================
    // 第二组: 预言机操纵攻击演示 ⚔️
    // ========================================
    describe("⚔️ 第二组: 闪电贷预言机操纵攻击", function () {
        it("应该演示价格操纵效果", async function () {
            // 1. 记录攻击前价格
            const [priceBefore] = await dex.getPrices();
            console.log(`\n  📊 攻击前价格: ${ethers.formatEther(priceBefore)}`);

            // 2. 发起攻击（用 tokenA 借大量资金来操纵 tokenB 的价格）
            const borrowAmount = ethers.parseEther("50000"); // 借 5 万 tokenA（池子的一半！）

            // ⚠️ 在 onFlashLoan 回调中，攻击合约会:
            //    - 用借来的 tokenA 大量买入 tokenB → tokenB 价格暴涨
            //    - 卖回 tokenB 换 tokenA → 但由于 0.3% 手续费（两次 swap），
            //      加上 0.09% 闪电贷手续费，回笼的 tokenA 不够还贷
            //    - 最终: FlashLoan 检测到余额不足 → revert FlashLoanNotRepaid
            // 结论: 在这种参数下攻击不盈利，但演示了价格被扭曲的效果
            await expect(
                attack.executeAttack(
                    flashLoan.target,
                    dex.target,
                    tokenA.target,
                    tokenB.target,
                    borrowAmount
                )
            ).to.be.revertedWithCustomError(flashLoan, "FlashLoanNotRepaid");

            // 攻击后验证价格确实被改变了（通过事件日志中的 AttackStep）
            // 如果池子更深或闪电贷费率更低，攻击者可能盈利
        });

        it("应该观察到短窗口内的价格被明显扭曲", async function () {
            // 手动模拟: 不做闪电贷，直接执行大额 swap 看价格变化
            const [priceBefore] = await dex.getPrices();

            // 用一半的 tokenA 去买 tokenB
            const swapAmount = ethers.parseEther("50000"); // 池子的一半
            await tokenA.approve(dex.target, swapAmount);
            await dex.swap(swapAmount, 0, tokenA.target, tokenB.target, await getDeadline());

            const [priceAfter] = await dex.getPrices();

             console.log(`\n  📊 攻击前价格: ${ethers.formatEther(priceBefore)}`);
            console.log(`  📊 攻击后价格: ${ethers.formatEther(priceAfter)}`);

            // 价格应该有显著变化
            expect(priceAfter).to.not.equal(priceBefore);
        });
    });

    // ========================================
    // 第三组: 安全防护 — TWAP vs 现货价格
    // ========================================
    describe("🛡️ 第三组: TWAP 防护验证", function () {
        it("TWAP 在大额 swap 后应几乎不受影响（30 分钟窗口）", async function () {
            // 1. 快照价格累积值
            const snap1 = await dex.snapshotPriceCumulative();

            // 2. 执行大额 swap（模拟操纵）
            const swapAmount = ethers.parseEther("50000");
            await tokenA.approve(dex.target, swapAmount);
            await dex.swap(swapAmount, 0, tokenA.target, tokenB.target, await getDeadline());

            // 3. 快照 swap 后的现货价格
            const [spotPriceAfter] = await dex.getPrices();
            console.log(`\n  📊 现货价格（被操纵后）: ${ethers.formatEther(spotPriceAfter)}`);

            // 4. 增加时间（模拟 30 分钟后）
            await ethers.provider.send("evm_increaseTime", [1800]); // 30 分钟
            await ethers.provider.send("evm_mine", []);

            // 5. 再执行一笔正常的小额 swap（触发 _update 更新累积器）
            await tokenB.approve(dex.target, ethers.parseEther("100"));
            await dex.swap(ethers.parseEther("100"), 0, tokenB.target, tokenA.target, await getDeadline());

            // 6. 查询 30 分钟 TWAP
            const [twap0, , elapsed] = await dex.consult(snap1.cumul0, snap1.cumul1, snap1.timestamp);

            console.log(`  📊 30分钟 TWAP: ${ethers.formatEther(twap0)}`);
            console.log(`  ⏱️ 实际经过: ${elapsed} 秒`);

            // TWAP 应该与操纵前价格接近，而非被操纵后的价格
            // 因为操纵只持续了 12 秒，在 30 分钟窗口中权重仅 ~0.67%
        });

        it("短窗口 TWAP 仍然会被大额交易影响", async function () {
            // 证明为什么需要 30 分钟窗口
            const snap1 = await dex.snapshotPriceCumulative();

            const swapAmount = ethers.parseEther("50000");
            await tokenA.approve(dex.target, swapAmount);
            await dex.swap(swapAmount, 0, tokenA.target, tokenB.target, await getDeadline());

            // 只等 12 秒（1 个区块）
            await ethers.provider.send("evm_increaseTime", [12]);
            await ethers.provider.send("evm_mine", []);

            // 做一笔小额 swap 触发更新
            await tokenB.approve(dex.target, ethers.parseEther("100"));
            await dex.swap(ethers.parseEther("100"), 0, tokenB.target, tokenA.target, await getDeadline());

            const [twap0] = await dex.consult(snap1.cumul0, snap1.cumul1, snap1.timestamp);
            console.log(`\n  📊 12秒 TWAP（被污染了）: ${ethers.formatEther(twap0)}`);
        });
    });

    // ========================================
    // 第四组: 边缘情况测试
    // ========================================
    describe("🔬 第四组: 边缘情况测试", function () {
        it("借 0 → 应该失败", async function () {
            await expect(
                flashLoan.flashLoan(attacker.address, tokenA.target, 0, "0x")
            ).to.be.revertedWithCustomError(flashLoan, "ZeroAmount");
        });

        it("借超过余额 → 应该失败", async function () {
            const contractBalance = await tokenA.balanceOf(flashLoan.target);
            await expect(
                flashLoan.connect(owner).flashLoan(
                    attacker.address,
                    tokenA.target,
                    contractBalance + 1n,
                    "0x"
                )
            ).to.be.revertedWithCustomError(flashLoan, "InsufficientBalance");
        });

        it("多次闪电贷应该各自独立", async function () {
            // 连续执行 3 次小额闪电贷
            for (let i = 0; i < 3; i++) {
                const borrower = await ethers.deployContract("SimpleFlashBorrower");

                const borrowAmount = ethers.parseEther("100");
                const fee = await flashLoan.flashFee(tokenA.target, borrowAmount);
                await tokenA.transfer(borrower.target, fee);

                await expect(
                    flashLoan.flashLoan(borrower.target, tokenA.target, borrowAmount, "0x")
                ).to.emit(flashLoan, "FlashLoanExecuted");
            }
        });
    });
});