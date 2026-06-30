import { network } from "hardhat";
import { expect } from "chai";

const { ethers } = await network.create();

describe("RoyaltyNFT", function(){
    let royaltyNft: any;
    let owner: any;
    let alice: any;
    let bob:any;
    let kric:any;
        
    const name = "RoyaltyNFt";
    const symbol = "RNF";
    //const royaltyReceiver = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"; // Hardhat 账户 #1
    const royaltyBps = 500; // 5%
    
    beforeEach(async function() {
        [owner, alice, bob, kric] = await ethers.getSigners();
        royaltyNft = await ethers.deployContract("RoyaltyNFT", [name, symbol, alice, royaltyBps]);
    });

    // =============================================
    // 测试组 1: 部署 + 基础信息
    // =============================================
    describe("Deployment", function(){
        it("应该正确设置名称和符号", async function () {
            expect(await royaltyNft.name()).to.equal(name);
            expect(await royaltyNft.symbol()).to.equal(symbol);
        });

        it("部署者应该是 owner", async function () {
            expect(await royaltyNft.owner()).to.equal(owner.address);
        });
    });

    // =============================================
    // 测试组 2: ERC2981 版税查询 — 今日核心！
    // =============================================
    describe("ERC2981 Royalty", function () {
        it("应该返回正确的默认版税信息", async function () {
            await royaltyNft.safeMint(bob.address);
            const tokenId = 0;

            // 查询版税: 假设售价 = 1 ETH = 1e18 wei
            const salePrice = ethers.parseEther("1.0");
            const [receiver, royaltyAmount] = await royaltyNft.royaltyInfo(tokenId, salePrice);

            // 验证版税接收者
            expect(receiver).to.equal(alice);

            // 验证版税金额: 1 ETH × 500 / 10000 = 0.05 ETH
            const expectedRoyalty = ethers.parseEther("0.05");
            expect(royaltyAmount).to.equal(expectedRoyalty);
        });

        it("不同售价应该返回等比例的版税", async function () {
            await royaltyNft.safeMint(bob.address);
            
            // 售价 2 ETH → 版税 = 2 × 5% = 0.1 ETH
            const [, royalty2] = await royaltyNft.royaltyInfo(0, ethers.parseEther("2.0"));
            expect(royalty2).to.equal(ethers.parseEther("0.1"));

            // 售价 0.5 ETH → 版税 = 0.5 × 5% = 0.025 ETH
            const [, royaltyHalf] = await royaltyNft.royaltyInfo(0, ethers.parseEther("0.5"));
            expect(royaltyHalf).to.equal(ethers.parseEther("0.025"));
        });

        it("ERC165 应该支持 IERC2981 接口", async function () {
            // IERC2981 的 interfaceId = bytes4(keccak256("royaltyInfo(uint256,uint256)"))
            const IERC2981_INTERFACE_ID = "0x2a55205a";
            expect(await royaltyNft.supportsInterface(IERC2981_INTERFACE_ID)).to.be.true;
        });

        it("也应该支持 IERC721 接口", async function () {
            // IERC721 的 interfaceId
            const IERC721_INTERFACE_ID = "0x80ac58cd";
            expect(await royaltyNft.supportsInterface(IERC721_INTERFACE_ID)).to.be.true;
        });

        it("非 owner 不能修改版税", async function () {
            // accounts[1] 不是 owner，尝试修改版税应该失败
            await expect(
                royaltyNft.connect(alice).setDefaultRoyalty(
                    alice.address,
                    1000 // 10%
                )
            ).to.be.revertedWithCustomError(royaltyNft, "OwnableUnauthorizedAccount");
        });

        it("owner 可以修改版税率", async function () {
            // 铸造一个 NFT
            await royaltyNft.safeMint(bob.address);

            // owner 把版税率改为 10% (1000 bps)
            await royaltyNft.setDefaultRoyalty(alice.address, 1000);

            // 验证: 1 ETH → 版税 = 1 × 10% = 0.1 ETH
            const [, royalty] = await royaltyNft.royaltyInfo(
                0,
                ethers.parseEther("1.0")
            );
            expect(royalty).to.equal(ethers.parseEther("0.1"));
        });
    });

    // =============================================
    // 测试组 3: 铸造 + 链上 SVG
    // =============================================
    describe("Minting & On-Chain SVG", function () {
        it("owner 可以铸造 NFT", async function () {
            await royaltyNft.safeMint(bob.address);
            expect(await royaltyNft.ownerOf(0)).to.equal(bob.address);
            expect(await royaltyNft.balanceOf(bob.address)).to.equal(1);
        });

        it("非 owner 铸造应该失败", async function () {
            await expect(
                royaltyNft.connect(bob).safeMint(bob.address)
            ).to.be.revertedWithCustomError(royaltyNft, "OwnableUnauthorizedAccount");
        });

        it("tokenURI 应该返回 data URI 格式", async function () {
            await royaltyNft.safeMint(bob.address);

            const uri = await royaltyNft.tokenURI(0);

            // 验证 data URI 格式
            expect(uri).to.include("data:application/json;base64,");

            console.log("\n📐 链上 tokenURI (前 100 字符):");
            console.log(uri.substring(0, 100) + "...");
        });

        it("不同 tokenId 应该生成不同颜色的 SVG", async function () {
            await royaltyNft.safeMint(bob.address);
            await royaltyNft.safeMint(bob.address);
            await royaltyNft.safeMint(bob.address);

            // 不同 token 的 SVG 应该不同 (颜色由 tokenId 决定)
            const svg0 = await royaltyNft.generateSVG(0);
            const svg1 = await royaltyNft.generateSVG(1);
            const svg2 = await royaltyNft.generateSVG(2);

            expect(svg0).to.not.equal(svg1);
            expect(svg1).to.not.equal(svg2);

            console.log("\n🎨 SVG #0 (前 200 字符):");
            console.log(svg0.substring(0, 200));
        });
    });

    // =============================================
    // 测试组 4: ERC721 基本功能
    // =============================================
    describe("ERC721 Basic", function () {
        it("应该正确转账", async function () {
            await royaltyNft.safeMint(bob.address);

            // accounts[1] 把 NFT 转给 accounts[2]
            await royaltyNft.connect(bob).transferFrom(
                bob.address,
                kric.address,
                0
            );

            expect(await royaltyNft.ownerOf(0)).to.equal(kric.address);
        });

        it("应该支持燃烧", async function () {
            await royaltyNft.safeMint(bob.address);

            // accounts[1] 燃烧自己的 NFT
            await royaltyNft.connect(bob).burn(0);

            // 燃烧后查询应该 revert
            await expect(royaltyNft.ownerOf(0)).to.be.revertedWithCustomError(
                royaltyNft,
                "ERC721NonexistentToken"
            );
        });

        it("枚举应该正常工作", async function () {
            // 铸造 3 个 NFT
            await royaltyNft.safeMint(bob.address);
            await royaltyNft.safeMint(bob.address);
            await royaltyNft.safeMint(bob.address);

            expect(await royaltyNft.totalSupply()).to.equal(3);
            expect(await royaltyNft.tokenByIndex(0)).to.equal(0);
            expect(await royaltyNft.tokenByIndex(1)).to.equal(1);
            expect(await royaltyNft.tokenByIndex(2)).to.equal(2);
        });
    });

    // =============================================
    // 测试组 5: 边界条件 + 版税边界值
    // =============================================
    describe("Edge Cases", function () {
        it("零地址铸造应该失败", async function () {
            await expect(
                royaltyNft.safeMint(ethers.ZeroAddress)
            ).to.be.revertedWithCustomError(royaltyNft, "RoyaltyNFT__ZeroAddress");
        });

        it("查询不存在的 tokenId 的 tokenURI 应该失败", async function () {
            await expect(
                royaltyNft.tokenURI(999)
            ).to.be.revertedWithCustomError(royaltyNft, "RoyaltyNFT__TokenNotFound");
        });

        it("版税为 0% 应该返回 0", async function () {
            // 先改版税为 0
            await royaltyNft.setDefaultRoyalty(alice.address, 0);

            await royaltyNft.safeMint(bob.address);

            const [, royalty] = await royaltyNft.royaltyInfo(0, ethers.parseEther("1.0"));
            expect(royalty).to.equal(0);
        });

        it("售价为 0 时版税也为 0", async function () {
            await royaltyNft.safeMint(bob.address);

            const [, royalty] = await royaltyNft.royaltyInfo(0, 0);
            expect(royalty).to.equal(0);
        });
    });
});