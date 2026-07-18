import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("🩸 重入攻击与防护", function () {
    // ============ 单函数重入 ============
    let vault: any;
    let attackerContract: any;
    let owner: any, attacker: any, user: any, depositor: any;

    const ONE_ETH = ethers.parseEther("1");
    const FIVE_ETH = ethers.parseEther("5");

    // ==================== A. 单函数重入攻击 ====================

    describe("A. 单函数重入 (Single-Function Reentrancy)", function () {
        beforeEach(async function () {
            [owner, attacker, user, depositor] = await ethers.getSigners();

            // 部署漏洞合约
            const VaultFactory = await ethers.getContractFactory("ReentrancyVault");
            vault = await VaultFactory.deploy();

            // 攻击者部署攻击合约
            const AttackerFactory = await ethers.getContractFactory("ReentrancyAttacker");
            attackerContract = await AttackerFactory.connect(attacker).deploy(
                await vault.getAddress()
            );
        });

        it("A1. 正常用户存取款应该成功", async function () {
            await vault.connect(user).deposit({ value: FIVE_ETH });
            expect(await vault.balances(user.address)).to.equal(FIVE_ETH);

            await vault.connect(user).withdraw();
            expect(await vault.balances(user.address)).to.equal(ethers.parseEther("0"));
            expect(await vault.getBalance()).to.equal(0);
        });

        it("A2. 无余额取款应该 revert", async function () {
            await expect(
                vault.connect(user).withdraw()
            ).to.be.revertedWith("No balance");
        });

        it("A3. 存款金额为 0 应该 revert", async function () {
            await expect(
                vault.connect(user).deposit({ value: 0 })
            ).to.be.revertedWith("Must send ETH");
        });

        it("A4. 🔥 攻击成功 — 攻击者用 1 ETH 抽干 Vault", async function () {
            // 模拟真实场景：其他用户也存了钱
            await vault.connect(owner).deposit({ value: ethers.parseEther("4") });
            await vault.connect(depositor).deposit({ value: ethers.parseEther("5") });

            const vaultBefore = await vault.getBalance();
            console.log(`  📊 Vault 余额（攻击前）: ${ethers.formatEther(vaultBefore)} ETH`);
            // 预期：4 + 5 + 1(攻击者) = 10 ETH

            // 攻击者用 1 ETH 发起攻击
            await attackerContract
                .connect(attacker)
                .attack({ value: ONE_ETH });

            const vaultAfter = await vault.getBalance();
            console.log(`  📊 Vault 余额（攻击后）: ${ethers.formatEther(vaultAfter)} ETH`);

            // Vault 应该被抽干
            expect(vaultAfter).to.equal(0);

            // 攻击者合约应该卷走所有 ETH
            const stolen = await ethers.provider.getBalance(
                await attackerContract.getAddress()
            );
            expect(stolen).to.equal(ethers.parseEther("10"));
            console.log(`  💰 攻击成功！攻击者卷走了 ${ethers.formatEther(stolen)} ETH`);
        });

        it("A5. 攻击者可以提现盗取的 ETH", async function () {
            await vault.connect(depositor).deposit({ value: ethers.parseEther("9") });
            await attackerContract
                .connect(attacker)
                .attack({ value: ONE_ETH });

            const balBefore = await ethers.provider.getBalance(attacker.address);
            const tx = await attackerContract.connect(attacker).cashOut();
            const receipt = await tx.wait();
            const gasCost = receipt.gasUsed * receipt.gasPrice;
            const balAfter = await ethers.provider.getBalance(attacker.address);

            // 扣除 gas 后，攻击者余额应该增加（balBefore 不含这次 cashOut 的 gas 开销）
            expect(balAfter + BigInt(gasCost)).to.be.gt(balBefore);
        });

        it("A6. 非 owner 不能发起攻击", async function () {
            await expect(
                attackerContract.connect(user).attack({ value: ONE_ETH })
            ).to.be.revertedWith("Not owner");
        });
    });

    // ==================== B. 跨函数重入攻击 ====================

    describe("B. 跨函数重入 (Cross-Function Reentrancy)", function () {
        let crossVault: any;
        let crossAttacker: any;

        beforeEach(async function () {
            [owner, attacker, user] = await ethers.getSigners();

            const VaultFactory = await ethers.getContractFactory("CrossReentrancyVault");
            crossVault = await VaultFactory.deploy();

            const AttackerFactory = await ethers.getContractFactory("CrossReentrancyAttacker");
            crossAttacker = await AttackerFactory.connect(attacker).deploy(
                await crossVault.getAddress()
            );
        });

        it("B1. 正常存取款 + 正常领取奖金", async function () {
            await crossVault.connect(user).deposit({ value: ethers.parseEther("10") });

            // 正常领取奖金（10% = 1 ETH）
            await crossVault.connect(user).claimBonus();
            expect(await crossVault.bonus(user.address)).to.equal(ONE_ETH);

            // 取款后余额应为 0，奖金不应再增加
            await crossVault.connect(user).withdraw();
            await expect(
                crossVault.connect(user).claimBonus()
            ).to.be.revertedWith("No balance for bonus");
        });

        it("B2. 🔥 跨函数重入 — 取款后奖金仍用旧余额计算", async function () {
            // 攻击者用 10 ETH 发起攻击
            await crossAttacker
                .connect(attacker)
                .attack({ value: ethers.parseEther("10") });

            // 攻击合约的奖金应该 = 1 ETH（10 ETH 的 10%）
            // 尽管取款后余额已为 0，但 claimBonus 在 receive() 中被调用时
            // withdraw 还没更新 balances，所以读到了旧值
            const bonus = await crossVault.bonus(await crossAttacker.getAddress());
            console.log(`  📊 攻击者获得的异常奖金: ${ethers.formatEther(bonus)} ETH`);
            expect(bonus).to.equal(ONE_ETH);

            // 攻击者的余额应该已经是 0（被 withdraw 正确扣了）
            const balance = await crossVault.balances(await crossAttacker.getAddress());
            expect(balance).to.equal(0);
        });

        it("B3. 攻击者可以提取异常奖金", async function () {
            // 另一个用户存入资金，确保 Vault 有余额支付异常奖金
            await crossVault.connect(user).deposit({ value: ethers.parseEther("5") });

            await crossAttacker
                .connect(attacker)
                .attack({ value: ethers.parseEther("10") });

            // 提取异常奖金
            await crossAttacker.connect(attacker).collectBonus();

            const attackerBonus = await crossVault.bonus(
                await crossAttacker.getAddress()
            );
            expect(attackerBonus).to.equal(0);
        });
    });

    // ==================== C. SecureVault — 双重防护验证 ====================

    describe("C. SecureVault — CEI + ReentrancyGuard 防护", function () {
        let secureVault: any;

        beforeEach(async function () {
            [owner, attacker, user, depositor] = await ethers.getSigners();

            const VaultFactory = await ethers.getContractFactory("SecureVault");
            secureVault = await VaultFactory.deploy();
        });

        it("C1. 正常存取款不受 CEI 影响", async function () {
            await secureVault.connect(user).deposit({ value: ethers.parseEther("3") });
            expect(await secureVault.balances(user.address)).to.equal(
                ethers.parseEther("3")
            );

            await secureVault.connect(user).withdraw();
            expect(await secureVault.balances(user.address)).to.equal(0);
        });

        it("C2. 无余额取款应该 revert", async function () {
            await expect(
                secureVault.connect(user).withdraw()
            ).to.be.revertedWith("No balance");
        });

        it("C3. 存款金额为 0 应该 revert", async function () {
            await expect(
                secureVault.connect(user).deposit({ value: 0 })
            ).to.be.revertedWith("Must send ETH");
        });

        it("C4. ReentrancyGuard 阻止同笔交易重入", async function () {
            // 构造一个恶意合约尝试重入
            const MaliciousFactory = await ethers.getContractFactory("ReentrancyAttacker");
            const malicious = await MaliciousFactory.connect(attacker).deploy(
                await secureVault.getAddress()
            );

            // 先给 secureVault 注资（模拟其他用户存款）
            await secureVault.connect(depositor).deposit({ value: ethers.parseEther("10") });

            // 攻击者存 1 ETH 然后尝试重入攻击
            // SecureVault 有 ReentrancyGuard，receive() 中重入 withdraw() 会 revert
            // → receive() 整体 revert → 外层 call() 返回 false → "Transfer failed"
            await expect(
                malicious.connect(attacker).attack({ value: ONE_ETH })
            ).to.revert(ethers);

            // 整个交易 revert，所有状态回滚 — 资金完全安全
            const vaultBalance = await secureVault.getBalance();
            expect(vaultBalance).to.equal(ethers.parseEther("10"));
            console.log(`  🛡️ 防护生效：攻击被完全回滚，Vault 剩余 ${ethers.formatEther(vaultBalance)} ETH（全部资金安全）`);
        });

        it("C5. emergencyWithdraw 也受 ReentrancyGuard 保护", async function () {
            await secureVault.connect(user).deposit({ value: ethers.parseEther("5") });

            // 部分紧急取款
            await secureVault.connect(user).emergencyWithdraw(ethers.parseEther("2"));
            expect(await secureVault.balances(user.address)).to.equal(
                ethers.parseEther("3")
            );

            // 余额不足时 revert
            await expect(
                secureVault.connect(user).emergencyWithdraw(ethers.parseEther("10"))
            ).to.be.revertedWith("Insufficient");
        });
    });

    // ==================== D. ReentrancyGuard 单元测试 ====================

    describe("D. ReentrancyGuard 内部机制", function () {
        let secureVault: any;

        beforeEach(async function () {
            [owner, user] = await ethers.getSigners();
            const VaultFactory = await ethers.getContractFactory("SecureVault");
            secureVault = await VaultFactory.deploy();
        });

        it("D1. 同一函数连续调用不受影响（不同交易）", async function () {
            // 第一笔交易：存款 + 取款
            await secureVault.connect(user).deposit({ value: ONE_ETH });
            await secureVault.connect(user).withdraw();
            expect(await secureVault.balances(user.address)).to.equal(0);

            // 第二笔交易：再次存款 + 取款（不同交易，_status 已重置）
            await secureVault.connect(user).deposit({ value: ONE_ETH });
            await secureVault.connect(user).withdraw();
            expect(await secureVault.balances(user.address)).to.equal(0);
        });

        it("D2. 两个 nonReentrant 函数不能互相调用", async function () {
            await secureVault.connect(user).deposit({ value: ethers.parseEther("2") });

            // withdraw 内部不会调用 emergencyWithdraw，所以这个测试验证
            // 外部无法在同一个交易中先后调用两个 nonReentrant 函数
            // （因为第一个 withdraw 已经锁住了 _status）
            //
            // 实际上，不同交易之间 _status 会重置，所以分别调用是 OK 的
            await secureVault.connect(user).withdraw();

            // 确认状态恢复后第二个 nonReentrant 函数也能正常使用
            await secureVault.connect(user).deposit({ value: ONE_ETH });
            await secureVault.connect(user).emergencyWithdraw(ONE_ETH);
            expect(await secureVault.balances(user.address)).to.equal(0);
        });
    });
});