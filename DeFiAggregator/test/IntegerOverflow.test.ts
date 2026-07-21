import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("🔢 整数溢出漏洞", function () {
    let overflowVuln: any;
    let overflowSecure: any;
    let owner: any, user1: any, user2: any;

    // ==================== A. 溢出概念演示 (OverflowVulnerable) ====================

    describe("A. 溢出概念 — assembly 模拟 0.7.x 无检查运算", function () {
        beforeEach(async function () {
            [owner, user1, user2] = await ethers.getSigners();
            const Factory = await ethers.getContractFactory("OverflowVulnerable");
            overflowVuln = await Factory.deploy();
        });

        it("A1. uint256 加法溢出 — 最大值+1 绕回 0", async function () {
            const maxUint = ethers.MaxUint256;
            await overflowVuln.mint(user1.address, maxUint);
            expect(await overflowVuln.balances(user1.address)).to.equal(maxUint);

            // 再 mint 1 个 → 溢出绕回 0
            await overflowVuln.mint(user1.address, 1n);
            const afterOverflow = await overflowVuln.balances(user1.address);
            console.log(`  📊 uint256.max + 1 = ${afterOverflow} (溢出绕回 0)`);
            expect(afterOverflow).to.equal(0n);
        });

        it("A2. 乘法溢出 — receivers.length * amount 溢出绕回 0", async function () {
            // amount = 2^255，receivers.length = 2
            // total = 2 * 2^255 = 2^256 → mod 2^256 = 0（溢出绕回）
            const amount = 1n << 255n;
            const receivers = [user1.address, user2.address]; // length = 2

            console.log(`  📊 amount = 2^255`);
            console.log(`  📊 receivers.length = ${receivers.length}`);
            console.log(`  📊 total = 2 * 2^255 = 2^256 → 溢出后 = 0`);

            // user1 余额为 0，但 total 溢出为 0
            // require(0 >= 0) → 通过！（这就是漏洞核心）
            // 循环中每次减 amount = 2^255，余额下溢
            await overflowVuln.connect(user1).batchTransfer(receivers, amount);

            // 验证：user1 余额从 0 减去 2*2^255（每次迭代减一次）
            // 实际：第一次迭代 _unsafeSub(0, 2^255) 下溢到 2^255
            //       然后 _unsafeAdd(2^255, 2^255) 加给自己 → 溢出绕回 0
            //       第二次迭代 _unsafeSub(0, 2^255) 再次下溢 → 加到 user2
            // 最终 user1 余额 = 2^255（下溢结果）
            const balUser1 = await overflowVuln.balances(user1.address);
            console.log(`  📊 user1 余额（攻击后）: ${balUser1}`);
            expect(balUser1).to.be.gt(0n); // 下溢导致余额极大

            // user2 拿到 2^255 代币（凭空铸造！）
            const balUser2 = await overflowVuln.balances(user2.address);
            console.log(`  📊 user2 余额（攻击后）: ${balUser2}`);
            expect(balUser2).to.equal(amount);
        });

        it("A3. 🔥 完整 BEC 攻击链 — 从有余额地址盗取/增发", async function () {
            // 给 user1 mint 100 代币（模拟真实用户余额）
            const initialBal = ethers.parseEther("100");
            await overflowVuln.mint(user1.address, initialBal);
            expect(await overflowVuln.balances(user1.address)).to.equal(initialBal);

            const amount = 1n << 255n; // 2^255 — 极大转账额
            const receivers = [user1.address, user2.address]; // length = 2

            console.log(`  📊 amount = 2^255 = ${amount}`);
            console.log(`  📊 期望 total = 2 * 2^255 = 2^256`);
            console.log(`  📊 实际 total = 0 (溢出绕回)`);
            console.log(`  📊 require(${ethers.formatEther(initialBal)} >= 0) → 通过 ✅`);

            // ─── 攻击执行 ───
            // total 溢出为 0 → require(balance >= 0) 通过
            // 循环中每人转 amount = 2^255，但 user1 只有 100 tokens
            // → 减法下溢：balances[user1] 变成一个天文数字
            await overflowVuln.connect(user1).batchTransfer(receivers, amount);

            const balUser1 = await overflowVuln.balances(user1.address);
            const balUser2 = await overflowVuln.balances(user2.address);

            console.log(`  📊 user1 攻击前: ${ethers.formatEther(initialBal)} tokens`);
            console.log(`  📊 user1 攻击后: ${ethers.formatEther(balUser1)} tokens (下溢到天文数字!)`);
            console.log(`  📊 user2 攻击后: ${ethers.formatEther(balUser2)} tokens (凭空获得!)`);

            // 断言 1: user2 凭空获得了 2^255 代币（无限铸币！）
            expect(balUser2).to.equal(amount);

            // 断言 2: user1 余额下溢到极大值（远超初始余额）
            expect(balUser1).to.be.gt(initialBal);
            // 具体来说，user1 余额应该是 2^255 + 100（第一次迭代下溢到 2^255 + 100e18，
            // 然后加回导致溢出绕回 100e18；第二次迭代从 100e18 下溢到 2^255 + 100e18）
        });

        it("A4. 加法溢出攻击 — 绕回 0 可无限铸币", async function () {
            // 场景：攻破 totalSupply 追踪
            // totalSupply 从 0 开始
            expect(await overflowVuln.totalSupply()).to.equal(0n);

            // mint maxUint 给 user1 → totalSupply 也变成 maxUint
            await overflowVuln.mint(user1.address, ethers.MaxUint256);
            expect(await overflowVuln.totalSupply()).to.equal(ethers.MaxUint256);

            // 再 mint 1 → totalSupply 溢出绕回 0
            // 但 user1 余额变成 0（也溢出了）
            await overflowVuln.mint(user1.address, 1n);
            expect(await overflowVuln.totalSupply()).to.equal(0n);

            console.log(`  ⚠️  totalSupply 溢出绕回 0，审计追踪被破坏！`);
        });
    });

    // ==================== B. Solidity 0.8+ 自动保护 ====================

    describe("B. Solidity 0.8+ 自动溢出保护", function () {
        beforeEach(async function () {
            [owner, user1, user2] = await ethers.getSigners();
            const Factory = await ethers.getContractFactory("OverflowSecure");
            overflowSecure = await Factory.deploy();
        });

        it("B1. 加法溢出 → Solidity 0.8+ 自动 panic", async function () {
            // ✅ mint 修复后正确铸币给 user1
            await overflowSecure.mint(user1.address, ethers.MaxUint256);
            expect(await overflowSecure.balances(user1.address)).to.equal(ethers.MaxUint256);

            // 再 mint 1 → Solidity 0.8+ 自动检测溢出并 panic
            // panic code 0x11 = ARITHMETIC_OVERFLOW
            await expect(
                overflowSecure.mint(user1.address, 1n)
            ).to.revert(ethers);
            // 注：hardhat-chai-matchers 中 to.revert(ethers) 接受任何 revert；
            // 这里是 Solidity 编译器插入的 overflow check 触发的 panic
        });

        it("B2. 安全减法 — require 先检查余额，不会下溢", async function () {
            // user1 余额为 0
            expect(await overflowSecure.balances(user1.address)).to.equal(0n);

            // batchTransfer: receivers.length=1, amount=1
            // total = 1 * 1 = 1, require(0 >= 1) → fail
            const receivers = [user2.address];
            await expect(
                overflowSecure.connect(user1).batchTransfer(receivers, 1n)
            ).to.be.revertedWith("Insufficient balance");

            // 验证：user1 余额仍然为 0（没有下溢）
            expect(await overflowSecure.balances(user1.address)).to.equal(0n);
        });

        it("B3. 乘法溢出 → Solidity 0.8+ 自动 panic", async function () {
            // ✅ mint 修复后 user1 有足够余额
            await overflowSecure.mint(user1.address, ethers.MaxUint256);
            expect(await overflowSecure.balances(user1.address)).to.equal(ethers.MaxUint256);

            const amount = 1n << 255n; // 2^255
            const receivers: string[] = [];
            for (let i = 0; i < 3; i++) receivers.push(user2.address);

            // 3 * 2^255 > 2^256 → 乘法溢出
            // Solidity 0.8+ 在乘法时直接 panic，不会执行到 require
            await expect(
                overflowSecure.connect(user1).batchTransfer(receivers, amount)
            ).to.revert(ethers);

            // 验证：user1 余额未被篡改（revert 回滚了状态）
            expect(await overflowSecure.balances(user1.address)).to.equal(ethers.MaxUint256);
        });

        it("B4. 🔥 面试题：unchecked 的正确用法 — 节省 gas", async function () {
            // totalTransfers 从 0 开始，每次 +1，永远不可能溢出
            // uint256 最大值 ≈ 1.15 * 10^77，循环自增不可能达到
            for (let i = 0; i < 10; i++) {
                await overflowSecure.incrementTotalTransfers();
            }
            expect(await overflowSecure.totalTransfers()).to.equal(10n);
            console.log(`  ✅ totalTransfers = 10，使用 unchecked 安全且省 gas`);
        });

        it("B5. 正常批量转账 — 0.8+ 保护下安全执行", async function () {
            // 给 user1 mint 100 代币
            const mintAmount = ethers.parseEther("100");
            await overflowSecure.mint(user1.address, mintAmount);
            expect(await overflowSecure.balances(user1.address)).to.equal(mintAmount);

            // 正常批量转账：3 个接收者各 1 个代币
            const receivers = [user2.address, owner.address, user2.address];
            const transferAmount = ethers.parseEther("1");
            const totalCost = transferAmount * BigInt(receivers.length);

            await overflowSecure.connect(user1).batchTransfer(receivers, transferAmount);

            // 验证 user1 余额正确扣除
            expect(await overflowSecure.balances(user1.address)).to.equal(
                mintAmount - totalCost
            );
            // 验证 user2 收到 2 个代币（出现两次）
            expect(await overflowSecure.balances(user2.address)).to.equal(
                transferAmount * 2n
            );
            // 验证 owner 收到 1 个代币
            expect(await overflowSecure.balances(owner.address)).to.equal(transferAmount);
        });
    });

    // ==================== C. 面试验证 ====================

    describe("C. 🔥 面试验证", function () {
        it("C1. 0.8+ 默认运算 → 溢出时 panic", async function () {
            const Factory = await ethers.getContractFactory("OverflowSecure");
            const secure: any = await Factory.deploy();
            await secure.mint(user1.address, ethers.MaxUint256);
            await expect(secure.mint(user1.address, 1n)).to.revert(ethers);
            console.log(`  ✅ 0.8+ 默认溢出检查生效，panic 保护了状态`);
        });

        it("C2. unchecked 块 → 跳过检查，溢出绕回", async function () {
            // OverflowVulnerable 用 assembly 模拟 unchecked 行为
            await overflowVuln.mint(user1.address, ethers.MaxUint256);
            await overflowVuln.mint(user1.address, 1n);
            expect(await overflowVuln.balances(user1.address)).to.equal(0n);
            console.log(`  ⚠️  unchecked 模式：最大值 + 1 绕回 0，没有 revert`);
        });

        it("C3. 🔥 面试题：什么时候用 unchecked？", async function () {
            // 场景 1：循环自增（永远不会溢出 uint256）
            // 场景 2：已知不会溢出的计数器
            // 场景 3：库函数中已经由上层保证安全的运算

            // 验证：incrementTotalTransfers 内部使用 unchecked
            for (let i = 0; i < 5; i++) {
                await overflowSecure.incrementTotalTransfers();
            }
            const transfers = await overflowSecure.totalTransfers();
            // 这个测试中 B4 已经增加了 10 次，C3 又加 5 次
            console.log(`  ✅ totalTransfers = ${transfers}，safe unchecked`);
            expect(transfers).to.be.at.least(5n);
        });
    });
});
