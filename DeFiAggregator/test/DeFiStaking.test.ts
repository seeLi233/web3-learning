import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create()

describe("🏦 DeFiStaking — 质押合约测试", function () {
    let staking: any;
    let stakingToken: any;  // LP Token (质押代币)
    let rewardToken: any;   // 治理代币 (奖励代币)
    let owner: any, user1: any, user2: any, user3: any;

    const ONE_ETH = ethers.parseEther("1");
    const WEEK = 7 * 24 * 3600;

    beforeEach(async function () {
        [owner, user1, user2, user3] = await ethers.getSigners();

        // 1. 部署质押代币 (MockToken 作为 LP)
        const MockToken = await ethers.getContractFactory("MockToken");
        stakingToken = await MockToken.deploy("LP Token", "LP", 18);

        // 2. 部署奖励代币
        rewardToken = await MockToken.deploy("Reward Token", "RWD", 18);

        // 3. 部署质押合约
        const DeFiStaking = await ethers.getContractFactory("DeFiStaking");
        staking = await DeFiStaking.deploy(
            await stakingToken.getAddress(),
            await rewardToken.getAddress()
        );

        // 4. 给用户发代币
        for (const user of [user1, user2, user3]) {
            await stakingToken.mint(user.address, ethers.parseEther("10000"));
            await stakingToken.connect(user).approve(
                await staking.getAddress(),
                ethers.parseEther("10000")
            );
        }

        // 5. Owner 准备奖励（mint + approve）
        await rewardToken.mint(owner.address, ethers.parseEther("100000"));
        await rewardToken.approve(
            await staking.getAddress(),
            ethers.parseEther("100000")
        );
    });

    // ==================== Part A: 基本功能 ====================

    describe("A. 质押 (stake)", function () {
        it("A1. 应该成功质押代币", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);

            expect(await staking.balanceOf(user1.address)).to.equal(ONE_ETH);
            expect(await staking.totalSupply()).to.equal(ONE_ETH);

            // 验证代币已转入合约
            expect(await stakingToken.balanceOf(await staking.getAddress()))
                .to.equal(ONE_ETH);
        });

        it("A2. 应该支持锁仓质押", async function () {
            const now = await networkHelpers.time.latest();
            await staking.connect(user1).stake(ONE_ETH, 30); // 锁 30 天

            const lockUntil = await staking.lockUntil(user1.address);
            expect(lockUntil).to.be.closeTo(now + 30 * 86400, 5); // 误差 5 秒内
        });

        it("A3. 应该拒绝 amount = 0", async function () {
            await expect(
                staking.connect(user1).stake(0, 0)
            ).to.be.revertedWith("Cannot stake 0");
        });

        it("A4. 应该拒绝锁仓天数太短", async function () {
            await expect(
                staking.connect(user1).stake(ONE_ETH, 3) // 最少 7 天
            ).to.be.revertedWith("Lock too short");
        });
        
        it("A5. 应该拒绝锁仓天数太长", async function () {
            await expect(
                staking.connect(user1).stake(ONE_ETH, 400) // 最多 365 天
            ).to.be.revertedWith("Lock too long");
        });

        it("A6. 应该拒绝缩短现有锁仓时间", async function () {
            await staking.connect(user1).stake(ONE_ETH, 60); // 锁 60 天
            await expect(
                staking.connect(user1).stake(ONE_ETH, 30) // 试图缩短为 30 天
            ).to.be.revertedWith("Cannot shorten lock");
        });

        it("A7. 应该允许延长锁仓时间", async function () {
            await staking.connect(user1).stake(ONE_ETH, 30);
            // 再质押并延长为 90 天
            await staking.connect(user1).stake(ONE_ETH, 90);
            const lockUntil = await staking.lockUntil(user1.address);
            const expected = (await networkHelpers.time.latest()) + 90 * 86400;
            expect(lockUntil).to.be.closeTo(expected, 5);
        });

        it("A8. 多用户质押应该正确累加 totalSupply", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
            await staking.connect(user2).stake(ONE_ETH * 2n, 0);

            expect(await staking.totalSupply()).to.equal(ONE_ETH * 3n);
            expect(await staking.balanceOf(user1.address)).to.equal(ONE_ETH);
            expect(await staking.balanceOf(user2.address)).to.equal(ONE_ETH * 2n);
        });
    });

    // ==================== Part B: 解除质押 (unstake) ====================

    describe("B. 解除质押 (unstake)", function () {
        beforeEach(async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
        });

        it("B1. 应该成功解除质押", async function () {
            const balBefore = await stakingToken.balanceOf(user1.address);

            await staking.connect(user1).unstake(ONE_ETH);

            expect(await staking.balanceOf(user1.address)).to.equal(0);
            expect(await staking.totalSupply()).to.equal(0);

            const balAfter = await stakingToken.balanceOf(user1.address);
            expect(balAfter - balBefore).to.equal(ONE_ETH);
        });

        it("B2. 应该拒绝解除超过余额的数量", async function () {
            await expect(
                staking.connect(user1).unstake(ONE_ETH * 2n)
            ).to.be.revertedWith("Insufficient balance");
        });

        it("B3. 锁仓期间应该拒绝解除质押", async function () {
            // 先全部取出
            await staking.connect(user1).unstake(ONE_ETH);

            // 重新锁仓质押
            await staking.connect(user1).stake(ONE_ETH, 30); // 锁 30 天

            await expect(
                staking.connect(user1).unstake(ONE_ETH)
            ).to.be.revertedWith("Tokens are locked");
        });

        it("B4. 锁仓到期后应该允许解除", async function () {
            await staking.connect(user1).unstake(ONE_ETH);
            await staking.connect(user1).stake(ONE_ETH, 30);

            // 快进 31 天
            await networkHelpers.time.increase(31 * 86400);

            await staking.connect(user1).unstake(ONE_ETH);
            expect(await staking.balanceOf(user1.address)).to.equal(0);
        });

        it("B5. 部分解除应该正确更新状态", async function () {
            await staking.connect(user1).unstake(ONE_ETH / 2n);

            expect(await staking.balanceOf(user1.address)).to.equal(ONE_ETH / 2n);
            expect(await staking.totalSupply()).to.equal(ONE_ETH / 2n);
        });

        it("B6. 应该拒绝 unstake amount = 0", async function () {
            await expect(
                staking.connect(user1).unstake(0)
            ).to.be.revertedWith("Cannot unstake 0");
        });
    });

    // ==================== Part C: 奖励计算 (核心!) ====================

    describe("C. 奖励计算 (核心测试)", function () {
        beforeEach(async function () {
            // user1 质押 1000 LP
            await staking.connect(user1).stake(ONE_ETH * 1000n, 0);
            // 注入 7000 RWD 奖励（1周内发完 → 每秒约 0.01157）
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
        });

        it("C1. 初始 earned 应该从 0 开始增长", async function () {
            // 刚质押，还没过时间，earned 应该 ≈ 0
            const earned = await staking.earned(user1.address);
            expect(earned).to.be.lt(ethers.parseEther("0.01"));
        });

        it("C2. 时间过半后应该累积约一半奖励", async function () {
            // 快进 3.5 天（半周）
            await networkHelpers.time.increase(3.5 * 86400);

            const earned = await staking.earned(user1.address);
            // user1 占 100%，应该拿到约 3500 RWD
            expect(earned).to.be.closeTo(ethers.parseEther("3500"), ethers.parseEther("5"));
        });

        it("C3. 奖励周期结束后 earned 不再增长", async function () {
            // 快进 8 天（超过 7 天周期）
            await networkHelpers.time.increase(8 * 86400);

            const earned1 = await staking.earned(user1.address);

            // 再快进 10 天
            await networkHelpers.time.increase(10 * 86400);

            const earned2 = await staking.earned(user1.address);

            // earned 不再增长
            expect(earned2).to.equal(earned1);
        });

        it("C4. 按比例分配: user1(60%) vs user2(40%)", async function () {
            // user1 已有 1000，user2 再质押 666.66（总量 1666.66, user2 占 40%）
            await stakingToken.mint(user2.address, ethers.parseEther("10000"));
            await stakingToken.connect(user2).approve(
                await staking.getAddress(), ethers.parseEther("10000")
            );
            await staking.connect(user2).stake(ethers.parseEther("666.666"), 0);

            // 快进到周期结束
            await networkHelpers.time.increase(8 * 86400);

            const earned1 = await staking.earned(user1.address);
            const earned2 = await staking.earned(user2.address);

            // user1 应该拿到约 60%（前一半时间独占 + 后一半时间占 60%）
            // 精确验证: earned1 + earned2 ≈ 7000
            const totalEarned = earned1 + earned2;
            expect(totalEarned).to.be.closeTo(ethers.parseEther("7000"), ethers.parseEther("1"));

            // user1 应该比 user2 多（因为质押更久）
            expect(earned1).to.be.gt(earned2);
        });

        it("C5. 后加入者不应拿到历史奖励", async function () {
            // 快进 6 天（将近结束）
            await networkHelpers.time.increase(6 * 86400);

            // user2 在最后一天加入
            await stakingToken.mint(user2.address, ethers.parseEther("10000"));
            await stakingToken.connect(user2).approve(
                await staking.getAddress(), ethers.parseEther("10000")
            );
            await staking.connect(user2).stake(ethers.parseEther("1000"), 0);

            // 快进到周期结束
            await networkHelpers.time.increase(2 * 86400);

            const earned1 = await staking.earned(user1.address);
            const earned2 = await staking.earned(user2.address);

            // 数学验证: 前6天user1独占(6000), 最后1天两人平分(各500)
            // earned1 ≈ 6500, earned2 ≈ 500, 总和 ≈ 7000
            expect(earned1 + earned2).to.be.closeTo(
                ethers.parseEther("7000"), ethers.parseEther("5")
            );
            // user2 只占最后 1/7 周期的 50% 份额 → 约 1/14 ≈ 7%
            expect(earned2).to.be.closeTo(
                ethers.parseEther("500"), ethers.parseEther("5")
            );
            // 历史奖励(user1前6天独得的6000)没有被user2分走
            expect(earned1).to.be.gt(earned2 * 10n);
        });
    });

    // ==================== Part D: 领取奖励 (claimReward) ====================

    describe("D. 领取奖励 (claimReward)", function () {
        beforeEach(async function () {
            await staking.connect(user1).stake(ONE_ETH * 1000n, 0);
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
        });

        it("D1. 应该成功领取奖励", async function () {
            await networkHelpers.time.increase(3 * 86400);

            const earnedBefore = await staking.earned(user1.address);
            const balBefore = await rewardToken.balanceOf(user1.address);

            await staking.connect(user1).claimReward();

            const balAfter = await rewardToken.balanceOf(user1.address);
            expect(balAfter - balBefore).to.be.closeTo(earnedBefore, ethers.parseEther("1"));

            // 领取后 pending 应归零
            expect(await staking.rewards(user1.address)).to.equal(0);
        });

        it("D2. 应该拒绝无奖励时领取", async function () {
            // user2 没质押，没奖励
            await expect(
                staking.connect(user2).claimReward()
            ).to.be.revertedWith("No rewards to claim");
        });

        it("D3. 领取后继续累积新奖励", async function () {
            await networkHelpers.time.increase(3 * 86400);

            // 第一次领取
            await staking.connect(user1).claimReward();

            // 再过 2 天
            await networkHelpers.time.increase(2 * 86400);

            const newEarned = await staking.earned(user1.address);
            expect(newEarned).to.be.gt(ethers.parseEther("500")); // 应该又有新奖励了
        });

        it("D4. getReward() 别名应该和 claimReward() 一样", async function () {
            await networkHelpers.time.increase(3 * 86400);

            const balBefore = await rewardToken.balanceOf(user1.address);
            await staking.connect(user1).getReward();
            const balAfter = await rewardToken.balanceOf(user1.address);

            expect(balAfter - balBefore).to.be.gt(0);
        });

        it("D5. 多用户各自领取互不影响", async function () {
            // user2 也质押
            await stakingToken.mint(user2.address, ethers.parseEther("10000"));
            await stakingToken.connect(user2).approve(
                await staking.getAddress(), ethers.parseEther("10000")
            );
            await staking.connect(user2).stake(ONE_ETH * 500n, 0);

            await networkHelpers.time.increase(4 * 86400);

            await staking.connect(user1).claimReward();
            await staking.connect(user2).claimReward();

            // 两个人都应该有奖励
            const bal1 = await rewardToken.balanceOf(user1.address);
            const bal2 = await rewardToken.balanceOf(user2.address);

            expect(bal1).to.be.gt(0);
            expect(bal2).to.be.gt(0);
            // user1 (1000) 应该比 user2 (500) 多
            expect(bal1).to.be.gt(bal2);
        });
    });

    // ==================== Part E: 奖励注入 (notifyRewardAmount) ====================

    describe("E. 奖励注入 (notifyRewardAmount)", function () {
        it("E1. 只有 owner 可以注入奖励", async function () {
            await expect(
                staking.connect(user1).notifyRewardAmount(ethers.parseEther("1000"))
            ).to.be.revert(ethers); // Ownable: caller is not the owner
        });

        it("E2. 注入奖励后 rewardRate 应该正确", async function () {
            await staking.notifyRewardAmount(ethers.parseEther("7000"));

            // rewardRate = 7000 / 604800 (7天秒数)
            const rate = await staking.rewardRate();
            expect(rate).to.be.gt(0);

            // periodFinish 应该是 7 天后
            const finish = await staking.periodFinish();
            const now = await networkHelpers.time.latest();
            expect(finish).to.be.closeTo(now + WEEK, 5);
        });

        it("E3. 新一轮奖励应累加剩余奖励", async function () {
            await staking.notifyRewardAmount(ethers.parseEther("7000"));

            // 过了 3 天
            await networkHelpers.time.increase(3 * 86400);

            const rateBefore = await staking.rewardRate();

            // 再注入 7000（此时还剩约 4000，合约会 transfer 7000+4000=11000）
            await rewardToken.mint(owner.address, ethers.parseEther("11000"));
            await rewardToken.approve(await staking.getAddress(), ethers.parseEther("11000"));
            await staking.notifyRewardAmount(ethers.parseEther("7000"));

            const rateAfter = await staking.rewardRate();

            // 新 rate 应该 ≈ (4000 + 7000) / 604800 ≈ 大于旧 rate
            // 因为我们加了 7000 到剩余的 ~4000
            expect(rateAfter).to.be.gt(rateBefore);
        });

        it("E4. 注入奖励拒绝 amount = 0", async function () {
            await expect(
                staking.notifyRewardAmount(0)
            ).to.be.revertedWith("Amount must be > 0");
        });
    });

    // ==================== Part F: 边界条件 ====================

    describe("F. 边界条件", function () {
        it("F1. totalSupply = 0 时 rewardPerToken 不变", async function () {
            const rptBefore = await staking.rewardPerToken();

            // 注入奖励但没人质押
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
            await networkHelpers.time.increase(3 * 86400);

            const rptAfter = await staking.rewardPerToken();

            // rewardPerToken 应该没变（没人质押就不累积）
            expect(rptAfter).to.equal(rptBefore);
        });

        it("F2. 所有用户都离开后 totalSupply = 0", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
            await networkHelpers.time.increase(1 * 86400);

            // 领取奖励 + 全部退出
            await staking.connect(user1).claimReward();
            await staking.connect(user1).unstake(ONE_ETH);

            expect(await staking.totalSupply()).to.equal(0);
        });

        it("F3. 精度: 小额质押也能正确获得奖励", async function () {
            // 质押 1 wei 和 2 wei，验证比例分配精度
            await staking.connect(user1).stake(1, 0);        // 占比 1/3
            await staking.connect(user2).stake(2, 0);        // 占比 2/3
            await staking.notifyRewardAmount(ethers.parseEther("3000"));

            // 快进到周期结束
            await networkHelpers.time.increase(8 * 86400);

            const earned1 = await staking.earned(user1.address);
            const earned2 = await staking.earned(user2.address);

            // 总和 ≈ 3000
            expect(earned1 + earned2).to.be.closeTo(
                ethers.parseEther("3000"), ethers.parseEther("1")
            );
            // 1:2 分配 → user2 约是 user1 的 2 倍
            expect(earned2).to.be.closeTo(
                earned1 * 2n, ethers.parseEther("0.1")
            );
        });

        it("F4. 奖励发完后 claim 应该成功但不给新奖励", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
            await staking.notifyRewardAmount(ethers.parseEther("7")); // 很少的奖励

            // 快进到奖励发完
            await networkHelpers.time.increase(8 * 86400);

            // 第一次领取应该成功
            await staking.connect(user1).claimReward();

            // 再次 claim 应该失败（没有新奖励）
            await expect(
                staking.connect(user1).claimReward()
            ).to.be.revertedWith("No rewards to claim");
        });
    });

    // ==================== Part G: 事件 ====================

    describe("G. 事件", function () {
        it("G1. stake 应该发出 Staked 事件", async function () {
            await expect(staking.connect(user1).stake(ONE_ETH, 0))
                .to.emit(staking, "Staked")
                .withArgs(user1.address, ONE_ETH, 0);
        });

        it("G2. unstake 应该发出 Unstaked 事件", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
            await expect(staking.connect(user1).unstake(ONE_ETH))
                .to.emit(staking, "Unstaked")
                .withArgs(user1.address, ONE_ETH);
        });

        it("G3. claimReward 应该发出 RewardClaimed 事件", async function () {
            await staking.connect(user1).stake(ONE_ETH, 0);
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
            await networkHelpers.time.increase(1 * 86400);

            await expect(staking.connect(user1).claimReward())
                .to.emit(staking, "RewardClaimed");
        });

        it("G4. 手动复投: claimReward + stake (替代 claimAndRestake)", async function () {
            // claimAndRestake 已移除——当 rewardToken ≠ stakingToken 时该函数存在设计缺陷。
            // 正确做法: 分两步——①claimReward 领取 RWD，②stake 单独质押 LP
            await staking.connect(user1).stake(ONE_ETH, 0);
            await staking.notifyRewardAmount(ethers.parseEther("7000"));
            await networkHelpers.time.increase(3 * 86400);

            const earnedBefore = await staking.earned(user1.address);
            const stakeBefore = await staking.balanceOf(user1.address);

            // Step 1: 领取奖励
            await staking.connect(user1).claimReward();
            const rwdBal = await rewardToken.balanceOf(user1.address);
            expect(rwdBal).to.be.closeTo(earnedBefore, ethers.parseEther("1"));

            // Step 2: 用已有的 LP 追加质押（两个独立操作，币种各自独立）
            await staking.connect(user1).stake(ONE_ETH, 0);
            expect(await staking.balanceOf(user1.address)).to.equal(stakeBefore + ONE_ETH);
        });

        it("G5. notifyRewardAmount 应该发出 RewardNotified 事件", async function () {
            await expect(staking.notifyRewardAmount(ethers.parseEther("7000")))
                .to.emit(staking, "RewardNotified");
        });
    });

    // ==================== Part H: 紧急回收 (recoverToken) ====================

    describe("H. 紧急回收 (recoverToken)", function () {
        it("H1. owner 可以回收误转的第三方代币", async function () {
            const MockToken = await ethers.getContractFactory("MockToken");
            const alienToken = await MockToken.deploy("Alien", "ALN", 18);
            await alienToken.mint(owner.address, ethers.parseEther("100"));
            await alienToken.transfer(await staking.getAddress(), ethers.parseEther("50"));

            const balBefore = await alienToken.balanceOf(owner.address);
            await staking.recoverToken(await alienToken.getAddress(), ethers.parseEther("50"));
            const balAfter = await alienToken.balanceOf(owner.address);

            expect(balAfter - balBefore).to.equal(ethers.parseEther("50"));
        });

        it("H2. 不能回收 stakingToken", async function () {
            await expect(
                staking.recoverToken(await stakingToken.getAddress(), ONE_ETH)
            ).to.be.revertedWith("Cannot recover staking or rewards token");
        });

        it("H3. 不能回收 rewardsToken", async function () {
            await expect(
                staking.recoverToken(await rewardToken.getAddress(), ONE_ETH)
            ).to.be.revertedWith("Cannot recover staking or rewards token");
        });

        it("H4. 非 owner 不能回收", async function () {
            await expect(
                staking.connect(user1).recoverToken(user1.address, ONE_ETH)
            ).to.be.revert(ethers);
        });

        it("H5. recoverToken 应该发出 Recovered 事件", async function () {
            const MockToken = await ethers.getContractFactory("MockToken");
            const alienToken = await MockToken.deploy("Alien2", "AL2", 18);
            await alienToken.mint(owner.address, ethers.parseEther("10"));
            await alienToken.transfer(await staking.getAddress(), ethers.parseEther("10"));

            await expect(staking.recoverToken(await alienToken.getAddress(), ethers.parseEther("10")))
                .to.emit(staking, "Recovered");
        });
    });
});