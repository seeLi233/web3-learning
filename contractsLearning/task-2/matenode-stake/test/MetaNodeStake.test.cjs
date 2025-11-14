const { expect, use } = require("chai")
const { ethers } = require("hardhat")

describe("MetaNodeStake", function () {
    let metaNodeToken, metaNodeStake, testERC20;
    let owner, user1, user2, nonOwner;
    let rewardPerBlock;
    let minterRole;

    // 增加区块 (模拟时间流逝)
    async function increaseBlocks(blocks) {
        for(let i = 0; i < blocks; i++) {
            await ethers.provider.send("evm_mine", []);
        }
    }

    this.beforeEach(async function () {
        [owner, user1, user2, nonOwner] = await ethers.getSigners();

        // 部署奖励代币
        const MetaNodeToken = await ethers.getContractFactory("MetaNodeToken");
        metaNodeToken = await MetaNodeToken.deploy(owner.address);
        await metaNodeToken.waitForDeployment();
        minterRole = await metaNodeToken.MINTER_ROLE();

        // 部署测试代币
        const TestERC20 = await ethers.getContractFactory("TestERC20");
        testERC20 = await TestERC20.deploy("TestToken", "TST");
        await testERC20.waitForDeployment()
        // 给 user1 mint 一些测试代币
        await testERC20.mint(user1.address, ethers.parseEther("1000"));

        // 设置每个区块的奖励
        rewardPerBlock = ethers.parseEther("10");

        // 部署质押合约
        const MetaNodeStake = await ethers.getContractFactory("MetaNodeStake");
        metaNodeStake = await MetaNodeStake.deploy(await metaNodeToken.getAddress(), rewardPerBlock, owner.address);
        await metaNodeStake.waitForDeployment();

        // 授权质押合约铸造奖励代币
        await metaNodeToken.connect(owner).grantRole(minterRole, await metaNodeStake.getAddress());
    });

    describe("部署和初始化", function() {
        it("应该正确初始化第一个质押池", async function () {
            const poolCount = await metaNodeStake.poolLength();
            expect(poolCount).to.equal(1);

            const pool = await metaNodeStake.pools(0);
            expect(pool.stTokenAddress).to.equal(ethers.ZeroAddress); // 原生代币
            expect(pool.poolWeight).to.equal(100);
        });

        it("应该正确设置角色权限", async function () {
            expect(await metaNodeStake.hasRole(await metaNodeStake.ADMIN_ROLE(), owner.address)).to.be.true;
            expect(await metaNodeStake.hasRole(await metaNodeStake.UPGRADER_ROLE(), owner.address)).to.be.true;
        })
    });

    describe("质押功能", function() {
        it("用户应该能够质押原生代币", async function () {
            const amount = ethers.parseEther("2");
            await expect(metaNodeStake.connect(user1).stake(0, amount, { value: amount })).to.emit(metaNodeStake, "Staked").withArgs(user1.address, 0, amount);

            const userData = await metaNodeStake.users(0, user1.address);
            expect(userData.stAmount).to.be.equal(amount);
            
            const pool = await metaNodeStake.pools(0);
            expect(pool.stTokenAmount).to.equal(amount);
        });

        it("质押数量低于最小值应该失败", async function () {
            const amount = ethers.parseEther("0.5");
            await expect(metaNodeStake.connect(user1).stake(0, amount, { value :amount })).to.be.revertedWith("Amount below minimum");
        });

        it("可质押ERC20代币(新增池)", async function () {
            // 管理员新增 ERC20 代币池
            const stToken = await testERC20.getAddress();
            await metaNodeStake.connect(owner).addPool(stToken, 50, ethers.parseEther("10"), 100); // 权重50， 最小质押10， 锁定期区块 100
            const pid = 1; // 新增池 ID 为 1

            // user1 授权质押合约使用 TEST 代币
            const amount = ethers.parseEther("100");
            await testERC20.connect(user1).approve(await metaNodeStake.getAddress(), amount);
            
            // 执行质押
            await expect(metaNodeStake.connect(user1).stake(pid, amount)).to.emit(metaNodeStake, "Staked").withArgs(user1.address, pid, amount);

            // 验证用户质押量
            const userData = await metaNodeStake.users(pid, user1.address);
            expect(userData.stAmount).to.equal(amount);

            // 验证质押池总量
            const pool = await metaNodeStake.pools(pid);
            expect(pool.stTokenAmount).to.equal(amount);
        });
    });

    // 测试解质押功能
    describe("解质押功能", function () {
        it("用户可以发起解质押请求(原生代币)", async function () {
            const pid = 0;
            const stakeAmount = ethers.parseEther("2");
            const unstakeAmount = ethers.parseEther("1");

            // 先质押
            await metaNodeStake.connect(user1).stake(pid, stakeAmount, { value: stakeAmount });

            // 获取当前区块号
            const currentBlock = await ethers.provider.getBlockNumber();
            await expect(metaNodeStake.connect(user1).unstake(pid, unstakeAmount)).to.emit(metaNodeStake, "Unstaked").withArgs(user1.address, pid, unstakeAmount, currentBlock + 20161); // 这里由于当前区块被打包，因此会产生 n + 1
            
            // 验证用户剩余质押量
            const userData = await metaNodeStake.users(pid, user1.address);
            expect(userData.stAmount).to.equal(stakeAmount - unstakeAmount);

            // 验证解质押请求
            const requests = await metaNodeStake.getUserUnstakeRequests(pid, user1.address);
            expect(requests.length).to.equal(1);
            expect(requests[0].amount).to.equal(unstakeAmount);
        });

        it("解质押量超过质押量应该失败", async function () {
            const pid = 0;
            const stakeAmount = ethers.parseEther("1");
            const unstakeAmount = ethers.parseEther("2"); // 超过质押量

            await metaNodeStake.connect(user1).stake(pid, stakeAmount, { value: stakeAmount });

            await expect(metaNodeStake.connect(user1).unstake(pid, unstakeAmount)).to.be.revertedWith("Insufficient staked amount");
        });

        it("锁定期结束可提取解质押资产", async function () {
            const pid = 0;
            const stakeAmount = ethers.parseEther("2");
            const unstakeAmount = ethers.parseEther("1");

            // 质押 -> 解质押
            await metaNodeStake.connect(user1).stake(pid, stakeAmount, { value:stakeAmount });
            await metaNodeStake.connect(user1).unstake(pid, unstakeAmount);
            const requests = await metaNodeStake.getUserUnstakeRequests(pid, user1.address);
            const requestIndex = 0;

            // 锁定期未结束时提取应该失败
            await expect(metaNodeStake.connect(user1).withdraw(pid, requestIndex)).to.be.revertedWith("Still locked");

            // 增加区块到锁定期结束
            await increaseBlocks(20160);

            // 提取资产
            const initialBalance = await ethers.provider.getBalance(user1.address);
            await expect(metaNodeStake.connect(user1).withdraw(pid, requestIndex)).to.emit(metaNodeStake, "Withdrawn").withArgs(user1.address, pid, unstakeAmount);

            // 验证余额增加 (扣除 gas 前)
            const finalBalance = await ethers.provider.getBalance(user1.address);
            expect(finalBalance).to.be.gt(initialBalance);

            // 验证请求已删除
            const finalRequests = await metaNodeStake.getUserUnstakeRequests(pid, user1.address);
            expect(finalRequests.length).to.equal(0);
        });
    });

    // 领取奖励功能
    describe("领取奖励功能", function() {
        it("用户应该获得质押奖励", async function () {
            const pid = 0;
            const stakeAmount = ethers.parseEther("2");

            // 质押
            await metaNodeStake.connect(user1).stake(pid, stakeAmount, { value: stakeAmount });

            // 增加 10 个区块 (应产生奖励)
            await increaseBlocks(10);

            // 领取奖励
            const initialRewardBalance = await metaNodeToken.balanceOf(user1.address);
            await expect(metaNodeStake.connect(user1).claimReward(pid)).to.emit(metaNodeStake, "RewardClaimed");

            // 验证奖励到账 (单个池权重 100%, 奖励 = 11 区块 * 10 MTA/区块 = 110 MTA)
            const finalRewardBalance = await metaNodeToken.balanceOf(user1.address);
            expect(finalRewardBalance - initialRewardBalance).to.equal(ethers.parseEther("110"));
        });

        it("多池权重影响奖励分配", async function () {
            // 新增一个权重相同的池 (总权重 = 100 + 100 = 200)
            await metaNodeStake.connect(owner).addPool(await testERC20.getAddress(), 100, ethers.parseEther("1"), 100);
            const pid0 = 0; // 原生池
            const pid1 = 1; // ERC20 池

            // user1 在两个池各质押相同金额
            await metaNodeStake.connect(user1).stake(pid0, ethers.parseEther("1"), { value: ethers.parseEther("1") });
            await testERC20.connect(user1).approve(await metaNodeStake.getAddress(), ethers.parseEther("1"));
            await metaNodeStake.connect(user1).stake(pid1, ethers.parseEther("1"));

            // 增加 2 个区块
            await increaseBlocks(2);

            // 领取两个池的奖励
            await metaNodeStake.connect(user1).claimReward(pid0);
            await metaNodeStake.connect(user1).claimReward(pid1);

            expect(await metaNodeToken.balanceOf(user1.address)).to.equal(ethers.parseEther("45"));
        });

        it("无奖励时领取应该失败", async function () {
            const pid = 0;
            // 未质押任何资产，无奖励
            await expect(metaNodeStake.connect(user1).claimReward(pid)).to.be.revertedWith("No reward to claim");
        });
    });

    // 测试管理员功能
    describe("管理员功能", function() {
        it("只有管理员可以添加新池", async function () {
            // 非管理员添加池应该失败
            await expect(metaNodeStake.connect(nonOwner).addPool(ethers.ZeroAddress, 50, ethers.parseEther("1"), 100)).to.be.revertedWithCustomError(metaNodeStake, "AccessControlUnauthorizedAccount");

            // 管理员添加池应该成功
            await expect(metaNodeStake.connect(owner).addPool(ethers.ZeroAddress, 50, ethers.parseEther("1"), 100)).to.emit(metaNodeStake, "PoolAdded");

            expect(await metaNodeStake.poolLength()).to.equal(2);
        });

        it("只有管理员可更新池配置", async function () {
            const pid = 0;
            // 非管理员更新池应该失败
            await expect(metaNodeStake.connect(nonOwner).updatePoolSet(pid, 200, ethers.parseEther("2"), 30000)).to.be.revertedWithCustomError(metaNodeStake, "AccessControlUnauthorizedAccount");

            // 管理员更新池应该成功
            await expect(metaNodeStake.connect(owner).updatePoolSet(pid, 200, ethers.parseEther("2"), 30000)).to.emit(metaNodeStake, "PoolSetUpdated");
            const updatePool = await metaNodeStake.pools(pid);
            expect(updatePool.poolWeight).to.be.equal(200);
            expect(updatePool.minDepositAmount).to.be.equal(ethers.parseEther("2"));
        });

        it("可暂停/恢复质押功能", async function () {
            const pid = 0;
            const amount = ethers.parseEther("1");

            // 暂停质押
            await metaNodeStake.connect(owner).setStakePaused(true);
            await expect(metaNodeStake.connect(user1).stake(pid, amount, { value: amount })).to.be.revertedWith("Staking is paused");
        });
    })
})