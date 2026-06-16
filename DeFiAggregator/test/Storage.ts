import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("Storage", function() {
    let storage: any;
    let owner: any;
    let user1: any;
    let user2: any;

    beforeEach(async function () {
        // 获取测试用户账户
        [owner, user1, user2] = await ethers.getSigners();
    
        // 部署合约 （Hardhat3 方式）
        storage = await ethers.deployContract("Storage");
    });

    describe("部署", function() {
        it("应该设置正确的 owner", async function () {
            expect(await storage.owner()).to.equal(owner.address);
        });

        it("初始状态应该正确", async function () {
            expect(await storage.storedNumber()).to.equal(0);
            expect(await storage.storedString()).to.equal("");
            expect(await storage.isLocked()).to.equal(false);
        });
    });

    describe("数字存储", function() {
        it("应该正确设置数字", async function () {
            await storage.setNumber(42);
            expect(await storage.storedNumber()).to.equal(42);
        });

        it("应该触发 NumberChanged 事件", async function () {
            await expect(storage.setNumber(42))
                .to.emit(storage, "NumberChanged")
                .withArgs(owner.address, 0, 42);
            });

            it("锁定时不能设置数字", async function () {
            await storage.lock();
            await expect(storage.setNumber(42)).to.be.revertedWith("Contract is locked");
        });
    });

     describe("字符串存储", function () {
        it("应该正确设置字符串", async function () {
            await storage.setString("Hello, Hardhat 3!");
            expect(await storage.storedString()).to.equal("Hello, Hardhat 3!");
        });

        it("应该触发 StringChanged 事件", async function () {
            await expect(storage.setString("test"))
                .to.emit(storage, "StringChanged")
                .withArgs(owner.address, "", "test");
        });
    });

    describe("ETH 存取", function () {
        it("应该正确存入 ETH", async function () {
            const amount = ethers.parseEther("1.0");
            await storage.connect(user1).deposit({ value: amount });
            expect(await storage.getBalance(user1.address)).to.equal(amount);
        });

        it("应该触发 Deposited 事件", async function () {
        const amount = ethers.parseEther("1.0");
            await expect(storage.connect(user1).deposit({ value: amount }))
                .to.emit(storage, "Deposited")
                .withArgs(user1.address, amount);
        });

        it("应该正确取出 ETH", async function () {
            const amount = ethers.parseEther("1.0");
            await storage.connect(user1).deposit({ value: amount });

            const halfAmount = ethers.parseEther("0.5");
            await storage.connect(user1).withdraw(halfAmount);

            expect(await storage.getBalance(user1.address)).to.equal(halfAmount);
        });

        it("余额不足时应该失败", async function () {
        const amount = ethers.parseEther("1.0");
        await expect(storage.connect(user1).withdraw(amount)).to.be.revertedWith(
            "Insufficient balance"
        );
        });
    });

    describe("数组操作", function () {
        it("应该正确添加元素", async function () {
            await storage.addToArray(10);
            await storage.addToArray(20);

            expect(await storage.getArrayLength()).to.equal(2);
            expect(await storage.getArrayElement(0)).to.equal(10);
            expect(await storage.getArrayElement(1)).to.equal(20);
        });

        it("超出索引应该失败", async function () {
            await expect(storage.getArrayElement(0)).to.be.revertedWith("Index out of bounds");
        });
    });

    describe("白名单", function () {
        it("owner 可以添加白名单", async function () {
            await storage.addToWhitelist(user1.address);
            expect(await storage.isWhitelisted(user1.address)).to.equal(true);
        });

        it("非 owner 不能添加白名单", async function () {
            await expect(
                storage.connect(user1).addToWhitelist(user2.address)
            ).to.be.revertedWith("Not owner");
        });
    });
});