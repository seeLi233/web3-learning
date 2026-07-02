import {expect} from "chai";
import {network} from "hardhat";

const {ethers} = await network.create();

/**
 * Extract the tokenAddress from the TokenCreated event in a receipt.
 * The receipt may contain logs from deployed child contracts (e.g. DeFiToken
 * constructor events), so we search for the TokenCreated log specifically.
 */
function getTokenAddressFromReceipt(receipt: any): string {
    // ethers v6 decoded logs have a `.fragment` property with the event name
    for (const log of receipt.logs) {
        if (log.fragment?.name === "TokenCreated") {
            return log.args.tokenAddress;
        }
    }
    throw new Error("TokenCreated event not found in receipt");
}

describe("TokenFactory", function () {
    let factory: any;
    let owner: any, alice: any, bob: any;

    beforeEach(async function () {
        [owner, alice, bob] = await ethers.getSigners();
        factory = await ethers.deployContract("TokenFactory");
    });

    describe("Deployment", function () {
        it("should deploy successfully", async function () {
            expect(await factory.getTotalTokens()).to.equal(0n);
        });

        it("should set owner correctly", async function () {
            expect(await factory.owner()).to.equal(owner.address);
        });

        it("should have default creation fee of 0.01 ETH", async function () {
            const fee = await factory.createFee();
            expect(fee).to.equal(ethers.parseEther("0.01"));
        });
    });

    describe("createERC20", function () {
        it("should create ERC20 token successfully", async function () {
            const tx = await factory.connect(alice).createERC20(
                "MyToken", "MTK", 1000n,
                {value: ethers.parseEther("0.01")}
            );

            const receipt = await tx.wait();

            // 验证事件已触发
            await expect(tx).to.emit(factory, "TokenCreated");

            // 验证计数器
            expect(await factory.getTotalTokens()).to.equal(1n);

            // 从 receipt 中提取 TokenCreated 事件
            const tokenAddr = getTokenAddressFromReceipt(receipt);
            expect(tokenAddr).to.match(/^0x[a-fA-F0-9]{40}$/);

            // 验证代币信息
            const info = await factory.getTokenInfo(tokenAddr);
            expect(info.tokenType).to.equal(0n);  // ERC20
            expect(info.creator).to.equal(alice.address);
            expect(info.name).to.equal("MyToken");
            expect(info.symbol).to.equal("MTK");
        });

        it("should reject if creation fee is insufficient", async function () {
            await expect(
                factory.connect(alice).createERC20(
                    "Test", "TST", 1000n,
                    {value: ethers.parseEther("0.001")}
                )
            ).to.be.revertedWithCustomError(factory, "InsufficientCreationFee");
        });

        it("should reject if name is empty", async function () {
            await expect(
                factory.connect(alice).createERC20(
                    "", "TST", 1000n,
                    {value: ethers.parseEther("0.01")}
                )
            ).to.be.revertedWithCustomError(factory, "EmptyName");
        });

        it("should reject if initial supply is zero", async function () {
            await expect(
                factory.connect(alice).createERC20(
                    "Test", "TST", 0n,
                    {value: ethers.parseEther("0.01")}
                )
            ).to.be.revertedWithCustomError(factory, "ZeroSupply");
        });

        it("should mint initial supply to creator", async function () {
            const tx = await factory.connect(alice).createERC20(
                "MyToken", "MTK", 1000n,
                {value: ethers.parseEther("0.01")}
            );
            const receipt = await tx.wait();
            const tokenAddr = getTokenAddressFromReceipt(receipt);

            const token = await ethers.getContractAt("DeFiToken", tokenAddr);
            const expectedBalance = 1000n * (10n ** 18n);
            expect(await token.balanceOf(alice.address)).to.equal(expectedBalance);
        });
    });

    describe("createERC721", function () {
        it("should create ERC721 collection successfully", async function () {
            const tx = await factory.connect(alice).createERC721(
                "MyNFT", "NFT", "https://api.example.com/nft/",
                {value: ethers.parseEther("0.01")}
            );

            const receipt = await tx.wait();
            const nftAddr = getTokenAddressFromReceipt(receipt);

            const info = await factory.getTokenInfo(nftAddr);
            expect(info.tokenType).to.equal(1n);  // ERC721
            expect(info.creator).to.equal(alice.address);
            expect(info.name).to.equal("MyNFT");
        });
    });

    describe("createERC1155", function () {
        it("should create ERC1155 collection successfully", async function () {
            const tx = await factory.connect(alice).createERC1155(
                "GameItems",
                "https://api.example.com/items/{id}.json",
                {value: ethers.parseEther("0.01")}
            );

            const receipt = await tx.wait();
            const multiAddr = getTokenAddressFromReceipt(receipt);

            const info = await factory.getTokenInfo(multiAddr);
            expect(info.tokenType).to.equal(2n);  // ERC1155
            expect(info.name).to.equal("GameItems");
        });
    });

    describe("Query functions", function () {
        it("should return user tokens", async function () {
            // 创建 3 个代币（ERC20 必须传入 >0 的初始供应量）
            await factory.connect(alice).createERC20(
                "A", "A", 1n, {value: ethers.parseEther("0.01")}
            );
            await factory.connect(alice).createERC721(
                "B", "B", "ipfs://base/", {value: ethers.parseEther("0.01")}
            );
            await factory.connect(bob).createERC20(
                "C", "C", 1n, {value: ethers.parseEther("0.01")}
            );

            const userTokens = await factory.getUserTokens(alice.address);
            expect(userTokens.length).to.equal(2);

            const user2Tokens = await factory.getUserTokens(bob.address);
            expect(user2Tokens.length).to.equal(1);
        });

        it("should filter by type", async function () {
            await factory.connect(alice).createERC20(
                "A", "A", 1n, {value: ethers.parseEther("0.01")}
            );
            await factory.connect(alice).createERC20(
                "B", "B", 1n, {value: ethers.parseEther("0.01")}
            );
            await factory.connect(alice).createERC721(
                "C", "C", "ipfs://base/", {value: ethers.parseEther("0.01")}
            );

            const erc20Tokens = await factory.getTokensByType(0n, 0n, 10n);
            expect(erc20Tokens.length).to.equal(2);

            const erc721Tokens = await factory.getTokensByType(1n, 0n, 10n);
            expect(erc721Tokens.length).to.equal(1);
        });

        it("should support pagination", async function () {
            for (let i = 0; i < 5; i++) {
                await factory.connect(alice).createERC20(
                    `Token${i}`, `T${i}`, 1n,
                    {value: ethers.parseEther("0.01")}
                );
            }

            const page1 = await factory.getAllTokens(0n, 2n);
            expect(page1.length).to.equal(2);

            const page2 = await factory.getAllTokens(2n, 2n);
            expect(page2.length).to.equal(2);

            const page3 = await factory.getAllTokens(4n, 2n);
            expect(page3.length).to.equal(1);
        });
    });

    describe("Admin functions", function () {
        it("should allow owner to set creation fee", async function () {
            await factory.setCreationFee(ethers.parseEther("0.05"));
            expect(await factory.createFee()).to.equal(ethers.parseEther("0.05"));
        });

        it("should reject non-owner setting fee", async function () {
            await expect(
                factory.connect(alice).setCreationFee(ethers.parseEther("0.05"))
            ).to.be.revertedWithCustomError(factory, "OwnableUnauthorizedAccount");
        });

        it("should allow owner to withdraw fees", async function () {
            await factory.connect(alice).createERC20(
                "Test", "TST", 1n,
                {value: ethers.parseEther("0.01")}
            );

            const ownerBalanceBefore = await ethers.provider.getBalance(owner.address);

            await factory.withdrawFees();

            const ownerBalanceAfter = await ethers.provider.getBalance(owner.address);

            expect(ownerBalanceAfter).to.be.gt(ownerBalanceBefore);
        });
    });
});
