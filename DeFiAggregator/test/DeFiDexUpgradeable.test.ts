import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ==================== 测试常量 ====================
const INITIAL_FEE = 30n;   // 0.3%

describe("🏦 DeFiDexUpgradeable — 可升级版 DEX", function () {
    // ==================== 变量声明 ====================
    let owner: any, user1: any, user2: any;
    let mockToken: any;
    let dexLogic: any;        // DeFiDexUpgradeable 逻辑合约
    let proxy: any;           // ERC1967Proxy
    let dex: any;             // 通过代理访问的 DeFiDexUpgradeable

    // ==================== A. 部署 ====================
    describe("A. 部署", function () {
        it("A1. 部署 MockToken 用于测试", async function () {
            [owner, user1, user2] = await ethers.getSigners();

            // MockToken 构造函数: name, symbol, decimals
            const MockTokenFactory = await ethers.getContractFactory("MockToken");
            mockToken = await MockTokenFactory.deploy("Test Token", "TST", 18);
            await mockToken.waitForDeployment();

            // 给 user1 和 user2 发代币
            const mintAmount = ethers.parseEther("10000");
            await mockToken.mint(user1.address, mintAmount);
            await mockToken.mint(user2.address, mintAmount);
        });

        it("A2. 部署 DeFiDexUpgradeable 逻辑合约", async function () {
            dexLogic = await ethers.deployContract("DeFiDexUpgradeable");

            // 验证 _disableInitializers 生效
            await expect(
                dexLogic.initialize(INITIAL_FEE)
            ).to.be.revertedWithCustomError(dexLogic, "InvalidInitialization");
        });

        it("A3. 部署 ERC1967 代理并初始化", async function () {
            const initData = dexLogic.interface.encodeFunctionData("initialize", [
                INITIAL_FEE,
            ]);

            // 使用 UUPSProxy 作为代理（封装了 ERC1967Proxy）
            const UUPSProxyFactory = await ethers.getContractFactory("UUPSProxy");
            proxy = await UUPSProxyFactory.deploy(
                await dexLogic.getAddress(),
                initData
            );
            await proxy.waitForDeployment();

            // 用 DeFiDexUpgradeable 的 ABI 连接代理
            dex = await ethers.getContractAt(
                "DeFiDexUpgradeable",
                await proxy.getAddress()
            );
        });

        it("A4. 初始化后状态正确", async function () {
            expect(await dex.feeRate()).to.equal(INITIAL_FEE);
            expect(await dex.owner()).to.equal(owner.address);
            expect(await dex.totalFeeCollected()).to.equal(0n);
        });
    });

    // ==================== B. 流动性管理 ====================
    describe("B. 流动性管理", function () {
        it("B1. Owner 添加代币到白名单", async function () {
            const tokenAddr = await mockToken.getAddress();
            await dex.connect(owner).updateWhitelist(tokenAddr, true);

            expect(await dex.whitelistedTokens(tokenAddr)).to.be.true;
        });

        it("B2. User 添加流动性", async function () {
            const tokenAddr = await mockToken.getAddress();
            const amount = ethers.parseEther("100");

            // 授权
            await mockToken.connect(user1).approve(await dex.getAddress(), amount);

            // 添加流动性
            await dex.connect(user1).addLiquidity(tokenAddr, amount);

            expect(await dex.getLiquidity(user1.address, tokenAddr)).to.equal(
                amount
            );
        });

        it("B3. 未白名单代币 → revert", async function () {
            const randomToken = "0x1111111111111111111111111111111111111111";
            await expect(
                dex.connect(user1).addLiquidity(randomToken, 100n)
            ).to.be.revertedWith("Token not whitelisted");
        });
    });

    // ==================== C. 代币交换 ====================
    describe("C. 代币交换", function () {
        let mockToken2: any;

        it("C1. 准备第二个代币并加入白名单", async function () {
            const MockTokenFactory = await ethers.getContractFactory("MockToken");
            mockToken2 = await MockTokenFactory.deploy("Token 2", "TK2", 18);
            await mockToken2.waitForDeployment();

            const tokenAddr = await mockToken2.getAddress();
            await dex.connect(owner).updateWhitelist(tokenAddr, true);

            // 给合约转一些 Token2 作为流动性（模拟）
            await mockToken2.mint(await dex.getAddress(), ethers.parseEther("5000"));
        });

        it("C2. Swap 应该成功", async function () {
            const tokenInAddr = await mockToken.getAddress();
            const tokenOutAddr = await mockToken2.getAddress();
            const amountIn = ethers.parseEther("10");

            await mockToken.connect(user1).approve(await dex.getAddress(), amountIn);

            const tx = await dex
                .connect(user1)
                .swap(tokenInAddr, tokenOutAddr, amountIn);

            // amountOut = amountIn - fee (30/10000 * amountIn)
            const expectedFee = (amountIn * INITIAL_FEE) / 10000n;
            const expectedOut = amountIn - expectedFee;

            await expect(tx)
                .to.emit(dex, "Swapped")
                .withArgs(user1.address, tokenInAddr, tokenOutAddr, amountIn, expectedOut);
        });

        it("C3. 暂停后 swap → revert", async function () {
            await dex.connect(owner).pause();

            const tokenInAddr = await mockToken.getAddress();
            const tokenOutAddr = await mockToken2.getAddress();

            await expect(
                dex.connect(user1).swap(tokenInAddr, tokenOutAddr, 100n)
            ).to.be.revertedWithCustomError(dex, "EnforcedPause");
        });

        it("C4. 恢复暂停 → swap 成功", async function () {
            await dex.connect(owner).unpause();

            const tokenInAddr = await mockToken.getAddress();
            const tokenOutAddr = await mockToken2.getAddress();
            const amountIn = ethers.parseEther("5");

            await mockToken.connect(user1).approve(await dex.getAddress(), amountIn);
            await dex.connect(user1).swap(tokenInAddr, tokenOutAddr, amountIn);

            // 不 revert 就是成功
        });
    });

    // ==================== D. 🔥 升级流程 ====================
    describe("D. 🔥 升级到 V2（模拟新增功能）", function () {
        let dexV2Logic: any;
        let valueBeforeUpgrade: any;

        it("D1. 记录升级前的状态", async function () {
            valueBeforeUpgrade = {
                feeRate: await dex.feeRate(),
                totalFee: await dex.totalFeeCollected(),
                owner: await dex.owner(),
            };
        });

        it("D2. 部署 V2 逻辑合约（这里用同一个合约模拟升级）", async function () {
            // ⚠️ 实际开发中，V2 是新的合约文件
            // 这里用重新部署同一个合约来模拟
            dexV2Logic = await ethers.deployContract("DeFiDexUpgradeable");
        });

        it("D3. Owner 执行升级", async function () {
            const tx = await dex
                .connect(owner)
                .upgradeToAndCall(await dexV2Logic.getAddress(), "0x");

            await expect(tx)
                .to.emit(dex, "Upgraded")
                .withArgs(await dexV2Logic.getAddress());
        });

        it("D4. 🔥 升级后存储持久性验证", async function () {
            // 重新连接代理
            dex = await ethers.getContractAt(
                "DeFiDexUpgradeable",
                await proxy.getAddress()
            );

            // ⭐ 核心验证：升级后存储不丢失
            expect(await dex.feeRate()).to.equal(valueBeforeUpgrade.feeRate);
            expect(await dex.totalFeeCollected()).to.equal(valueBeforeUpgrade.totalFee);
            expect(await dex.owner()).to.equal(valueBeforeUpgrade.owner);
        });

        it("D5. 🔥 升级后功能仍然正常", async function () {
            // 验证升级后可以正常使用
            const tokenAddr = await mockToken.getAddress();
            expect(await dex.whitelistedTokens(tokenAddr)).to.be.true;

            const user1Liquidity = await dex.getLiquidity(
                user1.address,
                tokenAddr
            );
            expect(user1Liquidity).to.be.gt(0n);
        });
    });

    // ==================== E. 🔥 面试重点 ====================
    describe("E. 🔥 面试重点 — 安全与架构", function () {
        it("E1. 非 owner 不能升级", async function () {
            await expect(
                dex.connect(user1).upgradeToAndCall(ethers.ZeroAddress, "0x")
            ).to.be.revertedWithCustomError(dex, "OwnableUnauthorizedAccount");
        });

        it("E2. 逻辑合约本身不能初始化", async function () {
            await expect(
                dexLogic.initialize(50n)
            ).to.be.revertedWithCustomError(dexLogic, "InvalidInitialization");
        });

        it("E3. 🔥 为什么可升级架构对 DEX 很重要？", async function () {
            console.log("面试回答要点：");
            console.log("1. DEX 费率需要调整 → 升级 feeRate 逻辑");
            console.log("2. 新增代币对路由算法 → 不需要迁移流动性");
            console.log("3. 修复安全漏洞 → 快速响应");
            console.log("4. 所有用户的流动性和交易历史保持不变");
        });
    });
});
