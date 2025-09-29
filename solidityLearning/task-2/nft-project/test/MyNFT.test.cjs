const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("MyNFT", function () {
  let MyNFT;
  let myNFT;
  let owner;
  let addr1;
  let tokenURI = "ipfs://bafkreigl2wgetrgx4gcd6mftph4koorcmakqlhsfzdfzllkobh5ibzjep4";

  beforeEach(async function () {
    // 获取合约工厂和签名者
    MyNFT = await ethers.getContractFactory("MyNFT");
    [owner, addr1] = await ethers.getSigners();

    // 部署合约
    myNFT = await MyNFT.deploy("My Awesome NFT", "MAN");
    await myNFT.waitForDeployment();
  });

  describe("部署", function () {
    it("应该设置正确的名称和符号", async function () {
      expect(await myNFT.name()).to.equal("My Awesome NFT");
      expect(await myNFT.symbol()).to.equal("MAN");
      expect(await myNFT.owner()).to.equal(owner.address);
    });

    it("初始tokenId应该为0", async function () {
      expect(await myNFT.getCurrentTokenId()).to.equal(0);
    });
  });

  describe("铸造NFT", function () {
    it("应该允许所有者铸造NFT", async function () {
      // 铸造NFT
      await expect(myNFT.mintNFT(addr1.address, tokenURI))
        .to.emit(myNFT, "Transfer")
        .withArgs(ethers.ZeroAddress, addr1.address, 0);

      // 验证所有权
      expect(await myNFT.ownerOf(0)).to.equal(addr1.address);
      
      // 验证tokenId递增
      expect(await myNFT.getCurrentTokenId()).to.equal(1);
    });

    it("不允许非所有者铸造NFT", async function () {
      // 尝试使用非所有者账户铸造
      await expect(
        myNFT.connect(addr1).mintNFT(addr1.address, tokenURI)
      ).to.be.revertedWithCustomError(myNFT, "OwnableUnauthorizedAccount");
    });
  });
});
