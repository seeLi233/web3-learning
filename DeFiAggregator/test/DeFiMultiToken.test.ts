import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

describe("DeFiMultiToken", function() {
    let token: any;
    let owner: any;
    let alice: any;
    let bob: any;
    let ownerAddress: any;
    let aliceAddress: any;
    let bobAddress: any;

    const ZERO_ADDRESS = ethers.ZeroAddress;
    const BASE_URI = "https://api.example.com/tokens/{id}.json";

    beforeEach(async function () {
        [ownerAddress, aliceAddress, bobAddress] = await ethers.getSigners()

        token = await ethers.deployContract("DeFiMultiToken", ["DeFi Multi Token", BASE_URI]);
    });

    // ========== 1. 部署测试 ==========

    describe("Deployment", function () {
        it("应该正确设置名称", async function () {
            expect(await token.name()).to.equal("DeFi Multi Token");
        });

        it("应该正确设置 baseURI", async function () {
            const tx = await token.createToken(1000, "");
            const receipt = await tx.wait();
            // 从 TokenCreated 事件中解析出 id
            // TokenCreated 事件的第一个 indexed 参数就是 id
            const id = 0; // 第一个 token id 一定是 0
            await token.mint(ownerAddress, id, 1, "0x");
            expect(await token.uri(id)).to.equal(BASE_URI);
        });

        it("应该设置 owner 为部署者", async function () {
            expect(await token.owner()).to.equal(ownerAddress);
        });

        it("初始 tokenCount 应该为 0", async function () {
            expect(await token.tokenCount()).to.equal(0);
        });
    });

    // ========== 2. 创建代币类型 ==========

    describe("Create Token", function () {
        it("owner 可以创建新的代币类型", async function () {
            const tx = await token.createToken(1000, "");
            const receipt = await tx.wait();

            // 检查事件
            await expect(tx)
                .to.emit(token, "TokenCreated")
                .withArgs(0, "", 1000, ownerAddress);

            expect(await token.tokenCount()).to.equal(1);
        });

        it("创建后 id 自增", async function () {
            await token.createToken(100, "");
            await token.createToken(500, "");
            expect(await token.tokenCount()).to.equal(2);
        });

        it("可以创建无限制供应的代币（maxSupply=0）", async function () {
            await token.createToken(0, "");
            const info = await token.getTokenInfo(0);
            expect(info.maxSupply_).to.equal(0);
        });
    });

    // ========== 3. 铸造测试 ==========

    describe("Minting", function () {
        let tokenId: bigint;

        beforeEach(async function () {
            const tx = await token.createToken(1000, "");
            const receipt = await tx.wait();
        });

        it("owner 可以铸造代币", async function () {
            await token.mint(aliceAddress, 0, 100, "0x");
            expect(await token.balanceOf(aliceAddress, 0)).to.equal(100);
        });

        it("非 owner 不能铸造", async function () {
            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.mint(aliceAddress, 0, 100, "0x")
            ).to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount");
        });

        it("不能超过最大供应量", async function () {
            const tx = await token.createToken(500, "");
            const receipt = await tx.wait();
            const newId = 1;

            await token.mint(aliceAddress, newId, 300, "0x");

            await expect(
                token.mint(aliceAddress, newId, 300, "0x")  // 300 + 300 = 600 > 500
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__MaxSupplyReached");
        });

        it("不能铸造到零地址", async function () {
            await expect(
                token.mint(ZERO_ADDRESS, 0, 100, "0x")
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__ZeroAddress");
        });

        it("零数量铸造应该失败", async function () {
            await expect(
                token.mint(aliceAddress, 0, 0, "0x")
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__ZeroAmount");
        });

        it("可以铸造 NFT（supply=1）", async function () {
            const tx = await token.mintNFT(aliceAddress, "ipfs://QmNFT/1.json");
            const receipt = await tx.wait();

            const newId = await token.tokenCount();
            // NFT 的 id 是自动创建的，供应量应该是 1
            expect(await token.balanceOf(aliceAddress, newId - 1n)).to.equal(1);
        });
    });

    // ========== 4. 批量铸造 ==========

    describe("Batch Minting", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
            await token.createToken(500, "");
            await token.createToken(100, "");
        });

        it("可以批量铸造多个 id", async function () {
            await token.mintBatch(
                aliceAddress,
                [0, 1, 2],
                [100, 50, 10],
                "0x"
            );

            expect(await token.balanceOf(aliceAddress, 0)).to.equal(100);
            expect(await token.balanceOf(aliceAddress, 1)).to.equal(50);
            expect(await token.balanceOf(aliceAddress, 2)).to.equal(10);
        });

        it("数组长度不匹配应该失败", async function () {
            await expect(
                token.mintBatch(aliceAddress, [0, 1], [100], "0x")
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__ArrayLengthMismatch");
        });
    });

    // ========== 5. 安全转账 ==========

    describe("Safe Transfer", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
            await token.mint(aliceAddress, 0, 200, "0x");
        });

        it("持有者可以转账", async function () {
            const aliceToken = token.connect(aliceAddress);

            await aliceToken.safeTransferFrom(
                aliceAddress, bobAddress, 0, 50, "0x"
            );

            expect(await token.balanceOf(aliceAddress, 0)).to.equal(150);
            expect(await token.balanceOf(bobAddress, 0)).to.equal(50);
        });

        it("未授权不能转账他人的代币", async function () {
            await expect(
                token.safeTransferFrom(aliceAddress, bobAddress, 0, 50, "0x")
            ).to.be.revertedWithCustomError(token, "ERC1155MissingApprovalForAll");
        });

        it("授权后可以转账他人的代币", async function () {
            const aliceToken = token.connect(aliceAddress);

            // Alice 授权 owner 操作她的所有代币
            await aliceToken.setApprovalForAll(ownerAddress, true);

            await token.safeTransferFrom(aliceAddress, bobAddress, 0, 50, "0x");

            expect(await token.balanceOf(bobAddress, 0)).to.equal(50);
        });

        it("不能转账到零地址", async function () {
            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.safeTransferFrom(
                    aliceAddress, ZERO_ADDRESS, 0, 50, "0x"
                )
            ).to.be.revertedWithCustomError(token, "ERC1155InvalidReceiver");
        });

        it("余额不足不能转账", async function () {
            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.safeTransferFrom(
                    aliceAddress, bobAddress, 0, 300, "0x"  // 只有 200
                )
            ).to.be.revertedWithCustomError(token, "ERC1155InsufficientBalance");
        });
    });

    // ========== 6. 批量安全转账 ==========

    describe("Safe Batch Transfer", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
            await token.createToken(500, "");
            await token.createToken(100, "");
            await token.mintBatch(aliceAddress, [0, 1, 2], [100, 50, 25], "0x");
        });

        it("可以批量转账多个 id", async function () {
            const aliceToken = token.connect(aliceAddress);

            await aliceToken.safeBatchTransferFrom(
                aliceAddress,
                bobAddress,
                [0, 1, 2],
                [30, 20, 10],
                "0x"
            );

            expect(await token.balanceOf(bobAddress, 0)).to.equal(30);
            expect(await token.balanceOf(bobAddress, 1)).to.equal(20);
            expect(await token.balanceOf(bobAddress, 2)).to.equal(10);

            expect(await token.balanceOf(aliceAddress, 0)).to.equal(70);
            expect(await token.balanceOf(aliceAddress, 1)).to.equal(30);
            expect(await token.balanceOf(aliceAddress, 2)).to.equal(15);
        });
    });

    // ========== 7. 燃烧测试 ==========

    describe("Burning", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
            await token.mint(aliceAddress, 0, 200, "0x");
        });

        it("持有者可以燃烧自己的代币", async function () {
            const aliceToken = token.connect(aliceAddress);

            await aliceToken.burn(aliceAddress, 0, 50);
            expect(await token.balanceOf(aliceAddress, 0)).to.equal(150);
        });

        it("不能燃烧超出余额的代币", async function () {
            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.burn(aliceAddress, 0, 300)
            ).to.be.revertedWithCustomError(token, "ERC1155InsufficientBalance");
        });
    });

    // ========== 8. 批量查询 ==========

    describe("Balance Of Batch", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
            await token.createToken(500, "");
            await token.createToken(100, "");
            await token.mintBatch(aliceAddress, [0, 1, 2], [100, 50, 25], "0x");
        });

        it("可以批量查询多个余额", async function () {
            const balances = await token.balanceOfBatch(
                [aliceAddress, aliceAddress, aliceAddress],
                [0, 1, 2]
            );
            expect(balances[0]).to.equal(100);
            expect(balances[1]).to.equal(50);
            expect(balances[2]).to.equal(25);
        });

        it("getBalances 辅助函数也能批量查询", async function () {
            const balances = await token.getBalances(
                aliceAddress,
                [0, 1, 2]
            );
            expect(balances[0]).to.equal(100);
            expect(balances[1]).to.equal(50);
            expect(balances[2]).to.equal(25);
        });
    });

    // ========== 9. 供应量管理 ==========

    describe("Supply Management", function () {
        beforeEach(async function () {
            await token.createToken(1000, "");
        });

        it("可以更新最大供应量", async function () {
            await token.setMaxSupply(0, 500);
            const info = await token.getTokenInfo(0);
            expect(info.maxSupply_).to.equal(500);
        });

        it("新供应量不能小于当前供应量", async function () {
            await token.mint(aliceAddress, 0, 100, "0x");

            await expect(
                token.setMaxSupply(0, 50)  // 当前 100，不能设成 50
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__InvalidMaxSupply");
        });

        it("可以锁定供应量", async function () {
            await token.lockSupply(0);
            const info = await token.getTokenInfo(0);
            expect(info.isSupplyLocked_).to.equal(true);

            // 锁定后不能铸造
            await expect(
                token.mint(aliceAddress, 0, 1, "0x")
            ).to.be.revertedWithCustomError(token, "DeFiMultiToken__SupplyLocked");
        });
    });

    // ========== 10. URI 管理 ==========

    describe("URI Management", function () {
        it("可以更新 baseURI", async function () {
            const newURI = "https://api.example.com/v2/tokens/{id}.json";
            await token.setBaseURI(newURI);

            await token.createToken(100, "");
            await token.mint(ownerAddress, 0, 1, "0x");
            expect(await token.uri(0)).to.equal(newURI);
        });

        it("可以为特定 id 设置独立 URI", async function () {
            await token.createToken(1000, "");
            const customURI = "ipfs://QmCustom/metadata.json";
            await token.setTokenURI(0, customURI);
            // setTokenURI emit URI event
        });
    });

    // ========== 11. 事件测试 ==========

    describe("Events", function () {
        it("safeTransferFrom 应该 emit TransferSingle", async function () {
            await token.createToken(1000, "");
            await token.mint(aliceAddress, 0, 100, "0x");

            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.safeTransferFrom(aliceAddress, bobAddress, 0, 50, "0x")
            )
                .to.emit(token, "TransferSingle")
                .withArgs(aliceAddress, aliceAddress, bobAddress, 0, 50);
        });

        it("safeBatchTransferFrom 应该 emit TransferBatch", async function () {
            await token.createToken(1000, "");
            await token.createToken(500, "");
            await token.mintBatch(aliceAddress, [0, 1], [100, 50], "0x");

            const aliceToken = token.connect(aliceAddress);

            await expect(
                aliceToken.safeBatchTransferFrom(
                    aliceAddress, bobAddress, [0, 1], [30, 20], "0x"
                )
            ).to.emit(token, "TransferBatch");
        });

        it("setApprovalForAll 应该 emit ApprovalForAll", async function () {
            await expect(token.setApprovalForAll(aliceAddress, true))
                .to.emit(token, "ApprovalForAll")
                .withArgs(ownerAddress, aliceAddress, true);
        });
    });
});