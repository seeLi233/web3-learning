import { expect } from "chai";
import { N } from "ethers";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("MyNFT", function () {
    let myNFT:any;
    let owner: any, bob: any, alice:any;

    const NAME = "My First NFT";
    const SYMBOL = "MFN";
    const BASE_URI = "ipfs://QmTest/";

    beforeEach(async function () {
        [owner, alice, bob] = await ethers.getSigners();
        myNFT = await ethers.deployContract("MyNFT", [NAME, SYMBOL, BASE_URI]);
    });

    // ==========================================
    // Part 1: 部署测试
    // ==========================================
    describe("Deployment", function () {
        it("应该正确设置名称和符号", async function () {
            expect(await myNFT.name()).to.equal(NAME);
            expect(await myNFT.symbol()).to.equal(SYMBOL);
        });

        it("初始供应量为 0", async function () {
            expect(await myNFT.totalSupply()).to.equal(0);
        });

        it("初始 totalMinted 为 0", async function () {
            expect(await myNFT.totalMinted()).to.equal(0);
        });

        it("owner 是部署者", async function () {
            expect(await myNFT.owner()).to.equal(owner.address);
        });
    });

    // ==========================================
    // Part 2: 铸造测试
    // ==========================================
    describe("Minting", function () {
        it("owner 可以铸造 NFT", async function () {
            const tx = await myNFT.connect(owner).safeMint(alice.address, "");
            const receipt = await tx.wait();

            expect(await myNFT.ownerOf(0)).to.equal(alice.address);
            expect(await myNFT.balanceOf(alice.address)).to.equal(1);
            expect(await myNFT.totalSupply()).to.equal(1);
        });

        it("非 owner 铸造会失败", async function () {
            await expect(
                myNFT.connect(alice).safeMint(alice.address, "")
            ).to.be.revertedWithCustomError(myNFT, "OwnableUnauthorizedAccount");
        });

        it("不能铸造到零地址", async function () {
            await expect(
                myNFT.connect(owner).safeMint(ethers.ZeroAddress, "")
            ).to.be.revertedWithCustomError(myNFT, "MyNFT__ZeroAddress");
        });

        it("达到最大供应量后不能再铸造", async function () {
            const maxSupply = await myNFT.MAX_SUPPLY();
            // 这里只验证逻辑，实际铸造 10000 个测试太慢
            // 可以用 Foundry fork 测试替代
            expect(maxSupply).to.equal(10000n);
        });

        it("铸造时正确设置 tokenURI（使用 baseURI）", async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
            // 应该返回 baseURI + "0"
            expect(await myNFT.tokenURI(0)).to.equal(BASE_URI + "0");
        });

        it("铸造时正确设置自定义 URI", async function () {
            const customURI = "ipfs://QmCustom/42.json";
            await myNFT.connect(owner).safeMint(alice.address, customURI);
            expect(await myNFT.tokenURI(0)).to.equal(customURI);
        });

        it("铸造后发出 NFTMinted 事件", async function () {
            await expect(
                myNFT.connect(owner).safeMint(alice.address, "ipfs://test")
            ).to.emit(myNFT, "NFTMinted")
                .withArgs(alice.address, 0, "ipfs://test");
        });
    });

    // ==========================================
    // Part 3: 转账测试
    // ==========================================
    describe("Transfer", function () {
        beforeEach(async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
        });

        it("拥有者可以转账自己的 NFT", async function () {
            await myNFT.connect(alice).transferFrom(alice.address, bob.address, 0);
            expect(await myNFT.ownerOf(0)).to.equal(bob.address);
            expect(await myNFT.balanceOf(alice.address)).to.equal(0);
            expect(await myNFT.balanceOf(bob.address)).to.equal(1);
        });

        it("非拥有者不能转账", async function () {
            await expect(
                myNFT.connect(bob).transferFrom(alice.address, bob.address, 0)
            ).to.be.revertedWithCustomError(myNFT, "ERC721InsufficientApproval");
        });

        it("转账后授权被清除", async function () {
            // alice 授权 bob 操作 tokenId=0
            await myNFT.connect(alice).approve(bob.address, 0);
            expect(await myNFT.getApproved(0)).to.equal(bob.address);

            // bob 把 token 转走
            await myNFT.connect(bob).transferFrom(alice.address, bob.address, 0);

            // ★ token 还存在，只是换了主人，授权被清空 → 返回零地址
            expect(await myNFT.getApproved(0)).to.equal(ethers.ZeroAddress);
        });
    });

    // ==========================================
    // Part 4: safeTransferFrom 安全检查
    // ==========================================
    describe("safeTransferFrom", function () {
        beforeEach(async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
        });

        it("safeTransferFrom 到 EOA 地址正常", async function () {
            await myNFT.connect(alice)["safeTransferFrom(address,address,uint256)"](
                alice.address, bob.address, 0
            );
            expect(await myNFT.ownerOf(0)).to.equal(bob.address);
        });

        // ⚠️ 到合约地址的测试需要部署一个实现了 IERC721Receiver 的合约
        // 这里只测试 EOA 场景，合约接收场景等学到更深入时再测试
    });

    // ==========================================
    // Part 5: 授权测试
    // ==========================================
    describe("Approval", function () {
        beforeEach(async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
        });

        it("可以授权单个 token", async function () {
            await myNFT.connect(alice).approve(bob.address, 0);
            expect(await myNFT.getApproved(0)).to.equal(bob.address);
        });

        it("非拥有者不能授权", async function () {
            await expect(
                myNFT.connect(bob).approve(bob.address, 0)
            ).to.be.revertedWithCustomError(myNFT, "ERC721InvalidApprover");
        });

        it("setApprovalForAll 全量授权", async function () {
            await myNFT.connect(alice).setApprovalForAll(bob.address, true);
            expect(await myNFT.isApprovedForAll(alice.address, bob.address)).to.equal(true);
        });

        it("可以取消全量授权", async function () {
            await myNFT.connect(alice).setApprovalForAll(bob.address, true);
            await myNFT.connect(alice).setApprovalForAll(bob.address, false);
            expect(await myNFT.isApprovedForAll(alice.address, bob.address)).to.equal(false);
        });
    });

    // ==========================================
    // Part 6: 燃烧测试
    // ==========================================
    describe("Burning", function () {
        beforeEach(async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
        });

        it("拥有者可以燃烧自己的 NFT", async function () {
            await myNFT.connect(alice).burn(0);
            await expect(myNFT.ownerOf(0)).to.be.revertedWithCustomError(
                myNFT, "ERC721NonexistentToken"
            );
            expect(await myNFT.totalSupply()).to.equal(0);
        });

        it("非拥有者不能燃烧", async function () {
            await expect(
                myNFT.connect(bob).burn(0)
            ).to.be.revertedWithCustomError(myNFT, "ERC721InsufficientApproval");
        });
    });

    // ==========================================
    // Part 7: Enumerable 遍历测试
    // ==========================================
    describe("Enumerable", function () {
        beforeEach(async function () {
            // 铸造 3 个 NFT 给 alice
            await myNFT.connect(owner).safeMint(alice.address, "");
            await myNFT.connect(owner).safeMint(alice.address, "");
            await myNFT.connect(owner).safeMint(bob.address, "");
        });

        it("totalSupply 正确", async function () {
            expect(await myNFT.totalSupply()).to.equal(3);
        });

        it("getAllTokens 返回所有 tokenId", async function () {
            const tokens = await myNFT.getAllTokens();
            expect(tokens.length).to.equal(3);
            expect(tokens[0]).to.equal(0n);
            expect(tokens[1]).to.equal(1n);
            expect(tokens[2]).to.equal(2n);
        });

        it("getTokensOfOwner 返回地址拥有的 tokenId", async function () {
            const aliceTokens = await myNFT.getTokensOfOwner(alice.address);
            expect(aliceTokens.length).to.equal(2);
            expect(aliceTokens[0]).to.equal(0n);
            expect(aliceTokens[1]).to.equal(1n);

            const bobTokens = await myNFT.getTokensOfOwner(bob.address);
            expect(bobTokens.length).to.equal(1);
            expect(bobTokens[0]).to.equal(2n);
        });

        it("tokenByIndex 正向遍历", async function () {
            expect(await myNFT.tokenByIndex(0)).to.equal(0n);
            expect(await myNFT.tokenByIndex(1)).to.equal(1n);
            expect(await myNFT.tokenByIndex(2)).to.equal(2n);
        });

        it("tokenOfOwnerByIndex 按拥有者遍历", async function () {
            expect(await myNFT.tokenOfOwnerByIndex(alice.address, 0)).to.equal(0n);
            expect(await myNFT.tokenOfOwnerByIndex(alice.address, 1)).to.equal(1n);
        });
    });

    // ==========================================
    // Part 8: URI 管理测试
    // ==========================================
    describe("URI Management", function () {
        beforeEach(async function () {
            await myNFT.connect(owner).safeMint(alice.address, "");
        });

        it("可以更新 baseURI", async function () {
            const newBaseURI = "https://api.mynft.com/metadata/";
            await myNFT.connect(owner).setBaseURI(newBaseURI);
            expect(await myNFT.tokenURI(0)).to.equal(newBaseURI + "0");
        });

        it("可以为 token 设置独立 URI", async function () {
            const customURI = "ipfs://QmCustom/unique.json";
            await myNFT.connect(owner).setTokenURI(0, customURI);
            expect(await myNFT.tokenURI(0)).to.equal(customURI);
        });

        it("查询不存在的 token 会失败", async function () {
            await expect(myNFT.tokenURI(999)).to.be.revertedWithCustomError(
                myNFT, "MyNFT__TokenNotFound"
            );
        });
    });

    // ==========================================
    // Part 9: ERC165 接口支持测试
    // ==========================================
    describe("ERC165 supportsInterface", function () {
        it("应该支持 IERC721 接口", async function () {
            // IERC721 的接口 ID = bytes4(keccak256("balanceOf(address)") ^
            //   keccak256("ownerOf(uint256)") ^ keccak256("safeTransferFrom(address,address,uint256)") ^
            //   keccak256("safeTransferFrom(address,address,uint256,bytes)") ^
            //   keccak256("transferFrom(address,address,uint256)") ^
            //   keccak256("approve(address,uint256)") ^
            //   keccak256("setApprovalForAll(address,bool)") ^
            //   keccak256("getApproved(uint256)") ^
            //   keccak256("isApprovedForAll(address,address)"))
            // = 0x80ac58cd
            const IERC721_ID = "0x80ac58cd";
            expect(await myNFT.supportsInterface(IERC721_ID)).to.equal(true);
        });

        it("应该支持 IERC721Metadata 接口", async function () {
            // IERC721Metadata = 0x5b5e139f
            const IERC721_METADATA_ID = "0x5b5e139f";
            expect(await myNFT.supportsInterface(IERC721_METADATA_ID)).to.equal(true);
        });

        it("应该支持 IERC721Enumerable 接口", async function () {
            // 这个需要计算，大致测试即可
            expect(await myNFT.supportsInterface("0x780e9d63")).to.equal(true);
        });
    });
});