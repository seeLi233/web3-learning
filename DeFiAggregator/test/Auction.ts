import { expect } from "chai";
import { network } from "hardhat";


const { ethers } = await network.create();

describe("Auction", function() {
    let auction: any;
    let owner: any;
    let alice: any;
    let bob: any;
    let carol: any;
    let minBid: bigint;

    // 辅助函数：快进时间
    async function fastForward(seconds: number) {
        const block = await ethers.provider.getBlock("latest");
        await ethers.provider.send("evm_setNextBlockTimestamp", [
            block!.timestamp + seconds,
        ]);
        await ethers.provider.send("evm_mine", []);
    }

    // 辅助函数：快进到拍卖结束
    async function fastForwardToEnd() {
        const endTime = await auction.endTime();
        const block = await ethers.provider.getBlock("latest");
        if (block!.timestamp < Number(endTime)) {
            await ethers.provider.send("evm_setNextBlockTimestamp", [
                Number(endTime) + 1,
            ]);
            await ethers.provider.send("evm_mine", []);
        }
    }

    beforeEach(async function () {
        [owner, alice, bob, carol] = await ethers.getSigners();
        minBid = ethers.parseEther("0.1"); // 最低出价 0.1 ETH

        // 部署拍卖：物品 "Rare NFT", 最低 0.1 ETH, 持续 60 分钟
        auction = await ethers.deployContract("Auction", [
            "Rare NFT",
            minBid,
            60,
        ]);
    });

    // ==========================================
    // 1. 部署测试
    // ==========================================
    describe("部署", function() {
        it("应该正确设置 owner", async function () {
            expect(await auction.owner()).to.equal(owner.address);
        });

        it("应该正确设置物品名称", async function () {
            expect(await auction.item()).to.equal("Rare NFT");
        });

        it("应该正确设置最低出价", async function () {
            expect(await auction.minBid()).to.equal(minBid);            
        });

        it("应该设置未来 60 分钟的结束时间", async function () {
            const endTime = await auction.endTime();
            const block = await ethers.provider.getBlock("latest");
            // endTime 应该在未来，误差在 60 分钟 ± 5秒
            expect(endTime).to.be.closeTo(BigInt(block!.timestamp + 3600), BigInt(5));
        });

        it("初始状态应该是活跃的", async function () {
            const status = await auction.getStatus();
            expect(status._ended).to.equal(false);
            expect(status._highestBidder).to.equal(
                "0x0000000000000000000000000000000000000000"
            );
            expect(status._highestBid).to.equal(0);
        });
    });

    // ==========================================
    // 2. 出价测试 — 正常流程
    // ==========================================
    describe("出价 - 正常流程", function() {
        it("应该接受第一个出价（>=minBid）", async function () {
            const bidAmount = ethers.parseEther("0.5");
            await auction.connect(alice).bid({value: bidAmount});

            expect(await auction.highestBidder()).to.equal(alice.address);
            expect(await auction.highestBid()).to.equal(bidAmount);
        });

        it("应该接受更高的出价并更新最高者", async function () {
            await auction.connect(alice).bid({value: ethers.parseEther("0.5")});
            await auction.connect(bob).bid({value: ethers.parseEther("1.0")});

            expect(await auction.highestBidder()).to.equal(bob.address);
            expect(await auction.highestBid()).to.equal(ethers.parseEther("1.0"));
        });

        it("被反超后，前最高者的资金应记录为待退款", async function () {
            const aliceBid = ethers.parseEther("0.5");
            await auction.connect(alice).bid({value: aliceBid});

            // Bob 出价更高 -> Alice 的钱应该变成待退款
            await auction.connect(bob).bid({value: ethers.parseEther("1.0")});

            expect(await auction.pendingReturns(alice.address)).to.equal(aliceBid);
        });

        it("三个出价者竞争, 退款记录应该正确", async function () {
            await auction.connect(alice).bid({value: ethers.parseEther("0.5")}); // Alice: 0.5
            await auction.connect(bob).bid({ value: ethers.parseEther("1.0") }); // Bob: 1.0 → Alice 待退款 0.5
            await auction.connect(carol).bid({ value: ethers.parseEther("1.5") }); // Carol: 1.5 → Bob 待退款 1.0

            expect(await auction.pendingReturns(alice.address)).to.equal(ethers.parseEther("0.5"));
            expect(await auction.pendingReturns(bob.address)).to.equal(ethers.parseEther("1.0"));
            expect(await auction.pendingReturns(carol.address)).to.equal(0); // Carol 仍是最高者
        });

        it("同一人出价两次，不应该重复加入 allBidders", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });
            await auction.connect(alice).bid({ value: ethers.parseEther("1.0") });

            expect(await auction.bidderCount()).to.equal(1); 
        });

        it("allBidders 应该正确记录所有不同的出价者", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });
            await auction.connect(bob).bid({ value: ethers.parseEther("1.0") });
            await auction.connect(carol).bid({ value: ethers.parseEther("1.5") });

            expect(await auction.bidderCount()).to.equal(3);
            expect(await auction.isBidder(alice.address)).to.equal(true);
            expect(await auction.isBidder(bob.address)).to.equal(true);
            expect(await auction.isBidder(carol.address)).to.equal(true);
        });
    });

    // ==========================================
    // 3. 出价测试 — 错误情况
    // ==========================================
    describe("出价 — 错误情况", function () {
        it("出价 <= 当前最高出价应该失败", async function () {
            const highBid = ethers.parseEther("1.0");
            await auction.connect(alice).bid({value: highBid});

            // Bob 出同样价格
            await expect(auction.connect(alice).bid({value: highBid})).to.be.revertedWithCustomError(auction, "BidTooLow").withArgs(highBid, highBid + 1n);

            // Bob 出更低价格
            const lowBid = ethers.parseEther("0.5");
            await expect(auction.connect(alice).bid({value: lowBid})).to.be.revertedWithCustomError(auction, "BidTooLow").withArgs(lowBid, highBid + 1n);
        });

        it("第一轮出低于 minBid 应该失败", async function () {
            const tooLow = ethers.parseEther("0.01");
            await expect(auction.connect(alice).bid({value: tooLow})).to.be.revertedWithCustomError(auction, "BidTooLow").withArgs(tooLow, minBid);
            // 合约: 当 highestBid == 0 && msg.value < minBid 时 revert BidTooLow(msg.value, minBid)
            // 所以这里是 withArgs(tooLow, minBid)
        });

        it("拍卖结束后出价应该失败", async function () {
            await fastForwardToEnd();
            await auction.endAuction();

            await expect(auction.connect(alice).bid({value: ethers.parseEther("1.0")})).to.be.revertedWithCustomError(auction, "AuctionNotActive");
        });
    });

    // ==========================================
    // 4. Events 测试
    // ==========================================
    describe("Event", function() {
        it("出价时应该发出 NewBid 事件", async function () {
            const amount = ethers.parseEther("0.5");
            await expect(auction.connect(alice).bid({value:amount})).to.be.emit(auction, "NewBid").withArgs(alice.address, amount);
        });

        it("结束拍卖时应该发出 AuctionEnded 事件", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });
            await fastForwardToEnd();

            await expect(auction.endAuction())
                .to.emit(auction, "AuctionEnded")
                .withArgs(alice.address, ethers.parseEther("0.5"));
        });

        it("退款时应该发出 Withdrawal 事件", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });
            await auction.connect(bob).bid({ value: ethers.parseEther("1.0") });

            await expect(auction.connect(alice).withdraw())
                .to.emit(auction, "Withdrawal")
                .withArgs(alice.address, ethers.parseEther("0.5"));
        });
    });

    // ==========================================
    // 5. 退款测试
    // ==========================================
    describe("退款", function() {
        beforeEach(async function () {
            await auction.connect(alice).bid({value: ethers.parseEther("0.5")});
            await auction.connect(bob).bid({value: ethers.parseEther("1.0")});
            // Alice 有 0.5 ETH 待退款
        });

        it("被反超者应该能成功取回退款", async function () {
            const balanceBefore = await ethers.provider.getBalance(alice.address);

            const tx = await auction.connect(alice).withdraw();
            const receipt = await tx.wait();

            const gasUsed: bigint = receipt!.gasUsed;
            const gasPrice: bigint = BigInt(receipt!.gasPrice ?? 0n);
            const gasCost: bigint = gasUsed * gasPrice;

            const balanceAfter = await ethers.provider.getBalance(alice.address);
            const refund = ethers.parseEther("0.5");

            expect(balanceAfter).to.equal(balanceBefore + refund - gasCost);
            // 三个全是 bigint ✅
            expect(await auction.pendingReturns(alice.address)).to.equal(0);
        });

        it("没有待退款时 withdraw 应该失败", async function () {
            await expect(
                auction.connect(carol).withdraw()
            ).to.be.revertedWithCustomError(auction, "NothingToWithdraw");
        });

        it("重复 withdraw 第二次应该失败（余额已清零）", async function () {
            await auction.connect(alice).withdraw();
            await expect(
                auction.connect(alice).withdraw()
            ).to.be.revertedWithCustomError(auction, "NothingToWithdraw");
        });
    });

    // ==========================================
    // 6. 结束拍卖测试
    // ==========================================
    describe("结束拍卖", function () {
        it("时间到后，任何人可以结束拍卖", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });
            await fastForwardToEnd();

            // Carol（非 owner 非 bidder）也能结束
            await auction.connect(carol).endAuction();
            expect(await auction.ended()).to.equal(true);
        });

         it("时间未到时不能结束拍卖", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });

            await expect(
                auction.endAuction()
            ).to.be.revertedWithCustomError(auction, "AuctionNotEnded");
        });

        it("已经结束的拍卖不能再次结束", async function () {
            await fastForwardToEnd();
            await auction.endAuction();

            await expect(
                auction.endAuction()
            ).to.be.revertedWithCustomError(auction, "AuctionAlreadyEnded");
        });
    });

    // ==========================================
    // 7. Owner 提款测试
    // ==========================================
    describe("Owner 提款", function () {
        it("拍卖结束后 owner 能提取最高出价", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("1.0") });
            await fastForwardToEnd();
            await auction.endAuction();

            const balanceBefore = await ethers.provider.getBalance(owner.address);

            const tx = await auction.connect(owner).ownerWithdraw();
            const receipt = await tx.wait();

            const gasUsed: bigint = receipt!.gasUsed;
            const gasPrice: bigint = BigInt(receipt!.gasPrice ?? 0n);
            const gasCost: bigint = gasUsed * gasPrice;

            const balanceAfter = await ethers.provider.getBalance(owner.address);

            expect(balanceAfter).to.equal(balanceBefore + ethers.parseEther("1.0") - gasCost);
        });

        it("非 owner 不能提款", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("1.0") });
            await fastForwardToEnd();
            await auction.endAuction();

            await expect(
                auction.connect(alice).ownerWithdraw()
            ).to.be.revertedWithCustomError(auction, "NotOwner");
        });

        it("拍卖未结束不能提款", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("1.0") });

            await expect(
                auction.connect(owner).ownerWithdraw()
            ).to.be.revertedWithCustomError(auction, "AuctionNotEndedYet");
        });

        it("无人出价时提款应该失败", async function () {
            await fastForwardToEnd();
            await auction.endAuction();

            await expect(
                auction.connect(owner).ownerWithdraw()
            ).to.be.revertedWithCustomError(auction, "NoBidsPlaced");
        });
    });

    // ==========================================
    // 8. 视图函数测试
    // ==========================================
    describe("视图函数", function () {
        it("timeRemaining 应该返回正确剩余时间", async function () {
            const remaining = await auction.timeRemaining();
            // 应该接近 3600 秒（误差在 5 秒内）
            expect(remaining).to.be.closeTo(BigInt(3600), BigInt(5));
        });

        it("时间结束后 timeRemaining 应该返回 0", async function () {
            await fastForwardToEnd();
            expect(await auction.timeRemaining()).to.equal(0);
        });

        it("getStatus 应该返回完整状态", async function () {
            await auction.connect(alice).bid({ value: ethers.parseEther("0.5") });

            const status = await auction.getStatus();
            expect(status._item).to.equal("Rare NFT");
            expect(status._highestBidder).to.equal(alice.address);
            expect(status._highestBid).to.equal(ethers.parseEther("0.5"));
            expect(status._ended).to.equal(false);
        });
    });
});