const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("BeggingContract", function() {
    let beggingContract;
    let owner;
    let address1;
    let address2;
    let address3;
    let address;
    
    beforeEach(async function () {
        // 获取合约工厂和签名者
        const BeggingContract = await ethers.getContractFactory("BeggingContract");
        [owner, address1, address2, address3, ...address] = await ethers.getSigners();

        // 部署合约
        beggingContract = await BeggingContract.deploy();
        await beggingContract.waitForDeployment();
    });
    
    describe("Deployment", function() {
        it("Should set the right owner", async function () {
            // 在合约中 owner 是 private 的，所以这个测试需要配合一个 getOwner() 函数
            // 
            expect(await beggingContract.getOwner()).to.equal(owner.address);
        });

        it("Should have zero balance initially", async function () {
            expect(await beggingContract.getContractBalance()).to.equal(0);
        });
    });

    describe("Donations", function () {
        it("Should allow users to donate", async function () {
            // address1 捐赠 1 ETH
            await expect(beggingContract.connect(address1).donate({ value: ethers.parseEther("1.0") })).to
            .changeEtherBalances(
                [address1, beggingContract],
                [ethers.parseEther("-1.0"), ethers.parseEther("1.0")]
            );

            // 检查捐赠记录
            expect(await beggingContract.getDonation(address1.address)).to.equal(ethers.parseEther("1.0"));
        });

        it("Should not allow zero donations", async function () {
            await expect(
                beggingContract.connect(address1).donate({ value: 0 })
            ).to.be.revertedWith("Donation amount must be greater than 0");
        });

        it("Should emit DonationReceived event", async function () {
            await expect(beggingContract.connect(address1).donate({ value:ethers.parseEther("0.5") })).to
            .emit(beggingContract, "DonationReceived").withArgs(address1.address, ethers.parseEther("0.5"));
        });
    });


    describe("Withdrawal", function () {
        it("Should allow owner to withdraw funds", async function () {
            // 先让 address1 捐赠 1 ETH
            await beggingContract.connect(address1).donate({ value: ethers.parseEther("1.0") });

            // 先记录所有者初始余额
            const initialOwnerBanlance = await ethers.provider.getBalance(owner.address);

            // 所有者提取资金
            const tx = await beggingContract.withdraw();
            const receipt = await tx.wait();
            const gasCost = receipt.gasUsed * receipt.gasPrice;

            // 检查余额变化（考虑 gas 费用）
            const finalOwnerBalance = await ethers.provider.getBalance(owner.address);
            expect(finalOwnerBalance).to.be.closeTo(
                initialOwnerBanlance + ethers.parseEther("1.0") - gasCost,
                ethers.parseEther("0.01") // 允许小额误差
            );

            // 合约余额应为 0
            expect(await beggingContract.getContractBalance()).to.equal(0);
        });

        it("Should not allow non-owner to withdraw", async function () {
            await expect(
                beggingContract.connect(address1).withdraw()
            ).to.be.rejectedWith("Only contract owner can call this function");
        });
    });

    describe("Top Donors", function () {
        it("Should maintain top 3 donors", async function () {
            // 三个地址分别捐赠不同金额
            await beggingContract.connect(address1).donate({ value: ethers.parseEther("1.0") });
            await beggingContract.connect(address2).donate({ value: ethers.parseEther("3.0") });
            await beggingContract.connect(address3).donate({ value: ethers.parseEther("2.0") });

            // 检查前三名
            expect(await beggingContract.topDonors(0)).to.equal(address2.address); // 3 ETH
            expect(await beggingContract.topDonors(1)).to.equal(address3.address); // 2 ETH
            expect(await beggingContract.topDonors(2)).to.equal(address1.address); // 1 ETH
        });
    });

    describe("Donation Period", function () {
        it("Should restrict donation outside period", async function () {
            // 设置捐赠时间为10秒后开始，持续10秒
            const now = Math.floor(Date.now() / 1000);
            const startTime = now + 100;
            const endTime = startTime + 100;

            await beggingContract.setDonationPeriod(startTime, endTime);

            // 现在尝试捐赠应该失败
            await expect(
                beggingContract.connect(address1).donate({ value:ethers.parseEther("1.0") })
            ).to.be.revertedWith("Donations are only allowed during the specified period");
        });
        
    });
});