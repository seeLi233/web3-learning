const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("Voting", function() {
    let Voting;
    let voting;
    let owner;
    let address1;
    let address2;

    beforeEach(async function () {
        // 获取合约工厂和签名者
        Voting = await ethers.getContractFactory("Voting");
        [owner, address1, address2] = await ethers.getSigners();

        // 部署合约
        voting = await Voting.deploy();
        // await voting.deployed();
    });

    describe("投票功能", function () {
        it("应该允许用户给候选人投票", async function () {
            // 投票给候选人"Alice"
            await voting.connect(address1).vote("Alice");

            // 检查得票数是否为1
            expect(await voting.getVotes("Alice")).to.equal(1);
        });

        it("应该允许对同一候选人多次投票", async function () {
            // address1 投票给 "Alice"
            await voting.connect(address1).vote("Alice");
            // address2 投票给 "Alice"
            await voting.connect(address2).vote("Alice");

            // 检查得票数是否为2
            expect(await voting.getVotes("Alice")).to.equal(2);
        });

        it("应该能跟踪多个候选人", async function () {
            // 投票给不同的候选人
            await voting.connect(address1).vote("Alice");
            await voting.connect(address1).vote("Bob");

            // 检查得票数
            expect(await voting.getVotes("Alice")).to.equal(1);
            expect(await voting.getVotes("Bob")).to.equal(1);
        });

        it("应该能重置所有投票", async function () {
            // 先投票
            await voting.connect(address1).vote("Alice");
            await voting.connect(address2).vote("Bob");

            // 重置投票
            await voting.resetVotes();

            // 检查得票数是否为0
            expect(await voting.getVotes("Alice")).to.equal(0);
            expect(await voting.getVotes("Bob")).to.equal(0);
        });

        it("应该能返回所有候选人", async function () {
            await voting.vote("Alice");
            await voting.vote("Bob");

            // 获取候选人列表
            const candidates = await voting.getCandidates();

            // 检查列表是否包含两个候选人
            expect(candidates.length).to.equal(2);
            expect(candidates).to.include.members(["Alice", "Bob"]);
        })
    })
})