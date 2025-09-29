const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("MyERC20", function () {
    let MyERC20
    let token;
    let owner;
    let address1;
    let address2;
    let address;

    // 在每个测试之前部署新的合约
    beforeEach(async function () {
        MyERC20 = await ethers.getContractFactory("MyERC20");
        [owner, address1, address2, ...address] = await ethers.getSigners();
        
        // 部署合约，初始供应商 1000 个代币
        token = await MyERC20.deploy("MyToken", "MTK", 1000);
    });

    // 测试基本信息
    describe("基本信息", function() {
        it("应该设置正确的名称、符号和初始供应商", async function () {
            expect(await token.name()).to.equal("MyToken");
            expect(await token.symbol()).to.equal("MTK");
            expect(await token.decimals()).to.equal(18);

            // 初始供应是 1000
            const initialSupply = ethers.parseEther("1000");
            expect(await token.totalSupply()).to.equal(initialSupply);

            // 初始化供应应该全部属于部署者
            expect(await token.balanceOf(owner.address)).to.equal(initialSupply);
        });
    });

    // 测试转账功能
    describe("转账功能", function() {
        it("应该正确转账代币", async function () {
            // 转账 100 个代币给 address1 
            const amount = ethers.parseEther("100");
            await token.transfer(address1.address, amount);

            // 检查余额是否正确
            expect(await token.balanceOf(address1.address)).to.equal(amount);

            // address1 转账 50 个代币给 address2
            await token.connect(address1).transfer(address2.address, amount/2n);
            expect(await token.balanceOf(address1.address)).to.equal(amount/2n);
            expect(await token.balanceOf(address2.address)).to.equal(amount/2n);
        });

        it("应该拒绝余额不足的转账", async function () {
            // 尝试转账超过余额的数量
            const largeAmount = ethers.parseEther("10000");
            await expect(
                token.connect(address1).transfer(owner.address, largeAmount)
            ).to.be.revertedWith("余额不足");
        });

        it("应该拒绝转账到零地址", async function () {
           const amount = ethers.parseEther("1000");
           await expect(
                token.transfer(ethers.ZeroAddress, amount)
           ).to.be.revertedWith("不能转账到零地址");
        });
    });

    // 测试授权和代扣功能
    describe("授权和代扣功能", function() {
        it("应该正确授权并使用授权额度转账", async function () {
            const amount = ethers.parseEther("200");

            // 所有者授权 address1 可以花费 200 个代币
            await token.approval(address1, amount);
            expect(await token.allowance(owner.address, address1.address)).to.equal(amount);

            // address1 从所有者账户转账 150 个代币到 address2
            const transferAmount = ethers.parseEther("150");
            await token.connect(address1).transferFrom(owner.address, address2.address, transferAmount);

            // 检查余额
            expect(await token.balanceOf(owner.address)).to.equal(
                ethers.parseEther("850")
            );
            expect(await token.balanceOf(address2)).to.equal(transferAmount);

            // 检查剩余授权额度
            expect(await token.allowance(owner.address, address1.address)).to.equal(amount - transferAmount);
        });
    });

    describe("增发功能", function() {
        it("应该允许所有者增发代币", async function () {
            const initialSupply = await token.totalSupply();
            const mintAmount = ethers.parseEther("500");

            // 所有者增发 500 个代币给 address1
            await token.mint(address1.address, mintAmount);

            // 检查总供量增加
            expect(await token.totalSupply()).to.equal(initialSupply + mintAmount);

            // 检查 address1 余额增加
            expect(await token.balanceOf(address1.address)).to.equal(mintAmount);
        });

        it("应该拒绝非所有者增发代币", async function () {
            const mintAmount = ethers.parseEther("500");

            // 非所有者尝试增发代币
            await expect(
                token.connect(address1).mint(address1.address, mintAmount)
            ).to.be.revertedWith("仅所有者可增发代币");
        });
    });
})