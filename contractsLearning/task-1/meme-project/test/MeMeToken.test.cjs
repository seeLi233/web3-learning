const { expect } = require("chai");
const { ethers } = require("hardhat");

const SEOPLIA_UNISWAP = {
  router: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D" // 社区 Router 地址
}

describe("MeMeToken Test", function () {
  let token;
  let owner, user1, user2;
  let treasury;
  let marketing;

  let router, factory, pair;

  // 复用部署场景
  this.beforeEach(async function () {
    [owner, user1, user2, treasury, marketing] = await ethers.getSigners();

    console.log(SEOPLIA_UNISWAP.router)
    router = await ethers.getContractAt("IUniswapV2Router02", SEOPLIA_UNISWAP.router);
    //router = await MockRouter.deploy();
    console.log(await router.address)
    
    // 部署合约
    const MeMeToken = await ethers.getContractFactory("MeMeToken");
    token = await MeMeToken.deploy("MeMeToken", "MMT", ethers.parseEther("1000000000"), router.address);

    await token.waitForDeployment();
    
    // 初始化地址（非必须，测试用）
    await token.setTreasuryAddress(treasury.address);
    await token.setMarketingAddress(marketing.address);
  })

  // 测试1：基本信息验证
  describe("Deployment", function () {
    it("Should set the right owner", async function () {
      expect(await token.owner()).to.equal(owner.address);
    });

    it("Should have correct name and symbol", async function () {
      expect(await token.name()).to.equal("MeMeToken");
      expect(await token.symbol()).to.equal("MMT");
    });

    it("Should have correct total supply", async function () {
      expect(await token.totalSupply()).to.equal(ethers.parseEther("1000000000"));
    });
  });

  // 测试2：交易税机制
  describe("Tax Mechanism", function () {
    it("Should deduct tax and distribute to treasury, burn, and marketing", async function () {     
      // 转账前余额
      const user1InitialBal = await token.balanceOf(user1.address);
      const treasuryInitialBal = await token.balanceOf(treasury.address);
      const marketingInitialBal = await token.balanceOf(marketing.address);

      // 转账100代币（owner是非豁免地址？不，构造函数中owner是豁免的，需先移除豁免）
      await token.removeExemptAddress(owner.address);
      const transferAmount = ethers.parseEther("100");
      await token.transfer(user1.address, transferAmount);

      // 验证税费分配（总税率10% = 10代币）
      // 国库占50%（5代币），销毁30%（3代币），营销20%（2代币）
      expect(await token.balanceOf(user1.address)).to.equal(user1InitialBal + ethers.parseEther("90"));  // 100 - 10税
      expect(await token.balanceOf(treasury.address)).to.equal(treasuryInitialBal + ethers.parseEther("5"));
      expect(await token.balanceOf(marketing.address)).to.equal(marketingInitialBal + ethers.parseEther("2"));
    });

    it("Should not deduct tax for exempt addresses", async function () {
      // owner是豁免地址（默认），转账不扣税
      const transferAmount = ethers.parseEther("100");
      await token.transfer(user1.address, transferAmount);
      expect(await token.balanceOf(user1.address)).to.equal(transferAmount);  // 全额到账
    });
  });

  // 测试3：交易限制
  describe("Transaction Limits", function () {
    it("Should reject transactions exceeding max amount", async function () {
      //const { token, owner, user1 } = await loadFixture(deployTokenFixture);
      // 总供应量100万，最大单笔5% = 5万代币
      const maxTxAmount = ethers.parseEther("50000000");
      const overLimitAmount = maxTxAmount + ethers.parseEther("1");  // 50001

      // 移除owner豁免，使其受限制
      await token.removeExemptAddress(owner.address);
      
      // 超过单笔限制的交易应失败
      await expect(
        token.transfer(user1.address, overLimitAmount)
      ).to.be.revertedWith("Transaction exceeds max amount");
    });

    it("Should reject transactions exceeding daily limit", async function () {
      // 移除owner豁免
      await token.removeExemptAddress(owner.address);
      // 每日最大10笔交易
      const txAmount = ethers.parseEther("100");

      // 发送10笔交易（正常）
      for (let i = 0; i < 10; i++) {
        await token.transfer(user1.address, txAmount);
      }

      // 第11笔应失败
      await expect(
        token.transfer(user1.address, txAmount)
      ).to.be.revertedWith("Exceeded daily transaction limit");
    });
  });

  // 测试4：权限管理
  describe("Access Control", function () {
    it("Should allow owner to update tax rate", async function () {
      await token.setTaxRate(15);
      expect(await token.taxRate()).to.equal(15);
    });

    it("Should reject non-owners from updating tax rate", async function () {
      await expect(
        token.connect(user1).setTaxRate(15)
      ).to.be.revertedWithCustomError(token, 'OwnableUnauthorizedAccount').withArgs(user1.address)
    });

    it("Should allow owner to add/remove exempt addresses", async function () {
      // 添加豁免
      await token.addExemptAddress(user1.address);
      expect(await token.isExempt(user1.address)).to.be.true;
      
      // 移除豁免
      await token.removeExemptAddress(user1.address);
      expect(await token.isExempt(user1.address)).to.be.false;
    });
  });
});