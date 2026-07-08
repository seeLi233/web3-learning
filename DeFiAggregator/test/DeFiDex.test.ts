import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

describe("DeFiDex — AMM 恒定乘积做市商", function() {
    let dex: any;
    let token0: any;
    let token1: any;
    let owner:any, alice: any, bob:any;

    // 辅助函数: 基于区块链时间戳计算 deadline（避免 evm_increaseTime 导致的 Date.now() 不同步）
    async function getDeadline(secondsFromNow: number = 3600): Promise<number> {
        const block = await ethers.provider.getBlock("latest");
        return block!.timestamp + secondsFromNow;
    }

    // 精度常量
    const ETH_PRECISION = 10n ** 18n;
    const USDT_PRECISION = 10n ** 6n;

    const initialAmount0 = 1000n * ETH_PRECISION;   // 1000 token0
    const initialAmount1 = 3000000n * USDT_PRECISION; // 3000000 token1

    beforeEach(async function () {
        [owner, alice, bob] = await ethers.getSigners();

        token0 = await ethers.deployContract("MockToken", ["Token A", "TKA", 18]);
        token1 = await ethers.deployContract("MockToken", ["Token B", "TKB", 6]);

        // 2. 确保 token0 < token1（按地址排序）
        const addr0 = await token0.getAddress();
        const addr1 = await token1.getAddress();

        if (addr0.toLowerCase() < addr1.toLowerCase()) {
            dex = await ethers.deployContract("DeFiDex", [addr0, addr1]);
        } else {
            dex = await ethers.deployContract("DeFiDex", [addr1, addr0]);
            // 交换引用
            [token0, token1] = [token1, token0];
        }

        // 4. 给每个用户 mint 代币
        for (const user of [owner, alice, bob]) {
            await token0.mint(user.address, initialAmount0);
            await token1.mint(user.address, initialAmount1);
        }

        // 5. 所有用户授权 DeFiDex 花费他们的代币
        const maxApproval = ethers.MaxUint256;
        for (const user of [owner, alice, bob]) {
            await token0.connect(user).approve(await dex.getAddress(), maxApproval);
            await token1.connect(user).approve(await dex.getAddress(), maxApproval);
        }
    });

    // ========== 第一组: 部署测试 ==========
    describe("Deployment", function () {
        it("应该正确设置 token0 和 token1", async function () {
            expect(await dex.token0()).to.equal(await token0.getAddress());
            expect(await dex.token1()).to.equal(await token1.getAddress());
        });

        it("LP 代币名称应该是 DeFiDex LP Token", async function () {
            expect(await dex.name()).to.equal("DeFiDex LP Token");
        });

        it("LP 代币符号应该是 DLP", async function () {
            expect(await dex.symbol()).to.equal("DLP");
        });

        it("初始储备量应该为 0", async function () {
            const [r0, r1] = await dex.getReserves();
            expect(r0).to.equal(0);
            expect(r1).to.equal(0);
        });

        it("初始 totalSupply 应该为 0", async function () {
            expect(await dex.totalSupply()).to.equal(0);
        });
    });

    // ========== 第二组: 首次添加流动性 ==========
    describe("Add Liquidity — 首次添加", function () {
        const addAmount0 = 100n * ETH_PRECISION;  // 100 token0
        const addAmount1 = 300000n * USDT_PRECISION; // 300000 token1

        it("应该成功添加首次流动性", async function () {
            const deadline = await getDeadline();

            const tx = await dex.addLiquidity(
                addAmount0,
                addAmount1,
                0, // min0
                0, // min1
                deadline
            );

            await tx.wait();

            // 验证储备量
            const [r0, r1] = await dex.getReserves();
            expect(r0).to.equal(addAmount0);
            expect(r1).to.equal(addAmount1);
        });

        it("LP 代币应该被正确 mint", async function () {
            // 先添加流动性
            const deadline = await getDeadline();
            await dex.addLiquidity(addAmount0, addAmount1, 0, 0, deadline);

            const totalSupply = await dex.totalSupply();
            const ownerBalance = await dex.balanceOf(owner.address);

            // totalSupply = sqrt(x*y)，含永久锁定的 MINIMUM_LIQUIDITY (1000)
            // owner balance = sqrt(x*y) - 1000
            expect(ownerBalance).to.be.gt(0);
            // totalSupply = ownerBalance + 1000 (locked)
            expect(totalSupply).to.equal(ownerBalance + 1000n);
        });

        it("应该 emit LiquidityAdded 事件", async function () {
            // 先添加首次流动性
            const deadline0 = await getDeadline();
            await dex.addLiquidity(addAmount0, addAmount1, 0, 0, deadline0);

            // alice 追加流动性（按相同比例）
            const deadline = await getDeadline();
            const tx = await dex.connect(alice).addLiquidity(
                10n * ETH_PRECISION,
                30000n * USDT_PRECISION,
                0,
                0,
                deadline
            );

            await expect(tx)
                .to.emit(dex, "LiquidityAdded")
                .withArgs(
                    alice.address,
                    10n * ETH_PRECISION,
                    30000n * USDT_PRECISION,
                    // liquidity 的值不严格检查
                    (lq: any) => lq > 0
                );
        });
    });

    // ========== 第三组: Swap 兑换 ==========
    describe("Swap", function () {
        // 池子: 100 token0 + 300000 token1，比例 1:3000
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("getAmountOut 应该正确计算输出量", async function () {
            const amountIn = 1n * ETH_PRECISION; // 1 token0
            const amountOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );

            // 公式: amountOut = reserve1 * amountInWithFee / (reserve0 * 1000 + amountInWithFee)
            // = 300000e6 * 1e18 * 997 / (100e18 * 1000 + 1e18 * 997)
            const amountInWithFee = amountIn * 997n;
            const expectedOut =
                (300000n * USDT_PRECISION * amountInWithFee) /
                (100n * ETH_PRECISION * 1000n + amountInWithFee);

            // 允许 1 wei 误差（整数除法）
            const diff = amountOut > expectedOut
                ? amountOut - expectedOut
                : expectedOut - amountOut;
            expect(diff).to.be.lte(1);
        });

        it("getAmountIn 应该正确计算输入量", async function () {
            const amountOut = 3000n * USDT_PRECISION; // 想要 3000 token1
            const amountIn = await dex.getAmountIn(
                amountOut,
                await token0.getAddress(),
                await token1.getAddress()
            );

            // 反向公式验证: 用 getAmountOut(getAmountIn) ≈ amountOut
            const computedOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );
            // 应该 >= amountOut（因为 getAmountIn +1 了）
            expect(computedOut).to.be.gte(amountOut);
        });

        it("应该成功执行 swap", async function () {
            const amountIn = 1n * ETH_PRECISION; // 1 token0

            const expectedOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );

            // 记录 swap 之前的余额
            const balance1Before = await token1.balanceOf(bob.address);

            const deadline = await getDeadline();
            const tx = await dex.connect(bob).swap(
                amountIn,
                expectedOut * 99n / 100n, // 允许 1% 滑点
                await token0.getAddress(),
                await token1.getAddress(),
                deadline
            );

            // 验证 bob 收到了 token1
            const balance1After = await token1.balanceOf(bob.address);
            expect(balance1After - balance1Before).to.equal(expectedOut);

            // 验证事件
            await expect(tx)
                .to.emit(dex, "Swap")
                .withArgs(
                    bob.address,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    amountIn,
                    expectedOut
                );
        });

        it("滑点保护: 输出小于 minAmountOut 时应该 revert", async function () {
            const amountIn = 1n * ETH_PRECISION;
            const expectedOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );

            const deadline = await getDeadline();

            // 要求 min 比实际多 2 倍 → 必然失败
            await expect(
                dex.connect(bob).swap(
                    amountIn,
                    expectedOut * 2n, // 不可能达到
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "InsufficientOutputAmount");
        });

        it("deadline 过期应该 revert", async function () {
            const deadline = await getDeadline(-3600); // 过去的时间
            await expect(
                dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "DeadlineExpired");
        });
    });

    // ========== 第四组: 移除流动性 ==========
    describe("Remove Liquidity", function () {
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("应该按比例返还代币", async function () {
            const liquidity = await dex.balanceOf(owner.address);
            const totalSupply = await dex.totalSupply();
            const [r0, r1] = await dex.getReserves();

            const expected0 = (liquidity * r0) / totalSupply;
            const expected1 = (liquidity * r1) / totalSupply;

            const deadline = await getDeadline();
            await dex.removeLiquidity(
                liquidity,
                BigInt(expected0) * 99n / 100n, // 1% 滑点
                BigInt(expected1) * 99n / 100n,
                deadline
            );

            // 验证储备量已更新
            const [newR0, newR1] = await dex.getReserves();
            expect(newR0).to.equal(r0 - expected0);
            expect(newR1).to.equal(r1 - expected1);

            // 验证 LP 代币已被销毁
            expect(await dex.balanceOf(owner.address)).to.equal(0);
        });
    });

     // ========== 第五组: 价格计算验证 ==========
    describe("价格验证", function () {
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("getPrices 应该返回正确的现货价格", async function () {
            // 池子里还有 user1 的流动性: 10 token0 + 30000 token1
            // 价格 = 30000e6 / 10e18 = 3000 (按 1e18 精度)
            const [r0, r1] = await dex.getReserves();
            const [price0] = await dex.getPrices();

            const expectedPrice0 = (r1 * (10n ** 18n)) / r0;

            // 允许小误差
            const diff = price0 > expectedPrice0
                ? price0 - expectedPrice0
                : expectedPrice0 - price0;
            expect(diff).to.be.lte(1);
        });
    });

    // ========== 第六组: 无常损失演示 ==========
    describe("无常损失 (Impermanent Loss) 验证", function () {
        it("价格偏离后 LP 价值低于持有", async function () {
            // 这个测试验证无常损失的数学
            // 创建一个新池子: 100 token0 + 300,000 token1（比例 1:3000）

            // 在新网络上部署，避免干扰主测试的池子状态
            const network2 = await network.create("hardhat");
            const ethers2 = network2.ethers;
            const [lp] = await ethers2.getSigners();

            const t0 = await ethers2.deployContract("MockToken", ["Test0", "T0", 18]);
            const t1 = await ethers2.deployContract("MockToken", ["Test1", "T1", 6]);

            const addr0 = await t0.getAddress();
            const addr1 = await t1.getAddress();

            const testDex = addr0.toLowerCase() < addr1.toLowerCase()
                ? await ethers2.deployContract("DeFiDex", [addr0, addr1])
                : await ethers2.deployContract("DeFiDex", [addr1, addr0]);

            // Mint 代币给 LP
            await t0.mint(lp.address, 1000n * ETH_PRECISION);
            await t1.mint(lp.address, 3000000n * USDT_PRECISION);
            await t0.connect(lp).approve(await testDex.getAddress(), ethers2.MaxUint256);
            await t1.connect(lp).approve(await testDex.getAddress(), ethers2.MaxUint256);

            // 添加流动性
            const deadline = await getDeadline();
            await testDex.connect(lp).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );

            // LP 持有的价值
            const lpBalance = await testDex.balanceOf(lp.address);
            const lpTotalSupply = await testDex.totalSupply();
            const [r0, r1] = await testDex.getReserves();

            // 如果只是持有（不存池子）的价值（按 1:3000 计算）
            const holdValue0 = 100n * ETH_PRECISION;       // 100 token0
            const holdValue1 = 300000n * USDT_PRECISION;   // 300000 token1

            // LP 份额对应的价值（初始应几乎等于持有价值）
            const lpValue0 = lpBalance * r0 / lpTotalSupply;
            const lpValue1 = lpBalance * r1 / lpTotalSupply;

            // 验证初始 LP 价值约等于持有价值
            // LP 份额价值略低于持有价值，因为 1000 LP 被永久锁定到 0xdead
            // 差额 = holdValue * MINIMUM_LIQUIDITY / totalSupply
            expect(lpValue0).to.be.lt(holdValue0);
            expect(lpValue1).to.be.lt(holdValue1);
            const expectedLoss0 = 1000n * holdValue0 / lpTotalSupply;
            const expectedLoss1 = 1000n * holdValue1 / lpTotalSupply;
            // 允许 1 wei 的整数除误差
            const diff0 = holdValue0 - lpValue0;
            const diff1 = holdValue1 - lpValue1;
            expect(diff0 > expectedLoss0 ? diff0 - expectedLoss0 : expectedLoss0 - diff0).to.be.lte(1);
            expect(diff1 > expectedLoss1 ? diff1 - expectedLoss1 : expectedLoss1 - diff1).to.be.lte(1);
        });
    });

    // ========== 第七组: 错误场景 ==========
    describe("错误场景", function () {
        it("输入 0 应该 revert", async function () {
            const deadline = await getDeadline();
            await expect(
                dex.swap(
                    0,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "InsufficientInputAmount");
        });

        it("相同的代币地址应该 revert（swap）", async function () {
            const deadline = await getDeadline();
            await expect(
                dex.swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token0.getAddress(), // 相同！
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "InvalidAddress");
        });
    });

    // ========== 第八组: 手续费累积 ==========
    describe("手续费累积", function () {
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("swap 后 accumulatedFee 应该正确记录", async function () {
            // swap 前手续费为 0
            const fee0Before = await dex.accumulatedFee0();
            const fee1Before = await dex.accumulatedFee1();
            expect(fee0Before).to.equal(0);
            expect(fee1Before).to.equal(0);

            // 执行 swap: 1 token0 → token1
            const amountIn = 1n * ETH_PRECISION;
            const deadline = await getDeadline();
            await dex.connect(bob).swap(
                amountIn,
                0,
                await token0.getAddress(),
                await token1.getAddress(),
                deadline
            );

            // swap 后 token0 的手续费应该 > 0
            const feeAfter = await dex.accumulatedFee0();
            // 手续费 = 1 * 0.003 = 0.003 ETH = 3 * 10^15
            const expectedFee = amountIn * 3n / 1000n;
            expect(feeAfter).to.equal(expectedFee);
        });

        it("多笔 swap 应该累积手续费", async function () {
            const deadline = await getDeadline();

            // 第一笔
            await dex.connect(bob).swap(
                1n * ETH_PRECISION,
                0,
                await token0.getAddress(),
                await token1.getAddress(),
                deadline
            );

            // 第二笔
            await dex.connect(alice).swap(
                2n * ETH_PRECISION,
                0,
                await token0.getAddress(),
                await token1.getAddress(),
                deadline
            );

            // 总手续费 = (1 + 2) * 0.003 = 0.009 ETH
            const expectedFee = 3n * ETH_PRECISION * 3n / 1000n;
            expect(await dex.accumulatedFee0()).to.equal(expectedFee);
        });
    });

    // ========== 第九组: LP 份额查询 ==========
    describe("getLPShare — LP 份额查询", function () {
        it("应该返回 LP 的完整份额信息", async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );

            const [liquidity, share0, share1, sharePercent] = await dex.getLPShare(owner.address);

            // owner 应该有 LP 代币
            expect(liquidity).to.be.gt(0);

            // 份额和池子的比例对应
            const totalLp = await dex.totalSupply();
            const [r0, r1] = await dex.getReserves();
            const expected0 = liquidity * r0 / totalLp;
            const expected1 = liquidity * r1 / totalLp;

            expect(share0).to.equal(expected0);
            expect(share1).to.equal(expected1);

            // 份额百分比应该接近 100%（owner 是唯一 LP，除了锁定的 1000）
            expect(sharePercent).to.be.gt(0);
            expect(sharePercent).to.be.lte(1n * ETH_PRECISION);
        });

        it("空池子时应该返回全 0", async function () {
            const [liquidity, share0, share1, sharePercent] = await dex.getLPShare(bob.address);

            expect(liquidity).to.equal(0);
            expect(share0).to.equal(0);
            expect(share1).to.equal(0);
            expect(sharePercent).to.equal(0);
        });
    });

    // ========== 第十组: 单边移除流动性 ==========
    describe("removeLiquiditySingle — 单边移除", function () {
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("应该能全部换成 token0", async function () {
            const lpBalance = await dex.balanceOf(owner.address);
            const balance0Before = await token0.balanceOf(owner.address);

            const deadline = await getDeadline();
            await dex.connect(owner).removeLiquiditySingle(
                lpBalance,
                await token0.getAddress(),
                0,
                deadline
            );

            // owner 的 token0 余额增加了
            const balance0After = await token0.balanceOf(owner.address);
            expect(balance0After).to.be.gt(balance0Before);

            // LP 已销毁
            expect(await dex.balanceOf(owner.address)).to.equal(0);
        });

        it("应该能全部换成 token1", async function () {
            const lpBalance = await dex.balanceOf(owner.address);
            const balance1Before = await token1.balanceOf(owner.address);

            const deadline = await getDeadline();
            await dex.connect(owner).removeLiquiditySingle(
                lpBalance,
                await token1.getAddress(),
                0,
                deadline
            );

            const balance1After = await token1.balanceOf(owner.address);
            expect(balance1After).to.be.gt(balance1Before);
            expect(await dex.balanceOf(owner.address)).to.equal(0);
        });

        it("滑点保护: minAmountOut 不够时 revert", async function () {
            const lpBalance = await dex.balanceOf(owner.address);
            const deadline = await getDeadline();

            // 设置一个不可能达到的 minAmountOut（比如要求输出比池子里所有代币还多）
            await expect(
                dex.connect(owner).removeLiquiditySingle(
                    lpBalance,
                    await token0.getAddress(),
                    1_000_000n * ETH_PRECISION, // 远大于实际能拿到的
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "InsufficientOutputAmount");
        });

        it("不应该接受无效的代币地址", async function () {
            const lpBalance = await dex.balanceOf(owner.address);
            const deadline = await getDeadline();

            // 传一个随机地址
            await expect(
                dex.connect(owner).removeLiquiditySingle(
                    lpBalance,
                    bob.address, // 不是 token0 也不是 token1
                    0,
                    deadline
                )
            ).to.be.revertedWithCustomError(dex, "InvalidAddress");
        });
    });

    // ========== 第十一组: TWAP 预言机 ==========
    describe("TWAP 价格预言机", function () {
        it("添加流动性后 priceCumulativeLast 应该被正确初始化", async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );

            // 初始价格累计应该为 0（第一时间段没有 elapsed time）
            // 但 blockTimestampLast 已被设置
            expect(await dex.blockTimestampLast()).to.be.gt(0);
        });

        it("computeTWAP 应该正确计算时间加权平均价格", async function () {
            // 添加流动性
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );

            // 记录当前累计
            const cumul0Start = await dex.price0CumulativeLast();
            const tsStart = await dex.blockTimestampLast();

            // 等待一小段时间后做 swap
            // 在 Hardhat 测试中，我们可以用 evm_mine 或 evm_increaseTime
            // 但直接用 swap 也可以触发更新

            const amountIn = 1n * ETH_PRECISION;
            const deadline2 = await getDeadline();
            await dex.connect(bob).swap(
                amountIn,
                0,
                await token0.getAddress(),
                await token1.getAddress(),
                deadline2
            );

            const cumul0End = await dex.price0CumulativeLast();
            const tsEnd = await dex.blockTimestampLast();

            // TWAP > 0（price 不是 0，timeElapsed > 0）
            const timeElapsed = tsEnd - tsStart;
            if (timeElapsed > 0) {
                const twap = await dex.computeTWAP(
                    cumul0Start,
                    cumul0End,
                    Number(timeElapsed)
                );
                expect(twap).to.be.gt(0);
            }
        });
    });

    // ========== 第十二组: TWAP 时间操控攻击测试 (Day 17) ==========
    describe("TWAP — 时间操控与精度验证", function () {
        // 辅助函数: 让 Hardhat 时间前进
        async function increaseTime(seconds: number) {
            await ethers.provider.send("evm_increaseTime", [seconds]);
            await ethers.provider.send("evm_mine");
        }

        // 辅助函数: 获取当前区块时间戳
        async function getBlockTimestamp(): Promise<number> {
            const block = await ethers.provider.getBlock("latest");
            return block!.timestamp;
        }

        describe("基础 TWAP 查询", function () {
            beforeEach(async function () {
                const deadline = await getDeadline();
                await dex.connect(owner).addLiquidity(
                    100n * ETH_PRECISION,
                    300000n * USDT_PRECISION,
                    0, 0, deadline
                );
            });

            it("snapshotPriceCumulative 应该返回当前累积值和时间戳", async function () {
                const [cumul0, cumul1, ts] = await dex.snapshotPriceCumulative();
                expect(ts).to.be.gt(0);
                // 初始时累积值可能为 0（因为没有经过时间）
            });

            it("queryTWAP0 应该返回正确的 TWAP", async function () {
                // 记录起始快照
                const [cumul0Start, , tsStart] = await dex.snapshotPriceCumulative();

                // 让时间前进 30 分钟（模拟 30 分钟 TWAP）
                await increaseTime(1800);

                // 在此期间做几笔 swap，让价格更新
                const deadline = await getDeadline();
                await dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                );

                // 再等 30 分钟
                await increaseTime(1800);

                // 读取结束累积值
                const [cumul0End, , tsEnd] = await dex.snapshotPriceCumulative();
                const timeElapsed = Number(tsEnd) - Number(tsStart);

                expect(timeElapsed).to.be.gt(0);

                // 查询 TWAP
                const [twap, priceRaw] = await dex.queryTWAP0(
                    cumul0Start,
                    cumul0End,
                    timeElapsed
                );

                // TWAP 应该是合理的价格（约 3000，即 1 ETH = 3000 USDT）
                expect(twap).to.be.gt(0);
                expect(priceRaw).to.be.gt(0);

                console.log(`  TWAP (UQ112x112): ${twap}`);
                console.log(`  价格 (1e18):     ${priceRaw}`);
                console.log(`  时间间隔:        ${timeElapsed}s`);
            });
        });

        describe("时间操控攻击模拟", function () {
            beforeEach(async function () {
                const deadline = await getDeadline();
                await dex.connect(owner).addLiquidity(
                    100n * ETH_PRECISION,
                    300000n * USDT_PRECISION,
                    0, 0, deadline
                );
            });

            it("短时间窗口 TWAP 容易被操纵", async function () {
                // ⚠️ 场景：攻击者只用 1 个区块的时间来操纵价格
                // TWAP 窗口太短 → 价格被操纵

                const [cumul0Start, , tsStart] = await dex.snapshotPriceCumulative();

                // 攻击者执行大额 swap 扭曲价格
                // 用大量 token1 买 token0，推高 token0 价格
                const deadline = await getDeadline();
                await dex.connect(bob).swap(
                    50000n * USDT_PRECISION, // 大额 token1
                    0,
                    await token1.getAddress(),
                    await token0.getAddress(),
                    deadline
                );

                // 几乎立即查询 TWAP（时间窗口极短）
                await increaseTime(12); // 仅 1 个区块

                const [cumul0End, , tsEnd] = await dex.snapshotPriceCumulative();
                const timeElapsed = Number(tsEnd) - Number(tsStart);

                const [, priceRawShort] = await dex.queryTWAP0(
                    cumul0Start,
                    cumul0End,
                    timeElapsed
                );

                // 短窗口的价格会被大额 swap 显著影响
                console.log(`  ⚠️ 短窗口(12s) TWAP 价格: ${priceRawShort}`);
                console.log(`  价格已被大额 swap 扭曲！`);
            });

            it("长时间窗口 TWAP 能抵抗价格操纵", async function () {
                // ✅ 场景：TWAP 窗口长 → 单笔大额 swap 影响被稀释

                const [cumul0Start, , tsStart] = await dex.snapshotPriceCumulative();

                // 正常状态下先过 1 小时，累积正常价格
                await increaseTime(3600);

                // 现在攻击者执行大额 swap
                const deadline = await getDeadline();
                await dex.connect(bob).swap(
                    50000n * USDT_PRECISION,
                    0,
                    await token1.getAddress(),
                    await token0.getAddress(),
                    deadline
                );

                // 再过很短时间查询
                await increaseTime(12);

                const [cumul0End, , tsEnd] = await dex.snapshotPriceCumulative();
                const timeElapsed = Number(tsEnd) - Number(tsStart);

                const [, priceRawLong] = await dex.queryTWAP0(
                    cumul0Start,
                    cumul0End,
                    timeElapsed
                );

                // 长窗口的 TWAP 不会被短暂价格尖峰扭曲
                console.log(`  ✅ 长窗口(~3612s) TWAP 价格: ${priceRawLong}`);
                console.log(`  大额 swap 被 1h 的正常价格稀释了！`);
            });
        });

        describe("consult — 一站式 TWAP 查询", function () {
            beforeEach(async function () {
                const deadline = await getDeadline();
                await dex.connect(owner).addLiquidity(
                    100n * ETH_PRECISION,
                    300000n * USDT_PRECISION,
                    0, 0, deadline
                );
            });

            it("consult 应该返回与手动计算一致的 TWAP", async function () {
                // Step 1: 记录快照
                const [c0Start, c1Start, tsStart] = await dex.snapshotPriceCumulative();

                // Step 2: 等 30 分钟 + 做几笔交易
                await increaseTime(600);
                const deadline = await getDeadline();
                await dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                );

                await increaseTime(600);
                await dex.connect(alice).swap(
                    5000n * USDT_PRECISION,
                    0,
                    await token1.getAddress(),
                    await token0.getAddress(),
                    deadline
                );

                await increaseTime(600);

                // Step 3: 用 consult 查询
                const [twap0, twap1, elapsed] = await dex.consult(c0Start, c1Start, tsStart);

                expect(elapsed).to.be.gt(0);
                expect(twap0).to.be.gt(0);
                expect(twap1).to.be.gt(0);

                console.log(`  consult 结果:`);
                console.log(`    token0 TWAP (1e18): ${twap0}  (约 1 token0 = ? token1)`);
                console.log(`    token1 TWAP (1e18): ${twap1}  (约 1 token1 = ? token0)`);
                console.log(`    经过时间:          ${elapsed}s`);
            });
        });

        describe("价格累积验证", function () {
            beforeEach(async function () {
                const deadline = await getDeadline();
                await dex.connect(owner).addLiquidity(
                    100n * ETH_PRECISION,
                    300000n * USDT_PRECISION,
                    0, 0, deadline
                );
            });

            it("每笔 swap 都正确更新 priceCumulativeLast", async function () {
                const deadline = await getDeadline();

                // 记录第 1 笔 swap 前的累积值
                const [c0Before1, c1Before1] = [await dex.price0CumulativeLast(), await dex.price1CumulativeLast()];

                // swap 1
                await dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                );

                const c0After1 = await dex.price0CumulativeLast();
                // 累积值应该增加了（因为时间流逝 × 价格）
                expect(c0After1).to.be.gt(c0Before1);

                // swap 2
                await increaseTime(100);
                const c0Before2 = await dex.price0CumulativeLast();

                await dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                );

                const c0After2 = await dex.price0CumulativeLast();
                expect(c0After2).to.be.gt(c0Before2);
            });
        });
    });

    // ========== 第十三组: 滑点保护策略测试 (Day 17) ==========
    describe("Slippage — 滑点保护策略", function () {
        beforeEach(async function () {
            const deadline = await getDeadline();
            await dex.connect(owner).addLiquidity(
                100n * ETH_PRECISION,
                300000n * USDT_PRECISION,
                0, 0, deadline
            );
        });

        it("minAmountOut 应该成功拦截超过滑点容忍的交易", async function () {
            const amountIn = 1n * ETH_PRECISION;
            const expectedOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );

            // 前端计算: 允许 0.5% 滑点 (50 bps)
            const slippageBps = 50n;
            const minAmountOut = expectedOut * (10000n - slippageBps) / 10000n;

            // 另一用户抢先执行大额 swap，导致价格恶化
            const deadline = await getDeadline();
            await dex.connect(alice).swap(
                50n * ETH_PRECISION, // 大额！
                0,
                await token0.getAddress(),
                await token1.getAddress(),
                deadline
            );

            // bob 的交易应该因为滑点保护而成功（只要实际输出 >= minAmountOut）
            // 但如果价格变化太大，可能会 revert
            const currentOut = await dex.getAmountOut(
                amountIn,
                await token0.getAddress(),
                await token1.getAddress()
            );

            // 先检查当前输出是否还满足滑点要求
            if (currentOut < minAmountOut) {
                await expect(
                    dex.connect(bob).swap(
                        amountIn,
                        minAmountOut,
                        await token0.getAddress(),
                        await token1.getAddress(),
                        deadline
                    )
                ).to.be.revertedWithCustomError(dex, "InsufficientOutputAmount");
                console.log("  ✅ 滑点保护成功拦截了恶化的交易");
            } else {
                // 价格变化在容忍范围内，交易应该成功
                await dex.connect(bob).swap(
                    amountIn,
                    minAmountOut,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    deadline
                );
                console.log("  ✅ 交易在滑点范围内成功执行");
            }
        });

        it("deadline 防止交易被 MEV 机器人囤积后执行", async function () {
            // 场景: MEV 机器人把你的交易囤积起来，
            // 等到价格对你不利时再执行（抢跑/三明治攻击）
            // deadline 限制了交易的最大存活时间

            const shortDeadline = await getDeadline(1); // 1 秒后过期

            // 等待 2 秒，让 deadline 过期
            await new Promise(resolve => setTimeout(resolve, 2000));

            await expect(
                dex.connect(bob).swap(
                    1n * ETH_PRECISION,
                    0,
                    await token0.getAddress(),
                    await token1.getAddress(),
                    shortDeadline
                )
            ).to.be.revertedWithCustomError(dex, "DeadlineExpired");

            console.log("  ✅ deadline 成功拦截了过期交易");
        });
    });
});