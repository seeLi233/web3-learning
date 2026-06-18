import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

describe("Voting 合约测试", function () {
    let voting: any;
    let chairperson: any;
    let voter1: any;
    let voter2: any;
    let voter3: any;
    let outsider: any;

    const PROPOSAL_NAMES = [
        ethers.encodeBytes32String("Proposal A"),
        ethers.encodeBytes32String("Proposal B"),
        ethers.encodeBytes32String("Proposal C"),
    ];

    const VOTING_DURATION = 60; // 60 分钟

    beforeEach(async function () {
        [chairperson, voter1, voter2, voter3, outsider] = await ethers.getSigners();

        const VotingFactory = await ethers.getContractFactory("Voting");
        voting = await VotingFactory.deploy(PROPOSAL_NAMES, VOTING_DURATION);
    });

    // ============================================================
    // 1. 部署测试
    // ============================================================
    describe("部署", function(){
        it("应该正确设置主席", async function () {
            expect(await voting.chairperson()).to.equal(chairperson.address);
        });

        it("应该创建正确数量的提案", async function () {
            expect(await voting.proposalCount()).to.equal(3);
        });

        it("提案名称应该匹配", async function () {
            const proposals = await voting.getProposals();
            expect(proposals[0].name).to.equal(PROPOSAL_NAMES[0]);
            expect(proposals[1].name).to.equal(PROPOSAL_NAMES[1]);
            expect(proposals[2].name).to.equal(PROPOSAL_NAMES[2]);
        });

        it("应该自动注册为选民", async function () {
            expect(await voting.isRegistered(chairperson.address)).to.equal(true);
            const voter = await voting.voters(chairperson.address);
            expect(voter.weight).to.equal(1);
        });

        it("应该设置正确的投票截止时间", async function () {
            const deadline = await voting.votingDeadline();
            // 应该在未来
            expect(deadline).to.be.gt(0);
        });
    });

    // ============================================================
    // 2. 选民注册测试
    // ============================================================
    describe("选民注册", function() {
        it("主席可以注册新选民", async function () {
            await voting.giveRightToVote(voter1.address);
            expect(await voting.isRegistered(voter1.address)).to.equal(true);
        });

        it("非主席不能注册选民", async function () {
            await expect(
                voting.connect(voter1).giveRightToVote(voter2.address)
            ).to.be.revertedWithCustomError(voting, "NotChairperson");
        });

        it("不能重复注册同一个选民", async function () {
            await expect(
                voting.giveRightToVote(chairperson.address)
            ).to.be.revertedWithCustomError(voting, "AlreadyRegistered");
        });

        it("注册后选民列表应该更新（可迭代 mapping 验证）", async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);

            const list = await voting.getVoterList();
            // chairperson + voter1 + voter2 = 3
            expect(list.length).to.equal(3);
            expect(list).to.include(voter1.address);
            expect(list).to.include(voter2.address);
        });
    });

    // ============================================================
    // 3. 投票测试
    // ============================================================
    describe("投票", function () {
        beforeEach(async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);
        });

        it("已注册选民可以正常投票", async function () {
            await voting.connect(voter1).vote(1);  // 投给 Proposal B
            const voter = await voting.voters(voter1.address);
            expect(voter.voted).to.equal(true);
            expect(voter.vote).to.equal(1);
        });

        it("投票后提案得票数应该增加", async function () {
            await voting.connect(voter1).vote(0);
            const proposal = await voting.proposals(0);
            expect(proposal.voteCount).to.equal(1);
        });

        it("不能重复投票", async function () {
            await voting.connect(voter1).vote(0);
            await expect(
                voting.connect(voter1).vote(1)
            ).to.be.revertedWithCustomError(voting, "AlreadyVoted");
        });

        it("未注册选民不能投票", async function () {
            await expect(
                voting.connect(outsider).vote(0)
            ).to.be.revertedWithCustomError(voting, "NotChairperson");
        });

        it("不能给不存在的提案投票", async function () {
            await expect(
                voting.connect(voter1).vote(99)
            ).to.be.revertedWithCustomError(voting, "InvalidProposal");
        });
    });

    // ============================================================
    // 4. 委托投票测试
    // ============================================================
    describe("委托投票", function () {
        beforeEach(async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);
        });

        it("可以委托投票权给另一个选民", async function () {
            // voter1 委托给 voter2
            await voting.connect(voter1).delegate(voter2.address);

            const v1 = await voting.voters(voter1.address);
            expect(v1.delegate).to.equal(voter2.address);
            expect(v1.voted).to.equal(true);

            // voter2 的权重应该增加
            const v2 = await voting.voters(voter2.address);
            expect(v2.weight).to.equal(2);  // 自己的 1 + voter1 的 1
        });

        it("不能委托给自己", async function () {
            await expect(
                voting.connect(voter1).delegate(voter1.address)
            ).to.be.revertedWithCustomError(voting, "SelfDelegationNotAllowed");
        });

        it("委托后受托人投票，委托人的票也生效", async function () {
            // voter1 委托给 voter2
            await voting.connect(voter1).delegate(voter2.address);

            // voter2 投票给 Proposal C
            await voting.connect(voter2).vote(2);

            // Proposal C 应该得到 2 票
            const proposal = await voting.proposals(2);
            expect(proposal.voteCount).to.equal(2);
        });
    });

    // ============================================================
    // 5. 时间窗口测试
    // ============================================================
    describe("时间窗口", function () {
        it("投票结束后不能投票", async function () {
            // 快进到投票结束后
            await ethers.provider.send("evm_increaseTime", [VOTING_DURATION * 60 + 1]);
            await ethers.provider.send("evm_mine", []);

            await expect(
                voting.connect(voter1).vote(0)
            ).to.be.revertedWithCustomError(voting, "VotingEnded");
        });
    });

    // ============================================================
    // 6. 结果查询测试
    // ============================================================
    describe("结果查询", function () {
        beforeEach(async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);
            await voting.giveRightToVote(voter3.address);

            // voter1 → Proposal A, voter2 → Proposal B, voter3 → Proposal B
            await voting.connect(voter1).vote(0);
            await voting.connect(voter2).vote(1);
            await voting.connect(voter3).vote(1);
        });

        it("投票未结束不能查询获胜者", async function () {
            await expect(
                voting.winnerName()
            ).to.be.revertedWithCustomError(voting, "VotingStillActive");
        });

        it("投票结束后可以正确查询获胜者", async function () {
            // 快进到投票结束
            await ethers.provider.send("evm_increaseTime", [VOTING_DURATION * 60 + 1]);
            await ethers.provider.send("evm_mine", []);

            const winner = await voting.winnerName();
            expect(winner).to.equal(PROPOSAL_NAMES[1]);  // Proposal B 有 2 票
        });

        it("可以获取完整的获胜提案信息", async function () {
            await ethers.provider.send("evm_increaseTime", [VOTING_DURATION * 60 + 1]);
            await ethers.provider.send("evm_mine", []);

            const [id, name, count] = await voting.winningProposal();
            expect(id).to.equal(1);
            expect(name).to.equal(PROPOSAL_NAMES[1]);
            expect(count).to.equal(2);
        });
    });

    // ============================================================
    // 7. 可迭代 mapping 验证
    // ============================================================
    describe("可迭代 mapping", function () {
        it("voterCount 应该正确反映注册选民数", async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);
            // chairperson(1) + voter1(1) + voter2(1) = 3
            expect(await voting.voterCount()).to.equal(3);
        });

        it("getVoterList 应该返回所有注册选民", async function () {
            await voting.giveRightToVote(voter1.address);
            const list = await voting.getVoterList();
            expect(list.length).to.equal(2);  // chairperson + voter1
        });
    });

    // ============================================================
    // 8. 事件测试
    // ============================================================
    describe("事件", function () {
        it("投票时应该 emit Voted 事件", async function () {
            await voting.giveRightToVote(voter1.address);
            await expect(voting.connect(voter1).vote(0))
                .to.emit(voting, "Voted")
                .withArgs(voter1.address, 0, 1);
        });

        it("委托时应该 emit Delegated 事件", async function () {
            await voting.giveRightToVote(voter1.address);
            await voting.giveRightToVote(voter2.address);

            await expect(voting.connect(voter1).delegate(voter2.address))
                .to.emit(voting, "Delegated")
                .withArgs(voter1.address, voter2.address);
        });
    });
});