import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// ==================== BridgeToken.test.ts ====================
// 验证跨链桥核心流程：
//   A. BridgeToken 基础功能
//   B. SourceBridge 锁定流程
//   C. DestinationBridge 铸造流程
//   D. 赎回流程（目标链销毁 → 源链解锁）
//   E. 安全边界（防重放 / 权限控制）
//   F. 🔥 面试重点：Lock-Mint 模式的安全分析

describe("🌉 CrossChainBridge — 跨链桥 Lock-Mint 模式", function () {
    // ==================== 变量声明 ====================
    let token: any;           // 源链 BridgeToken
    let wrappedToken: any;    // 目标链 BridgeToken（模拟另一条链）
    let sourceBridge: any;
    let destinationBridge: any;

    let owner: any, user1: any, user2: any, relayer: any;

    const INITIAL_SUPPLY = ethers.parseEther("1000000");
    const ONE_HUNDRED = ethers.parseEther("100");

    // ==================== beforeEach ====================
    beforeEach(async function () {
        [owner, user1, user2, relayer] = await ethers.getSigners();

        // 部署源链代币（初始 supply 给 owner）
        token = await ethers.deployContract("BridgeToken", [
            "Bridge Token",
            "BRG",
            owner.address,
        ]);
        await token.waitForDeployment();

        // 部署目标链代币（初始 supply 也给 owner，但目标链上为 0）
        wrappedToken = await ethers.deployContract("BridgeToken", [
            "Wrapped Bridge Token",
            "wBRG",
            owner.address,
        ]);
        await wrappedToken.waitForDeployment();

        // 部署源链桥
        sourceBridge = await ethers.deployContract("SourceBridge", [
            await token.getAddress(),
            owner.address,
        ]);
        await sourceBridge.waitForDeployment();

        // 部署目标链桥
        destinationBridge = await ethers.deployContract("DestinationBridge", [
            await wrappedToken.getAddress(),
            owner.address,
        ]);
        await destinationBridge.waitForDeployment();

        // 把目标链桥加入 wrappedToken 的授权桥列表
        await wrappedToken.addBridge(await destinationBridge.getAddress());

        // 给 user1 转一些 token 用于测试
        await token.transfer(user1.address, ONE_HUNDRED);
    });

    // ==================== A. BridgeToken 基础功能 ====================
    describe("A. BridgeToken 基础功能", function () {
        it("A1. 初始供应量应该给 owner", async function () {
            const balance = await token.balanceOf(owner.address);
            // owner 有 1000000 减掉转给 user1 的 100
            expect(balance).to.equal(INITIAL_SUPPLY - ONE_HUNDRED);
        });

        it("A2. 只有桥合约能调用 bridgeMint", async function () {
            await expect(
                token.bridgeMint(user1.address, 1000n)
            ).to.be.revertedWith("BridgeToken: caller is not a bridge");
        });

        it("A3. 只有 owner 能添加/移除桥合约", async function () {
            await expect(
                token.connect(user1).addBridge(user2.address)
            ).to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount");
        });

        it("A4. 添加桥合约后，桥合约可以 mint", async function () {
            // owner 把 sourceBridge 加入桥列表
            await token.addBridge(await sourceBridge.getAddress());

            // 由于 sourceBridge 的 lock() 需要 transferFrom，
            // 这里直接验证 bridgeMint 权限已开放
            const isBridge = await token.bridges(await sourceBridge.getAddress());
            expect(isBridge).to.be.true;
        });
    });

    // ==================== B. SourceBridge 锁定流程 ====================
    describe("B. SourceBridge 锁定流程", function () {
        it("B1. user1 锁定 50 BRG → 桥合约余额应该增加", async function () {
            const amount = ethers.parseEther("50");

            // user1 approve 桥合约
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount
            );

            // user1 锁定
            const tx = await sourceBridge.connect(user1).lock(
                amount,
                user1.address // 目标链也是自己
            );

            // 桥合约余额
            const bridgeBal = await token.balanceOf(await sourceBridge.getAddress());
            expect(bridgeBal).to.equal(amount);

            // user1 余额减少
            const userBal = await token.balanceOf(user1.address);
            expect(userBal).to.equal(ONE_HUNDRED - amount);
        });

        it("B2. 锁定应该发出 TokenLocked 事件", async function () {
            const amount = ethers.parseEther("30");

            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount
            );

            await expect(
                sourceBridge.connect(user1).lock(amount, user1.address)
            ).to.emit(sourceBridge, "TokenLocked");
        });

        it("B3. 锁定金额为 0 → revert", async function () {
            await expect(
                sourceBridge.connect(user1).lock(0, user1.address)
            ).to.be.revertedWith("SourceBridge: amount is zero");
        });

        it("B4. 目标地址为零地址 → revert", async function () {
            const amount = ethers.parseEther("10");
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount
            );

            await expect(
                sourceBridge.connect(user1).lock(
                    amount,
                    "0x0000000000000000000000000000000000000000"
                )
            ).to.be.revertedWith("SourceBridge: zero recipient");
        });

        it("B5. 两次锁定应该生成不同的 txId", async function () {
            const amount = ethers.parseEther("10");
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount * 2n
            );

            const tx1 = await sourceBridge.connect(user1).lock(amount, user1.address);
            const tx2 = await sourceBridge.connect(user1).lock(amount, user1.address);

            const receipt1 = await tx1.wait();
            const receipt2 = await tx2.wait();

            // 从事件中提取 txId
            // logs[0] = ERC20 Transfer, logs[1] = TokenLocked
            const txId1 = receipt1.logs[1].topics[1];
            const txId2 = receipt2.logs[1].topics[1];

            expect(txId1).to.not.equal(txId2);
        });
    });

    // ==================== C. DestinationBridge 铸造流程 ====================
    describe("C. DestinationBridge 铸造流程", function () {
        let txId: string;
        const amount = ethers.parseEther("50");

        beforeEach(async function () {
            // 先在源链锁定
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount
            );
            const tx = await sourceBridge.connect(user1).lock(
                amount,
                user1.address
            );
            const receipt = await tx.wait();
            // logs[0] = ERC20 Transfer, logs[1] = TokenLocked
            txId = receipt.logs[1].topics[1];
        });

        it("C1. owner（中继者）铸造 → user1 在目标链收到 wBRG", async function () {
            const balBefore = await wrappedToken.balanceOf(user1.address);

            await destinationBridge.connect(owner).mint(txId, user1.address, amount);

            const balAfter = await wrappedToken.balanceOf(user1.address);
            expect(balAfter - balBefore).to.equal(amount);
        });

        it("C2. 非 owner 不能铸造", async function () {
            await expect(
                destinationBridge.connect(user1).mint(txId, user1.address, amount)
            ).to.be.revertedWithCustomError(
                destinationBridge,
                "OwnableUnauthorizedAccount"
            );
        });

        it("C3. 同一个 txId 不能铸造两次 → revert", async function () {
            await destinationBridge.connect(owner).mint(txId, user1.address, amount);

            await expect(
                destinationBridge.connect(owner).mint(txId, user1.address, amount)
            ).to.be.revertedWith("DestinationBridge: already minted");
        });

        it("C4. 铸造金额为 0 → revert", async function () {
            await expect(
                destinationBridge.connect(owner).mint(txId, user1.address, 0)
            ).to.be.revertedWith("DestinationBridge: amount is zero");
        });
    });

    // ==================== D. 赎回流程 ====================
    describe("D. 赎回流程 — 目标链销毁 → 源链解锁", function () {
        let lockTxId: string;
        let burnTxId: string;
        const amount = ethers.parseEther("40");

        beforeEach(async function () {
            // Step 1: 源链锁定
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount
            );
            const lockTx = await sourceBridge.connect(user1).lock(
                amount,
                user1.address
            );
            const lockReceipt = await lockTx.wait();
            // logs[0] = ERC20 Transfer, logs[1] = TokenLocked
            lockTxId = lockReceipt.logs[1].topics[1];

            // Step 2: 目标链铸造
            await destinationBridge.connect(owner).mint(
                lockTxId,
                user1.address,
                amount
            );

            // Step 3: user1 approve 目标链桥销毁 wBRG
            await wrappedToken.connect(user1).approve(
                await destinationBridge.getAddress(),
                amount
            );

            // Step 4: user1 在目标链发起赎回（销毁 wBRG）
            const burnTx = await destinationBridge.connect(user1).burn(
                amount,
                user1.address
            );
            const burnReceipt = await burnTx.wait();
            // logs[0] = ERC20 Transfer (bridgeBurn), logs[1] = TokenBurned
            burnTxId = burnReceipt.logs[1].topics[1];
        });

        it("D1. 赎回后，user1 的 wBRG 应该被销毁", async function () {
            const wrappedBal = await wrappedToken.balanceOf(user1.address);
            expect(wrappedBal).to.equal(0n);
        });

        it("D2. 源链 owner 执行 unlock → user1 在源链收到 BRG", async function () {
            const balBefore = await token.balanceOf(user1.address);

            await sourceBridge.connect(owner).unlock(burnTxId, user1.address, amount);

            const balAfter = await token.balanceOf(user1.address);
            expect(balAfter - balBefore).to.equal(amount);
        });

        it("D3. unlock 不能重放", async function () {
            await sourceBridge.connect(owner).unlock(burnTxId, user1.address, amount);

            await expect(
                sourceBridge.connect(owner).unlock(burnTxId, user1.address, amount)
            ).to.be.revertedWith("SourceBridge: already processed");
        });

        it("D4. 非 owner 不能 unlock", async function () {
            await expect(
                sourceBridge.connect(user1).unlock(burnTxId, user1.address, amount)
            ).to.be.revertedWithCustomError(
                sourceBridge,
                "OwnableUnauthorizedAccount"
            );
        });
    });

    // ==================== E. 安全边界 ====================
    describe("E. 安全边界测试", function () {
        it("E1. 锁定金额超过余额 → revert", async function () {
            const hugeAmount = ethers.parseEther("999999");
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                hugeAmount
            );

            await expect(
                sourceBridge.connect(user1).lock(hugeAmount, user1.address)
            ).to.be.revert(ethers); // ERC20 insufficient balance
        });

        it("E2. 没有 approve 就 lock → revert", async function () {
            const amount = ethers.parseEther("10");

            await expect(
                sourceBridge.connect(user2).lock(amount, user2.address)
            ).to.be.revert(ethers); // ERC20 insufficient allowance
        });

        it("E3. 没有加入桥列表的合约不能 bridgeMint", async function () {
            const fakeBridge = await ethers.deployContract("SourceBridge", [
                await token.getAddress(),
                owner.address,
            ]);
            await fakeBridge.waitForDeployment();

            // fakeBridge 没有被加入 token 的桥列表
            const isBridge = await token.bridges(await fakeBridge.getAddress());
            expect(isBridge).to.be.false;
        });
    });

    // ==================== F. 🔥 面试重点 ====================
    describe("F. 🔥 面试重点 — Lock-Mint 安全分析", function () {
        it("F1. 🔥 桥合约锁仓金额 = 源链用户锁定的总额", async function () {
            const amount1 = ethers.parseEther("20");
            const amount2 = ethers.parseEther("30");

            // user1 锁定 20
            await token.connect(user1).approve(
                await sourceBridge.getAddress(),
                amount1
            );
            await sourceBridge.connect(user1).lock(amount1, user1.address);

            // owner 给 user2 转 token
            await token.transfer(user2.address, amount2);
            // user2 锁定 30
            await token.connect(user2).approve(
                await sourceBridge.getAddress(),
                amount2
            );
            await sourceBridge.connect(user2).lock(amount2, user2.address);

            // 验证 lockedBalance = amount1 + amount2
            const locked = await sourceBridge.lockedBalance();
            expect(locked).to.equal(amount1 + amount2);

            // ⭐ 面试重点：这就是桥合约成为蜜罐的原因
            // 所有锁定资产都在桥合约里，攻破桥 = 拿走所有资金
            console.log(
                `🔒 锁仓总额: ${ethers.formatEther(locked)} BRG\n` +
                `💡 面试金句: Lock-Mint 模式下，桥合约持有的 TVL 越大，` +
                `攻击的潜在收益越高，这就是为什么桥是黑客的首要目标。`
            );
        });

        it("F2. 🔥 签名验证缺失 — 本教学版的安全边界", async function () {
            // 本教学版用 onlyOwner 控制 mint/unlock
            // 生产环境必须用签名验证：中继者用自己的私钥签名 → 合约验证签名
            // 否则 owner 私钥泄露 = 桥被完全控制

            console.log(
                "⚠️  本教学版用 onlyOwner 模拟中继者\n" +
                "✅ 生产级方案：\n" +
                "  1. 中继者网络用多签（如 5/9）签名\n" +
                "  2. 桥合约用 ECDSA.recover() 验证签名\n" +
                "  3. 验证者私钥分散管理（HSM/KMS）\n" +
                "  4. 设置每日限额和延迟提现"
            );
        });

        it("F3. 🔥 txId 包含 chainId → 防跨链重放", async function () {
            const chainId = (await ethers.provider.getNetwork()).chainId;

            // txId = hash(sender, amount, recipient, chainId, bridge, nonce)
            // 即使相同的参数在不同链上也会产生不同的 txId
            // ⭐ 这是 EIP-155 设计思想在跨链层的延伸

            console.log(
                `⛓️  当前 chainId: ${chainId}\n` +
                `💡 如果 txId 不包含 chainId，攻击者可以在另一条链上` +
                `重放同样的 txId 来 double-mint`
            );
        });
    });
});