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

    // ==================== 高级事件分析测试（Day 13 新增）========================
    describe("Advanced Event Analysis", function() {
        it("TokenCreated 事件的 tokenType 应对应正确的枚举值", async function() {
            const txERC20 = await factory.connect(alice).createERC20(
                "Test20", "T20", 1n, {value: ethers.parseEther("0.01")}
            );
            const receipt20 = await txERC20.wait();

            const txERC721 = await factory.connect(alice).createERC721(
                "Test721", "T721", "ipfs://base/", {value: ethers.parseEther("0.01")}
            );
            const receipt721 = await txERC721.wait();

            const txERC1155 = await factory.connect(alice).createERC1155(
                "Test1155", "https://api.example.com/{id}.json",
                {value: ethers.parseEther("0.01")}
            );
            const receipt1155 = await txERC1155.wait();

            // 提取每个 receipt 中的 TokenCreated 事件
            function findTokenCreatedLog(receipt: any): any {
                for (const log of receipt.logs) {
                    if (log.fragment?.name === "TokenCreated") {
                        return log;
                    }
                }
                throw new Error("TokenCreated event not found");
            }

            const log20 = findTokenCreatedLog(receipt20);
            const log721 = findTokenCreatedLog(receipt721);
            const log1155 = findTokenCreatedLog(receipt1155);

            // 验证 tokenType
            expect(log20.args.tokenType).to.equal(0n);    // ERC20 = 0
            expect(log721.args.tokenType).to.equal(1n);   // ERC721 = 1
            expect(log1155.args.tokenType).to.equal(2n);  // ERC1155 = 2

            // 验证 creator 字段
            expect(log20.args.creator).to.equal(alice.address);
            expect(log721.args.creator).to.equal(alice.address);
            expect(log1155.args.creator).to.equal(alice.address);

            // 验证 tokenAddress 不为零地址
            expect(log20.args.tokenAddress).to.not.equal(ethers.ZeroAddress);
            expect(log721.args.tokenAddress).to.not.equal(ethers.ZeroAddress);
            expect(log1155.args.tokenAddress).to.not.equal(ethers.ZeroAddress);
        });

        it("每次创建 token，TokenCreated 事件中的 tokenAddress 应该唯一", async function() {
            const addresses = new Set<string>();

            for (let i = 0; i < 3; i++) {
                const tx = await factory.connect(alice).createERC20(
                    `Token${i}`, `T${i}`, 1n, {value: ethers.parseEther("0.01")}
                );
                const receipt = await tx.wait();
                const addr = getTokenAddressFromReceipt(receipt);

                // 验证地址唯一
                expect(addresses.has(addr)).to.equal(false, `Duplicate address: ${addr}`);
                addresses.add(addr);
            }

            expect(addresses.size).to.equal(3);
        });
    });

    describe("Gas Comparison: CREATE3 token types", function() {
        it("对比 ERC20 vs ERC721 vs ERC1155 的创建 Gas 成本", async function() {
            const results: Array<{ type: string; gasUsed: bigint; address: string }> = [];

            // ERC20
            const tx20 = await factory.connect(alice).createERC20(
                "G20", "G20", 1n, {value: ethers.parseEther("0.01")}
            );
            const r20 = await tx20.wait();
            results.push({
                type: "ERC20",
                gasUsed: r20.gasUsed,
                address: getTokenAddressFromReceipt(r20)
            });

            // ERC721
            const tx721 = await factory.connect(alice).createERC721(
                "G721", "G721", "ipfs://base/", {value: ethers.parseEther("0.01")}
            );
            const r721 = await tx721.wait();
            results.push({
                type: "ERC721",
                gasUsed: r721.gasUsed,
                address: getTokenAddressFromReceipt(r721)
            });

            // ERC1155
            const tx1155 = await factory.connect(alice).createERC1155(
                "G1155", "https://api.example.com/{id}.json",
                {value: ethers.parseEther("0.01")}
            );
            const r1155 = await tx1155.wait();
            results.push({
                type: "ERC1155",
                gasUsed: r1155.gasUsed,
                address: getTokenAddressFromReceipt(r1155)
            });

            console.log("\n📊 ===== TokenFactory 创建 Gas 对比 =====");
            for (const r of results) {
                console.log(`  ${r.type.padEnd(10)} gas: ${String(r.gasUsed).padStart(8)}  address: ${r.address}`);
            }
            console.log("==========================================\n");

            // 实际排序：ERC721 最便宜（无 mint），ERC20 次之，ERC1155 最贵（supply 追踪逻辑最复杂）
            expect(r721.gasUsed).to.be.lt(r20.gasUsed);
            expect(r20.gasUsed).to.be.lt(r1155.gasUsed);
        });
    });

    describe("Factory Fee Boundary Tests", function() {
        it("刚好 0 ETH 费用应该 revert", async function() {
            // 尝试不付任何费用
            await expect(
                factory.connect(alice).createERC20(
                    "Free", "FRE", 1n,
                    {value: 0n}
                )
            ).to.be.revertedWithCustomError(factory, "InsufficientCreationFee");
        });

        it("多付的费用应该不退（合约只收所需费用）", async function() {
            // 多付了，但合约只检查 >= fee
            const overpay = ethers.parseEther("0.05");  // fee 是 0.01
            const tx = await factory.connect(alice).createERC20(
                "Overpay", "OVR", 1n,
                {value: overpay}
            );
            const receipt = await tx.wait();

            // 代币创建成功
            const addr = getTokenAddressFromReceipt(receipt);
            expect(addr).to.match(/^0x[a-fA-F0-9]{40}$/);

            // ⚠️ 面试点：合约不会自动退款，多付的 ETH 存在合约里
            // 用户如果需要精确支付，应该在链下计算好
        });

        it("设置创建费为 0 后，任何人都可以免费创建", async function() {
            await factory.setCreationFee(0n);
            expect(await factory.createFee()).to.equal(0n);

            const tx = await factory.connect(alice).createERC20(
                "FreeToken", "FREE", 1n,
                {value: 0n}
            );
            const receipt = await tx.wait();

            const addr = getTokenAddressFromReceipt(receipt);
            expect(addr).to.match(/^0x[a-fA-F0-9]{40}$/);
        });
    });
});
