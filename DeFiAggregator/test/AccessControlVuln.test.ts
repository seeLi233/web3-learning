import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("🔐 访问控制漏洞", function () {
    let wallet: any;
    let phishing: any;
    let vault: any;
    let owner: any, attacker: any, user: any, victim: any;

    const ONE_ETH = ethers.parseEther("1");
    const FIVE_ETH = ethers.parseEther("5");

    // ==================== A. tx.origin 钓鱼攻击 ====================

    describe("A. tx.origin 钓鱼攻击", function () {
        beforeEach(async function () {
            [owner, attacker, user, victim] = await ethers.getSigners();

            // ✅ constructor payable 修复后可附带 ETH 部署
            const WalletFactory = await ethers.getContractFactory("TxOriginWallet");
            wallet = await WalletFactory.connect(owner).deploy({ value: FIVE_ETH });
        });

        it("A1. owner 正常提款 — 提走全部余额", async function () {
            const walletBal = await wallet.getBalance();
            expect(walletBal).to.equal(FIVE_ETH);

            const balBefore = await ethers.provider.getBalance(owner.address);
            const tx = await wallet.connect(owner).withdraw();
            const receipt = await tx.wait();

            // 计算实际收益（ETH 转入 - gas 消耗）
            const balAfter = await ethers.provider.getBalance(owner.address);
            const gasCost = BigInt(receipt.gasUsed) * BigInt(receipt.gasPrice ?? 0);
            const netGain = balAfter - balBefore + gasCost;

            console.log(`  ✅ owner 提款 ${ethers.formatEther(netGain)} ETH (扣除 gas)`);
            expect(netGain).to.equal(FIVE_ETH);
            // 钱包余额归零
            expect(await wallet.getBalance()).to.equal(0n);
        });

        it("A2. 非 owner 直接调用 withdraw → revert", async function () {
            // attacker 直接调用 wallet.withdraw()
            // tx.origin = attacker ≠ owner → revert
            await expect(
                wallet.connect(attacker).withdraw()
            ).to.be.revertedWith("Not owner");
        });

        it("A3. 🔥 钓鱼攻击 — owner 误点 phishing 合约 → 钱被偷", async function () {
            // attacker 部署钓鱼合约，指向 owner 的钱包
            const PhishFactory = await ethers.getContractFactory("TxOriginPhishing");
            phishing = await PhishFactory.connect(attacker).deploy(
                await wallet.getAddress()
            );

            const walletBalBefore = await wallet.getBalance();
            console.log(`  📊 钱包余额（攻击前）: ${ethers.formatEther(walletBalBefore)} ETH`);
            console.log(`  📊 owner 地址: ${owner.address}`);
            console.log(`  🎣 钓鱼合约地址: ${await phishing.getAddress()}`);

            // owner 误点了钓鱼合约的 claimAirdrop()
            // tx.origin = owner，msg.sender = phishing 合约
            // wallet 检查 tx.origin == owner → 通过！
            await phishing.connect(owner).claimAirdrop();

            const walletBalAfter = await wallet.getBalance();
            console.log(`  📊 钱包余额（攻击后）: ${ethers.formatEther(walletBalAfter)} ETH`);

            // 断言 1: 钱包被洗劫一空
            expect(walletBalAfter).to.equal(0n);

            // 断言 2: 钓鱼合约收到了 ETH
            const phishingBal = await ethers.provider.getBalance(
                await phishing.getAddress()
            );
            console.log(`  💰 钓鱼合约卷走了 ${ethers.formatEther(phishingBal)} ETH`);
            expect(phishingBal).to.equal(FIVE_ETH);
        });

        it("A4. 攻击者从钓鱼合约提取赃款", async function () {
            // 先发动攻击
            const PhishFactory = await ethers.getContractFactory("TxOriginPhishing");
            phishing = await PhishFactory.connect(attacker).deploy(
                await wallet.getAddress()
            );
            await phishing.connect(owner).claimAirdrop();

            // attacker 提款
            const attackerBalBefore = await ethers.provider.getBalance(attacker.address);
            const tx = await phishing.connect(attacker).cashOut();
            const receipt = await tx.wait();
            const attackerBalAfter = await ethers.provider.getBalance(attacker.address);

            const gasCost = BigInt(receipt.gasUsed) * BigInt(receipt.gasPrice ?? 0);
            const netGain = attackerBalAfter - attackerBalBefore + gasCost;

            console.log(`  💰 攻击者净收益: ${ethers.formatEther(netGain)} ETH`);
            // 攻击者净收益 ≈ 5 ETH（扣除 gas 后略少）
            expect(netGain).to.equal(FIVE_ETH);
        });

        it("A5. 🔥 对比：如果用 msg.sender 就不会被钓鱼", async function () {
            // 知识点：tx.origin vs msg.sender
            // msg.sender = 直接调用者（phishing 合约）
            // tx.origin = 交易发起者（owner）
            //
            // 用 tx.origin → attacker 能钓鱼
            // 用 msg.sender → attacker 无法钓鱼（phishing 合约的地址 ≠ owner）

            // 演示：从 phishing 合约的视角
            // claimAirdrop() 内部调 wallet.withdraw()
            //   - tx.origin = owner    → wallet 的检查通过
            //   - msg.sender = phishing → wallet 如果用 msg.sender 则检查失败

            const PhishFactory = await ethers.getContractFactory("TxOriginPhishing");
            phishing = await PhishFactory.connect(attacker).deploy(
                await wallet.getAddress()
            );

            // 发动攻击（成功，因为合约用 tx.origin）
            await phishing.connect(owner).claimAirdrop();
            expect(await wallet.getBalance()).to.equal(0n);

            console.log(`  ⚠️  tx.origin == owner 通过了，但 msg.sender == phishing 合约！`);
            console.log(`  ✅ 正确做法：永远用 msg.sender 做身份认证`);
        });
    });

    // ==================== B. 权限检查遗漏 ====================

    describe("B. 权限检查遗漏", function () {
        beforeEach(async function () {
            [owner, attacker, user] = await ethers.getSigners();
            const VaultFactory = await ethers.getContractFactory("AccessControlVault");
            vault = await VaultFactory.connect(owner).deploy();
        });

        it("B1. 任何人都能改费率 — 没有 onlyOwner", async function () {
            expect(await vault.fee()).to.equal(10n);

            // attacker 直接改费率
            await vault.connect(attacker).setFee(999);
            expect(await vault.fee()).to.equal(999n);
            console.log(`  🔴 attacker 成功将费率从 10 改为 999，不需要 owner 权限！`);
        });

        it("B2. 任何人都能改 owner", async function () {
            expect(await vault.owner()).to.equal(owner.address);

            // attacker 直接篡改 owner
            await vault.connect(attacker).changeOwner(attacker.address);
            expect(await vault.owner()).to.equal(attacker.address);
            console.log(`  🔴 attacker 成功将自己设为 owner！`);
        });

        it("B3. setFee 超过上限 1000 会 revert（边界测试）", async function () {
            // require(_newFee <= 1000)
            await expect(
                vault.connect(attacker).setFee(1001)
            ).to.be.revertedWith("Fee too high");

            // 边界值 1000 可以
            await vault.connect(attacker).setFee(1000);
            expect(await vault.fee()).to.equal(1000n);
            console.log(`  ✅ 费率上限 1000 有效，但任何人都能调到 1000`);
        });

        it("B4. 🔥 漏洞链条：改 owner + 改费率 = 完整劫持", async function () {
            // Step 1: attacker 把自己设为 owner
            await vault.connect(attacker).changeOwner(attacker.address);
            expect(await vault.owner()).to.equal(attacker.address);

            // Step 2: attacker 把费率改到上限
            await vault.connect(attacker).setFee(1000);
            expect(await vault.fee()).to.equal(1000n);

            // Step 3: 原 owner 失去了所有控制
            console.log(`  🔴 原 owner ${owner.address} 不再是 owner`);
            console.log(`  🔴 协议被完全劫持 — owner=${attacker.address}, fee=1000`);
        });
    });

    // ==================== C. 签名重放攻击 ====================

    describe("C. 签名重放攻击", function () {
        let chainId: bigint;

        beforeEach(async function () {
            [owner, attacker, user] = await ethers.getSigners();
            // ✅ constructor payable 修复后可附带 ETH 部署
            const VaultFactory = await ethers.getContractFactory("AccessControlVault");
            vault = await VaultFactory.connect(owner).deploy({ value: FIVE_ETH });

            const { chainId: cid } = await ethers.provider.getNetwork();
            chainId = BigInt(cid);
        });

        // 辅助函数：用 owner 的私钥签名（模拟 unsafe 版本）
        async function signWithdraw(
            signer: any,
            to: string,
            amount: bigint
        ): Promise<string> {
            const hash = ethers.solidityPackedKeccak256(
                ["address", "uint256"],
                [to, amount]
            );
            return await signer.signMessage(ethers.getBytes(hash));
        }

        it("C1. owner 签名后，relayer 正常提交应该成功", async function () {
            const amount = ONE_ETH;
            const sig = await signWithdraw(owner, user.address, amount);

            const userBalBefore = await ethers.provider.getBalance(user.address);
            await vault.connect(attacker).withdrawBySignature(user.address, amount, sig);
            const userBalAfter = await ethers.provider.getBalance(user.address);

            expect(userBalAfter - userBalBefore).to.equal(amount);
            console.log(`  ✅ owner 签名授权 ${ethers.formatEther(amount)} ETH → user`);
        });

        it("C2. 同一签名不能重复使用（基础防护）", async function () {
            const amount = ONE_ETH;
            const sig = await signWithdraw(owner, user.address, amount);

            // 第一次成功
            await vault.connect(attacker).withdrawBySignature(user.address, amount, sig);

            // 第二次 → revert（executedSigs 去重）
            await expect(
                vault.connect(attacker).withdrawBySignature(user.address, amount, sig)
            ).to.be.revertedWith("Already executed");
        });

        it("C3. 🔥 签名重放 — 同一签名可跨合约重放", async function () {
            const amount = ONE_ETH;
            const sig = await signWithdraw(owner, user.address, amount);

            // Step 1: 在 Vault #1 使用签名
            const userBalBefore = await ethers.provider.getBalance(user.address);
            await vault.connect(attacker).withdrawBySignature(user.address, amount, sig);
            const userBalAfter1 = await ethers.provider.getBalance(user.address);
            expect(userBalAfter1 - userBalBefore).to.equal(amount);
            console.log(`  ✅ Vault #1: 签名验证通过，提取 ${ethers.formatEther(amount)} ETH`);

            // Step 2: 部署 Vault #2（新实例，新的 executedSigs 映射）
            const VaultFactory = await ethers.getContractFactory("AccessControlVault");
            const vault2: any = await VaultFactory.connect(owner).deploy({ value: ONE_ETH });

            // 🔴 同一签名在 Vault #2 也能用！
            // 因为签名只有 (to, amount)，没有合约地址/nonce/chainId
            // Vault #2 的 executedSigs 是空映射，签名可以再次使用
            await vault2.connect(attacker).withdrawBySignature(user.address, amount, sig);
            const userBalAfter2 = await ethers.provider.getBalance(user.address);
            expect(userBalAfter2 - userBalAfter1).to.equal(amount);

            console.log(`  🔴 Vault #2: 同一签名再次提取 ${ethers.formatEther(amount)} ETH！`);
            console.log(`  🔴 跨合约重放成功 — 签名缺少 contract address`);
            console.log(`  💀 同理可跨链重放 — 签名缺少 chainId`);

            // 断言：user 总共收到 2 * amount ETH（从两个不同的 vault）
            expect(userBalAfter2 - userBalBefore).to.equal(amount * 2n);
        });

        it("C4. 错误签名 → revert Invalid signature", async function () {
            const amount = ONE_ETH;
            const sig = await signWithdraw(attacker, user.address, amount);
            // attacker 不是 owner → 签名无效
            await expect(
                vault.connect(attacker).withdrawBySignature(user.address, amount, sig)
            ).to.be.revertedWith("Invalid signature");
        });

        it("C5. 正确签名但篡改 amount → revert Invalid signature", async function () {
            const amount = ONE_ETH;
            const sig = await signWithdraw(owner, user.address, amount);
            // 签名对 amount=1 ETH 有效，但尝试提取 amount+1
            await expect(
                vault.connect(attacker).withdrawBySignature(user.address, amount + 1n, sig)
            ).to.be.revertedWith("Invalid signature");
        });

        // ==================== 安全签名（修复版）====================

        it("C6. ✅ 安全签名 — 带 nonce + chainId + 合约地址", async function () {
            const amount = ONE_ETH;
            const nonce = 1n;

            const hash = ethers.solidityPackedKeccak256(
                ["address", "uint256", "uint256", "uint256", "address"],
                [user.address, amount, nonce, chainId, await vault.getAddress()]
            );
            const sig = await owner.signMessage(ethers.getBytes(hash));

            const userBalBefore = await ethers.provider.getBalance(user.address);
            await vault.withdrawBySignatureSecure(user.address, amount, nonce, sig);
            const userBalAfter = await ethers.provider.getBalance(user.address);

            expect(userBalAfter - userBalBefore).to.equal(amount);
            console.log(`  ✅ 安全签名：{to, amount, nonce, chainId, contract} → 不可跨合约/跨链重放`);
        });

        it("C7. 安全版本 — nonce 已使用后不能重放", async function () {
            const amount = ONE_ETH;
            const nonce = 1n;

            const hash = ethers.solidityPackedKeccak256(
                ["address", "uint256", "uint256", "uint256", "address"],
                [user.address, amount, nonce, chainId, await vault.getAddress()]
            );
            const sig = await owner.signMessage(ethers.getBytes(hash));

            // 第一次成功
            await vault.withdrawBySignatureSecure(user.address, amount, nonce, sig);

            // 第二次同一 nonce → revert
            await expect(
                vault.withdrawBySignatureSecure(user.address, amount, nonce, sig)
            ).to.be.revertedWith("Nonce used");
            console.log(`  ✅ nonce 机制阻止了同一 nonce 的重放`);
        });

        it("C8. 安全版本 — 不同 nonce 可用（顺序独立）", async function () {
            const amount = ONE_ETH;

            // nonce=5
            const hash5 = ethers.solidityPackedKeccak256(
                ["address", "uint256", "uint256", "uint256", "address"],
                [user.address, amount, 5n, chainId, await vault.getAddress()]
            );
            const sig5 = await owner.signMessage(ethers.getBytes(hash5));
            await vault.withdrawBySignatureSecure(user.address, amount, 5n, sig5);
            expect(await vault.usedNonces(owner.address, 5n)).to.be.true;

            // nonce=1（跳跃使用）
            const hash1 = ethers.solidityPackedKeccak256(
                ["address", "uint256", "uint256", "uint256", "address"],
                [user.address, amount, 1n, chainId, await vault.getAddress()]
            );
            const sig1 = await owner.signMessage(ethers.getBytes(hash1));
            await vault.withdrawBySignatureSecure(user.address, amount, 1n, sig1);
            expect(await vault.usedNonces(owner.address, 1n)).to.be.true;

            console.log(`  ✅ nonce 顺序无关 — 支持乱序使用`);
        });

        it("C9. 安全版本 — 签名无法跨合约重放", async function () {
            const amount = ONE_ETH;
            const nonce = 1n;

            const hash = ethers.solidityPackedKeccak256(
                ["address", "uint256", "uint256", "uint256", "address"],
                [user.address, amount, nonce, chainId, await vault.getAddress()]
            );
            const sig = await owner.signMessage(ethers.getBytes(hash));

            // 在 Vault #1 使用签名 → 成功
            await vault.withdrawBySignatureSecure(user.address, amount, nonce, sig);

            // 部署 Vault #2
            const VaultFactory = await ethers.getContractFactory("AccessControlVault");
            const vault2: any = await VaultFactory.connect(owner).deploy({ value: ONE_ETH });

            // 🔴 同一签名在 Vault #2 能否使用？
            // 安全签名包含了 address(this)，Vault #2 的地址不同
            // → recover 出的 signer 不等于 owner → revert！
            await expect(
                vault2.withdrawBySignatureSecure(user.address, amount, nonce, sig)
            ).to.revert(ethers);

            console.log(`  ✅ 安全签名包含合约地址 → 跨合约重放被阻止`);
        });
    });
});
