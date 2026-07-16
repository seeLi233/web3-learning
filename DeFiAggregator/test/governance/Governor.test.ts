import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("Governor", function () {
    // ============ 类型声明 ============
    let governor: any;
    let timelock: any;
    let token: any;
    let owner: any, proposer: any, voter1: any, voter2: any, voter3: any, nonVoter: any;

    // ============ 角色常量 ============
    let PROPOSER_ROLE: string;
    let EXECUTOR_ROLE: string;

    // ============ 枚举常量 ============
    const ProposalState = {
        Pending: 0,
        Active: 1,
        Canceled: 2,
        Defeated: 3,
        Succeeded: 4,
        Queued: 5,
        Expired: 6,
        Executed: 7,
    };

    const VOTE_AGAINST = 0;
    const VOTE_FOR = 1;
    const VOTE_ABSTAIN = 2;

    // ============ 配置常量 ============
    const VOTING_DELAY = 1;                          // 1 区块（测试用）
    const VOTING_PERIOD = 50;                        // 50 区块
    const PROPOSAL_THRESHOLD = ethers.parseEther("100");  // 至少 100 tokens
    const QUORUM_VOTES = ethers.parseEther("200");        // Quorum = 200 tokens
    const MINT_AMOUNT = ethers.parseEther("500");         // 测试用铸币量

    // ============ 每次测试前的公共部署 ============

    beforeEach(async function () {
        [owner, proposer, voter1, voter2, voter3, nonVoter] = await ethers.getSigners();

        PROPOSER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("PROPOSER_ROLE"));
        EXECUTOR_ROLE = ethers.keccak256(ethers.toUtf8Bytes("EXECUTOR_ROLE"));

        // 1. 部署 TestVoteToken（集成 Delegation）
        const TestVoteToken = await ethers.getContractFactory("TestVoteToken");
        token = await TestVoteToken.deploy();

        // 2. 部署 Timelock（minDelay=1 秒，测试用）
        const Timelock = await ethers.getContractFactory("Timelock");
        timelock = await Timelock.deploy(
            5,                          // minDelay: 5 秒（大于 1 区块时间，能真正测试延迟）
            [],                          // proposers: 后面给 Governor 加
            [owner.address],             // executors
            [owner.address]              // cancellers
        );

        // 3. 部署 Governor
        const Governor = await ethers.getContractFactory("Governor");
        governor = await Governor.deploy(
            await timelock.getAddress(),
            await token.getAddress(),
            VOTING_DELAY,
            VOTING_PERIOD,
            PROPOSAL_THRESHOLD,
            QUORUM_VOTES
        );

        // 4. 给 Governor 添加 Timelock PROPOSER_ROLE（才能 schedule）和 EXECUTOR_ROLE（才能 execute）
        await timelock.grantRole(PROPOSER_ROLE, await governor.getAddress());
        await timelock.grantRole(EXECUTOR_ROLE, await governor.getAddress());

        // 5. 给测试地址 mint 代币并委托给自己
        await token.mint(proposer.address, MINT_AMOUNT);
        await token.mint(voter1.address, MINT_AMOUNT);
        await token.mint(voter2.address, MINT_AMOUNT);
        await token.mint(voter3.address, MINT_AMOUNT);

        // 委托给自己（激活投票权）
        await token.connect(proposer).delegate(proposer.address);
        await token.connect(voter1).delegate(voter1.address);
        await token.connect(voter2).delegate(voter2.address);
        await token.connect(voter3).delegate(voter3.address);
    });

    // ============ 辅助函数：发起一个测试提案 ============

    /**
     * 创建测试提案并返回 proposalId。
     * 合约使用 hash-based ID，不能假设 ID=1。
     */
    async function createTestProposal(desc: string = "测试提案") {
        const targets = [await token.getAddress()];
        const calldata = token.interface.encodeFunctionData("mint", [
            nonVoter.address,
            ethers.parseEther("100"),
        ]);
        const values = [0];
        const calldatas = [calldata];

        // 提前计算 proposalId（hash-based）
        const descHash = ethers.keccak256(ethers.toUtf8Bytes(desc));
        const proposalId = await governor.hashProposal(targets, values, calldatas, descHash);

        await governor.connect(proposer).propose(targets, values, calldatas, desc);

        return { proposalId, targets, values, calldatas };
    }

    // ==================== A. 部署 ====================

    describe("A. 部署", function () {
        it("A1. 应该正确设置配置参数", async function () {
            expect(await governor.votingDelay()).to.equal(VOTING_DELAY);
            expect(await governor.votingPeriod()).to.equal(VOTING_PERIOD);
            expect(await governor.proposalThreshold()).to.equal(PROPOSAL_THRESHOLD);
            expect(await governor.quorum()).to.equal(QUORUM_VOTES);
        });

        it("A2. Timelock 应该赋予 Governor PROPOSER_ROLE", async function () {
            expect(
                await timelock.hasRole(PROPOSER_ROLE, await governor.getAddress())
            ).to.be.true;
        });

        it("A3. 初始提案数为 0", async function () {
            expect(await governor.proposalCount()).to.equal(0);
        });
    });

    // ==================== B. 创建提案 ====================

    describe("B. 创建提案 (propose)", function () {
        it("B1. 投票权 ≥ threshold → 成功创建", async function () {
            const targets = [await token.getAddress()];
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);
            const values = [0];
            const calldatas = [calldata];
            const description = "提案 #1: 给 nonVoter mint 100 TVT";

            const tx = await governor
                .connect(proposer)
                .propose(targets, values, calldatas, description);

            await expect(tx)
                .to.emit(governor, "ProposalCreated");

            expect(await governor.proposalCount()).to.equal(1);
        });

        it("B2. 投票权不足 → 回滚", async function () {
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);

            // nonVoter 没有代币，投票权 = 0 < threshold
            await expect(
                governor.connect(nonVoter).propose(
                    [await token.getAddress()],
                    [0],
                    [calldata],
                    "应该失败"
                )
            ).to.be.revertedWithCustomError(governor, "Governor__BelowThreshold");
        });

        it("B3. 重复提案 → 回滚", async function () {
            const { targets, values, calldatas } = await createTestProposal("重复提案");

            // 相同的 targets + values + calldatas + description → 相同 ID
            await expect(
                governor.connect(proposer).propose(targets, values, calldatas, "重复提案")
            ).to.be.revertedWith("Governor: proposal exists");
        });

        it("B4. 空描述 → 回滚", async function () {
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);

            await expect(
                governor.connect(proposer).propose(
                    [await token.getAddress()],
                    [0],
                    [calldata],
                    "" // 空字符串
                )
            ).to.be.revertedWith("Governor: empty description");
        });

        it("B5. 空操作数组 → 回滚", async function () {
            await expect(
                governor.connect(proposer).propose([], [], [], "空提案")
            ).to.be.revertedWith("Governor: empty proposal");
        });

        it("B6. 数组长度不一致 → 回滚", async function () {
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);

            await expect(
                governor.connect(proposer).propose(
                    [await token.getAddress(), await token.getAddress()], // 2 个 targets
                    [0],                                                   // 只有 1 个 value
                    [calldata],                                            // 只有 1 个 calldata
                    "长度不匹配"
                )
            ).to.be.revertedWith("Governor: length mismatch");
        });
    });

    // ==================== C. 提案状态流转 ====================

    describe("C. 提案状态流转 (stateOf)", function () {
        it("C1. 刚创建 → Pending", async function () {
            const { proposalId } = await createTestProposal("状态测试");
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Pending);
        });

        it("C2. 到达 startBlock → Active", async function () {
            const { proposalId } = await createTestProposal("状态测试");

            // 挖到投票开始区块
            await ethers.provider.send("evm_mine", []);

            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Active);
        });

        it("C3. 投票结束后 For > Against 且 ≥ Quorum → Succeeded", async function () {
            const { proposalId } = await createTestProposal("通过测试");

            // 进入 Active
            await ethers.provider.send("evm_mine", []);

            // voter1(500票) + voter2(500票) = 1000 赞成 → ≥ Quorum(200) ✅
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);
            await governor.connect(voter2).castVote(proposalId, VOTE_FOR);

            // 挖到投票结束
            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Succeeded);
        });

        it("C4. For ≤ Against → Defeated（反对票多）", async function () {
            const { proposalId } = await createTestProposal("反对测试");

            await ethers.provider.send("evm_mine", []);

            // voter1 赞成(500), voter2 反对(500), voter3 反对(500)
            // For=500, Against=1000 → For ≤ Against → Defeated
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);
            await governor.connect(voter2).castVote(proposalId, VOTE_AGAINST);
            await governor.connect(voter3).castVote(proposalId, VOTE_AGAINST);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Defeated);
        });

        it("C5. 无效 ID → 回滚", async function () {
            // 提案 999 不存在
            await expect(
                governor.stateOf(999)
            ).to.be.revertedWithCustomError(governor, "Governor__InvalidProposalId");
        });
    });

    // ==================== D. 投票 ====================

    describe("D. 投票 (castVote)", function () {
        let proposalId: bigint;

        beforeEach(async function () {
            const result = await createTestProposal("投票测试");
            proposalId = result.proposalId;
            // 进入 Active
            await ethers.provider.send("evm_mine", []);
        });

        it("D1. 投赞成票 → 成功", async function () {
            await expect(governor.connect(voter1).castVote(proposalId, VOTE_FOR))
                .to.emit(governor, "VoteCast")
                .withArgs(voter1.address, proposalId, VOTE_FOR, MINT_AMOUNT, "");

            const receipt = await governor.getReceipt(proposalId, voter1.address);
            expect(receipt.hasVoted).to.be.true;
            expect(receipt.support).to.equal(VOTE_FOR);
            expect(receipt.votes).to.equal(MINT_AMOUNT);
        });

        it("D2. 投反对票 → 成功", async function () {
            await governor.connect(voter1).castVote(proposalId, VOTE_AGAINST);

            const receipt = await governor.getReceipt(proposalId, voter1.address);
            expect(receipt.hasVoted).to.be.true;
            expect(receipt.support).to.equal(VOTE_AGAINST);
            expect(receipt.votes).to.equal(MINT_AMOUNT);
        });

        it("D3. 投弃权票 → 成功", async function () {
            await governor.connect(voter1).castVote(proposalId, VOTE_ABSTAIN);

            const receipt = await governor.getReceipt(proposalId, voter1.address);
            expect(receipt.hasVoted).to.be.true;
            expect(receipt.support).to.equal(VOTE_ABSTAIN);
            expect(receipt.votes).to.equal(MINT_AMOUNT);
        });

        it("D4. 重复投票 → 回滚", async function () {
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);

            await expect(
                governor.connect(voter1).castVote(proposalId, VOTE_AGAINST)
            ).to.be.revertedWithCustomError(governor, "Governor__AlreadyVoted");
        });

        it("D5. 不在 Active 状态 → 回滚", async function () {
            // 挖到投票结束
            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            await expect(
                governor.connect(voter1).castVote(proposalId, VOTE_FOR)
            ).to.be.revertedWithCustomError(governor, "Governor__NotActive");
        });

        it("D6. 无投票权 → 回滚", async function () {
            // nonVoter 持有 0 代币
            await expect(
                governor.connect(nonVoter).castVote(proposalId, VOTE_FOR)
            ).to.be.revertedWith("Governor: no votes");
        });

        it("D7. 带理由投票 → 成功", async function () {
            const reason = "这是一个好提案，支持！";
            await expect(
                governor.connect(voter1).castVoteWithReason(proposalId, VOTE_FOR, reason)
            )
                .to.emit(governor, "VoteCast")
                .withArgs(voter1.address, proposalId, VOTE_FOR, MINT_AMOUNT, reason);
        });

        it("D8. 多个选民投票 → 票数正确累加", async function () {
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);      // +500
            await governor.connect(voter2).castVote(proposalId, VOTE_AGAINST);  // +500
            await governor.connect(voter3).castVote(proposalId, VOTE_ABSTAIN);  // +500

            const p = await governor.getProposal(proposalId);
            expect(p.forVotes).to.equal(MINT_AMOUNT);                   // 500
            expect(p.againstVotes).to.equal(MINT_AMOUNT);               // 500
            expect(p.abstainVotes).to.equal(MINT_AMOUNT);               // 500
        });

        it("D9. Abstain 计入总票数但不影响胜负", async function () {
            // Quorum = 200, 只有一个选民投弃权 500
            // totalVotes=500 ≥ Quorum(200) ✅，但 For(0) ≤ Against(0) → 平局 → Defeated
            await governor.connect(voter1).castVote(proposalId, VOTE_ABSTAIN);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            // 虽然满足 Quorum，但 For=0, Against=0 → For ≤ Against → Defeated
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Defeated);
        });
    });

    // ==================== E. 完整生命周期 ====================

    describe("E. 完整生命周期: Propose → Vote → Queue → Execute", function () {
        it("E1. 正常流程走通并验证链上效果", async function () {
            const targets = [await token.getAddress()];
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);
            const values = [0];
            const calldatas = [calldata];

            const descHash = ethers.keccak256(ethers.toUtf8Bytes("完整流程"));
            const proposalId = await governor.hashProposal(targets, values, calldatas, descHash);

            // Step 1: Propose
            await governor.connect(proposer).propose(targets, values, calldatas, "完整流程");
            expect(await governor.proposalCount()).to.equal(1);
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Pending);

            // Step 2: 进入 Active
            await ethers.provider.send("evm_mine", []);
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Active);

            // Step 3: 投票 — 两个赞成，一个反对
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);      // 500
            await governor.connect(voter2).castVote(proposalId, VOTE_FOR);      // 500
            await governor.connect(proposer).castVote(proposalId, VOTE_AGAINST); // 500

            // For=1000, Against=500, Abstain=0
            // For > Against ✅, Total=1500 ≥ Quorum(200) ✅

            // Step 4: 挖到投票结束 → Succeeded
            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Succeeded);

            // Step 5: Queue → 加入 Timelock
            await governor.queue(proposalId);

            const timelockId = await timelock.hashOperation(
                targets[0], values[0], calldatas[0],
                ethers.ZeroHash, ethers.ZeroHash
            );
            expect(await timelock.isOperationSchedule(timelockId)).to.be.true;

            // Step 6: 等待 Timelock minDelay (5 秒)
            await ethers.provider.send("evm_increaseTime", [6]);
            await ethers.provider.send("evm_mine", []);

            // Step 7: Execute
            const balanceBefore = await token.balanceOf(nonVoter.address);
            expect(balanceBefore).to.equal(0);

            await governor.execute(proposalId);

            const balanceAfter = await token.balanceOf(nonVoter.address);
            expect(balanceAfter).to.equal(ethers.parseEther("100"));

            // 最终状态
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Executed);
        });

        it("E2. getProposal 返回完整提案信息", async function () {
            const { proposalId } = await createTestProposal("查询测试");

            const p = await governor.getProposal(proposalId);
            expect(p.proposer).to.equal(proposer.address);
            expect(p.forVotes).to.equal(0);
            expect(p.againstVotes).to.equal(0);
            expect(p.abstainVotes).to.equal(0);
            // 状态: 刚创建 → Pending
            expect(p.state).to.equal(ProposalState.Pending);
        });

        it("E3. getProposal 无效 ID → 回滚", async function () {
            await expect(
                governor.getProposal(999)
            ).to.be.revertedWithCustomError(governor, "Governor__InvalidProposalId");
        });

        it("E4. getActions 返回提案的操作列表", async function () {
            const { proposalId, targets, calldatas } = await createTestProposal("操作查询");

            const actions = await governor.getActions(proposalId);
            // ethers v6 返回 Result 对象，使用位置索引访问
            expect(actions[0][0]).to.equal(targets[0]);   // targets[]
            expect(actions[1][0]).to.equal(0);             // values[]
            expect(actions[2][0]).to.equal(calldatas[0]);  // calldatas[]
        });
    });

    // ==================== F. 取消提案 ====================

    describe("F. 取消提案 (cancel)", function () {
        let proposalId: bigint;

        beforeEach(async function () {
            const result = await createTestProposal("取消测试");
            proposalId = result.proposalId;
        });

        it("F1. 提案人取消自己的提案 → 成功", async function () {
            await governor.connect(proposer).cancel(proposalId);
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Canceled);
        });

        it("F2. 非提案人取消 → 回滚", async function () {
            await expect(
                governor.connect(voter1).cancel(proposalId)
            ).to.be.revertedWithCustomError(governor, "Governor__NotProposer");
        });

        it("F3. 投票结束后的提案不能取消", async function () {
            // 先进入 Active 并投票通过
            await ethers.provider.send("evm_mine", []);
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);
            await governor.connect(voter2).castVote(proposalId, VOTE_FOR);

            // 挖到投票结束 → Succeeded
            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            // 投票结束后不能取消
            await expect(
                governor.connect(proposer).cancel(proposalId)
            ).to.be.revertedWith("Governor: cannot cancel");
        });
    });

    // ==================== G. Queue & Execute 边界 ====================

    describe("G. Queue & Execute 边界条件", function () {
        let proposalId: bigint;

        beforeEach(async function () {
            // 创建一个通过投票的提案
            const result = await createTestProposal("Queue 测试");
            proposalId = result.proposalId;
            await ethers.provider.send("evm_mine", []);
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);
            await governor.connect(voter2).castVote(proposalId, VOTE_FOR);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }
            // 现在是 Succeeded 状态
        });

        it("G1. 未通过的提案不能 Queue", async function () {
            // 创建第二个提案，不投票让它 Defeated
            const { proposalId: id2 } = await createTestProposal("必败提案");
            await ethers.provider.send("evm_mine", []);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }
            expect(await governor.stateOf(id2)).to.equal(ProposalState.Defeated);

            await expect(
                governor.queue(id2)
            ).to.be.revertedWithCustomError(governor, "Governor__NotSucceeded");
        });

        it("G2. Timelock 时间未到 → Execute 回滚", async function () {
            await governor.queue(proposalId);

            // 立即执行 — Timelock minDelay 未到 → 失败
            await expect(
                governor.execute(proposalId)
            ).to.be.revertedWithCustomError(timelock, "Timelock_NotReady");
        });

        it("G3. 未 Queue 的提案不能 Execute", async function () {
            // 没调用 queue()，直接 execute → 失败
            await expect(
                governor.execute(proposalId)
            ).to.be.revertedWithCustomError(governor, "Governor__NotQueued");
        });

        it("G4. hashProposal 公开函数正确", async function () {
            const targets = [await token.getAddress()];
            const calldata = token.interface.encodeFunctionData("mint", [
                nonVoter.address,
                ethers.parseEther("100"),
            ]);
            const values = [0];
            const calldatas = [calldata];
            const descHash = ethers.keccak256(ethers.toUtf8Bytes("hash 测试"));

            const id = await governor.hashProposal(targets, values, calldatas, descHash);
            expect(id).to.be.gt(0);  // 非零 ID
        });
    });

    // ==================== H. Quorum 法定人数边界 ====================

    describe("H. Quorum 法定人数边界", function () {
        it("H1. 刚好等于 Quorum → Succeeded", async function () {
            // Quorum = 200, voter1(500) 投票 → totalVotes=500 ≥ 200 ✅
            // For(500) > Against(0) ✅
            const { proposalId } = await createTestProposal("Quorum 边界");
            await ethers.provider.send("evm_mine", []);
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Succeeded);
        });

        it("H2. 未达 Quorum → Defeated（即使 For > Against）", async function () {
            // 无人投票 → totalVotes=0 < Quorum(200) → Defeated
            const { proposalId } = await createTestProposal("无人投票");
            await ethers.provider.send("evm_mine", []);

            // 不投票，直接跳到结束
            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Defeated);
        });
    });

    // ==================== I. 🔥 闪电贷治理攻击防御（面试重点） ====================

    describe("I. 🔥 闪电贷治理攻击防御（面试重点）", function () {
        it("I1. 投票权锚定在 startBlock → 闪电贷无效", async function () {
            // 模拟闪电贷攻击场景:
            // 1. 攻击者在区块 N 闪电贷借到大笔代币
            // 2. 提案在区块 N+1 开始投票(startBlock = N+1)
            // 3. 攻击者在区块 N+2 投票 → 查询的是 N+1 的投票权(攻击者当时还没借到)
            //
            // 结论: getPastVotes 查的是历史快照，闪电贷在同一区块借还 → 投票权为 0

            const { proposalId } = await createTestProposal("闪电贷攻击");

            // 投票开始时，nonVoter 没有代币
            await ethers.provider.send("evm_mine", []);

            // 即使现在 mint 给 nonVoter，他也投不了票
            // 因为 getPastVotes 查的是 startBlock 时的快照
            await token.mint(nonVoter.address, ethers.parseEther("100000"));
            await token.connect(nonVoter).delegate(nonVoter.address);

            // nonVoter 尝试投票 → 他在 startBlock 时投票权为 0 → 回滚
            await expect(
                governor.connect(nonVoter).castVote(proposalId, VOTE_FOR)
            ).to.be.revertedWith("Governor: no votes");
        });

        it("I2. Timelock 延迟阻止恶意提案立即执行", async function () {
            // 即使闪电贷攻击者成功通过提案（假设有足够投票权）
            // Timelock 强制等待也让攻击无法完成
            //
            // 本测试验证: Queue 后必须等 minDelay 才能 Execute

            const { proposalId } = await createTestProposal("Timelock 防御");
            await ethers.provider.send("evm_mine", []);

            // 投票通过
            await governor.connect(voter1).castVote(proposalId, VOTE_FOR);
            await governor.connect(voter2).castVote(proposalId, VOTE_FOR);

            for (let i = 0; i < VOTING_PERIOD + 5; i++) {
                await ethers.provider.send("evm_mine", []);
            }

            await governor.queue(proposalId);

            // 尝试立即执行 → Timelock 阻止
            await expect(
                governor.execute(proposalId)
            ).to.be.revertedWithCustomError(timelock, "Timelock_NotReady");

            // 只有在 minDelay 过后才能执行
            await ethers.provider.send("evm_increaseTime", [6]);
            await ethers.provider.send("evm_mine", []);

            await governor.execute(proposalId);
            expect(await governor.stateOf(proposalId)).to.equal(ProposalState.Executed);
        });
    });
});
