import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("🗳️ Delegation — 委托投票合约测试", function () {
    let token: any;
    let alice: any, bob: any, carol: any;

    const ONE_TOKEN = ethers.parseEther("1000");
    const HALF_TOKEN = ethers.parseEther("500");

    beforeEach(async function () {
        [alice, bob, carol] = await ethers.getSigners();

        // 部署测试代币（需要先用一个简单的 Mock 继承 Delegation）
        const TestVoteToken = await ethers.getContractFactory("TestVoteToken");
        token = await TestVoteToken.deploy();
    });

    // ==================== 初始状态 ====================

    describe("A. 初始状态", function () {
        it("A1. 默认 delegate 是自己", async function () {
            expect(await token.delegates(alice.address)).to.equal(alice.address);
            expect(await token.delegates(bob.address)).to.equal(bob.address);
        });

        it("A2. 初始投票权为 0", async function () {
            expect(await token.getVotes(alice.address)).to.equal(0);
            expect(await token.getVotes(bob.address)).to.equal(0);
        });

        it("A3. 初始总投票权为 0", async function () {
            expect(await token.totalVotingPower()).to.equal(0);
        });
    });

    // ==================== 铸币 → 投票权 ====================

    describe("B. 铸币与投票权", function () {
        it("B1. mint 给用户 → 该用户获得投票权（默认委托自己）", async function () {
            await token.mint(alice.address, ONE_TOKEN);

            expect(await token.getVotes(alice.address)).to.equal(ONE_TOKEN);
            expect(await token.totalVotingPower()).to.equal(ONE_TOKEN);
        });

        it("B2. mint 给委托者 → 代表获得投票权", async function () {
            // Alice 委托给 Bob
            await token.connect(alice).delegate(bob.address);

            // 给 Alice 铸币
            await token.mint(alice.address, ONE_TOKEN);

            // Alice 没有投票权（已委托出去）
            expect(await token.getVotes(alice.address)).to.equal(0);
            // Bob 获得 Alice 的投票权
            expect(await token.getVotes(bob.address)).to.equal(ONE_TOKEN);
        });

        it("B3. 多人铸币 → 总投票权累加", async function () {
            await token.mint(alice.address, ONE_TOKEN);
            await token.mint(bob.address, HALF_TOKEN);

            expect(await token.totalVotingPower()).to.equal(ONE_TOKEN + HALF_TOKEN);
        });
    });

    // ==================== 委托操作 ====================

    describe("C. 委托变更", function () {
        beforeEach(async function () {
            await token.mint(alice.address, ONE_TOKEN);
        });

        it("C1. delegate 后投票权转移到代表", async function () {
            expect(await token.getVotes(alice.address)).to.equal(ONE_TOKEN);

            // Alice 委托给 Bob
            await token.connect(alice).delegate(bob.address);

            expect(await token.getVotes(alice.address)).to.equal(0);
            expect(await token.getVotes(bob.address)).to.equal(ONE_TOKEN);
            expect(await token.delegates(alice.address)).to.equal(bob.address);
        });

        it("C2. delegateToSelf 恢复自委托", async function () {
            await token.connect(alice).delegate(bob.address);
            expect(await token.getVotes(alice.address)).to.equal(0);

            await token.connect(alice).delegateToSelf();
            expect(await token.getVotes(alice.address)).to.equal(ONE_TOKEN);
            expect(await token.getVotes(bob.address)).to.equal(0);
        });

        it("C3. 多人委托给同一代表 → 投票权叠加", async function () {
            await token.mint(bob.address, HALF_TOKEN);

            // Alice → Carol
            await token.connect(alice).delegate(carol.address);
            // Bob → Carol
            await token.connect(bob).delegate(carol.address);

            expect(await token.getVotes(carol.address)).to.equal(
                ONE_TOKEN + HALF_TOKEN
            );
        });

        it("C4. 切换委托目标 → 旧代表失去投票权", async function () {
            await token.connect(alice).delegate(bob.address);
            expect(await token.getVotes(bob.address)).to.equal(ONE_TOKEN);

            // Alice 改委托给 Carol
            await token.connect(alice).delegate(carol.address);

            expect(await token.getVotes(bob.address)).to.equal(0);
            expect(await token.getVotes(carol.address)).to.equal(ONE_TOKEN);
        });

        it("C5. 重新委托给同一个人 → 幂等操作", async function () {
            await token.connect(alice).delegate(bob.address);
            const votesBefore = await token.getVotes(bob.address);

            // 再次委托给 Bob（不应该翻倍）
            await token.connect(alice).delegate(bob.address);
            const votesAfter = await token.getVotes(bob.address);

            expect(votesAfter).to.equal(votesBefore);
        });
    });

    // ==================== 转账对投票权的影响 ====================

    describe("D. 转账更新投票权", function () {
        beforeEach(async function () {
            await token.mint(alice.address, ONE_TOKEN);
            await token.mint(bob.address, HALF_TOKEN);
        });

        it("D1. 转账后投票权跟着转出方的代表减少", async function () {
            // 默认委托自己
            const aliceBefore = await token.getVotes(alice.address);

            // Alice → Bob 转账 300
            const amount = ethers.parseEther("300");
            await token.connect(alice).transfer(bob.address, amount);

            expect(await token.getVotes(alice.address)).to.equal(aliceBefore - amount);
        });

        it("D2. 委托场景下的转账 — 代表投票权变化", async function () {
            // Alice 委托给 Carol
            await token.connect(alice).delegate(carol.address);

            // Carol 有 Alice 的 1000 票
            expect(await token.getVotes(carol.address)).to.equal(ONE_TOKEN);

            // Alice → Bob 转账 300
            const amount = ethers.parseEther("300");
            await token.connect(alice).transfer(bob.address, amount);

            // Carol 的票减少 300（因为 Alice 余额减少）
            expect(await token.getVotes(carol.address)).to.equal(ONE_TOKEN - amount);
            // Bob 的票增加 300（默认委托自己）
            expect(await token.getVotes(bob.address)).to.equal(HALF_TOKEN + amount);
        });

        it("D4. 总投票权在转账后不变", async function () {
            const totalBefore = await token.totalVotingPower();
            const amount = ethers.parseEther("300");

            await token.connect(alice).transfer(bob.address, amount);

            const totalAfter = await token.totalVotingPower();
            expect(totalAfter).to.equal(totalBefore);
        });
    });

    // ==================== 历史查询 ====================

    describe("E. getPastVotes — 历史投票权查询", function () {
        it("E1. 过去区块的投票权正确", async function () {
            const mintBlock = await ethers.provider.getBlockNumber();
            await token.mint(alice.address, ONE_TOKEN);

            // 推进 5 个区块
            for (let i = 0; i < 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }
            const snapshotBlock = await ethers.provider.getBlockNumber();

            // 再推进 5 个区块
            for (let i = 0; i < 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            // 在 snapshotBlock 时，Alice 有 1000 票
            expect(await token.getPastVotes(alice.address, snapshotBlock))
                .to.equal(ONE_TOKEN);

            // 在 mint 之前，Alice 有 0 票
            expect(await token.getPastVotes(alice.address, mintBlock - 1))
                .to.equal(0);
        });

        it("E2. 多次转账后历史查询正确", async function () {
            await token.mint(alice.address, ethers.parseEther("100"));

            // 区块1: Alice 转 20 给 Bob
            await ethers.provider.send("evm_mine", []);
            await token.connect(alice).transfer(bob.address, ethers.parseEther("20"));
            const block1 = await ethers.provider.getBlockNumber();

            // 区块2: Alice 转 30 给 Bob
            await ethers.provider.send("evm_mine", []);
            await token.connect(alice).transfer(bob.address, ethers.parseEther("30"));
            const block2 = await ethers.provider.getBlockNumber();

            // 区块3: Alice 转 10 给 Carol
            await ethers.provider.send("evm_mine", []);
            await token.connect(alice).transfer(carol.address, ethers.parseEther("10"));

            // 查询各区块的投票权
            expect(await token.getPastVotes(alice.address, block1))
                .to.equal(ethers.parseEther("80"));  // 100 - 20
            expect(await token.getPastVotes(bob.address, block1))
                .to.equal(ethers.parseEther("20"));

            expect(await token.getPastVotes(alice.address, block2))
                .to.equal(ethers.parseEther("50")); // 100 - 20 - 30
            expect(await token.getPastVotes(bob.address, block2))
                .to.equal(ethers.parseEther("50")); // 20 + 30
        });

        it("E3. 空地址历史查询返回 0", async function () {
            // 从未接收过 token 的地址
            expect(await token.getPastVotes(carol.address, 1)).to.equal(0);
        });

        it("E4. 请求未来区块 → 应该失败", async function () {
            const futureBlock = (await ethers.provider.getBlockNumber()) + 1000;
            await expect(
                token.getPastVotes(alice.address, futureBlock)
            ).to.be.revertedWith("Delegation: not yet determined");
        });
    });

    // ==================== 销毁 ====================

    describe("F. 销毁 (burn)", function () {
        beforeEach(async function () {
            await token.mint(alice.address, ONE_TOKEN);
        });

        it("F1. burn 减少总投票权", async function () {
            const totalBefore = await token.totalVotingPower();
            const amount = ethers.parseEther("300");

            await token.burn(alice.address, amount);

            expect(await token.totalVotingPower()).to.equal(totalBefore - amount);
            expect(await token.getVotes(alice.address)).to.equal(ONE_TOKEN - amount);
        });

        it("F2. burn 全部余额后投票权归零", async function () {
            await token.burn(alice.address, ONE_TOKEN);

            expect(await token.getVotes(alice.address)).to.equal(0);
            expect(await token.totalVotingPower()).to.equal(0);
        });
    });

    // ==================== Checkpoint 数量 ====================

    describe("G. Checkpoint 管理", function () {
        it("G1. 每次转账产生 checkpoint（不同区块）", async function () {
            await token.mint(alice.address, ethers.parseEther("1000"));

            const cp1 = await token.numCheckpoints(alice.address);

            // 不同区块转账会产生新 checkpoint
            await ethers.provider.send("evm_mine", []);
            await token.connect(alice).transfer(bob.address, ethers.parseEther("100"));

            const cp2 = await token.numCheckpoints(alice.address);
            expect(cp2).to.equal(cp1 + 1n);
        });
    });
});